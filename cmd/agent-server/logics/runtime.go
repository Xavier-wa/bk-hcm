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

	"hcm/cmd/agent-server/logics/logger"
	"hcm/cmd/agent-server/logics/model"
	"hcm/cmd/agent-server/logics/skill"
	"hcm/cmd/agent-server/logics/storage"
	"hcm/cmd/agent-server/logics/tool"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"

	_ "github.com/ncruces/go-sqlite3/driver" // import sqlite3 driver, used by memory/sqlitevec
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
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
	closeOnce         sync.Once
}

// DynamicToolFilter returns the dynamic tool filter function, or nil if not enabled.
func (rt *Runtime) DynamicToolFilter() trpctool.FilterFunc {
	return rt.dynamicToolFilter
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

	allowedModels := aguiCfg.AllowedModelNames()
	// Register operator-supplied context windows for private models so that
	// the framework's token tailoring can resolve them correctly.
	if cw := aguiCfg.ModelContextWindows(); len(cw) > 0 {
		trpcmodel.RegisterModelContextWindows(cw)
		logs.Infof("registered custom model context windows: %v", cw)
	}

	// Build one model instance per allowed model name.
	// Each model is associated with its provider's gateway config; models without
	// an explicit provider use the default "aidev" provider.
	defaultMdl, modelsMap, err := model.BuildAllModels(aguiCfg.DefaultModel, allowedModels,
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
	sessionSvc, memorySvc, err := storage.BuildStorageServices(defaultMdl, aidevGW)
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

	// Build skill repository from configuration.
	skillRepo, err := skill.BuildSkillRepo()
	if err != nil {
		return nil, fmt.Errorf("build skill repo: %w", err)
	}

	// When any MCP toolset requires per-request authentication (e.g. type "bkaidev"),
	// disable eager tool loading at construction time. Tools are fetched lazily on
	// the first agent run, at which point the real request context (with bk_ticket)
	// is available so MCP session initialization can authenticate successfully.
	refreshOnRun := cc.AgentServer().Tools.NeedToRefreshToolSetsOnRun()
	agt := newAgentWithModel(defaultMdl, modelsMap, aguiCfg.Stream,
		aguiCfg.Prompt.SystemPrompt, aguiCfg.Prompt.Instruction, skillRepo, mcpToolSets.TS, refreshOnRun)
	runnerOpts := buildRunnerOpts(sessionSvc, memorySvc)
	agUIRunner := runner.NewRunner(agt.Info().Name, agt, runnerOpts...)

	return &Runtime{
		AGUIRunner:        agUIRunner,
		aguiSessionSvc:    sessionSvc,
		aguiMemorySvc:     memorySvc,
		aguiMCPToolSets:   mcpToolSets,
		dynamicToolFilter: dynFilter,
	}, nil
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

// newAgentWithModel assembles the AGUI llm agent.
// defaultMdl is the fallback model used when no per-request model name is specified.
// modelsMap registers all models that can be selected per-request via agent.WithModelName.
// systemPrompt is the GlobalInstruction content (prepended to every LLM request).
// instruction is the per-request task instruction content (appended to every LLM request).
// skillRepo is the optional skill repository for progressive skill loading (may be nil).
// toolSets contains ToolSet instances (e.g. MCP server toolsets).
// refreshOnRun controls whether toolset tool lists are resolved lazily per-run.
func newAgentWithModel(defaultMdl trpcmodel.Model, modelsMap map[string]trpcmodel.Model, isStream bool,
	systemPrompt, instruction string, skillRepo skillpkg.Repository, toolSets []trpctool.ToolSet,
	refreshOnRun bool) agent.Agent {

	generationConfig := trpcmodel.GenerationConfig{
		MaxTokens:   cvt.ValToPtr(38000),
		Temperature: cvt.ValToPtr(0.7),
		Stream:      isStream,
	}

	opts := []llmagent.Option{
		llmagent.WithGenerationConfig(generationConfig),
		llmagent.WithMaxLLMCalls(constant.DefaultMaxLLMCalls),
		llmagent.WithMaxToolIterations(constant.DefaultMaxToolIterations),
		llmagent.WithAddCurrentTime(true),
		llmagent.WithTimezone("Asia/Shanghai"),
		llmagent.WithAddSessionSummary(true),
		llmagent.WithMaxHistoryRuns(constant.DefaultMaxHistoryRuns),
		llmagent.WithPreloadMemory(cc.AgentServer().Storage.Memory.PreloadLimit),
	}
	if systemPrompt != "" {
		opts = append(opts, llmagent.WithGlobalInstruction(systemPrompt))
	}
	if instruction != "" {
		opts = append(opts, llmagent.WithInstruction(instruction))
	}
	if defaultMdl != nil {
		opts = append(opts, llmagent.WithModel(defaultMdl))
	}
	if len(modelsMap) > 0 {
		opts = append(opts, llmagent.WithModels(modelsMap))
	}
	if skillRepo != nil {
		opts = append(opts, llmagent.WithSkills(skillRepo))
		opts = append(opts, llmagent.WithSkillLoadMode(llmagent.SkillLoadModeSession))
		// NOTE: skill_run 幻觉严重，使用仅知识注入模式，避免模型编造 script 执行;
		//  该模式下不支持在 skill 中引入 command / script
		opts = append(opts, llmagent.WithSkillToolProfile(llmagent.SkillToolProfileKnowledgeOnly))
		// TODO 增加 prompt cache 命中率，不再把 skill 注入到 system prompt，而是单独提供 tool result;
		//  启用该模式需改造 tool result 的压缩功能
		// opts = append(opts, llmagent.WithSkillsLoadedContentInToolResults(true))
	}
	if len(toolSets) > 0 {
		opts = append(opts, llmagent.WithToolSets(toolSets))
		if refreshOnRun {
			opts = append(opts, llmagent.WithRefreshToolSetsOnRun(true))
		}
	}

	toolCb := logger.ToolLoggerCallback()
	toolCb.BeforeTool = append(toolCb.BeforeTool, tool.MakeParamFixCallbacks())
	opts = append(opts, llmagent.WithToolCallbacks(toolCb))

	modelCb := logger.ModelLoggerCallback()
	modelCb.BeforeModel = append(modelCb.BeforeModel, model.MakeHistoricalToolResultFilter())
	opts = append(opts, llmagent.WithModelCallbacks(modelCb))

	logs.Infof("AGUI agent: models=%d skills=%v toolSets=%d refreshToolSetsOnRun=%v systemPrompt=%v instruction=%v",
		len(modelsMap), skillRepo != nil, len(toolSets), refreshOnRun, systemPrompt != "", instruction != "")
	return llmagent.New(cc.AgentServer().AGUI.AppName, opts...)
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
		if cerr := rt.aguiMCPToolSets.Close(); cerr != nil {
			logs.Warnf("close AGUI MCP toolsets: %v", cerr)
		}
	})
	return err
}
