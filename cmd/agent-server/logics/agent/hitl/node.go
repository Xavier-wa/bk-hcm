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

package hitl

import (
	"context"
	"errors"
	"fmt"

	"hcm/cmd/agent-server/logics/toolproxy"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// makeHITLNode 返回由给定 handler 注册表驱动的统一 HITL 节点。
//
// 执行流程：
//  1. 首次调用：定位由已注册 Handler 处理的工具调用，构造其中断 payload，并以
//     "<eventKind>:<tool>:<callID>" 为 key 调用 graph.Interrupt。translator 据中断值发出对应的
//     自定义事件（hitl.interrupt / tool.confirm）。
//  2. 恢复调用：graph.Interrupt 返回用户的 resume 值；handler 将其转换为 ResumeResult，
//     决定下一节点（tool/llm）与消息增量。
func makeHITLNode(reg *Registry) graph.NodeFunc {
	return func(ctx context.Context, state graph.State) (any, error) {
		rid := rest.RidFromContext(ctx)
		messages, _ := state[graph.StateKeyMessages].([]model.Message)
		logs.Infof("hitl node: start, total messages=%d, rid: %s", len(messages), rid)

		tc := extractHandledToolCall(messages, reg)
		if tc == nil {
			logs.Errorf("hitl node: no handled tool call found, rid: %s", rid)
			return nil, errors.New("hitl node: no handled tool call found")
		}
		// 真实 MCP 工具经 proxy execute_tool 调用，需解出真实工具名再匹配 handler。
		toolName := toolproxy.ResolveToolCall(tc).Name
		handler, ok := reg.Lookup(toolName)
		if !ok {
			logs.Errorf("hitl node: no handler registered for tool %q, rid: %s", toolName, rid)
			return nil, fmt.Errorf("hitl node: no handler registered for tool %q", toolName)
		}

		payload, err := handler.BuildPayload(ctx, tc)
		if err != nil {
			logs.Errorf("hitl node: build payload failed for tool %q, err: %v, rid: %s",
				toolName, err, rid)
			return nil, fmt.Errorf("hitl node: build payload: %w", err)
		}

		interruptKey := buildHITLInterruptKey(handler.EventKind(), toolName, tc.ID)
		resumeValue, err := graph.Interrupt(ctx, state, interruptKey, payload)
		if err != nil {
			// 首次调用：err 为 InterruptError，向上传播以暂停执行。
			logs.Infof("hitl node: interrupt for tool %q, key: %s, rid: %s", toolName, interruptKey, rid)
			return nil, err
		}

		var result ResumeResult
		if forwarded := resolveHITLForwardedResumeValue(ctx); forwarded != "" {
			// 前端通过 forwardedProps.resumeValue 返回了结构化选择：发出恢复事件并走确认路径。
			emitHITLEvent(ctx, state, buildHITLResumeEventName(interruptKey), map[string]any{"value": forwarded}, rid)
			logs.Infof("hitl node: resume with forwarded value for tool %q, rid: %s", toolName, rid)
			result, err = handler.OnResume(ctx, tc, forwarded)
		} else {
			// 无结构化回复：合成取消信号走 OnCancel 路径，并附加用户输入消息。
			logs.Infof("hitl node: no forwarded value, treating as cancel for tool %q, rid: %s", toolName, rid)
			result, err = handler.OnResume(ctx, tc, CancelActionSignal)
			if err == nil {
				injectCancelUserMessage(&result, toolName, resumeValue)
			}
		}
		if err != nil {
			logs.Errorf("hitl node: on resume failed for tool %q, err: %v, rid: %s",
				toolName, err, rid)
			return nil, fmt.Errorf("hitl node: on resume: %w", err)
		}

		return buildResumeDelta(result, toolName, rid), nil
	}
}

// buildResumeDelta 将 handler 的 ResumeResult 转换为图状态增量。
func buildResumeDelta(result ResumeResult, toolName, rid string) graph.State {
	next := result.Next
	if next != enumor.CvmApplyNodeTool {
		next = enumor.CvmApplyNodeLLM
	}
	logs.Infof("hitl node: tool %q resume routes to %s, rid: %s", toolName, next, rid)

	delta := graph.State{constant.StateKeyHITLRoute: next}
	if result.ClearUserInput {
		delta[graph.StateKeyUserInput] = ""
	}
	switch {
	case len(result.MessageOps) > 0:
		delta[graph.StateKeyMessages] = result.MessageOps
	case len(result.AppendMessages) > 0:
		delta[graph.StateKeyMessages] = result.AppendMessages
	}
	return delta
}

// buildHITLInterruptKey 构造中断 key "<eventKind>:<tool>:<callID>"。
// toolName 为解析后的真实工具名，使前端看到的是实际受门禁守护的工具。
func buildHITLInterruptKey(eventKind, toolName, callID string) string {
	return fmt.Sprintf("%s%s%s%s%s", eventKind, constant.InterruptKeySeparator,
		toolName, constant.InterruptKeySeparator, callID)
}

// buildHITLResumeEventName returns the custom-event name for the post-selection resume event,
// derived from the interrupt key used for the pre-select event by appending ".resume".
func buildHITLResumeEventName(interruptKey string) string {
	return interruptKey + ".resume"
}

// emitHITLEvent emits a custom AG-UI event with the given name and payload.
// Failures are logged at warn level and do not abort the node execution.
func emitHITLEvent(ctx context.Context, state graph.State, eventName string, payload any, rid string) {
	emitter := graph.GetEventEmitterWithContext(ctx, state)
	if err := emitter.EmitCustom(eventName, payload); err != nil {
		logs.Warnf("hitl node: emit event %q failed, err: %v, rid: %s", eventName, err, rid)
	}
}

// resolveHITLForwardedResumeValue reads the structured resume value passed by the frontend via
// forwardedProps. The value is stored in RuntimeState as map[string]any (parsed from JSON by the
// AG-UI layer); this function marshals it back to a JSON string so that handlers receive a uniform
// string contract. Returns an empty string when the value is absent or marshalling fails.
func resolveHITLForwardedResumeValue(ctx context.Context) string {
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

// injectCancelUserMessage adjusts a cancel ResumeResult so that the user's free-form input
// (typed when dismissing the interrupt) is forwarded to the LLM as a user message instead of
// being discarded. If userInput is empty, a synthetic "用户取消申领 <toolName>" message is used.
func injectCancelUserMessage(result *ResumeResult, toolName string, userInput any) {
	userMsg, _ := userInput.(string)
	if userMsg == "" {
		userMsg = "用户取消申领 " + toolName
	}
	// Override ClearUserInput so that StateKeyUserInput is not blanked by buildResumeDelta.
	result.ClearUserInput = false
	result.AppendMessages = append(result.AppendMessages,
		model.Message{Role: model.RoleUser, Content: userMsg},
	)
}

// extractHandledToolCall 从最近一条 assistant 消息中返回由注册表处理的工具调用。
// 它反向扫描并在遇到第一条 user 消息时停止，以保证 tool_call 配对落在当前轮次内。
// 每个工具调用都经 proxy execute_tool 信封解析，使被 proxy 包装的受门禁 MCP 工具能按真实名识别。
func extractHandledToolCall(messages []model.Message, reg *Registry) *model.ToolCall {
	for i := len(messages) - 1; i >= 0; i-- {
		switch messages[i].Role {
		case model.RoleAssistant:
			for j := range messages[i].ToolCalls {
				if reg.Has(toolproxy.ResolveToolCall(&messages[i].ToolCalls[j]).Name) {
					tc := messages[i].ToolCalls[j]
					return &tc
				}
			}
		case model.RoleUser:
			return nil
		}
	}
	return nil
}
