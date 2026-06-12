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

// extractDataMaxDepth bounds the recursion when unwrapping nested result/data envelopes.
const extractDataMaxDepth = 5

// recommendToolCall holds the resolved recommend tool call located in the message history.
type recommendToolCall struct {
	// toolName is the normalized business tool name (one of constant.Recommend*ToolName).
	toolName string
	// id is the tool call id used to match the corresponding tool result message.
	id string
	// limit is the requested recommend count, parsed from the tool call arguments.
	limit int
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
		case constant.RecommendByStaticToolName:
			return handleStaticRecommend(ctx, state, messages, call, result)
		case constant.RecommendByPlanToolName:
			return handlePlanRecommend(ctx, state, messages, result)
		case constant.RecommendSplitSuborderToolName:
			return handleSplitSuborderRecommend(ctx, state, messages, result)
		default:
			return graph.State{}, nil
		}
	}
}

// handleStaticRecommend 处理 by_static（离线偏好推荐）：覆盖写候选；方案数 >= limit 则中断，否则回 llm。
func handleStaticRecommend(ctx context.Context, state graph.State, messages []trpcmodel.Message,
	call *recommendToolCall, result string) (any, error) {

	rid := rest.RidFromContext(ctx)
	items := parseItems(result)
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
	merged := mergeCandidates(readCandidates(state), parseItems(result))
	logs.Infof("after_tool_hitl: plan merged candidates=%d, rid: %s", len(merged), rid)

	if len(merged) > 0 {
		return interruptWithRecommend(ctx, state, messages, merged)
	}
	return graph.State{constant.StateKeyRecommendCandidates: marshalCandidates(merged)}, nil
}

// handleSplitSuborderRecommend 处理 split_suborder（拆单试算）：出子单则构造主单+子单 payload 并中断，否则回 llm。
func handleSplitSuborderRecommend(ctx context.Context, state graph.State, messages []trpcmodel.Message,
	result string) (any, error) {

	rid := rest.RidFromContext(ctx)
	subs := parseSuborders(result)
	logs.Infof("after_tool_hitl: split suborders=%d, rid: %s", len(subs), rid)

	if len(subs) >= 1 {
		return interruptWithRecommendSuborders(ctx, state, messages, subs)
	}
	return graph.State{}, nil
}

// interruptWithRecommend emits a prompt, interrupts with the candidate recommendations payload, and
// on resume appends the user's choice and routes back to llm.
func interruptWithRecommend(ctx context.Context, state graph.State, messages []trpcmodel.Message,
	items []*woaserver.ApplyRecommendItem) (any, error) {

	lastResp := fmt.Sprintf("请确认所选方案信息，并提交申请单")
	payload := map[string]any{
		"recommendations": items,
	}
	return doInterrupt(ctx, state, messages, lastResp, constant.AfterToolHITLRecommendSelectInterruptKey, payload)
}

// interruptWithRecommendSuborders emits a prompt, interrupts with the main-order + suborders payload, and on
// resume appends the user's choice and routes back to llm.
func interruptWithRecommendSuborders(ctx context.Context, state graph.State, messages []trpcmodel.Message,
	subs []*woaserver.ApplyRecommendSuborder) (any, error) {

	lastResp := fmt.Sprintf("方案已解析，请确认以下参数后提交")
	payload := map[string]any{
		"suborders": subs,
	}
	return doInterrupt(ctx, state, messages, lastResp,
		constant.AfterToolHITLRecommendSuborderConfirmInterruptKey, payload)
}

// doInterrupt performs the emit → interrupt → resume cycle shared by both interrupt branches.
func doInterrupt(ctx context.Context, state graph.State, messages []trpcmodel.Message,
	lastResp, baseKey string, payload map[string]any) (any, error) {

	rid := rest.RidFromContext(ctx)
	key := buildInterruptKey(state, baseKey)
	message.EmitFallbackMessage(ctx, messages, state, key, string(enumor.CvmApplyNodeAfterToolHITL), lastResp)

	resumeValue, err := graph.Interrupt(ctx, state, key, payload)
	if err != nil {
		logs.Infof("after_tool_hitl: interrupt triggered, rid: %s", rid)
		return nil, err
	}

	// 优先使用前端通过 forwardedProps 传入的结构化回复（与自由文本 resumeValue 区分）：
	// 命中时构造自定义事件携带该值，并以该值作为用户选择，忽略 resumeValue。
	if forwarded := resolveForwardedResumeValue(ctx); forwarded != "" {
		emitForwardedResumeEvent(ctx, state, forwarded)
		logs.Infof("after_tool_hitl: resume with forwarded value=%s, rid: %s", forwarded, rid)
		return message.BuildFallbackResumeDelta(ctx, state, messages, lastResp, forwarded), nil
	}

	choice, ok := resumeValue.(string)
	if !ok || choice == "" {
		logs.Errorf("after_tool_hitl: invalid resume value, expected non-empty string, got %T, rid: %s",
			resumeValue, rid)
		return nil, fmt.Errorf("after_tool_hitl: invalid resume value: expected non-empty string")
	}
	logs.Infof("after_tool_hitl: resume with user choice=%s, rid: %s", choice, rid)
	return message.BuildFallbackResumeDelta(ctx, state, messages, lastResp, choice), nil
}

// resolveForwardedResumeValue reads the structured resume value that the frontend passed via
// forwardedProps (stored in RuntimeState under StateKeyForwardedResumeValue). It returns an empty
// string when the invocation, RuntimeState, or value is absent or not a non-empty string.
func resolveForwardedResumeValue(ctx context.Context) string {
	rid := rest.RidFromContext(ctx)

	inv, ok := trpcagent.InvocationFromContext(ctx)
	if !ok || inv == nil || inv.RunOptions.RuntimeState == nil {
		return ""
	}

	raw := inv.RunOptions.RuntimeState[constant.StateKeyForwardedResumeValue]
	forwarded, ok := raw.(string)
	if !ok || forwarded == "" {
		return ""
	}

	logs.Infof("after_tool_hitl: forwarded resume value=%s, rid: %s", forwarded, rid)
	return forwarded
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
