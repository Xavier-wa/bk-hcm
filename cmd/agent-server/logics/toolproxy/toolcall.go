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

package toolproxy

import (
	"encoding/json"
	"strings"

	"hcm/pkg/criteria/constant"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

// ResolvedToolCall 是解开 tool proxy execute_tool 信封后，工具调用的有效视图。
// 真实 MCP 工具不再直接暴露给 LLM，而是通过 proxy 元工具 execute_tool 调用，其入参携带真实工具名
// 与参数。门禁匹配与 payload 构造都必须基于此解析后的视图。
type ResolvedToolCall struct {
	// Name 是有效工具名：当调用为 proxy execute_tool 时为真实 MCP 工具名（已去除 toolset 前缀），
	// 否则为 tool_call 自身的函数名。
	Name string
	// Arguments 是有效的原始 JSON 入参：当调用为 proxy execute_tool 时为 proxy 内层 "parameters"
	// 对象，否则为 tool_call 自身入参。
	Arguments []byte
	// Proxy 标识该工具调用是否为 proxy execute_tool 信封。
	Proxy bool
}

// ResolveToolCall 将工具调用解析为其有效工具名与入参。
//
// 当调用为 proxy execute_tool 元工具时，解开 ExecuteToolParams 信封：
// Name 取真实 MCP 工具名并去除 toolset 前缀（如 "bkhcm-devhk/create_biz_apply" → "create_biz_apply"），
// Arguments 取原始 "parameters" 对象。否则原样返回 tool_call 自身的名称与入参。
// tc 为 nil 时返回零值。
func ResolveToolCall(tc *model.ToolCall) ResolvedToolCall {
	if tc == nil {
		return ResolvedToolCall{}
	}
	if tc.Function.Name != constant.ProxyExecuteToolFullName {
		return ResolvedToolCall{Name: tc.Function.Name, Arguments: tc.Function.Arguments}
	}

	var envelope ExecuteToolParams
	if err := json.Unmarshal(tc.Function.Arguments, &envelope); err != nil {
		// 信封解析失败时退回原始工具名/入参，由上层按未识别处理，避免误触发门禁。
		return ResolvedToolCall{Name: tc.Function.Name, Arguments: tc.Function.Arguments, Proxy: true}
	}
	return ResolvedToolCall{
		Name:      StripToolSetPrefix(envelope.ToolName),
		Arguments: envelope.Parameters,
		Proxy:     true,
	}
}

// StripToolSetPrefix 去除规范 MCP 工具名中的 "<toolset>/" 前缀，返回原始工具名。
// proxy 以 "<prefix>/<rawName>" 形式注册工具；门禁 handler 以 rawName 作为键。
func StripToolSetPrefix(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}
