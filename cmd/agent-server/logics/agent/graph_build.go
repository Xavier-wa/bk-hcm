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
	"time"

	"hcm/cmd/agent-server/logics/agent/hitl"
	"hcm/cmd/agent-server/logics/agent/intent"
	"hcm/cmd/agent-server/logics/logger"
	"hcm/cmd/agent-server/logics/model"
	"hcm/cmd/agent-server/logics/prompt"
	"hcm/cmd/agent-server/logics/skill"
	"hcm/cmd/agent-server/logics/timer"
	agenttool "hcm/cmd/agent-server/logics/tool"
	"hcm/cmd/agent-server/logics/toolproxy"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/uuid"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
	toolskill "trpc.group/trpc-go/trpc-agent-go/tool/skill"
)

// BuildGraph constructs a ReAct graph topology with five nodes:
//
//	START → intent_recognition → ConditionalEdge(by intent)
//	  ├─ host_apply → llm → ConditionalEdge(by tool_calls)
//	  │     ├─ human_confirm → hitl → llm (loop back)
//	  │     ├─ other_tool_calls → tool → llm (loop back)
//	  │     └─ no tool_calls → fallback (interrupt) → llm (session stays on host_apply)
//	  └─ other intents → fallback (interrupt) → intent_recognition
//
// The intent_recognition node classifies user intent and writes StateKeyIntent.
// The llm node drives the ReAct loop for host_apply.
// The hitl node handles human-in-the-loop interrupts when LLM calls human_confirm.
// The tool node executes MCP and skill tools.
// The fallback node normalizes the LLM response and interrupts to wait for the next user message.
func BuildGraph(mdl trpcmodel.Model, skillRepo skillpkg.Repository, toolset *agenttool.MCPToolSet,
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

	// 构建 skill 工具集：skill 工具 + HITL 工具（声明性工具，路由到 hitl 节点）
	skillTools := make(map[string]trpctool.Tool)
	skillTools[constant.SkillLoadToolName] = toolskill.NewLoadTool(skillRepo)
	skillTools[constant.SkillListDocsToolName] = toolskill.NewListDocsTool(skillRepo)
	skillTools[constant.SkillSelectDocsToolName] = toolskill.NewSelectDocsTool(skillRepo)
	// 注册 HITL 工具（纯声明工具，路由到 hitl 节点处理，不经过 tool 节点执行）
	skillTools[constant.HumanConfirmToolName] = hitl.GetToolWrapper()

	// 1. Intent Recognition Node: classifies intent and writes StateKeyIntent before routing.
	stateGraph.AddNode("intent_recognition",
		intent.MakeIntentRecognitionNode(mdl, promptStore, cc.AgentServer().Intent.ContextWindowSize))

	// 2. LLM Node: drives the ReAct loop for supported intents.
	stateGraph.AddLLMNode("llm", mdl, staticPrompt, skillTools, llmOpts...)

	// 3. HITL Node: handles human_confirm tool calls mid-task.
	stateGraph.AddNode("hitl", hitl.GetNode())

	// 3. Tool Node: executes tools when the LLM requests them.
	toolsOpts := genToolNodeOptions(toolset, toolProxy, agentName)
	stateGraph.AddToolsNode("tool", skillTools, toolsOpts...)

	// 5. Fallback Node: delivers LLM response, interrupts, and routes the next user message.
	stateGraph.AddNode("fallback", makeFallbackNode())

	// Entry point: every new run starts with intent recognition.
	stateGraph.SetEntryPoint("intent_recognition")

	// intent_recognition → llm (supported intent) or fallback (unsupported intent)
	stateGraph.AddConditionalEdges("intent_recognition", makeIntentRoutingFunc(), map[string]string{
		"llm":      "llm",
		"fallback": "fallback",
	})

	// llm → hitl / tool / fallback based on tool_calls in the last message
	stateGraph.AddConditionalEdges("llm", makeRoutingFunc(), map[string]string{
		"hitl":     "hitl",
		"tool":     "tool",
		"fallback": "fallback",
	})

	// hitl and tool loop back to llm to continue the current task.
	stateGraph.AddEdge("hitl", "llm")
	stateGraph.AddEdge("tool", "llm")

	// fallback routes by StateKeyIntent after collecting the user's next message:
	//   host_apply → llm (session-bound host apply, skip intent recognition)
	//   other/missing → intent_recognition
	stateGraph.AddConditionalEdges("fallback", makePostFallbackRoutingFunc(), map[string]string{
		"llm":                "llm",
		"intent_recognition": "intent_recognition",
	})

	return stateGraph.Compile()
}

// makeIntentRoutingFunc routes after intent_recognition based on StateKeyIntent.
// Only host_apply enters the host-apply ReAct sub-flow; other intents route to fallback.
func makeIntentRoutingFunc() func(ctx context.Context, state graph.State) (string, error) {
	return func(ctx context.Context, state graph.State) (string, error) {
		rid := rest.RidFromContext(ctx)
		intentStr, _ := state[constant.StateKeyIntent].(string)
		// TODO: only host_apply is supported now; add new sub-flows here
		if enumor.IntentType(intentStr) == enumor.IntentTypeHostApply {
			logs.Infof("[intent routing] intent=%s, route to llm, rid: %s", intentStr, rid)
			return "llm", nil
		}

		logs.Infof("[intent routing] intent=%s (unsupported), route to fallback, rid: %s", intentStr, rid)
		return "fallback", nil
	}
}

// makePostFallbackRoutingFunc routes after fallback interrupt/resume based on StateKeyIntent.
// When intent is host_apply the session stays on the host-apply ReAct loop without re-running intent recognition.
// For other or missing intents a fresh intent recognition run starts.
func makePostFallbackRoutingFunc() func(ctx context.Context, state graph.State) (string, error) {
	return func(ctx context.Context, state graph.State) (string, error) {
		rid := rest.RidFromContext(ctx)
		intentStr, _ := state[constant.StateKeyIntent].(string)
		if enumor.IntentType(intentStr) == enumor.IntentTypeHostApply {
			logs.Infof("[fallback routing] intent=host_apply, route to llm, rid: %s", rid)
			return "llm", nil
		}
		logs.Infof("[fallback routing] intent=%s, route to intent_recognition, rid: %s", intentStr, rid)
		return "intent_recognition", nil
	}
}

// makeRoutingFunc returns the conditional edge routing function for the llm node.
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

		if len(lastMsg.ToolCalls) == 0 {
			logs.Infof("routing: no tool_calls, route to fallback, rid: %s", rid)
			return "fallback", nil
		}

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

		// R1: human_confirm must not be combined with other tool calls
		if hasHumanConfirm && hasOtherTools {
			return "", fmt.Errorf("invalid tool calls: human_confirm cannot be combined with other tools")
		}

		if hasHumanConfirm {
			logs.Infof("routing: only human_confirm, route to hitl, rid: %s", rid)
			return "hitl", nil
		}

		logs.Infof("routing: other tool calls, route to tool, rid: %s", rid)
		return "tool", nil
	}
}

func genLLMNodeOptions(toolset *agenttool.MCPToolSet, toolProxy *toolproxy.ToolProxy,
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
	toolCb.BeforeTool = append(toolCb.BeforeTool, agenttool.MakeParamFixCallbacks())
	opts = append(opts, graph.WithToolCallbacks(toolCb))

	return opts
}

func genToolNodeOptions(toolset *agenttool.MCPToolSet, proxy *toolproxy.ToolProxy, agentName string) []graph.Option {

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

// makeFallbackNode returns the fallback node.
// It delivers response to user, interrupts to wait for next input, and resumes graph routing.
//
// When the graph skips llm (unsupported intent), it emits an intent-specific fallback message.
// On resume replay, if assistant tail already exists, it skips duplicate emit.
//
// In InterruptError paths, executor does not apply node delta immediately, so the assistant
// fallback message is appended again in resume success path when needed.
func makeFallbackNode() graph.NodeFunc {
	return func(ctx context.Context, state graph.State) (any, error) {
		rid := rest.RidFromContext(ctx)
		messages, _ := state[graph.StateKeyMessages].([]trpcmodel.Message)
		lastResp := resolveFallbackLastResp(state)
		// 上一步没有产生助手回复，需要进行emit事件封装，生成AGUI消息
		if shouldEmitFallbackResponse(state, messages, lastResp) {
			emitFallbackMessage(ctx, state, "fallback", lastResp)
		}

		interruptKey := buildFallbackInterruptKey(state, lastResp)
		resumeValue, err := graph.Interrupt(ctx, state, interruptKey, map[string]any{
			"last_response": lastResp,
		})
		if err != nil {
			logs.Infof("fallback node: waiting for user input, rid: %s", rid)
			return graph.State{
				graph.StateKeyLastResponse: lastResp,
				graph.StateKeyMessages: []trpcmodel.Message{
					{Role: trpcmodel.RoleAssistant, Content: lastResp},
				},
			}, err
		}

		userInput, ok := resumeValue.(string)
		if !ok {
			logs.Errorf("fallback node: invalid resume value type, expected string, got %T, rid: %s",
				resumeValue, rid)
			return nil, fmt.Errorf("fallback node: invalid resume value type, expected string, got %T", resumeValue)
		}
		logs.Infof("fallback node: resume with user input=%s, rid: %s", userInput, rid)

		return buildFallbackResumeDelta(ctx, state, messages, lastResp, userInput), nil
	}
}

// resolveFallbackLastResp returns response text for current fallback turn.
// Priority: assistant tail (llm->fallback path) > StateKeyLastResponse > fallback by intent.
func resolveFallbackLastResp(state graph.State) string {
	messages, _ := state[graph.StateKeyMessages].([]trpcmodel.Message)
	if len(messages) > 0 && messages[len(messages)-1].Role == trpcmodel.RoleAssistant {
		return messages[len(messages)-1].Content
	}
	if lastResp, ok := state[graph.StateKeyLastResponse].(string); ok && lastResp != "" {
		return lastResp
	}
	return unsupportedIntentFallbackMessage(state)
}

// shouldEmitFallbackResponse reports whether fallback should emit model execution event.
// During interrupt replay or when assistant tail already exists, skip duplicate emit.
func shouldEmitFallbackResponse(state graph.State, messages []trpcmodel.Message, lastResp string) bool {
	if len(messages) > 0 && messages[len(messages)-1].Role == trpcmodel.RoleAssistant {
		return false
	}
	return lastResp != ""
}

// hasAssistantTailWithContent reports whether messages tail is assistant with given content.
func hasAssistantTailWithContent(messages []trpcmodel.Message, content string) bool {
	if content == "" || len(messages) == 0 {
		return false
	}
	last := messages[len(messages)-1]
	return last.Role == trpcmodel.RoleAssistant && last.Content == content
}

// buildFallbackResumeDelta builds state delta after fallback resumes with next user input.
// For unsupported intent turns, it rebuilds history to assistant+user to avoid stale anchoring.
//
// StateKeyUserInput is explicitly cleared in every resume delta. The framework's
// mergeInitialStateNonInternal only merges keys absent from the restored checkpoint,
// so a stale user_input written during a run that never reached the LLM node
// (e.g. "查看预测" → fallback interrupt) persists across checkpoint/resume cycles.
// Without the explicit clear, the LLM node's executeUserInputStage would use the
// stale value and overwrite the correctly-rebuilt messages tail.
func buildFallbackResumeDelta(ctx context.Context, state graph.State, messages []trpcmodel.Message, lastResp,
	userInput string) graph.State {

	rid := rest.RidFromContext(ctx)
	delta := graph.State{
		graph.StateKeyLastResponse: "",
		// 清空 StateKeyUserInput：LLM 节点正常执行后会在自己的 delta 里将其置空，
		// 但不支持意图时流程绕过了 LLMNode
		// 旧值会残留在 checkpoint 里。若不清空，下一轮 resume 进入 LLM 节点时，
		// executeUserInputStage 会用旧的 user_input 覆盖正确重建的 messages 末尾消息。
		// resume的时候是把用户的输入追加到user message里面，所以这里可以清空
		graph.StateKeyUserInput: "",
	}

	intentStr, _ := state[constant.StateKeyIntent].(string)
	intentType := enumor.IntentType(intentStr)
	clearingUnsupported := intentStr != "" && intentType != enumor.IntentTypeHostApply

	if clearingUnsupported {
		logs.Infof("[fallback] clear unsupported intent=%s for re-recognition, rid: %s", intentStr, rid)
		delta[constant.StateKeyIntent] = ""
		delta[graph.StateKeyMessages] = []graph.MessageOp{
			graph.AppendMessages{
				Items: []trpcmodel.Message{
					{Role: trpcmodel.RoleAssistant, Content: lastResp},
					{Role: trpcmodel.RoleUser, Content: userInput},
				},
			},
		}
		return delta
	}

	msgDelta := []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: userInput}}
	if !hasAssistantTailWithContent(messages, lastResp) && lastResp != "" {
		msgDelta = []trpcmodel.Message{
			{Role: trpcmodel.RoleAssistant, Content: lastResp},
			{Role: trpcmodel.RoleUser, Content: userInput},
		}
	}

	delta[graph.StateKeyMessages] = msgDelta
	return delta
}

// buildFallbackInterruptKey builds a stable interrupt key for the fallback node.
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

// unsupportedIntentFallbackMessage returns a user-facing reply when llm was not invoked.
func unsupportedIntentFallbackMessage(state graph.State) string {
	intentStr, _ := state[constant.StateKeyIntent].(string)
	switch enumor.IntentType(intentStr) {
	case enumor.IntentTypeResourceQuery:
		return "资源查询功能正在建设中，敬请期待。如需主机申领，请直接描述您的申领需求。"
	case enumor.IntentTypeChat:
		return "您好，当前我主要支持主机申领相关能力。如需申请主机，请描述您的配置与业务需求。"
	default:
		return "抱歉，我暂时无法处理您的请求。"
	}
}

// emitFallbackMessage emits the fallback text as a proper model execution event so it appears
// as an assistant text message in the AG-UI stream. This is necessary when the graph routes
// directly from intent_recognition to fallback (skipping the llm node), because no LLM response
// is available to produce the TextMessage event sequence.
func emitFallbackMessage(ctx context.Context, state graph.State, nodeID, message string) {
	rid := rest.RidFromContext(ctx)
	emitter := graph.GetEventEmitterWithContext(ctx, state)
	now := time.Now()
	responseID := uuid.UUID()
	evt := graph.NewModelExecutionEvent(
		graph.WithModelEventNodeID(nodeID),
		graph.WithModelEventResponseID(responseID),
		graph.WithModelEventOutput(message),
		graph.WithModelEventPhase(graph.ModelExecutionPhaseComplete),
		graph.WithModelEventStartTime(now),
		graph.WithModelEventEndTime(now),
	)
	if err := emitter.Emit(evt); err != nil {
		logs.Warnf("fallback node: emit fallback message failed, err: %v, rid: %s", err, rid)
	}
}
