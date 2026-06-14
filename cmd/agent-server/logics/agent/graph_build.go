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
	"encoding/json"
	"fmt"
	"strings"

	cvmapply "hcm/cmd/agent-server/logics/agent/cvm_apply"
	"hcm/cmd/agent-server/logics/agent/hitl"
	"hcm/cmd/agent-server/logics/agent/intent"
	"hcm/cmd/agent-server/logics/agent/message"
	"hcm/cmd/agent-server/logics/agent/toolgate"
	"hcm/cmd/agent-server/logics/logger"
	"hcm/cmd/agent-server/logics/model"
	"hcm/cmd/agent-server/logics/prompt"
	"hcm/cmd/agent-server/logics/skill"
	"hcm/cmd/agent-server/logics/timer"
	agenttool "hcm/cmd/agent-server/logics/tool"
	"hcm/cmd/agent-server/logics/toolproxy"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/graphagent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
	toolskill "trpc.group/trpc-go/trpc-agent-go/tool/skill"
)

// BuildGraph constructs a ReAct graph topology with scene_dispatch as the single routing hub:
//
//	START → scene_dispatch → ConditionalEdge
//	  ├─ supported session_tag (host_apply)        → host_apply
//	  ├─ this-turn intent recognised but unsupported → fallback
//	  └─ no tag / no intent this turn               → intent_recognition → scene_dispatch
//	  ├─ host_apply → llm → ConditionalEdge(by tool_calls)
//	  │     ├─ human_confirm → hitl → llm (loop back)
//	  │     ├─ other_tool_calls → tool → llm (loop back)
//	  │     └─ no tool_calls → fallback (interrupt) → llm (session stays on host_apply)
//	  ├─ resource_query → resource_query(subgraph) → fallback (interrupt) → resource_query
//	fallback (interrupt) → scene_dispatch (re-dispatch; this-turn intent always cleared)
//
// The scene_dispatch node is the single routing brain: it commits a recognised supported
// intent into StateKeySessionTag and decides the next hop.
// The intent_recognition node only classifies user intent (writes StateKeyIntent) and returns to scene_dispatch.
// The llm node drives the ReAct loop for host_apply.
// The hitl node handles human-in-the-loop interrupts when LLM calls human_confirm.
// The tool node executes MCP and skill tools.
// The resource_query subgraph node handles the resource query ReAct sub-flow.
// The fallback node normalizes the LLM response and interrupts to wait for the next user message.
//
// The saver parameter is required to configure checkpoint support on the resource_query sub-agent,
// which is necessary for nested interrupt/resume when the subgraph's hitl node triggers.
func BuildGraph(mdl trpcmodel.Model, skillRepo skillpkg.Repository, toolset *agenttool.MCPToolSet,
	proxies *toolproxy.ToolProxies, agentName string, modelCfg cc.AgentModelGeneralConfig,
	promptStore *prompt.Store, clientSet *client.ClientSet, saver graph.CheckpointSaver) (
	*graph.Graph, []trpcagent.Agent, error) {

	schema := graph.MessagesStateSchema()
	stateGraph := graph.NewStateGraph(schema)

	staticPrompt := resolveStaticPrompt(promptStore)
	llmOpts := genLLMNodeOptions(toolset, proxies.SceneProxy(enumor.IntentTypeHostApply), modelCfg)

	// 主图 LLM 回调：默认场景（host_apply）的系统提示词
	modelCb := buildModelCallbacks(promptStore, skillRepo, agentName, "")
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
	stateGraph.AddLLMNode(string(enumor.CvmApplyNodeLLM), mdl, staticPrompt, skillTools, llmOpts...)

	// 4. HITL Node: the unified human-in-the-loop interrupt node. It handles every tool call that
	// has a registered handler: LLM-initiated human_confirm questions and pre-execution confirm
	// gates of real tools (e.g. create_biz_apply). Gates are filtered by the confirm-gate config.
	hitlReg := hitl.NewRegistry()
	hitlReg.Register(hitl.NewHumanConfirmHandler())
	// 注册需要进行门禁中断的工具handler
	for _, h := range toolgate.GetEnabledGateHandlers(cc.AgentServer().Tools.ConfirmGate, clientSet) {
		hitlReg.Register(h)
	}
	stateGraph.AddNode("hitl", hitl.GetNode(hitlReg))

	// 5. Tool Node: executes tools when the LLM requests them.
	toolsOpts := genToolNodeOptions(toolset, proxies.SceneProxy(enumor.IntentTypeHostApply), agentName)
	stateGraph.AddToolsNode(string(enumor.CvmApplyNodeTool), skillTools, toolsOpts...)

	// 6. Fallback Node: delivers LLM response, interrupts, and routes the next user message.
	stateGraph.AddNode("fallback", makeFallbackNode())

	// 6. Resource Query Subgraph: build and register as a subgraph node.
	rqSubgraph, err := buildResourceQuerySubgraph(mdl, skillRepo, toolset,
		proxies.SceneProxy(enumor.IntentTypeResourceQuery), agentName,
		modelCfg, promptStore, clientSet)
	if err != nil {
		return nil, nil, fmt.Errorf("build resource_query subgraph: %w", err)
	}
	rqSubAgent, err := graphagent.New(
		string(enumor.ResourceQueryGraphNode),
		rqSubgraph,
		graphagent.WithDescription("HCM resource query ReAct subgraph agent"),
		graphagent.WithCheckpointSaver(saver),
		graphagent.WithInitialState(graph.State{}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create resource_query sub-agent: %w", err)
	}
	subAgents := []trpcagent.Agent{rqSubAgent}

	// Register resource_query as a subgraph node.
	// WithSubgraphInputMapper 给子图按轮次分配独立 checkpoint namespace，避免下一轮复用上一轮
	// 已完成（NextNodes=[__end__]）的子图 checkpoint 而直接 resume 到结束、子图什么都不做。
	// WithSubgraphOutputMapper merges the subgraph's final messages back into the main graph,
	// so the fallback node can read the subgraph's assistant reply via resolveFallbackLastResp.
	stateGraph.AddSubgraphNode(string(enumor.ResourceQueryGraphNode),
		graph.WithSubgraphInputMapper(makeResourceQueryInputMapper()),
		graph.WithSubgraphOutputMapper(makeResourceQueryOutputMapper()),
	)

	// 7. Account Select Node: queries biz accounts and auto-selects or triggers HITL.
	stateGraph.AddNode(string(enumor.CvmApplyNodeAccountSelect), cvmapply.NewAccountSelectNode(clientSet.CloudServer()))

	// Entry point: every new run starts with intent recognition.
	stateGraph.SetEntryPoint("scene_dispatch")

	// scene_dispatch → account_select (supported tag/intent) / fallback (unsupported tag/intent) /
	// intent_recognition (no tag, no intent)
	stateGraph.AddConditionalEdges("scene_dispatch", makeSceneDispatchRoutingFunc(), map[string]string{
		"account_select":     string(enumor.CvmApplyNodeAccountSelect),
		"resource_query":     "resource_query",
		"fallback":           "fallback",
		"intent_recognition": "intent_recognition",
	})

	// intent_recognition always returns to scene_dispatch for the routing decision.
	stateGraph.AddEdge("intent_recognition", "scene_dispatch")

	// Conditional edges from account_select: count==0 → fallback, else → llm.
	stateGraph.AddConditionalEdges(string(enumor.CvmApplyNodeAccountSelect), makeAccountSelectRoutingFunc(),
		map[string]string{
			string(enumor.CvmApplyNodeFallback): string(enumor.CvmApplyNodeFallback),
			string(enumor.CvmApplyNodeLLM):      string(enumor.CvmApplyNodeLLM),
		})

	// llm → hitl / tool / fallback based on tool_calls in the last message
	stateGraph.AddConditionalEdges(string(enumor.CvmApplyNodeLLM), makeRoutingFunc(hitlReg), map[string]string{
		"hitl":                          "hitl",
		string(enumor.CvmApplyNodeTool): string(enumor.CvmApplyNodeTool),
		"fallback":                      "fallback",
	})

	// hitl → tool (confirmed gate) / llm (question answered or cancelled) based on the resume decision.
	stateGraph.AddConditionalEdges("hitl", hitl.MakeRoutingFunc(), map[string]string{
		string(enumor.CvmApplyNodeTool): string(enumor.CvmApplyNodeTool),
		string(enumor.CvmApplyNodeLLM):  string(enumor.CvmApplyNodeLLM),
	})

	// tool loops back to llm to continue the current task.
	stateGraph.AddEdge(string(enumor.CvmApplyNodeTool), string(enumor.CvmApplyNodeLLM))

	// After resource_query subgraph finishes (no tool_calls), enter main fallback
	// to deliver the reply and interrupt waiting for the next user message.
	stateGraph.AddEdge("resource_query", "fallback")

	// fallback always returns to scene_dispatch so the unified routing hub handles
	// the next user message regardless of whether the session already has a tag.
	stateGraph.AddEdge("fallback", "scene_dispatch")

	compiledGraph, err := stateGraph.Compile()
	if err != nil {
		return nil, nil, err
	}
	return compiledGraph, subAgents, nil
}

// makeResourceQueryInputMapper 返回 resource_query 子图的输入映射函数，给子图按「轮次」分配
// 独立的 checkpoint namespace。
//
// 默认情况下框架以子 agent 名作为子图 checkpoint namespace，而父图 lineage 在整轮会话内稳定。
// 子图正常结束后会留下一个「已完成」(NextNodes=[__end__]) 的 checkpoint；下一轮再次进入子图时，
// 子图 executor 以空 checkpoint_id 取该 (lineage, namespace) 的最新 checkpoint，命中的正是上一轮
// 的完成态，于是直接 resume 到 __end__、子图什么都不做。
//
// 这里以「进入子图时的消息条数」区分轮次（会话内单调递增），使每轮对应一个全新 namespace、
// 找不到旧的完成态 checkpoint，从而每轮都从入口节点重新执行。轮内若触发 hitl 中断/恢复，框架会用
// 中断时记录的 namespace 覆盖（applyCheckpointResumeFields），不影响同一轮内的中断恢复。
//
// 注意：提供 InputMapper 后框架不再执行默认的 copyRuntimeStateFiltered，需要在此复刻其
// 「过滤内部/临时 state key」的行为，否则会把 exec_context、callbacks 等不可传播的 key 带入子图。
func makeResourceQueryInputMapper() graph.SubgraphInputMapper {
	return func(parent graph.State) graph.State {
		child := make(graph.State, len(parent))
		for k, v := range parent {
			if isFrameworkInternalStateKey(k) {
				continue
			}
			child[k] = v
		}
		// 对齐框架默认行为：进入子图前清掉父图残留的 checkpoint_id，避免子图误用父图 checkpoint。
		delete(child, graph.CfgKeyCheckpointID)
		// 按轮次分配独立 namespace。
		msgs, _ := parent[graph.StateKeyMessages].([]trpcmodel.Message)
		child[graph.CfgKeyCheckpointNS] = fmt.Sprintf("%s_%d", enumor.ResourceQueryGraphNode, len(msgs))
		return child
	}
}

// isFrameworkInternalStateKey 复刻 trpc-agent-go 的 copyRuntimeStateFiltered/isInternalStateKey 行为：
// 内部/临时 state key 不应传播给子图。框架内部 key 分两类：
//   - 以 "_" 开头的（__command__、__resume_map__、各类 _xxx_metadata、__current_trace_step_id__ 等），
//     遵循框架统一命名约定，用前缀判断可同时覆盖未来新增的同类 key；
//   - 少数无前缀的不可序列化/会话级 key（exec_context、parent_agent、各类 callbacks、
//     current_node_id、session），逐一列出（均为框架导出常量）。
//
// 版本升级时如框架新增「无 "_" 前缀」的内部 key，需同步此列表。
func isFrameworkInternalStateKey(key string) bool {
	if strings.HasPrefix(key, "_") {
		return true
	}
	switch key {
	case graph.StateKeyExecContext,
		graph.StateKeyParentAgent,
		graph.StateKeyNodeCallbacks,
		graph.StateKeyToolCallbacks,
		graph.StateKeyModelCallbacks,
		graph.StateKeyAgentCallbacks,
		graph.StateKeyCurrentNodeID,
		graph.StateKeySession:
		return true
	default:
		return false
	}
}

// makeResourceQueryOutputMapper 返回 resource_query 子图的输出映射函数，
// 负责把子图最终的 messages 合并回主图，使主图 fallback 能读到子图的 assistant 回复。
//
// 注意：r.FinalState 是框架对子图完成事件 StateDelta 做 JSON 解码重建出来的，
// StateKeyMessages 实际类型为 []interface{}（元素为 map[string]interface{}），无法直接断言为
// []trpcmodel.Message。原始字节保存在 r.RawStateDelta 中，用 json.Unmarshal 还原为 []trpcmodel.Message。
func makeResourceQueryOutputMapper() graph.SubgraphOutputMapper {
	const logPrefix = "[rq subgraph output mapper]"
	return func(_ graph.State, r graph.SubgraphResult) graph.State {
		// 从 RawStateDelta 的原始 JSON 字节解码出完整 messages。
		raw, exist := r.RawStateDelta[graph.StateKeyMessages]
		if !exist {
			logs.Warnf("%s RawStateDelta has no %s key, give up merge", logPrefix, graph.StateKeyMessages)
			return nil
		}
		var decoded []trpcmodel.Message
		if err := json.Unmarshal(raw, &decoded); err != nil {
			logs.Errorf("%s decode RawStateDelta[messages] failed, err: %v, raw: %s",
				logPrefix, err, string(raw))
			return nil
		}
		if len(decoded) == 0 {
			logs.Warnf("%s decoded messages from RawStateDelta is empty, give up merge", logPrefix)
			return nil
		}
		logs.Infof("%s decoded %d messages from RawStateDelta, last role=%s, last content len=%d",
			logPrefix, len(decoded), decoded[len(decoded)-1].Role, len(decoded[len(decoded)-1].Content))
		return graph.State{graph.StateKeyMessages: decoded}
	}
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

// sceneNodeTarget 将受支持的场景标签映射到主图中对应的入口节点。
func sceneNodeTarget(scene enumor.IntentType) string {
	switch scene {
	case enumor.IntentTypeResourceQuery:
		return "resource_query"
	default:
		// host_apply 及其它默认进入主 ReAct account_select 节点的场景
		return "account_select"
	}
}

// makeSceneDispatchRoutingFunc 根据会话标签与本轮意图决定 scene_dispatch 的后续路由。
func makeSceneDispatchRoutingFunc() func(ctx context.Context, state graph.State) (string, error) {
	return func(ctx context.Context, state graph.State) (string, error) {
		rid := rest.RidFromContext(ctx)
		tag, _ := state[constant.StateKeySessionTag].(enumor.IntentType)
		logs.Infof("scene dispatch routing: graph state session_tag=%s, type is %T, rid: %s", tag, tag, rid)
		if tag.IsSupportedScene() {
			target := sceneNodeTarget(tag)
			logs.Infof("[scene dispatch routing] session_tag=%s, route to %s, rid: %s", tag, target, rid)
			return target, nil
		}

		// 本轮意图非空且不受支持：直接路由到 fallback 给出拒识回复，
		// 避免 intent_recognition 与 scene_dispatch 之间反复循环。
		if intentStr, _ := state[constant.StateKeyIntent].(string); intentStr != "" {
			intentType := enumor.IntentType(intentStr)
			if !intentType.IsSupportedScene() {
				logs.Infof("[scene dispatch routing] intent=%s unsupported, route to fallback, rid: %s",
					intentType, rid)
				return "fallback", nil
			}
		}

		logs.Infof("[scene dispatch routing] no tag/intent, route to intent_recognition, rid: %s", rid)
		return "intent_recognition", nil
	}
}

// makeAccountSelectRoutingFunc returns the conditional edge routing function for account_select.
// It reads the AccountSelectNextNodeKey written by the account_select node and maps it to a
// destination node name. Defaults to "fallback" if the key is absent.
func makeAccountSelectRoutingFunc() func(ctx context.Context, state graph.State) (string, error) {
	return func(ctx context.Context, state graph.State) (string, error) {
		rid := rest.RidFromContext(ctx)
		next, _ := state[constant.AccountSelectNextNodeKey].(enumor.CvmApplyNode)
		switch next {
		case enumor.CvmApplyNodeLLM:
			logs.Infof("account_select routing: → llm, rid: %s", rid)
			return string(enumor.CvmApplyNodeLLM), nil
		default:
			logs.Infof("account_select routing: → fallback (next=%q), rid: %s", next, rid)
			return string(enumor.CvmApplyNodeFallback), nil
		}
	}
}

// makeRoutingFunc 返回 llm 节点的条件边路由函数。
// 它依据最后一条消息中的 tool_calls 进行路由：
//   - 无 tool_calls → "fallback"
//   - 由 hitl 注册表处理的工具调用（human_confirm 或确认门禁）→ "hitl"；
//     此类工具必须是该批次中唯一的 tool call，否则返回错误。
//   - 其他工具 → CvmApplyNodeTool
func makeRoutingFunc(hitlReg *hitl.Registry) func(ctx context.Context, state graph.State) (string, error) {
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

		hasHITL := false
		execToolsName := ""
		for i := range lastMsg.ToolCalls {
			tc := &lastMsg.ToolCalls[i]
			// 真实 MCP 工具经 proxy execute_tool 调用，需解出真实工具名再匹配门禁。
			resolvedName := toolproxy.ResolveToolCall(tc).Name
			logs.Infof("routing: toolCall name=%s, resolved=%s, id=%s, rid: %s",
				tc.Function.Name, resolvedName, tc.ID, rid)

			// hitl注册过该工具的handler则路由到人机中断节点
			if hitlReg.Has(resolvedName) {
				hasHITL = true
				execToolsName = resolvedName
				break
			}
		}

		// 需要人机中断的工具（human_confirm / 确认门禁）必须单独成批，不能与其他工具混批。
		if hasHITL {
			if len(lastMsg.ToolCalls) > 1 {
				return "", fmt.Errorf("invalid tool calls: tool %s is human interaction tool, "+
					"must be the only tool call in one batch", execToolsName)
			}
			logs.Infof("routing: human interaction tool, route to hitl, rid: %s", rid)
			return "hitl", nil
		}

		logs.Infof("routing: other tool calls, route to tool, rid: %s", rid)
		return string(enumor.CvmApplyNodeTool), nil
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
		message.EmitFallbackMessage(ctx, messages, state, interruptKey, "fallback", lastResp)

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

		return message.BuildFallbackResumeDelta(ctx, state, messages, lastResp, userInput), nil
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
	// account_select 因当前业务无可用云账号路由到 fallback 时，返回无权限提示文案
	if next, _ := state[constant.AccountSelectNextNodeKey].(enumor.CvmApplyNode); next == enumor.CvmApplyNodeFallback {
		return constant.NoPermissionFallbackMessage
	}
	return unsupportedIntentFallbackMessage(state)
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

// buildModelCallbacks assembles the BeforeModel/AfterModel callbacks shared by the main graph
// llm node and scene subgraphs. The scene parameter selects which scene's system prompt the
// replace callback injects (empty scene = default/host_apply prompt). Each node needs its own
// callbacks so the system prompt replace does not clobber a scene's static prompt.
func buildModelCallbacks(promptStore *prompt.Store, skillRepo skillpkg.Repository,
	agentName, scene string) *trpcmodel.Callbacks {

	modelCb := trpcmodel.NewCallbacks()
	modelCb.AfterModel = append(modelCb.AfterModel, logger.MakeModelLoggerCallback())
	// 历史工具调用结果优化，减少 LLM 上下文长度
	modelCb.BeforeModel = append(modelCb.BeforeModel, model.MakeHistoricalToolResultFilter())
	// 远程 prompt 注入系统提示词（按场景，优先于其他要入System的提示词，如skill,time）
	if promptStore != nil {
		modelCb.BeforeModel = append(modelCb.BeforeModel,
			prompt.MakeSceneSystemPromptReplaceCallback(promptStore, scene))
	}
	// 技能上下文注入
	modelCb.BeforeModel = append(modelCb.BeforeModel, skill.MakeSkillInjectWithModelCallback(agentName, skillRepo))
	// 当前时间注入
	modelCb.BeforeModel = append(modelCb.BeforeModel, timer.MakeTimeInjectCallback())
	return modelCb
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

// buildResourceQuerySubgraph builds the resource_query ReAct subgraph.
// The subgraph contains only llm/hitl/tool nodes (no fallback).
// When the llm node produces no tool_calls it routes to graph.End, finishing the subgraph
// and returning control to the main graph's fallback node.
func buildResourceQuerySubgraph(mdl trpcmodel.Model, skillRepo skillpkg.Repository,
	toolset *agenttool.MCPToolSet, rqToolProxy *toolproxy.ToolProxy, agentName string,
	modelCfg cc.AgentModelGeneralConfig, promptStore *prompt.Store, clientSet *client.ClientSet) (
	*graph.Graph, error) {

	rqStaticPrompt := prompt.ResolveSceneStaticPrompt(promptStore, string(enumor.IntentTypeResourceQuery))
	rqLLMOpts := genLLMNodeOptions(toolset, rqToolProxy, modelCfg)
	// 子图 LLM 回调：resource_query 场景的系统提示词（避免被默认场景提示词覆盖）
	modelCb := buildModelCallbacks(promptStore, skillRepo, agentName, string(enumor.IntentTypeResourceQuery))
	rqLLMOpts = append(rqLLMOpts, graph.WithModelCallbacks(modelCb))

	// 构建 skill 工具集：skill 工具 + HITL 工具（声明性工具，路由到 hitl 节点）
	skillTools := make(map[string]trpctool.Tool)
	skillTools[constant.SkillLoadToolName] = toolskill.NewLoadTool(skillRepo)
	skillTools[constant.SkillListDocsToolName] = toolskill.NewListDocsTool(skillRepo)
	skillTools[constant.SkillSelectDocsToolName] = toolskill.NewSelectDocsTool(skillRepo)
	skillTools[constant.HumanConfirmToolName] = hitl.GetToolWrapper()

	rqToolsOpts := genToolNodeOptions(toolset, rqToolProxy, agentName)

	schema := graph.MessagesStateSchema()
	sg := graph.NewStateGraph(schema)

	sg.AddLLMNode("llm", mdl, rqStaticPrompt, skillTools, rqLLMOpts...)

	// 4. HITL Node: the unified human-in-the-loop interrupt node. It handles every tool call that
	// has a registered handler: LLM-initiated human_confirm questions and pre-execution confirm
	// gates of real tools (e.g. create_biz_apply). Gates are filtered by the confirm-gate config.
	hitlReg := hitl.NewRegistry()
	hitlReg.Register(hitl.NewHumanConfirmHandler())
	// 注册需要进行门禁中断的工具handler
	for _, h := range toolgate.GetEnabledGateHandlers(cc.AgentServer().Tools.ConfirmGate, clientSet) {
		hitlReg.Register(h)
	}
	sg.AddNode("hitl", hitl.GetNode(hitlReg))

	sg.AddToolsNode("tool", skillTools, rqToolsOpts...)

	sg.SetEntryPoint("llm")
	sg.SetFinishPoint("llm")

	// llm routing: human_confirm → hitl, other tools → tool, no tool_calls → graph.End (subgraph finish)
	sg.AddConditionalEdges("llm", makeSubgraphRoutingFunc(), map[string]string{
		"hitl":    "hitl",
		"tool":    "tool",
		graph.End: graph.End,
	})

	sg.AddEdge("hitl", "llm")
	sg.AddEdge("tool", "llm")

	return sg.Compile()
}

// makeSubgraphRoutingFunc is a variant of makeRoutingFunc for use inside the resource_query subgraph.
// When there are no tool_calls, it returns graph.End to finish the subgraph and return control
// to the main graph, instead of routing to a fallback node.
func makeSubgraphRoutingFunc() func(ctx context.Context, state graph.State) (string, error) {
	return func(ctx context.Context, state graph.State) (string, error) {
		rid := rest.RidFromContext(ctx)
		messages, _ := state[graph.StateKeyMessages].([]trpcmodel.Message)
		if len(messages) == 0 {
			return graph.End, nil
		}

		lastMsg := messages[len(messages)-1]

		if len(lastMsg.ToolCalls) == 0 {
			logs.Infof("[rq subgraph routing] no tool_calls, finish subgraph, rid: %s", rid)
			return graph.End, nil
		}

		hasHumanConfirm := false
		hasOtherTools := false
		for _, tc := range lastMsg.ToolCalls {
			if tc.Function.Name == constant.HumanConfirmToolName {
				hasHumanConfirm = true
			} else {
				hasOtherTools = true
			}
		}

		if hasHumanConfirm && hasOtherTools {
			return "", fmt.Errorf("rq subgraph: invalid tool calls: human_confirm cannot be combined with other tools")
		}

		if hasHumanConfirm {
			logs.Infof("[rq subgraph routing] only human_confirm, route to hitl, rid: %s", rid)
			return "hitl", nil
		}

		logs.Infof("[rq subgraph routing] other tool calls, route to tool, rid: %s", rid)
		return "tool", nil
	}
}

// unsupportedIntentFallbackMessage 在未调用 llm 时返回面向用户的兜底回复。
func unsupportedIntentFallbackMessage(state graph.State) string {
	intentStr, _ := state[constant.StateKeyIntent].(string)
	if !enumor.IntentType(intentStr).IsSupportedScene() {
		return "目前AI助手仅支持主机申领相关能力，其他云资源管理功能即将上线，如需要申领主机，请直接描述您的配置需求。"
	}
	return "抱歉，我暂时无法处理您的请求。"
}
