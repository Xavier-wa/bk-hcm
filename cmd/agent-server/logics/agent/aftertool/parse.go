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

package aftertool

import (
	"encoding/json"
	"strings"

	woaserver "hcm/pkg/api/woa-server"
	"hcm/pkg/criteria/constant"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

// findRecommendToolCall locates the recommend tool call (by_static / by_plan / split_suborder) in
// the most recent assistant message that actually issued tool calls, returning the normalized tool
// name, call id and requested limit.
func findRecommendToolCall(messages []trpcmodel.Message) *recommendToolCall {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != trpcmodel.RoleAssistant || len(messages[i].ToolCalls) == 0 {
			continue
		}
		for _, tc := range messages[i].ToolCalls {
			name := businessToolName(tc)
			if isRecommendTool(name) {
				return &recommendToolCall{toolName: name, id: tc.ID, limit: extractLimit(tc.Function.Arguments)}
			}
		}
		return nil
	}
	return nil
}

// findToolResult returns the content of the tool result message matching the given tool call id.
func findToolResult(messages []trpcmodel.Message, toolID string) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == trpcmodel.RoleTool && messages[i].ToolID == toolID {
			return messages[i].Content
		}
	}
	return ""
}

// businessToolName resolves the effective business tool name from a tool call, unwrapping the
// tool-proxy execute_tool meta-tool (which carries the real name in its tool_name argument).
// 框架会给 toolset 提供的工具名加上 toolset 前缀，因此既要兼容裸名 execute_tool，
// 也要兼容带前缀的 tool_proxy_execute_tool。
func businessToolName(tc trpcmodel.ToolCall) string {
	name := tc.Function.Name
	if name == constant.ExecuteToolToolName || name == constant.ProxyExecuteToolFullName {
		var p struct {
			ToolName string `json:"tool_name"`
		}
		_ = json.Unmarshal(tc.Function.Arguments, &p)
		name = p.ToolName
	}
	return normalizeRecommendName(name)
}

// normalizeRecommendName maps a possibly-prefixed tool name (e.g. "mcp/get_biz_apply_recommend_by_plan")
// to the canonical recommend tool name when it matches a known recommend tool, otherwise returns it as is.
func normalizeRecommendName(name string) string {
	name = strings.TrimSpace(name)
	for _, n := range []string{
		constant.RecommendByStaticToolName,
		constant.RecommendByPlanToolName,
		constant.RecommendSplitSuborderToolName,
	} {
		if name == n || strings.HasSuffix(name, "/"+n) {
			return n
		}
	}
	return name
}

// isRecommendTool reports whether the given canonical name is one of the recommend tools.
func isRecommendTool(name string) bool {
	switch name {
	case constant.RecommendByStaticToolName, constant.RecommendByPlanToolName,
		constant.RecommendSplitSuborderToolName:
		return true
	default:
		return false
	}
}

// bodyParamLimit models the recommend limit nested under body_param in a tool call's arguments.
type bodyParamLimit struct {
	BodyParam struct {
		Limit int `json:"limit"`
	} `json:"body_param"`
}

// extractLimit parses the requested recommend count from tool call arguments. The limit is nested in
// body_param: under parameters for the tool-proxy schema ({parameters:{body_param:{limit}}}), or at the
// top level for the direct schema ({body_param:{limit}}). Falls back to the default when absent.
func extractLimit(argsRaw json.RawMessage) int {
	var proxy struct {
		Parameters bodyParamLimit `json:"parameters"`
	}
	if err := json.Unmarshal(argsRaw, &proxy); err == nil && proxy.Parameters.BodyParam.Limit > 0 {
		return proxy.Parameters.BodyParam.Limit
	}
	var direct bodyParamLimit
	if err := json.Unmarshal(argsRaw, &direct); err == nil && direct.BodyParam.Limit > 0 {
		return direct.BodyParam.Limit
	}
	return constant.DefaultRecommendLimit
}

// parseItems extracts recommend items from a (possibly enveloped) tool result string.
func parseItems(result string) []*woaserver.ApplyRecommendItem {
	data := extractData([]byte(result), 0)
	if len(data) == 0 {
		return nil
	}
	var resp woaserver.ApplyRecommendByStaticResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	return resp.Items
}

// parseSuborders extracts split suborders from a (possibly enveloped) tool result string.
func parseSuborders(result string) []*woaserver.ApplyRecommendSuborder {
	data := extractData([]byte(result), 0)
	if len(data) == 0 {
		return nil
	}
	var resp woaserver.ApplyRecommendSplitSubOrderResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	return resp.Suborders
}

// extractData unwraps nested response envelopes to reach the business payload. It peels the
// tool-proxy success wrapper ({success, result}), the MCP content array ([{type:text, text}]),
// the API gateway wrapper ({request_id, response_body}) and the rest envelope ({code, data});
// a false success short-circuits to nil.
func extractData(content []byte, depth int) []byte {
	if depth > extractDataMaxDepth || len(content) == 0 {
		return content
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(content, &m); err != nil {
		// MCP 工具结果序列化为 content 数组 [{type:text, text:...}]，
		// 取出拼接的 text 文本后继续向下解包。
		if text := extractMCPContentText(content); text != "" {
			return extractData([]byte(text), depth+1)
		}
		return content
	}
	if raw, ok := m["success"]; ok {
		var success bool
		if json.Unmarshal(raw, &success) == nil && !success {
			return nil
		}
	}
	// 依次剥离已知的包裹层：rest envelope data、API 网关 response_body、tool-proxy result。
	// data 优先于 result：response_body 内同时存在 data(业务数据) 与 result(布尔成功标志)，
	// 且仅对 JSON 对象/数组的值下钻，避免误把标量 result:true 当作包裹层。
	for _, key := range []string{"data", "response_body", "result"} {
		if raw, ok := m[key]; ok && isJSONContainer(raw) {
			return extractData(raw, depth+1)
		}
	}
	return content
}

// isJSONContainer reports whether the raw JSON value is an object or array (vs a scalar).
func isJSONContainer(raw json.RawMessage) bool {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{', '[':
			return true
		default:
			return false
		}
	}
	return false
}

// mcpContentItem mirrors an MCP TextContent entry ({type, text}) returned by tool calls.
type mcpContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// extractMCPContentText concatenates the text fields of an MCP content array, returning empty
// when the payload is not a non-empty text-content array.
func extractMCPContentText(content []byte) string {
	var items []mcpContentItem
	if err := json.Unmarshal(content, &items); err != nil || len(items) == 0 {
		return ""
	}
	var b strings.Builder
	for _, it := range items {
		if it.Type == "text" {
			b.WriteString(it.Text)
		}
	}
	return b.String()
}

// mergeCandidates merges recommend items, keeping existing (by_static) entries first
// and appending all valid ones from add (by_plan).
func mergeCandidates(existing, add []*woaserver.ApplyRecommendItem) []*woaserver.ApplyRecommendItem {
	out := make([]*woaserver.ApplyRecommendItem, 0, len(existing)+len(add))
	for _, list := range [][]*woaserver.ApplyRecommendItem{existing, add} {
		for _, it := range list {
			if it == nil || it.Suborder == nil {
				continue
			}
			out = append(out, it)
		}
	}
	return out
}

// readCandidates reads the accumulated candidates from state (stored as a JSON string).
func readCandidates(state graph.State) []*woaserver.ApplyRecommendItem {
	s, _ := state[constant.StateKeyRecommendCandidates].(string)
	if s == "" {
		return nil
	}
	var items []*woaserver.ApplyRecommendItem
	if err := json.Unmarshal([]byte(s), &items); err != nil {
		return nil
	}
	return items
}

// marshalCandidates serializes candidates to a JSON string for state storage.
func marshalCandidates(items []*woaserver.ApplyRecommendItem) string {
	if len(items) == 0 {
		return ""
	}
	b, err := json.Marshal(items)
	if err != nil {
		return ""
	}
	return string(b)
}
