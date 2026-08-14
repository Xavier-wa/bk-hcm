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

package tool

import (
	"strings"

	"hcm/pkg/tools/uuid"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

// RegenToolCallID 基于原始 tool_call ID 生成一个新的唯一 ID，保留 LLM 厂商特定的前缀格式，
// 仅替换末段的唯一标识符。
//
// HITL 恢复后，如果直接复用原始 tool_call_id 继续执行，tool 节点会再次流式输出相同 ID 的
// ToolCallStart/End 事件，导致前端历史回放出现重复 ID 而失败。使用此函数生成新 ID 可从源头规避该问题。
//
// 示例：原始 "chatcmpl-tool-a0b16f2bd28a42cfa5fee710b74e556d"
// → 新 ID "chatcmpl-tool-{新的32位hex}"
//
// 当原始 ID 中不含 "-" 时，直接返回新生成的纯 hex 字符串。
func RegenToolCallID(originalID string) string {
	// 去除 UUID 中的连字符，与原始 ID 后缀格式（32位纯hex）保持一致
	newSuffix := strings.ReplaceAll(uuid.UUID(), "-", "")
	idx := strings.LastIndex(originalID, "-")
	if idx < 0 {
		return newSuffix
	}
	// 保留原始 ID 的厂商前缀（如 "chatcmpl-tool-"），只替换末段唯一标识
	return originalID[:idx+1] + newSuffix
}

// ReplaceToolCallArgs 是一个 graph.MessageOp，用于覆盖某个 tool_call（由 ToolID 标识）的入参，
// 该 tool_call 位于持有它的最近一条 assistant 消息中。
// 用于用户在确认卡片上修改参数后，将修改后的入参回写到对应 tool_call。
type ReplaceToolCallArgs struct {
	// ToolID 是要替换入参的 tool_call ID。
	ToolID string
	// NewArgs 是新的原始 JSON 入参。
	NewArgs []byte
}

// Apply 实现 graph.MessageOp 接口。
// MessageReducer 通过接口断言接受外部的 MessageOp 实现。
func (op ReplaceToolCallArgs) Apply(dst []model.Message) []model.Message {
	for i := len(dst) - 1; i >= 0; i-- {
		for j := range dst[i].ToolCalls {
			if dst[i].ToolCalls[j].ID == op.ToolID {
				dst[i].ToolCalls[j].Function.Arguments = op.NewArgs
				return dst
			}
		}
	}
	return dst
}
