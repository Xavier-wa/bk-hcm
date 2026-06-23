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

	"hcm/pkg/criteria/enumor"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// CancelActionSignal is the synthetic resume value the HITL node passes to OnResume when the
// user did not return a structured forwardedProps selection (i.e. they cancelled). Handlers
// detect this value to execute their cancel logic without relying on action-field parsing.
const CancelActionSignal = `{"action":"cancel"}`

// Handler 定义某个工具在流程继续前需要人工中断时，按工具维度的 HITL 行为。
//
// 两类 handler 共用同一节点：
//   - LLM 主动发起的提问（如 human_confirm）：向用户提问，随后在 llm 节点继续。
//   - 真实工具的执行前门禁（如 create_biz_apply）：在工具执行前确认，然后路由到 tool 节点（放行）
//     或回退到 llm 节点（取消/校验未通过）。
type Handler interface {
	// ToolName 返回该 handler 响应的工具名。
	ToolName() string
	// EventKind 返回用于选择前端自定义事件的中断 key 前缀
	// （constant.HITLInterruptKey 或 constant.ToolConfirmInterruptKey）。
	EventKind() string
	// BuildPayload 构造自定义事件携带的中断 payload。
	BuildPayload(ctx context.Context, tc *model.ToolCall) (any, error)
	// OnResume 将用户的 resume 值处理为路由决策与消息增量。
	OnResume(ctx context.Context, tc *model.ToolCall, resumeValue any) (ResumeResult, error)
}

// ResumeResult 是 handler 处理用户 resume 值后的结果。
type ResumeResult struct {
	// Next 是要路由到的下一节点：enumor.CvmApplyNodeTool 或 enumor.CvmApplyNodeLLM。
	Next enumor.CvmApplyNode
	// AppendMessages 是追加到对话中的消息（工具结果 / 用户消息）。
	AppendMessages []model.Message
	// MessageOps 可选地修改已有消息（如回写 tool_call 入参）。
	// AppendMessages 与 MessageOps 互斥，最多设置其一。
	MessageOps []graph.MessageOp
	// ClearUserInput 清空 StateKeyUserInput，避免 resume token 被当作用户输入重放。
	ClearUserInput bool
}
