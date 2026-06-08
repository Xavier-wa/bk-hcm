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
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"hcm/cmd/agent-server/logics/agent/hitl"
	"hcm/cmd/agent-server/logics/logger"
	"hcm/cmd/agent-server/logics/model"
	"hcm/cmd/agent-server/logics/prompt"
	"hcm/cmd/agent-server/logics/skill"
	"hcm/cmd/agent-server/logics/timer"
	"hcm/cmd/agent-server/logics/tool"
	"hcm/cmd/agent-server/logics/toolproxy"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
	toolskill "trpc.group/trpc-go/trpc-agent-go/tool/skill"
)

// BuildGraph constructs a simplified ReAct graph topology with four nodes:
//
//	START → llm → ConditionalEdge
//	  ├─ human_confirm → hitl → llm (loop back)
//	  ├─ other_tool_calls → tool → llm (loop back)
//	  └─ no tool_calls → fallback → llm (loop back, interrupt at fallback)
//
// The llm node is an LLM Node that decides whether to call tools or respond directly.
// The hitl node handles human-in-the-loop interrupts when LLM calls human_confirm.
// The tool node executes tools when the LLM requests them.
// The fallback node normalizes the LLM response and interrupts to wait for the next user message.
func BuildGraph(mdl trpcmodel.Model, skillRepo skillpkg.Repository, toolset *tool.MCPToolSet,
	toolProxy *toolproxy.ToolProxy, agentName string, modelCfg cc.AgentModelGeneralConfig, promptStore *prompt.Store) (
	*graph.Graph, error) {

	schema := graph.MessagesStateSchema()
	stateGraph := graph.NewStateGraph(schema)

	staticPrompt := resolveStaticPrompt(promptStore)
	llmOpts := genLLMNodeOptions(toolset, toolProxy, modelCfg)

	// 构建 LLM 调用 callback
	modelCb := trpcmodel.NewCallbacks()
	modelCb.AfterModel = append(modelCb.AfterModel, logger.MakeModelLoggerCallback())
	// 历史工具调用结果优化，减少 LLM 上下文长度
	modelCb.BeforeModel = append(modelCb.BeforeModel, model.MakeHistoricalToolResultFilter())
	// 远程 prompt 注入系统提示词（优先于其他要入System的提示词，如skill,time）
	if promptStore != nil {
		modelCb.BeforeModel = append(modelCb.BeforeModel, prompt.MakeSystemPromptReplaceCallback(promptStore))
	}
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
	// 注册 HITL 工具（纯声明工具，无执行逻辑）
	skillTools[constant.HumanConfirmToolName] = hitl.GetToolWrapper()
	// TODO 验证最终加载的 tool 有哪些
	// 1. LLM Node: decides whether to call tools or respond directly.
	stateGraph.AddLLMNode("llm", mdl, staticPrompt, skillTools, llmOpts...)

	// 2. HITL Node: handles human_confirm tool calls.
	stateGraph.AddNode("hitl", hitl.GetNode())

	// 3. Tool Node: executes tools when the LLM requests them.
	toolsOpts := genToolNodeOptions(toolset, toolProxy, agentName)
	stateGraph.AddToolsNode("tool", skillTools, toolsOpts...)

	// 4. Fallback Node: normalizes output when the LLM does not call any tools.
	stateGraph.AddNode("fallback", makeFallbackNode())

	// Set entry point.
	stateGraph.SetEntryPoint("llm")

	// Configure conditional edges with three-way routing:
	// - human_confirm only → hitl
	// - other tools → tool
	// - no tool_calls → fallback
	stateGraph.AddConditionalEdges("llm", makeRoutingFunc(), map[string]string{
		"hitl":     "hitl",
		"tool":     "tool",
		"fallback": "fallback",
	})

	// Loop back edges: hitl → llm, tool → llm, fallback → llm
	stateGraph.AddEdge("hitl", "llm")
	stateGraph.AddEdge("tool", "llm")
	stateGraph.AddEdge("fallback", "llm")

	return stateGraph.Compile()
}

// makeRoutingFunc returns the conditional edge routing function.
// It implements three-way routing based on tool_calls in the last message:
//   - Only human_confirm → "hitl"
//   - human_confirm + other tools → error (R1 boundary case)
//   - Other tools (no human_confirm) → "tool"
//   - No tool_calls → "fallback"
func makeRoutingFunc() func(ctx context.Context, state graph.State) (string, error) {
	return func(ctx context.Context, state graph.State) (string, error) {
		rid := rest.RidFromContext(ctx)
		messages, _ := state[graph.StateKeyMessages].([]trpcmodel.Message)
		if len(messages) == 0 {
			return "fallback", nil
		}

		lastMsg := messages[len(messages)-1]

		// Check if there are tool_calls
		if len(lastMsg.ToolCalls) == 0 {
			logs.Infof("routing: no tool_calls, route to fallback, rid: %s", rid)
			return "fallback", nil
		}

		// Check tool_calls content
		hasHumanConfirm := false
		hasOtherTools := false

		for _, tc := range lastMsg.ToolCalls {
			logs.Infof("routing: toolCall name=%s, id=%s, rid: %s", tc.Function.Name, tc.ID, rid)
			if tc.Function.Name == constant.HumanConfirmToolName {
				hasHumanConfirm = true
			} else {
				hasOtherTools = true
			}
		}
		logs.Infof("routing: hasHumanConfirm=%v, hasOtherTools=%v, rid: %s", hasHumanConfirm, hasOtherTools, rid)

		// R1: Multi-tool call boundary case handling
		if hasHumanConfirm && hasOtherTools {
			return "", fmt.Errorf("invalid tool calls: human_confirm cannot be combined with other tools")
		}

		if hasHumanConfirm {
			// Only human_confirm, route to hitl node
			logs.Infof("routing: only human_confirm, route to hitl, rid: %s", rid)
			return "hitl", nil
		}

		// Other tool calls (no human_confirm)
		logs.Infof("routing: other tool calls, route to tool, rid: %s", rid)
		return "tool", nil
	}
}

func genLLMNodeOptions(toolset *tool.MCPToolSet, toolProxy *toolproxy.ToolProxy,
	modelCfg cc.AgentModelGeneralConfig) []graph.Option {

	generationConfig := trpcmodel.GenerationConfig{
		MaxTokens:   cvt.ValToPtr(modelCfg.MaxTokens),
		Temperature: cvt.ValToPtr(modelCfg.Temperature),
		Stream:      modelCfg.Stream,
	}

	opts := []graph.Option{
		graph.WithGenerationConfig(generationConfig),
	}

	if toolProxy != nil && toolProxy.IsBuildOK() {
		// Tool Proxy 元工具为静态注册，无需 WithRefreshToolSetsOnRun。
		// MCP 实际工具已在启动/刷新时用 access_token 预加载，调用时仍使用请求 ctx 中的 bk_ticket 鉴权。
		opts = append(opts, graph.WithToolSets([]trpctool.ToolSet{toolProxy.GetProxyToolSet()}))
	} else {
		opts = append(opts,
			graph.WithToolSets(toolset.TS),
			graph.WithRefreshToolSetsOnRun(true),
		)
	}

	toolCb := logger.ToolLoggerCallback()
	toolCb.BeforeTool = append(toolCb.BeforeTool, tool.MakeParamFixCallbacks())
	opts = append(opts, graph.WithToolCallbacks(toolCb))

	return opts
}

func genToolNodeOptions(toolset *tool.MCPToolSet, proxy *toolproxy.ToolProxy, agentName string) []graph.Option {

	opts := []graph.Option{
		// skill tool 调用后将 skill 的加载状态写入 session.State
		graph.WithPostNodeCallback(skill.MakeSkillLoadAfterToolCallback(agentName)),
	}

	if proxy != nil && proxy.IsBuildOK() {
		opts = append(opts, graph.WithToolSets([]trpctool.ToolSet{proxy.GetProxyToolSet()}))
	} else {
		opts = append(opts,
			graph.WithToolSets(toolset.TS),
			graph.WithRefreshToolSetsOnRun(true),
		)
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

// makeFallbackNode returns a Function Node that normalizes the LLM response and
// interrupts to wait for the user's next message before looping back to llm.
func makeFallbackNode() graph.NodeFunc {
	return func(ctx context.Context, state graph.State) (any, error) {
		rid := rest.RidFromContext(ctx)
		lastResp, _ := state[graph.StateKeyLastResponse].(string)
		if lastResp == "" {
			lastResp = "抱歉，我暂时无法处理您的请求。"
		}

		// Interrupt to pause the graph and wait for the next user message.
		// Without this, fallback → llm would loop indefinitely when LLM keeps
		// responding without tool calls.
		// The interrupt key is a hash of the last response to uniquely identify the fallback node.
		interruptKey := buildFallbackInterruptKey(state, lastResp)
		resumeValue, err := graph.Interrupt(ctx, state, interruptKey, map[string]any{
			"last_response": lastResp,
		})
		if err != nil {
			logs.Infof("fallback node: waiting for user input, rid: %s", rid)
			return graph.State{graph.StateKeyLastResponse: lastResp}, err
		}

		userInput, ok := resumeValue.(string)
		if !ok {
			logs.Errorf("fallback node: invalid resume value type, expected string, got %T, rid: %s", resumeValue, rid)
			return nil, fmt.Errorf("fallback node: invalid resume value type, expected string, got %T", resumeValue)
		}
		logs.Infof("fallback node: resume with user input=%s, rid: %s", userInput, rid)

		return graph.State{
			graph.StateKeyMessages:     []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: userInput}},
			graph.StateKeyLastResponse: lastResp,
		}, nil
	}
}

// buildFallbackInterruptKey builds the interrupt key for the fallback node.
func buildFallbackInterruptKey(state graph.State, lastResp string) string {
	messages, _ := state[graph.StateKeyMessages].([]trpcmodel.Message)
	sum := sha256.Sum256([]byte(lastResp))
	hash := hex.EncodeToString(sum[:])
	if len(hash) > constant.FallbackInterruptKeyHashLen {
		hash = hash[:constant.FallbackInterruptKeyHashLen]
	}
	return fmt.Sprintf("%s%s%d%s%s", constant.FallbackInterruptKey, constant.InterruptKeySeparator,
		len(messages), constant.InterruptKeySeparator, hash)
}

// resolveStaticPrompt returns the system prompt to pass to AddLLMNode.
// It prefers Store content (populated by local-file or BKAIDev sync),
// falling back to raw cc config values only when the store has no system prompt yet.
func resolveStaticPrompt(store *prompt.Store) string {
	if store != nil {
		sp, _ := store.Get(constant.SystemPromptKey)
		inst, _ := store.Get(constant.InstructionKey)
		if p := prompt.BuildSystemPrompt(sp.Content, inst.Content); p != "" {
			return p
		}
	}
	promptCfg := cc.AgentServer().Prompt
	return prompt.BuildSystemPrompt(promptCfg.SystemPrompt, promptCfg.Instruction)
}
