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
	"context"
	"encoding/json"
	"testing"

	agenttool "hcm/cmd/agent-server/logics/tool"
	"hcm/pkg/criteria/constant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/model"
	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// TestMetaToolsDeclareToolIntent 锁住三个元工具都暴露可选 tool_intent，且不进 Required。
func TestMetaToolsDeclareToolIntent(t *testing.T) {
	proxy := newTestProxy(t)

	testCases := []struct {
		name string
		decl *trpctool.Declaration
	}{
		{name: constant.SearchToolsToolName, decl: proxy.searchTool.Declaration()},
		{name: constant.GetToolSchemaToolName, decl: proxy.schemaTool.Declaration()},
		{name: constant.ExecuteToolToolName, decl: proxy.executeTool.Declaration()},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.decl.InputSchema)

			prop, ok := tc.decl.InputSchema.Properties[constant.ToolIntentArgKey]
			require.True(t, ok, "meta tool must expose tool_intent")
			assert.Equal(t, "string", prop.Type)
			assert.Equal(t, agenttool.ToolIntentSchemaProperty().Description, prop.Description)
			assert.NotContains(t, tc.decl.InputSchema.Required, constant.ToolIntentArgKey)
		})
	}
}

// TestExecuteToolTool_WithToolIntent 信封带 tool_intent 时仍能通过校验并执行，
// 且 tool_intent 不会被转发给内层 MCP 工具（内层对未定义字段一律拒绝）。
func TestExecuteToolTool_WithToolIntent(t *testing.T) {
	searchTool := newMockTool("search_code", "在代码库中搜索代码", map[string]string{"query": "string"},
		[]string{"query"})
	searchTool.result = "ok"
	proxy := newTestProxyWithTool(t, searchTool, &mockEmbedder{})

	args, _ := json.Marshal(map[string]interface{}{
		"tool_name":               "mcp/search_code",
		"parameters":              map[string]interface{}{"query": "TODO"},
		"schema_token":            schemaTokenFor(t, proxy, "mcp/search_code"),
		constant.ToolIntentArgKey: "正在检索代码中的待办项",
	})

	result, err := proxy.executeTool.Call(context.Background(), args)
	require.NoError(t, err)
	success, ok := result.(toolSuccessResult)
	require.True(t, ok, "unexpected result: %#v", result)
	assert.Equal(t, "ok", success.Result)

	var forwarded map[string]interface{}
	require.NoError(t, json.Unmarshal(searchTool.gotArgs, &forwarded))
	assert.Equal(t, map[string]interface{}{"query": "TODO"}, forwarded)
	assert.NotContains(t, forwarded, constant.ToolIntentArgKey)
}

// TestExecuteToolTool_WithoutToolIntent 信封不含 tool_intent 时行为与改动前一致。
func TestExecuteToolTool_WithoutToolIntent(t *testing.T) {
	searchTool := newMockTool("search_code", "在代码库中搜索代码", map[string]string{"query": "string"},
		[]string{"query"})
	searchTool.result = "ok"
	proxy := newTestProxyWithTool(t, searchTool, &mockEmbedder{})

	args, _ := json.Marshal(map[string]interface{}{
		"tool_name":    "mcp/search_code",
		"parameters":   map[string]interface{}{"query": "TODO"},
		"schema_token": schemaTokenFor(t, proxy, "mcp/search_code"),
	})

	result, err := proxy.executeTool.Call(context.Background(), args)
	require.NoError(t, err)
	success, ok := result.(toolSuccessResult)
	require.True(t, ok, "unexpected result: %#v", result)
	assert.Equal(t, "ok", success.Result)

	var forwarded map[string]interface{}
	require.NoError(t, json.Unmarshal(searchTool.gotArgs, &forwarded))
	assert.Equal(t, map[string]interface{}{"query": "TODO"}, forwarded)
}

// TestExecuteToolTool_ToolIntentInsideParametersRejected tool_intent 只允许放在信封顶层：
// 塞进内层 parameters 会被 schema 校验按未知字段拒绝，避免污染真实 MCP 入参。
func TestExecuteToolTool_ToolIntentInsideParametersRejected(t *testing.T) {
	proxy := newTestProxy(t)

	args, _ := json.Marshal(map[string]interface{}{
		"tool_name": "mcp/search_code",
		"parameters": map[string]interface{}{
			"query":                   "TODO",
			constant.ToolIntentArgKey: "正在检索代码中的待办项",
		},
		"schema_token": schemaTokenFor(t, proxy, "mcp/search_code"),
	})

	result, err := proxy.executeTool.Call(context.Background(), args)
	require.NoError(t, err)
	errResp, ok := result.(toolErrorResult)
	require.True(t, ok, "unexpected result: %#v", result)
	assert.Equal(t, "invalid_parameters", errResp.Error.Type)
}

// TestResolveToolCall_WithToolIntent 门禁依赖的信封解析不受 tool_intent 影响。
func TestResolveToolCall_WithToolIntent(t *testing.T) {
	args, _ := json.Marshal(map[string]interface{}{
		"tool_name":               "mcp/create_biz_apply",
		"parameters":              map[string]interface{}{"biz_id": 1},
		"schema_token":            "token",
		constant.ToolIntentArgKey: "正在提交主机申领单",
	})

	tc := &model.ToolCall{}
	tc.Function.Name = constant.ProxyExecuteToolFullName
	tc.Function.Arguments = args

	resolved := ResolveToolCall(tc)

	assert.True(t, resolved.Proxy)
	assert.Equal(t, "create_biz_apply", resolved.Name)
	assert.JSONEq(t, `{"biz_id":1}`, string(resolved.Arguments))
}
