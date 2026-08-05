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

// Package aftertool provides the generic after_tool_hitl graph node, which interrupts the graph
// after a tool call completes to bring a human into the loop (HITL). It dispatches by tool name to
// scenario-specific handlers and is designed to host any "interrupt after tool" scenario.
package aftertool

import (
	"context"
	"encoding/json"
	"fmt"

	"hcm/cmd/agent-server/logics/agent/message"
	woaserver "hcm/pkg/api/woa-server"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

// recommendToolCall holds the resolved recommend tool call located in the message history.
type recommendToolCall struct {
	// toolName is the normalized business tool name (one of enumor.ToolNameRecommend*).
	toolName enumor.ToolName
	// id is the tool call id used to match the corresponding tool result message.
	id string
	// limit is the requested recommend count, parsed from the tool call arguments.
	limit int
	// args is the tool call arguments, parsed from the tool call.
	args json.RawMessage
}

// MakeRoutingFunc returns the conditional edge routing function for the tool node.
// 当本轮调用的工具需要"走工具后中断"时路由到 after_tool_hitl，否则回 llm 保持原行为。
// 目前的判定条件是推荐工具（by_static / by_plan / split_suborder），后续可在此扩展其他场景。
func MakeRoutingFunc() func(ctx context.Context, state graph.State) (string, error) {
	return func(ctx context.Context, state graph.State) (string, error) {
		rid := rest.RidFromContext(ctx)
		messages, _ := state[graph.StateKeyMessages].([]trpcmodel.Message)
		if call := findRecommendToolCall(messages); call != nil {
			logs.Infof("after_tool routing: tool=%s, route to after_tool_hitl, rid: %s", call.toolName, rid)
			return string(enumor.CvmApplyNodeAfterToolHITL), nil
		}
		return "llm", nil
	}
}

// GetNode returns the after_tool_hitl node function.
// 它定位触发中断的工具调用，按工具类型分发到对应场景的 handler，由 handler 做确定性判定：
// 满足条件则构造 payload 并中断，否则回 llm。当前覆盖推荐场景，新增场景在 switch 中扩展分支即可。
func GetNode() graph.NodeFunc {
	return func(ctx context.Context, state graph.State) (any, error) {
		rid := rest.RidFromContext(ctx)
		messages, _ := state[graph.StateKeyMessages].([]trpcmodel.Message)
		call := findRecommendToolCall(messages)
		if call == nil {
			logs.Warnf("after_tool_hitl: no recommend tool call found, route to llm, rid: %s", rid)
			return graph.State{}, nil
		}

		result := findToolResult(messages, call.id)
		switch call.toolName {
		case enumor.ToolNameRecommendByStatic:
			return handleStaticRecommend(ctx, state, messages, call, result)
		case enumor.ToolNameRecommendByPlan:
			return handlePlanRecommend(ctx, state, messages, result)
		case enumor.ToolNameRecommendSplitSuborder:
			return handleSplitSuborderRecommend(ctx, state, messages, call, result)
		default:
			return graph.State{}, nil
		}
	}
}

// handleStaticRecommend 处理 by_static（离线偏好推荐）：覆盖写候选；方案数 >= limit 则中断，否则回 llm。
func handleStaticRecommend(ctx context.Context, state graph.State, messages []trpcmodel.Message,
	call *recommendToolCall, result string) (any, error) {

	rid := rest.RidFromContext(ctx)
	items := parseItems(result, rid)
	logs.Infof("after_tool_hitl: static items=%d, limit=%d, rid: %s", len(items), call.limit, rid)

	if len(items) >= call.limit {
		return interruptWithRecommend(ctx, state, messages, items)
	}
	return graph.State{constant.StateKeyRecommendCandidates: marshalCandidates(items)}, nil
}

// handlePlanRecommend 处理 by_plan（预测余量推荐）：与 by_static 候选合并后有方案则中断，否则回 llm。
func handlePlanRecommend(ctx context.Context, state graph.State, messages []trpcmodel.Message,
	result string) (any, error) {

	rid := rest.RidFromContext(ctx)
	merged := mergeCandidates(readCandidates(state), parseItems(result, rid))
	logs.Infof("after_tool_hitl: plan merged candidates=%d, rid: %s", len(merged), rid)

	if len(merged) > 0 {
		return interruptWithRecommend(ctx, state, messages, merged)
	}
	return graph.State{constant.StateKeyRecommendCandidates: marshalCandidates(merged)}, nil
}

// handleSplitSuborderRecommend 处理 split_suborder（拆单试算）：出增量子单则构造主单+子单 payload 并中断，
// 否则回 llm。
//
// 拆单试算接口在增量拆分场景下只返回本次新算出的增量子单、不回显入参的 occupied_suborders，
// 因此确认卡片所需的完整清单需把入参中的已占用子单还原后合并，否则「已有 S2 再加一条 S3」时
// 卡片只剩 S3，此前已确认的子单被静默丢弃。
//
// 中断判定仍只看增量子单数：若改用合并后总数，本次增量为 0（余量/库存不足）时会因历史占用
// 把总数撑到 >= 1，从而弹出一张没有任何新内容的确认卡片。
func handleSplitSuborderRecommend(ctx context.Context, state graph.State, messages []trpcmodel.Message,
	call *recommendToolCall, result string) (any, error) {

	rid := rest.RidFromContext(ctx)
	subs := parseSuborders(result, rid)
	if len(subs) < 1 {
		logs.Infof("after_tool_hitl: split suborders=0, route to llm, rid: %s", rid)
		return graph.State{}, nil
	}

	merged := mergeSuborders(extractOccupiedSuborders(call.args, rid), subs)
	logs.Infof("after_tool_hitl: split suborders=%d, merged=%d, rid: %s", len(subs), len(merged), rid)
	return interruptWithRecommendSuborders(ctx, state, messages, merged)
}

// interruptWithRecommend emits a prompt, interrupts with the candidate recommendations payload, and
// on resume appends the user's choice and routes back to llm.
func interruptWithRecommend(ctx context.Context, state graph.State, messages []trpcmodel.Message,
	items []*woaserver.ApplyRecommendItem) (any, error) {

	lastResp := "请确认所选方案信息，并提交申请单"
	payload := map[string]any{
		"recommendations": items,
	}
	return doInterrupt(ctx, state, messages, lastResp, constant.AfterToolHITLRecommendSelectInterruptKey, payload)
}

// interruptWithRecommendSuborders emits a prompt, interrupts with the main-order + suborders payload, and on
// resume appends the user's choice and routes back to llm.
func interruptWithRecommendSuborders(ctx context.Context, state graph.State, messages []trpcmodel.Message,
	subs []*woaserver.ApplyRecommendSuborder) (any, error) {

	lastResp := "方案已解析，请确认以下参数后提交"
	payload := map[string]any{
		"suborders": subs,
	}
	return doInterrupt(ctx, state, messages, lastResp,
		constant.AfterToolHITLRecommendSuborderConfirmInterruptKey, payload)
}

// doInterrupt performs the emit → interrupt → resume cycle shared by both interrupt branches.
//
// 恢复来源区分（command=自由输入 / forwarded=结构化协议）：
//   - 优先读 inv.RunOptions.RuntimeState[StateKeyForwardedResumeValue]（前端结构化协议，由
//     service.go: tryPrepareAutoResume 每轮重建进 RuntimeState，请求级、不脏），命中则视为前端
//     结构化选择（选方案/选账号等），发出 after_tool_hitl.resume_forwarded 事件并以该值作为用户选择；
//   - 未命中则回退 graph.Interrupt 返回的 resumeValue（用户在输入框输入的自由文本），校验为非空字符串。
//
// 不再依赖「JSON 嗅探」区分结构化 vs 自由文本：自由文本即使是合法 JSON 也走 resumeValue 通道，
// 不再误发 resume_forwarded 事件。该 key 由 makeSubgraphInputMapper 在子图入口剥键，不进入子图持久化
// state，故恢复时直接读 RuntimeState（每 Run 重建）即可，无需消费后清除。
func doInterrupt(ctx context.Context, state graph.State, messages []trpcmodel.Message,
	lastResp, baseKey string, payload map[string]any) (any, error) {

	rid := rest.RidFromContext(ctx)
	key := buildInterruptKey(state, baseKey)
	message.EmitFallbackMessage(ctx, messages, state, key, string(enumor.CvmApplyNodeAfterToolHITL), lastResp)

	resumeValue, err := graph.Interrupt(ctx, state, key, payload)
	if err != nil {
		logs.Errorf("after_tool_hitl: interrupt triggered, err: %v, rid: %s", err, rid)
		return nil, err
	}

	// 用户看到的推荐方案并不在模型对话上下文中，这里将其拼到助手消息里注入，
	// 让模型理解 resume 后用户输入（如「选第几个方案」）所针对的具体方案。
	// 注意 emit 已用原始 lastResp，此处仅用于注入模型上下文，不改变前端展示。
	respWithOptions := lastResp
	if optionsMsg := buildRecommendOptionsMessage(ctx, payload); optionsMsg != "" {
		respWithOptions = lastResp + "\n" + optionsMsg
	}

	// 优先使用前端通过 forwardedProps 传入的结构化回复（与自由文本 resumeValue 区分）：
	// 命中时构造自定义事件携带该值，并以该值作为用户选择，忽略 resumeValue。
	if forwarded := resolveForwardedResumeValue(ctx); forwarded != "" {
		emitForwardedResumeEvent(ctx, state, forwarded)
		logs.Infof("after_tool_hitl: resume with forwarded value=%s, rid: %s", forwarded, rid)
		return message.BuildFallbackResumeDelta(ctx, state, messages, respWithOptions, forwarded), nil
	}

	// 回退自由文本：graph.Interrupt 返回的 resumeValue，对应用户在输入框输入的本轮内容。
	choice, ok := resumeValue.(string)
	if !ok || choice == "" {
		logs.Errorf("after_tool_hitl: invalid resume value, expected non-empty string, got %T, rid: %s",
			resumeValue, rid)
		return nil, fmt.Errorf("after_tool_hitl: invalid resume value: expected non-empty string")
	}
	logs.Infof("after_tool_hitl: resume with user choice=%s, rid: %s", choice, rid)
	return message.BuildFallbackResumeDelta(ctx, state, messages, respWithOptions, choice), nil
}

// buildRecommendOptionsMessage 将中断 payload（含 recommendations 或 suborders）序列化为文本，
// 作为助手消息内容注入模型上下文，使 resume 后的对话历史反映用户实际看到的推荐方案。
func buildRecommendOptionsMessage(ctx context.Context, payload map[string]any) string {
	if len(payload) == 0 {
		return ""
	}
	result, err := json.Marshal(payload)
	if err != nil {
		logs.Warnf("after_tool_hitl: marshal recommend options failed, err: %v, rid: %s", err, rest.RidFromContext(ctx))
		return ""
	}
	return string(result)
}

// resolveForwardedResumeValue reads the structured resume value passed by the frontend via
// forwardedProps from RuntimeState. Returns an empty string when the value is absent or not a
// non-empty string. The value is rebuilt into RuntimeState (via tryPrepareAutoResume) on every
// resume Run, so reading it here is safe and not subject to stale checkpoint state.
func resolveForwardedResumeValue(ctx context.Context) string {
	rid := rest.RidFromContext(ctx)

	inv, ok := trpcagent.InvocationFromContext(ctx)
	if !ok || inv == nil || inv.RunOptions.RuntimeState == nil {
		return ""
	}

	raw := inv.RunOptions.RuntimeState[constant.StateKeyForwardedResumeValue]
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		if v == "" {
			return ""
		}
		logs.Infof("after_tool_hitl: forwarded resume value=%s, rid: %s", v, rid)
		return v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			logs.Warnf("after_tool_hitl: marshal forwarded resume value failed, err: %v, rid: %s", err, rid)
			return ""
		}
		logs.Infof("after_tool_hitl: forwarded resume value=%s, rid: %s", string(b), rid)
		return string(b)
	}
}

// emitForwardedResumeEvent emits a custom event carrying the structured forwarded resume value,
// so the frontend can observe the value that drove the resume.
func emitForwardedResumeEvent(ctx context.Context, state graph.State, forwarded string) {
	rid := rest.RidFromContext(ctx)
	emitter := graph.GetEventEmitterWithContext(ctx, state)
	payload := map[string]any{"value": forwarded}
	if err := emitter.EmitCustom(constant.AfterToolHITLResumeForwardedEventName, payload); err != nil {
		logs.Warnf("after_tool_hitl: emit forwarded resume event failed, err: %v, rid: %s", err, rid)
	}
}

// buildInterruptKey returns a unique interrupt key by appending the current message count, mirroring
// the pattern used by account_select to avoid stale checkpoint conflicts.
func buildInterruptKey(state graph.State, baseKey string) string {
	msgs, _ := state[graph.StateKeyMessages].([]trpcmodel.Message)
	return fmt.Sprintf("%s%s%d", baseKey, constant.InterruptKeySeparator, len(msgs))
}
