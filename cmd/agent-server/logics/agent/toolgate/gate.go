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

// Package toolgate 提供受保护工具（如主机申领提单工具 create_biz_apply）的执行前确认门禁。
//
// 每个门禁实现 hitl.Handler，由统一的 hitl 节点消费：无论 LLM 是否主动请求确认，门禁都会在真实工具
// 执行前强制触发一次人工确认中断（发出 tool.confirm 自定义事件）。用户确认后，门禁校验请求并决定
// 放行到 tool 节点还是回退到 llm 节点。
//
// 门禁集合由运行期 confirm-gate 配置（启用/禁用、工具白名单）通过 EnabledGateHandlers /
// EnabledGateToolNames 过滤，供图构建与通用确认工具集使用。
package toolgate

// ConfirmPayload is the structured payload carried by the tool confirm custom event.
// The backend only sends structural data (data); no presentation-layer content (titles, labels, etc.)
// is included — the frontend renders the card by tool name using the interrupt event name.
type ConfirmPayload struct {
	// Tool is the protected tool name (gate identity marker).
	Tool string `json:"tool"`
	// Data carries the structured tool input args (the single source of truth for display and
	// post-confirm editing); must not include any presentation-layer strings.
	Data map[string]any `json:"data"`
}
