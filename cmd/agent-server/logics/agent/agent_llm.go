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

// Package agent provides the llm agent implementation.
package agent

import (
	"hcm/cmd/agent-server/logics/logger"
	"hcm/cmd/agent-server/logics/model"
	"hcm/cmd/agent-server/logics/tool"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// NewLLMAgent assembles the AGUI llm agent.
// defaultMdl is the fallback model used when no per-request model name is specified.
// modelsMap registers all models that can be selected per-request via agent.WithModelName.
// systemPrompt is the GlobalInstruction content (prepended to every LLM request).
// instruction is the per-request task instruction content (appended to every LLM request).
// skillRepo is the optional skill repository for progressive skill loading (may be nil).
// toolSets contains ToolSet instances (e.g. MCP server toolsets).
// refreshOnRun controls whether toolset tool lists are resolved lazily per-run.
func NewLLMAgent(defaultMdl trpcmodel.Model, modelsMap map[string]trpcmodel.Model, modelCfg cc.AgentModelGeneralConfig,
	systemPrompt, instruction string, skillRepo skillpkg.Repository, toolSets []trpctool.ToolSet,
	refreshOnRun bool) trpcagent.Agent {

	generationConfig := trpcmodel.GenerationConfig{
		MaxTokens:   cvt.ValToPtr(modelCfg.MaxTokens),
		Temperature: cvt.ValToPtr(modelCfg.Temperature),
		Stream:      modelCfg.Stream,
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

	modelCb := trpcmodel.NewCallbacks()
	modelCb.AfterModel = append(modelCb.AfterModel, logger.MakeModelLoggerCallback())
	modelCb.BeforeModel = append(modelCb.BeforeModel, model.MakeHistoricalToolResultFilter())
	opts = append(opts, llmagent.WithModelCallbacks(modelCb))

	logs.Infof("AGUI agent: models=%d skills=%v toolSets=%d refreshToolSetsOnRun=%v systemPrompt=%v instruction=%v",
		len(modelsMap), skillRepo != nil, len(toolSets), refreshOnRun, systemPrompt != "", instruction != "")
	return llmagent.New(cc.AgentServer().AGUI.AppName, opts...)
}
