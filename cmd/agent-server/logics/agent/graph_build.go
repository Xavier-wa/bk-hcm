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

// BuildGraph constructs a ReAct graph topology with scene_dispatch as the single routing hub:
//
//	START → scene_dispatch → ConditionalEdge
//	  ├─ supported session_tag (host_apply)        → llm
//	  ├─ this-turn intent recognised but unsupported → fallback
//	  └─ no tag / no intent this turn               → intent_recognition → scene_dispatch
//	llm → ConditionalEdge(by tool_calls)
//	  ├─ human_confirm → hitl → llm (loop back)
//	  ├─ other_tool_calls → tool → llm (loop back)
//	  └─ no tool_calls → fallback (interrupt)
//	fallback (interrupt) → scene_dispatch (re-dispatch; this-turn intent always cleared)
//
// The scene_dispatch node is the single routing brain: it commits a recognised supported
// intent into StateKeySessionTag and decides the next hop.
// The intent_recognition node only classifies user intent (writes StateKeyIntent) and returns to scene_dispatch.
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

	// 1. Scene Dispatch Node: single routing hub; commits supported intent into StateKeySessionTag.
	stateGraph.AddNode("scene_dispatch", makeSceneDispatchNode())

	// 2. Intent Recognition Node: classifies intent and writes StateKeyIntent, then returns to scene_dispatch.
	stateGraph.AddNode("intent_recognition", intent.MakeIntentRecognitionNode(mdl, promptStore,
		cc.AgentServer().Intent.ContextWindowSize))

	// 3. LLM Node: drives the ReAct loop for supported scenes.
	stateGraph.AddLLMNode("llm", mdl, staticPrompt, skillTools, llmOpts...)

	// 4. HITL Node: handles human_confirm tool calls mid-task.
	stateGraph.AddNode("hitl", hitl.GetNode())

	// 5. Tool Node: executes tools when the LLM requests them.
	toolsOpts := genToolNodeOptions(toolset, toolProxy, agentName)
	stateGraph.AddToolsNode("tool", skillTools, toolsOpts...)

	// 6. Fallback Node: delivers LLM response, interrupts, and routes the next user message.
	stateGraph.AddNode("fallback", makeFallbackNode())

	// Entry point: every new run starts with scene dispatch.
	stateGraph.SetEntryPoint("scene_dispatch")

	// scene_dispatch → llm (supported tag/intent) / fallback (unsupported intent) / intent_recognition (no tag, no intent)
	stateGraph.AddConditionalEdges("scene_dispatch", makeSceneDispatchRoutingFunc(), map[string]string{
		"llm":                "llm",
		"fallback":           "fallback",
		"intent_recognition": "intent_recognition",
	})

	// intent_recognition always returns to scene_dispatch for the routing decision.
	stateGraph.AddEdge("intent_recognition", "scene_dispatch")

	// llm → hitl / tool / fallback based on tool_calls in the last message
	stateGraph.AddConditionalEdges("llm", makeRoutingFunc(), map[string]string{
		"hitl":     "hitl",
		"tool":     "tool",
		"fallback": "fallback",
	})

	// hitl and tool loop back to llm to continue the current task.
	stateGraph.AddEdge("hitl", "llm")
	stateGraph.AddEdge("tool", "llm")

	// fallback always returns to scene_dispatch so the unified routing hub handles
	// the next user message regardless of whether the session already has a tag.
	stateGraph.AddEdge("fallback", "scene_dispatch")

	return stateGraph.Compile()
}

// makeSceneDispatchNode returns the scene_dispatch node function.
// 场景分发节点：会话无标签但本轮意图命中受支持场景时，将其提交到 StateKeySessionTag。
// 仅做 state 提交，不写 DB；DB 回写由 middleware 在 Run 结束后对账完成。
func makeSceneDispatchNode() graph.NodeFunc {
	return func(ctx context.Context, state graph.State) (any, error) {
		rid := rest.RidFromContext(ctx)
		tag, _ := state[constant.StateKeySessionTag].(enumor.IntentType)
		if tag.IsSupportedScene() {
			return graph.State{}, nil
		}

		// 检查是否意图识别出来了支持的场景，是则提交到 StateKeySessionTag
		intentStr, _ := state[constant.StateKeyIntent].(string)
		intentType := enumor.IntentType(intentStr)
		if intentType.IsSupportedScene() {
			logs.Infof("[scene dispatch] commit session_tag=%s from intent, rid: %s", intentType, rid)
			return graph.State{constant.StateKeySessionTag: intentType}, nil
		}

		return graph.State{}, nil
	}
}

// makeSceneDispatchRoutingFunc 根据会话标签与本轮意图决定 scene_dispatch 的后续路由。
func makeSceneDispatchRoutingFunc() func(ctx context.Context, state graph.State) (string, error) {
	return func(ctx context.Context, state graph.State) (string, error) {
		rid := rest.RidFromContext(ctx)
		tag, _ := state[constant.StateKeySessionTag].(enumor.IntentType)
		if tag.IsSupportedScene() {
			logs.Infof("[scene dispatch routing] session_tag=%s, route to llm, rid: %s", tag, rid)
			return "llm", nil
		}

		// 本轮意图非空且不受支持：直接路由到 fallback 给出拒识回复，
		// 避免 intent_recognition 与 scene_dispatch 之间反复循环。
		if intentStr, _ := state[constant.StateKeyIntent].(string); intentStr != "" {
			intentType := enumor.IntentType(intentStr)
			if !intentType.IsSupportedScene() {
				logs.Infof("[scene dispatch routing] intent=%s unsupported, route to fallback, rid: %s", intentType, rid)
				return "fallback", nil
			}
		}

		logs.Infof("[scene dispatch routing] no tag/intent, route to intent_recognition, rid: %s", rid)
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

// buildPrompt 将 system prompt 与 instruction 组合成最终提示词。
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

// makeFallbackNode 返回 fallback 节点：
// 负责向用户返回回复、执行 interrupt 等待下一轮输入，并在 resume 后返回用户输入供路由继续执行。
//
// 当图在未经过 llm 节点（不支持意图）时，会先通过 model execution event 输出兜底文案后再 interrupt。
// 节点在 resume 时会被重执行；若检测到存在 resume 值，或消息尾部已有 assistant 回复，则跳过重复 emit。
//
// 注意：executor 在 InterruptError 场景不会应用节点返回的 delta，因此 assistant 消息需要在
// resume 成功路径补齐（并在不支持意图切换时裁剪历史），不能仅依赖 StateKeyLastResponse 跨 checkpoint 持久化。
func makeFallbackNode() graph.NodeFunc {
	return func(ctx context.Context, state graph.State) (any, error) {
		rid := rest.RidFromContext(ctx)
		messages, _ := state[graph.StateKeyMessages].([]trpcmodel.Message)
		lastResp := resolveFallbackLastResp(state)
		interruptKey := buildFallbackInterruptKey(state, lastResp)

		// 上一步没有产生助手回复，需要进行emit事件封装，生成AGUI消息
		if shouldEmitFallbackResponse(state, interruptKey, messages, lastResp) {
			emitFallbackMessage(ctx, state, "fallback", lastResp)
		}

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

// resolveFallbackLastResp 计算当前 fallback 轮次应使用的回复文本。
// 优先顺序：消息尾部 assistant 回复（LLM→fallback 路径）→ StateKeyLastResponse → 意图兜底文案。
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

// shouldEmitFallbackResponse 判断 fallback 节点是否需要 emit model execution event。
// 在 interrupt resume 重放、或消息尾部已存在 assistant 回复时，跳过 emit 防止重复输出。
func shouldEmitFallbackResponse(state graph.State, interruptKey string, messages []trpcmodel.Message,
	lastResp string) bool {

	// 查看是否是resume重放，不能单靠下面最后一条assistant回复来判断，因为触发中断的时候，delta并不会被更新到message中
	// 这里通过检查： 1. ResumeChannel 是否有值； 2. ResumeMap 是否有值判断是否为中断恢复
	if graph.HasResumeValue(state, interruptKey) {
		return false
	}

	if len(messages) > 0 && messages[len(messages)-1].Role == trpcmodel.RoleAssistant {
		return false
	}
	return lastResp != ""
}

// hasAssistantTailWithContent 判断消息尾部是否已存在指定内容的 assistant 回复。
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
	clearingUnsupported := intentStr != "" && !intentType.IsSupportedScene()

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

// buildFallbackInterruptKey 为 fallback 节点生成稳定的 interrupt key。
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

// resolveStaticPrompt 返回传给 AddLLMNode 的系统提示词。
// 优先使用 Store 中的内容（来自本地文件或 BKAIDev 同步），
// 仅在 Store 暂无 system prompt 时回退到 cc 原始配置。
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

// unsupportedIntentFallbackMessage 在未调用 llm 时返回面向用户的兜底回复。
func unsupportedIntentFallbackMessage(state graph.State) string {
	intentStr, _ := state[constant.StateKeyIntent].(string)
	switch enumor.IntentType(intentStr) {
	case enumor.IntentTypeResourceQuery, enumor.IntentTypeChat:
		return "目前AI助手仅支持主机申领相关能力，其他云资源管理功能即将上线，如需要申领主机，请直接描述您的配置需求。"
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
