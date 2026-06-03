/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

// Package logics holds the business-logic layer for agent-server: Runtime lifecycle,
// Channel management, and version information.
package logics

import (
	"fmt"
	"sync"

	"hcm/cmd/agent-server/logics/agent"
	"hcm/cmd/agent-server/logics/model"
	"hcm/cmd/agent-server/logics/prompt"
	"hcm/cmd/agent-server/logics/skill"
	"hcm/cmd/agent-server/logics/storage"
	"hcm/cmd/agent-server/logics/tool"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"

	_ "github.com/ncruces/go-sqlite3/driver" // import sqlite3 driver, used by memory/sqlitevec
	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/memory"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session"
	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// Runtime wraps app.Runtime and adds an idempotent Close.
type Runtime struct {
	// AGUIRunner is the AGUI runner instance that orchestrates agent execution.
	AGUIRunner        runner.Runner
	aguiSessionSvc    session.Service     // non-nil when MySQL session backend is configured
	aguiMemorySvc     memory.Service      // non-nil when MySQL memory backend is configured
	aguiMCPToolSets   *tool.MCPToolSet    // non-nil when MCP toolsets are configured
	dynamicToolFilter trpctool.FilterFunc // non-nil when dynamic tool loading is enabled
	skillMgr          *skill.Manager
	promptMgr         *prompt.Manager
	readiness         *Readiness
	checkpointSaver   graph.CheckpointSaver
	closeOnce         sync.Once
}

// Readiness returns agent-server readiness state (skill + prompt sync).
func (rt *Runtime) Readiness() *Readiness {
	return rt.readiness
}

// SkillSyncer returns the BKAIDev skill syncer, or nil when disabled.
func (rt *Runtime) SkillSyncer() *skill.Syncer {
	if rt.skillMgr == nil {
		return nil
	}
	return rt.skillMgr.Syncer
}

// PromptSyncer returns the BKAIDev prompt syncer, or nil when disabled.
func (rt *Runtime) PromptSyncer() *prompt.Syncer {
	if rt.promptMgr == nil {
		return nil
	}
	return rt.promptMgr.Syncer
}

// PromptStore returns the prompt store for agent callback injection.
func (rt *Runtime) PromptStore() *prompt.Store {
	if rt.promptMgr == nil {
		return nil
	}
	return rt.promptMgr.Store
}

// DynamicToolFilter returns the dynamic tool filter function, or nil if not enabled.
func (rt *Runtime) DynamicToolFilter() trpctool.FilterFunc {
	return rt.dynamicToolFilter
}

// CheckpointSaver returns the checkpoint saver used by the graph agent.
// Returns nil when checkpoint storage is not configured.
func (rt *Runtime) CheckpointSaver() graph.CheckpointSaver {
	return rt.checkpointSaver
}

// SessionSvc returns the session service used by the AGUI runner.
// Returns nil when the MySQL session backend is not configured (in-memory mode).
func (rt *Runtime) SessionSvc() session.Service {
	return rt.aguiSessionSvc
}

// MemorySvc returns the memory service used by the AGUI runner.
// Returns nil when the MySQL memory backend is not configured.
func (rt *Runtime) MemorySvc() memory.Service {
	return rt.aguiMemorySvc
}

// MCPToolSets returns the MCP toolsets used by the AGUI runner.
// Returns nil when the MCP toolsets are not configured.
func (rt *Runtime) MCPToolSets() *tool.MCPToolSet {
	return rt.aguiMCPToolSets
}

// New initialises the Agent Runtime from the global configuration (cc.AgentServer).
// Runtime contains the Agent and Runner.
func New() (*Runtime, error) {
	aguiCfg := cc.AgentServer().AGUI
	toolsCfg := cc.AgentServer().Tools
	storageCfg := cc.AgentServer().Storage

	allowedModels := aguiCfg.AllowedModelNames()
	// Register operator-supplied context windows for private models so that
	// the framework's token tailoring can resolve them correctly.
	if cw := aguiCfg.ModelContextWindows(); len(cw) > 0 {
		trpcmodel.RegisterModelContextWindows(cw)
		logs.Infof("registered custom model context windows: %v", cw)
	}

	// Build Readiness to record the readiness status of the Agent Server
	readiness := NewReadiness()
	skillMgr, err := skill.NewManager(readiness)
	if err != nil {
		logs.Errorf("build skill manager failed, err: %v", err)
		return nil, fmt.Errorf("build skill manager: %w", err)
	}

	promptMgr, err := prompt.NewManager(readiness)
	if err != nil {
		logs.Errorf("build prompt manager failed, err: %v", err)
		return nil, fmt.Errorf("build prompt manager: %w", err)
	}

	// Build one model instance per allowed model name.
	// Each model is associated with its provider's gateway config; models without
	// an explicit provider use the default "bkaidev" provider.
	defaultMdl, modelsMap, err := model.BuildAllModels(aguiCfg.Model.DefaultModel, allowedModels,
		aguiCfg.ModelProviderMapping())
	if err != nil {
		return nil, fmt.Errorf("build all models: %w", err)
	}

	// Share the default model with the session summarizer (both need the same LLM endpoint).
	// The aidev gateway config is also passed so the embedding client can reuse
	// the same endpoint and BK auth middleware.
	aidevGW, err := cc.AgentServer().GetProvider(constant.DefaultProviderName)
	if err != nil {
		return nil, fmt.Errorf("resolve default provider: %w, provider name: %s", err,
			constant.DefaultProviderName)
	}
	sessionSvc, memorySvc, err := storage.BuildStorageServices(defaultMdl, aidevGW, promptMgr.Store)
	if err != nil {
		return nil, err
	}

	// Build MCP toolsets from configuration.
	mcpToolSets, err := tool.BuildMCPToolSets()
	if err != nil {
		return nil, fmt.Errorf("build MCP toolsets: %w", err)
	}

	// Build dynamic tool filter when enabled. The filter performs per-invocation
	// BM25/keyword retrieval so the LLM only sees relevant MCP tools.
	var dynFilter trpctool.FilterFunc
	if d := toolsCfg.DynamicToolLoading; d != nil && d.Enabled {
		dynFilter = tool.MakeDynamicToolFilter(tool.NewLazyToolIndex(mcpToolSets, d, aidevGW))
		logs.Infof("dynamic tool loading: enabled (strategy=%s, topN=%d, scoreThreshold=%.2f, tags=%d)",
			d.Strategy, d.TopN, d.ScoreThreshold, len(d.ToolTags))
	}
	// Build checkpoint saver for graph agent interrupt/resume support.
	checkpointSaver, err := agent.BuildCheckpointSaver(storageCfg.Checkpoint)
	if err != nil {
		return nil, fmt.Errorf("build checkpoint saver: %w", err)
	}

	runnerOpts := buildRunnerOpts(sessionSvc, memorySvc)
	agUIRunner, err := newAGUIRunner(defaultMdl, modelsMap, mcpToolSets, skillMgr, promptMgr.Store, runnerOpts,
		checkpointSaver)
	if err != nil {
		return nil, fmt.Errorf("build AGUI runner: %w", err)
	}

	runtime := &Runtime{
		AGUIRunner:        agUIRunner,
		aguiSessionSvc:    sessionSvc,
		aguiMemorySvc:     memorySvc,
		aguiMCPToolSets:   mcpToolSets,
		dynamicToolFilter: dynFilter,
		skillMgr:          skillMgr,
		promptMgr:         promptMgr,
		readiness:         readiness,
		checkpointSaver:   checkpointSaver,
	}
	return runtime, nil
}

// buildRunnerOpts assembles runner.Option slice from the provided services.
// Nil services are omitted so the runner applies its own defaults.
func buildRunnerOpts(sessionSvc session.Service, memorySvc memory.Service) []runner.Option {
	var opts []runner.Option
	if sessionSvc != nil {
		opts = append(opts, runner.WithSessionService(sessionSvc))
	}
	if memorySvc != nil {
		opts = append(opts, runner.WithMemoryService(memorySvc))
	}
	return opts
}

func newAGUIRunner(defaultMdl trpcmodel.Model, modelsMap map[string]trpcmodel.Model, mcpToolSets *tool.MCPToolSet,
	skillMgr *skill.Manager, promptStore *prompt.Store, runnerOpts []runner.Option,
	checkpointSaver graph.CheckpointSaver) (runner.Runner, error) {

	aguiCfg := cc.AgentServer().AGUI

	var skillRepo skillpkg.Repository
	if skillMgr != nil {
		skillRepo = skillMgr.Repository
	}

	// When any MCP toolset requires per-request authentication (e.g. type "bkaidev"),
	// disable eager tool loading at construction time. Tools are fetched lazily on
	// the first agent run, at which point the real request context (with bk_ticket)
	// is available so MCP session initialization can authenticate successfully.
	refreshOnRun := cc.AgentServer().Tools.NeedToRefreshToolSetsOnRun()

	var agt trpcagent.Agent
	switch aguiCfg.Model.Mode {
	case enumor.AgentModeGraph:
		compiledGraph, err := agent.BuildGraph(defaultMdl, skillRepo, mcpToolSets,
			aguiCfg.AppName, aguiCfg.Model, promptStore)
		if err != nil {
			return nil, fmt.Errorf("build graph: %w", err)
		}
		agt, err = agent.NewGraphAgent(aguiCfg.AppName, compiledGraph, checkpointSaver)
		if err != nil {
			return nil, fmt.Errorf("create graph agent: %w", err)
		}
	default:
		promptCfg := cc.AgentServer().Prompt
		agt = agent.NewLLMAgent(defaultMdl, modelsMap, aguiCfg.Model,
			promptCfg.SystemPrompt, promptCfg.Instruction, skillRepo, mcpToolSets.TS,
			refreshOnRun, promptStore)
	}

	return runner.NewRunner(aguiCfg.AppName, agt, runnerOpts...), nil
}

// Close releases all resources held by the Runtime. It is safe to call multiple
// times; the underlying close is executed exactly once.
func (rt *Runtime) Close() error {
	var err error
	rt.closeOnce.Do(func() {
		if rt.aguiMemorySvc != nil {
			if cerr := rt.aguiMemorySvc.Close(); cerr != nil {
				logs.Warnf("close AGUI memory service: %v", cerr)
			}
		}
		if rt.aguiSessionSvc != nil {
			if cerr := rt.aguiSessionSvc.Close(); cerr != nil {
				logs.Warnf("close AGUI session service: %v", cerr)
			}
		}
		if rt.aguiMCPToolSets != nil {
			if cerr := rt.aguiMCPToolSets.Close(); cerr != nil {
				logs.Warnf("close AGUI MCP toolsets: %v", cerr)
			}
		}
		if rt.checkpointSaver != nil {
			if cerr := rt.checkpointSaver.Close(); cerr != nil {
				logs.Warnf("close checkpoint saver: %v", cerr)
			}
		}
	})
	return err
}
