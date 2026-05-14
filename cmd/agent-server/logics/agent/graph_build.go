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

package agent

import (
	"context"

	"hcm/cmd/agent-server/logics/logger"
	"hcm/cmd/agent-server/logics/model"
	"hcm/cmd/agent-server/logics/skill"
	"hcm/cmd/agent-server/logics/timer"
	"hcm/cmd/agent-server/logics/tool"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	cvt "hcm/pkg/tools/converter"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
	toolskill "trpc.group/trpc-go/trpc-agent-go/tool/skill"
)

// BuildGraph constructs a simplified ReAct graph topology with three nodes:
//
//	START → llm → ConditionalEdge
//	  ├─ tool_calls → tool → llm (loop back)
//	  └─ no tool_calls → fallback → END
//
// The llm node is an LLM Node that decides whether to call tools or respond directly.
// The tool node executes the tools requested by the LLM.
// The fallback node is a Function Node that normalizes the final response when no tools are called.
func BuildGraph(mdl trpcmodel.Model, skillRepo skillpkg.Repository, toolset *tool.MCPToolSet,
	systemPrompt, instruction, agentName string, modelCfg cc.AgentModelGeneralConfig) (*graph.Graph, error) {

	schema := graph.MessagesStateSchema()
	stateGraph := graph.NewStateGraph(schema)

	// Build the combined prompt from external prompt sources.
	prompt := buildPrompt(systemPrompt, instruction)
	llmOpts := genLLMNodeOptions(toolset, modelCfg)

	// 构建 LLM 调用 callback
	modelCb := trpcmodel.NewCallbacks()
	modelCb.AfterModel = append(modelCb.AfterModel, logger.MakeModelLoggerCallback())
	// 历史工具调用结果优化，减少 LLM 上下文长度
	modelCb.BeforeModel = append(modelCb.BeforeModel, model.MakeHistoricalToolResultFilter())
	// 技能上下文注入
	modelCb.BeforeModel = append(modelCb.BeforeModel, skill.MakeSkillInjectWithModelCallback(agentName, skillRepo))
	// 当前时间注入
	modelCb.BeforeModel = append(modelCb.BeforeModel, timer.MakeTimeInjectCallback())
	llmOpts = append(llmOpts, graph.WithModelCallbacks(modelCb))

	// 构建 skill 工具集
	skillTools := make(map[string]trpctool.Tool)
	skillTools[constant.SkillLoadToolName] = toolskill.NewLoadTool(skillRepo)
	skillTools[constant.SkillListDocsToolName] = toolskill.NewListDocsTool(skillRepo)
	skillTools[constant.SkillSelectDocsToolName] = toolskill.NewSelectDocsTool(skillRepo)
	// TODO 验证最终加载的 tool 有哪些
	// 1. LLM Node: decides whether to call tools or respond directly.
	stateGraph.AddLLMNode("llm", mdl, prompt, skillTools, llmOpts...)

	// 2. Tool Node: executes tools when the LLM requests them.
	toolsOpts := genToolNodeOptions(toolset, agentName)
	stateGraph.AddToolsNode("tool", skillTools, toolsOpts...)

	// 3. Fallback Node: normalizes output when the LLM does not call any tools.
	stateGraph.AddNode("fallback", makeFallbackNode())

	// Set entry point.
	stateGraph.SetEntryPoint("llm")

	// Configure tool conditional edges.
	stateGraph.AddToolsConditionalEdges("llm", "tool", "fallback")
	stateGraph.AddEdge("tool", "llm")

	// Fallback always leads to END.
	stateGraph.AddEdge("fallback", graph.End)
	stateGraph.SetFinishPoint("fallback")

	return stateGraph.Compile()
}

func genLLMNodeOptions(toolset *tool.MCPToolSet, modelCfg cc.AgentModelGeneralConfig) []graph.Option {
	generationConfig := trpcmodel.GenerationConfig{
		MaxTokens:   cvt.ValToPtr(modelCfg.MaxTokens),
		Temperature: cvt.ValToPtr(modelCfg.Temperature),
		Stream:      modelCfg.Stream,
	}

	opts := []graph.Option{
		graph.WithGenerationConfig(generationConfig),
		graph.WithToolSets(toolset.TS),
		graph.WithRefreshToolSetsOnRun(true),
	}

	toolCb := logger.ToolLoggerCallback()
	toolCb.BeforeTool = append(toolCb.BeforeTool, tool.MakeParamFixCallbacks())
	opts = append(opts, graph.WithToolCallbacks(toolCb))

	return opts
}

func genToolNodeOptions(toolset *tool.MCPToolSet, agentName string) []graph.Option {
	opts := []graph.Option{
		graph.WithToolSets(toolset.TS),
		graph.WithRefreshToolSetsOnRun(true),
		// skill tool 调用后将 skill 的加载状态写入 session.State
		graph.WithPostNodeCallback(skill.MakeSkillLoadAfterToolCallback(agentName)),
	}
	return opts
}

// buildPrompt combines system prompt and instruction into a single prompt string.
func buildPrompt(systemPrompt, instruction string) string {
	prompt := systemPrompt
	if instruction != "" {
		if prompt != "" {
			prompt += "\n\n"
		}
		prompt += instruction
	}
	return prompt
}

// makeFallbackNode returns a Function Node that extracts the LLM's final response
// from state and writes it back as the normalized output.
func makeFallbackNode() graph.NodeFunc {
	return func(_ context.Context, state graph.State) (any, error) {
		// Extract the last response from the LLM node output.
		lastResp, _ := state[graph.StateKeyLastResponse].(string)
		if lastResp == "" {
			lastResp = "抱歉，我暂时无法处理您的请求。"
		}
		return graph.State{graph.StateKeyLastResponse: lastResp}, nil
	}
}
