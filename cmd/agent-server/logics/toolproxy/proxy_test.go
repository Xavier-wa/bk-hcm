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
	"errors"
	"testing"

	"hcm/cmd/agent-server/logics/tool"
	"hcm/pkg/cc"
	"hcm/pkg/kit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
)

func testToolProxyCfg(topN int) *cc.AgentToolProxyConfig {
	if topN <= 0 {
		topN = 5
	}
	return &cc.AgentToolProxyConfig{TopN: topN}
}

type mockTool struct {
	decl   *trpctool.Declaration
	result any
	err    error
}

func (m *mockTool) Declaration() *trpctool.Declaration { return m.decl }

func (m *mockTool) Call(_ context.Context, _ []byte) (any, error) {
	return m.result, m.err
}

type mockToolSet struct {
	name       string
	tools      []trpctool.Tool
	panicTools bool
}

func (m *mockToolSet) Name() string { return m.name }

func (m *mockToolSet) Tools(_ context.Context) []trpctool.Tool {
	if m.panicTools {
		panic("toolset unavailable")
	}
	return m.tools
}

func (m *mockToolSet) Close() error { return nil }

type mockEmbedder struct {
	vectors map[string][]float64
	err     error
}

func (m *mockEmbedder) GetEmbedding(_ context.Context, text string) ([]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	if v, ok := m.vectors[text]; ok {
		return v, nil
	}
	return []float64{0, 0, 0}, nil
}

func (m *mockEmbedder) GetEmbeddingWithUsage(ctx context.Context, text string) ([]float64, map[string]any, error) {
	v, err := m.GetEmbedding(ctx, text)
	return v, nil, err
}

func (m *mockEmbedder) GetDimensions() int { return 3 }

func newTestProxy(t *testing.T) *ToolProxy {
	t.Helper()

	searchTool := newMockTool("search_code", "在代码库中搜索代码", map[string]string{"query": "string"},
		[]string{"query"})
	searchTool.result = "ok"
	mcpTS := &mockToolSet{
		name:  "mcp",
		tools: []trpctool.Tool{searchTool},
	}
	mcpSets := &tool.MCPToolSet{TS: []trpctool.ToolSet{mcpTS}}

	emb := &mockEmbedder{
		vectors: map[string][]float64{
			"mcp/search_code 在代码库中搜索代码 query": {1, 0, 0},
			"搜索 TODO": {1, 0, 0},
		},
	}

	proxy := NewToolProxy(mcpSets, emb, testToolProxyCfg(5), nil)
	require.NoError(t, proxy.buildForTest(kit.New()))
	require.True(t, proxy.IsBuildOK())
	return proxy
}

func newMockTool(name, desc string, params map[string]string, required []string) *mockTool {
	props := make(map[string]*trpctool.Schema, len(params))
	for k, typ := range params {
		props[k] = &trpctool.Schema{Type: typ}
	}
	return &mockTool{decl: &trpctool.Declaration{
		Name:        name,
		Description: desc,
		InputSchema: &trpctool.Schema{
			Type:       "object",
			Properties: props,
			Required:   required,
		},
	}}
}

func newTestProxyWithTool(t *testing.T, searchTool *mockTool, emb *mockEmbedder) *ToolProxy {
	t.Helper()

	mcpTS := &mockToolSet{
		name:  "mcp",
		tools: []trpctool.Tool{searchTool},
	}
	mcpSets := &tool.MCPToolSet{TS: []trpctool.ToolSet{mcpTS}}
	proxy := NewToolProxy(mcpSets, emb, testToolProxyCfg(5), nil)
	require.NoError(t, proxy.buildForTest(kit.New()))
	require.True(t, proxy.IsBuildOK())
	return proxy
}

func TestToolProxy_BuildAndSearch(t *testing.T) {
	proxy := newTestProxy(t)

	tools := proxy.GetProxyToolSet().Tools(context.Background())
	assert.Len(t, tools, 3)

	args, _ := json.Marshal(map[string]interface{}{
		"query": "搜索 TODO",
		"top_k": 5,
	})
	result, err := proxy.searchTool.Call(context.Background(), args)
	require.NoError(t, err)

	resp, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.True(t, resp["success"].(bool))
	items, ok := resp["tools"].([]interface{})
	if !ok {
		// direct struct slice from Execute
		assert.NotNil(t, resp["tools"])
		return
	}
	assert.NotEmpty(t, items)
}

func TestGetToolSchemaTool_Execute(t *testing.T) {
	proxy := newTestProxy(t)

	args, _ := json.Marshal(map[string]string{"tool_name": "mcp/search_code"})
	result, err := proxy.schemaTool.Call(context.Background(), args)
	require.NoError(t, err)

	resp, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.True(t, resp["success"].(bool))
	assert.NotNil(t, resp["schema"])
}

func TestGetToolSchemaTool_ExecuteWithRawName(t *testing.T) {
	proxy := newTestProxy(t)

	args, _ := json.Marshal(map[string]string{"tool_name": "search_code"})
	result, err := proxy.schemaTool.Call(context.Background(), args)
	require.NoError(t, err)

	resp, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.True(t, resp["success"].(bool))
	assert.Equal(t, "mcp/search_code", resp["tool_name"])
}

// schemaTokenFor is a test helper that returns a valid schema_token for the given tool.
func schemaTokenFor(t *testing.T, proxy *ToolProxy, toolName string) string {
	t.Helper()
	meta, err := proxy.RegistrySnapshot().GetTool(toolName)
	require.NoError(t, err)
	return proxy.GenerateSchemaToken(meta.Name, meta.Schema)
}

func TestExecuteToolTool_RawToolName(t *testing.T) {
	proxy := newTestProxy(t)

	execArgs, _ := json.Marshal(map[string]interface{}{
		"tool_name":    "search_code",
		"parameters":   map[string]interface{}{"query": "TODO"},
		"schema_token": schemaTokenFor(t, proxy, "mcp/search_code"),
	})
	result, err := proxy.executeTool.Call(context.Background(), execArgs)
	require.NoError(t, err)
	okResp := result.(toolSuccessResult)
	assert.True(t, okResp.Success)
	assert.Equal(t, "ok", okResp.Result)
}

func TestExecuteToolTool_ValidationAndExecution(t *testing.T) {
	proxy := newTestProxy(t)
	token := schemaTokenFor(t, proxy, "mcp/search_code")

	// missing required parameter
	args, _ := json.Marshal(map[string]interface{}{
		"tool_name":    "mcp/search_code",
		"parameters":   map[string]interface{}{},
		"schema_token": token,
	})
	result, err := proxy.executeTool.Call(context.Background(), args)
	require.NoError(t, err)
	errResp := result.(toolErrorResult)
	assert.False(t, errResp.Success)
	assert.Equal(t, "invalid_parameters", errResp.Error.Type)
	assert.NotNil(t, errResp.Error.RequiredSchema)

	// successful execution
	execArgs, _ := json.Marshal(map[string]interface{}{
		"tool_name":    "mcp/search_code",
		"parameters":   map[string]interface{}{"query": "TODO"},
		"schema_token": token,
	})
	result, err = proxy.executeTool.Call(context.Background(), execArgs)
	require.NoError(t, err)
	okResp := result.(toolSuccessResult)
	assert.True(t, okResp.Success)
	assert.Equal(t, "ok", okResp.Result)
}

func TestExecuteToolTool_PermissionDenied(t *testing.T) {
	searchTool := newMockTool("search_code", "搜索", map[string]string{"query": "string"}, []string{"query"})
	searchTool.err = errors.New("403 permission denied")
	mcpTS := &mockToolSet{name: "mcp", tools: []trpctool.Tool{searchTool}}
	mcpSets := &tool.MCPToolSet{TS: []trpctool.ToolSet{mcpTS}}

	proxy := NewToolProxy(mcpSets, &mockEmbedder{vectors: map[string][]float64{
		"mcp/search_code 搜索 query": {1, 0, 0},
	}}, testToolProxyCfg(5), nil)
	require.NoError(t, proxy.buildForTest(kit.New()))

	execArgs, _ := json.Marshal(map[string]interface{}{
		"tool_name":    "mcp/search_code",
		"parameters":   map[string]interface{}{"query": "x"},
		"schema_token": schemaTokenFor(t, proxy, "mcp/search_code"),
	})
	result, err := proxy.executeTool.Call(context.Background(), execArgs)
	require.NoError(t, err)
	errResp := result.(toolErrorResult)
	assert.Equal(t, "permission_denied", errResp.Error.Type)
	assert.Contains(t, errResp.Error.Message, "无权限")
}

func TestSearchToolsTool_BuildNotOK(t *testing.T) {
	proxy := NewToolProxy(&tool.MCPToolSet{}, &mockEmbedder{}, testToolProxyCfg(5), nil)
	args, _ := json.Marshal(map[string]string{"query": "test"})
	result, err := proxy.searchTool.Call(context.Background(), args)
	require.NoError(t, err)
	errResp := result.(toolErrorResult)
	assert.Equal(t, "embedding_error", errResp.Error.Type)
}

func TestSearchToolsTool_NoMatchedTools(t *testing.T) {
	proxy := newTestProxy(t)

	args, _ := json.Marshal(map[string]interface{}{
		"query": "天气预报",
		"top_k": 5,
	})
	result, err := proxy.searchTool.Call(context.Background(), args)
	require.NoError(t, err)

	resp, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.True(t, resp["success"].(bool))
	assert.Equal(t, 0, resp["total"])
}

func TestToolProxy_BuildFallbackToKeywordIndex(t *testing.T) {
	searchTool := newMockTool("search_code", "在代码库中搜索代码", map[string]string{"query": "string"},
		[]string{"query"})
	searchTool.result = "ok"
	proxy := newTestProxyWithTool(t, searchTool, &mockEmbedder{err: errors.New("embedding unavailable")})

	args, _ := json.Marshal(map[string]interface{}{
		"query": "搜索代码",
		"top_k": 5,
	})
	result, err := proxy.searchTool.Call(context.Background(), args)
	require.NoError(t, err)

	resp, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.True(t, resp["success"].(bool))
	assert.Equal(t, 1, resp["total"])
}

func TestExecuteToolTool_ValidateParameterType(t *testing.T) {
	proxy := newTestProxy(t)

	args, _ := json.Marshal(map[string]interface{}{
		"tool_name":    "mcp/search_code",
		"parameters":   map[string]interface{}{"query": 1},
		"schema_token": schemaTokenFor(t, proxy, "mcp/search_code"),
	})
	result, err := proxy.executeTool.Call(context.Background(), args)
	require.NoError(t, err)

	errResp := result.(toolErrorResult)
	assert.False(t, errResp.Success)
	assert.Equal(t, "invalid_parameters", errResp.Error.Type)
	assert.Contains(t, errResp.Error.Message, "应为 string")
	assert.NotNil(t, errResp.Error.RequiredSchema)
}

func TestToolProxy_BuildToolSetLoadError(t *testing.T) {
	mcpSets := &tool.MCPToolSet{TS: []trpctool.ToolSet{
		&mockToolSet{name: "mcp", panicTools: true},
	}}
	proxy := NewToolProxy(mcpSets, &mockEmbedder{}, testToolProxyCfg(5), nil)

	err := proxy.buildForTest(kit.New())
	require.NoError(t, err)
	assert.True(t, proxy.IsBuildOK())
	assert.Empty(t, proxy.RegistrySnapshot().ListTools())
}

func TestExecuteToolTool_SchemaTokenValidation(t *testing.T) {
	proxy := newTestProxy(t)

	// missing schema_token
	args, _ := json.Marshal(map[string]interface{}{
		"tool_name":  "mcp/search_code",
		"parameters": map[string]interface{}{"query": "TODO"},
	})
	result, err := proxy.executeTool.Call(context.Background(), args)
	require.NoError(t, err)
	errResp := result.(toolErrorResult)
	assert.False(t, errResp.Success)
	assert.Equal(t, "schema_token_invalid", errResp.Error.Type)
	assert.NotNil(t, errResp.Error.RequiredSchema)

	// wrong schema_token
	args, _ = json.Marshal(map[string]interface{}{
		"tool_name":    "mcp/search_code",
		"parameters":   map[string]interface{}{"query": "TODO"},
		"schema_token": "invalid-token",
	})
	result, err = proxy.executeTool.Call(context.Background(), args)
	require.NoError(t, err)
	errResp = result.(toolErrorResult)
	assert.False(t, errResp.Success)
	assert.Equal(t, "schema_token_invalid", errResp.Error.Type)
}

func TestExecuteToolTool_UnknownParameterKey(t *testing.T) {
	proxy := newTestProxy(t)
	token := schemaTokenFor(t, proxy, "mcp/search_code")

	args, _ := json.Marshal(map[string]interface{}{
		"tool_name":    "mcp/search_code",
		"parameters":   map[string]interface{}{"query": "TODO", "hallucinated_field": "x"},
		"schema_token": token,
	})
	result, err := proxy.executeTool.Call(context.Background(), args)
	require.NoError(t, err)
	errResp := result.(toolErrorResult)
	assert.False(t, errResp.Success)
	assert.Equal(t, "invalid_parameters", errResp.Error.Type)
	assert.Contains(t, errResp.Error.Message, "hallucinated_field")
	assert.NotNil(t, errResp.Error.RequiredSchema)
}

func TestIsPermissionDenied(t *testing.T) {
	assert.True(t, isPermissionDenied(errors.New("403 Forbidden")))
	assert.True(t, isPermissionDenied(errors.New("用户无权限执行")))
	assert.False(t, isPermissionDenied(errors.New("connection timeout")))
}
