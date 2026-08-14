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

package internalmcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"hcm/cmd/api-server/service/mcp/middleware"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"

	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

// sampleSpec is a minimal OpenAPI 3.0 spec covering the relevant fixtures:
// a POST with requestBody, a POST with path parameter + response example,
// and an operation outside the IncludeOperationIDs whitelist.
const sampleSpec = `openapi: 3.0.1
paths:
  /api/v1/cloud/accounts/list:
    post:
      operationId: list_account
      description: 查询账号列表
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: object
                    additionalProperties: true
              example:
                code: 0
                data:
                  count: 2
                  details:
                  - id: "00000031"
                    vendor: tcloud
                message: ''
          description: 成功
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [page]
              properties:
                page:
                  type: object
                  description: 分页设置
                filter:
                  type: object
                  description: 查询过滤条件
      x-bk-apigateway-resource:
        backend:
          method: post
          path: "{env.url_path_prefix}/api/v1/cloud/accounts/list"
  /api/v1/cloud/vendors/{vendor}/regions/list:
    post:
      operationId: list_region
      description: 查询地域列表
      parameters:
      - in: path
        name: vendor
        schema: { type: string }
        required: true
        description: 云厂商
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                page:
                  type: object
      x-bk-apigateway-resource:
        backend:
          method: post
          path: "{env.url_path_prefix}/api/v1/cloud/vendors/{vendor}/regions/list"
  /api/v1/cloud/danger/op:
    post:
      operationId: danger_op
      description: 危险操作（不在白名单内）
      requestBody:
        content:
          application/json:
            schema: { type: object }
      x-bk-apigateway-resource:
        backend:
          method: post
          path: /api/v1/cloud/danger/op
`

func writeSampleSpec(t *testing.T) string {
	t.Helper()
	specPath := filepath.Join(t.TempDir(), "spec.yaml")
	if err := os.WriteFile(specPath, []byte(sampleSpec), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return specPath
}

func testInternalServerCfg(t *testing.T) cc.MCPInternalServerSetting {
	t.Helper()
	specPath := writeSampleSpec(t)
	return cc.MCPInternalServerSetting{
		Name:            "hcm-internal-mcp",
		BasePath:        "/api/v1/mcp/internal/hcm/mcp",
		ServerVersion:   constant.MCPInternalServerDefaultVersion,
		OpenAPISpecPath: specPath,
	}
}

func testInternalCfg(t *testing.T) cc.MCPInternalSetting {
	t.Helper()
	return cc.MCPInternalSetting{
		Enable:              true,
		EnforceCallerSource: true,
		Servers:             []cc.MCPInternalServerSetting{testInternalServerCfg(t)},
	}
}

func TestLoadOpenAPISpec_BuildsToolsFromOperations(t *testing.T) {
	cfg := testInternalServerCfg(t)
	loaded, err := LoadOpenAPISpec(cfg.OpenAPISpecPath, nil)
	if err != nil {
		t.Fatalf("LoadOpenAPISpec: %v", err)
	}
	names := toolNames(loaded.Tools)
	for _, want := range []string{"list_account", "list_region", "danger_op"} {
		if !names[want] {
			t.Fatalf("missing tool %q in %v", want, names)
		}
	}

	// list_account: backend path strips {env.url_path_prefix}.
	if got := loaded.Backends["list_account"].Path; got != "/api/v1/cloud/accounts/list" {
		t.Fatalf("list_account backend path = %q, want /api/v1/cloud/accounts/list", got)
	}
	if got := loaded.Backends["list_account"].Method; got != "POST" {
		t.Fatalf("list_account backend method = %q, want POST", got)
	}

	// description should have the response example appended.
	listAccount := findTool(loaded.Tools, "list_account")
	if listAccount == nil {
		t.Fatal("list_account tool not found")
	}
	if !strings.Contains(listAccount.Description, "Example response:") {
		t.Fatalf("description missing Example response, got %q", listAccount.Description)
	}
	if !strings.Contains(listAccount.Description, `"vendor":"tcloud"`) {
		t.Fatalf("description missing example payload, got %q", listAccount.Description)
	}

	// list_region: path parameter should be in path_param, not top-level.
	listRegion := findTool(loaded.Tools, "list_region")
	if listRegion == nil {
		t.Fatal("list_region tool not found")
	}
	props := listRegion.InputSchema.Properties
	if _, ok := props["path_param"]; !ok {
		t.Fatalf("list_region inputSchema missing path_param, got %v", schemaPropKeys(listRegion))
	}
	if _, ok := props["vendor"]; ok {
		t.Fatalf("list_region inputSchema should not have top-level vendor, got %v", schemaPropKeys(listRegion))
	}
	// path_param should be in top-level required.
	hasReq := false
	for _, r := range listRegion.InputSchema.Required {
		if r == "path_param" {
			hasReq = true
		}
	}
	if !hasReq {
		t.Fatalf("list_region inputSchema.required missing path_param: %v", listRegion.InputSchema.Required)
	}
}

func TestLoadOpenAPISpec_IncludeWhitelistGlob(t *testing.T) {
	cfg := testInternalServerCfg(t)
	loaded, err := LoadOpenAPISpec(cfg.OpenAPISpecPath, []string{"list_*"})
	if err != nil {
		t.Fatalf("LoadOpenAPISpec: %v", err)
	}
	names := toolNames(loaded.Tools)
	if !names["list_account"] || !names["list_region"] || names["danger_op"] {
		t.Fatalf("whitelist glob list_* mismatch, got %v", names)
	}
}

func TestLoadOpenAPISpec_FileMissingFailsLoudly(t *testing.T) {
	if _, err := LoadOpenAPISpec("/tmp/no-such-spec-xxxxxx.yaml", nil); err == nil {
		t.Error("expected error on missing openapi spec, got nil")
	}
}

func TestLoadOpenAPISpec_EmptyPathFailsLoudly(t *testing.T) {
	if _, err := LoadOpenAPISpec("", nil); err == nil {
		t.Error("expected error on empty openapi spec path, got nil")
	}
}

func TestDispatcher_LocalBackendCall(t *testing.T) {
	cfg := testInternalServerCfg(t)
	loaded, err := LoadOpenAPISpec(cfg.OpenAPISpecPath, nil)
	if err != nil {
		t.Fatalf("LoadOpenAPISpec: %v", err)
	}

	caller := &mockBackendCaller{
		response: &BackendResponse{StatusCode: 200, Body: []byte(`{"code":0,"data":{}}`)},
	}
	dispatcher := NewDispatcherWithCaller(cfg, caller, 0)
	dispatcher.SetBackends(loaded.Backends)

	// list_region requires path variable {vendor} in path_param.
	result, err := dispatcher.Dispatch(testCtx(), &mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{
			Name: "list_region",
			Arguments: map[string]interface{}{
				"path_param": map[string]interface{}{"vendor": "tcloud"},
				"body_param": map[string]interface{}{"page": map[string]interface{}{"limit": 10}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Dispatch list_region: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", resultText(result))
	}

	last := caller.lastReq()
	if last == nil {
		t.Fatal("backend caller never invoked")
	}
	if last.Method != "POST" {
		t.Fatalf("backend method = %q, want POST", last.Method)
	}
	if last.Path != "/api/v1/cloud/vendors/tcloud/regions/list" {
		t.Fatalf("backend path = %q, want path-template substituted", last.Path)
	}
	if got := last.Headers.Get(constant.UserKey); got != "alice" {
		t.Fatalf("backend X-Bkapi-User-Name = %q, want alice", got)
	}
	if got := last.Headers.Get(constant.MCPCallerSourceHeader); got != string(cc.APIServerName) {
		t.Fatalf("backend X-Bkhcm-Caller-Source = %q, want api-server", got)
	}

	// Body should be the content of body_param (without path variables).
	var body map[string]interface{}
	if err := json.Unmarshal(last.Body, &body); err != nil {
		t.Fatalf("backend body unmarshal: %v", err)
	}
	if _, ok := body["vendor"]; ok {
		t.Fatalf("backend body should not have path arg vendor: %v", body)
	}
	if _, ok := body["page"]; !ok {
		t.Fatalf("backend body missing page: %v", body)
	}
}

func TestDispatcher_UnknownToolReturnsErrorResult(t *testing.T) {
	cfg := testInternalServerCfg(t)
	dispatcher := NewDispatcherWithCaller(cfg, &mockBackendCaller{}, 0)
	result, err := dispatcher.Dispatch(testCtx(), &mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{Name: "unknown_tool"},
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(result), "unknown internal MCP tool") {
		t.Fatalf("result = %+v, want unknown tool error", result)
	}
}

func TestDispatcher_BackendBadStatusReturnsErrorResult(t *testing.T) {
	cfg := testInternalServerCfg(t)
	loaded, err := LoadOpenAPISpec(cfg.OpenAPISpecPath, []string{"list_account"})
	if err != nil {
		t.Fatalf("LoadOpenAPISpec: %v", err)
	}
	caller := &mockBackendCaller{
		response: &BackendResponse{StatusCode: 500, Body: []byte("oops")},
	}
	dispatcher := NewDispatcherWithCaller(cfg, caller, 0)
	dispatcher.SetBackends(loaded.Backends)
	result, err := dispatcher.Dispatch(testCtx(), &mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{Name: "list_account",
			Arguments: map[string]interface{}{"body_param": map[string]interface{}{"page": map[string]interface{}{}}}},
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !result.IsError || !strings.Contains(resultText(result), "backend returned status 500") {
		t.Fatalf("result = %+v, want backend status error", result)
	}
}

func TestRegister_InternalOnlyOverridesOpenAPI(t *testing.T) {
	cfg := testInternalCfg(t)
	cfg.Servers[0].InternalOnlyTools = []cc.InternalOnlyToolSetting{
		{
			Name:        "list_account",
			Description: "internal only override",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"x": map[string]interface{}{"type": "string"},
				},
			},
			Backend: cc.InternalOnlyToolBackend{Method: "POST", Path: "/api/v1/internal/account/list"},
		},
	}

	mux := http.NewServeMux()
	dispatchers, err := RegisterAll(context.Background(), mux, cfg)
	if err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if len(dispatchers) != 1 {
		t.Fatalf("dispatchers count = %d, want 1", len(dispatchers))
	}
	dispatcher := dispatchers[0]
	if dispatcher == nil {
		t.Fatal("dispatcher is nil")
	}
	backend, ok := dispatcher.backendOf("list_account")
	if !ok {
		t.Fatal("list_account backend missing")
	}
	if backend.Path != "/api/v1/internal/account/list" {
		t.Fatalf("list_account backend path = %q, want override path", backend.Path)
	}
}

func TestRegister_MissingCallerSourceReturnsForbidden(t *testing.T) {
	cfg := testInternalCfg(t)

	mux := http.NewServeMux()
	if _, err := RegisterAll(context.Background(), mux, cfg); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, cfg.Servers[0].BasePath+"/",
		strings.NewReader(jsonRPC(t, 1, mcpsdk.MethodToolsList, nil)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(constant.UserKey, "alice")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		body, _ := io.ReadAll(w.Body)
		t.Fatalf("status = %d, body=%s, want 403", w.Code, string(body))
	}
}

func TestRegister_DisabledSkips(t *testing.T) {
	cfg := testInternalCfg(t)
	cfg.Enable = false

	mux := http.NewServeMux()
	dispatchers, err := RegisterAll(context.Background(), mux, cfg)
	if err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if len(dispatchers) != 0 {
		t.Errorf("expected empty dispatchers when disabled, got %+v", dispatchers)
	}
}

func TestRegisterAll_MultipleServersExposeSeparateToolLists(t *testing.T) {
	cfg := testInternalCfg(t)
	specPath := writeSampleSpec(t)
	cfg.Servers = []cc.MCPInternalServerSetting{
		{
			Name:                "hcm-account",
			BasePath:            "/api/v1/mcp/internal/hcm-account/mcp",
			OpenAPISpecPath:     specPath,
			IncludeOperationIDs: []string{"list_account"},
		},
		{
			Name:                "hcm-region",
			BasePath:            "/api/v1/mcp/internal/hcm-region/mcp",
			OpenAPISpecPath:     specPath,
			IncludeOperationIDs: []string{"list_region"},
		},
	}

	mux := http.NewServeMux()
	dispatchers, err := RegisterAll(context.Background(), mux, cfg)
	if err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if len(dispatchers) != 2 {
		t.Fatalf("dispatchers count = %d, want 2", len(dispatchers))
	}

	accountTools := listToolsFromMux(t, mux, "/api/v1/mcp/internal/hcm-account/mcp/")
	if !accountTools["list_account"] || accountTools["list_region"] {
		t.Fatalf("account tools = %v, want only list_account", accountTools)
	}
	regionTools := listToolsFromMux(t, mux, "/api/v1/mcp/internal/hcm-region/mcp/")
	if !regionTools["list_region"] || regionTools["list_account"] {
		t.Fatalf("region tools = %v, want only list_region", regionTools)
	}
}

// helpers -----------------------------------------------------------------

type mockBackendCaller struct {
	count    int32
	last     atomic.Value // *BackendRequest
	response *BackendResponse
	err      error
}

func (m *mockBackendCaller) Call(_ context.Context, req *BackendRequest) (*BackendResponse, error) {
	atomic.AddInt32(&m.count, 1)
	cp := *req
	cp.Headers = req.Headers.Clone()
	cp.Body = append([]byte(nil), req.Body...)
	m.last.Store(&cp)
	if m.err != nil {
		return nil, m.err
	}
	if m.response == nil {
		return &BackendResponse{StatusCode: 200, Body: []byte(`{}`)}, nil
	}
	return m.response, nil
}

func (m *mockBackendCaller) lastReq() *BackendRequest {
	v := m.last.Load()
	if v == nil {
		return nil
	}
	return v.(*BackendRequest)
}

func toolNames(tools []*mcpsdk.Tool) map[string]bool {
	names := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if tool != nil {
			names[tool.Name] = true
		}
	}
	return names
}

func findTool(tools []*mcpsdk.Tool, name string) *mcpsdk.Tool {
	for _, t := range tools {
		if t != nil && t.Name == name {
			return t
		}
	}
	return nil
}

func schemaPropKeys(tool *mcpsdk.Tool) []string {
	if tool == nil || tool.InputSchema == nil {
		return nil
	}
	keys := make([]string, 0, len(tool.InputSchema.Properties))
	for k := range tool.InputSchema.Properties {
		keys = append(keys, k)
	}
	return keys
}

func testCtx() context.Context {
	kt := &kit.Kit{
		User:     "alice",
		Rid:      "rid-test",
		AppCode:  constant.WebSourceAppCode,
		TenantID: "default",
		Ctx:      context.Background(),
	}
	return middleware.WithKit(context.Background(), kt)
}

func resultText(result *mcpsdk.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	if text, ok := result.Content[0].(mcpsdk.TextContent); ok {
		return text.Text
	}
	return ""
}

func listToolsFromMux(t *testing.T, mux http.Handler, path string) map[string]bool {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(jsonRPC(t, 1, mcpsdk.MethodToolsList, nil)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(constant.UserKey, "alice")
	req.Header.Set(constant.MCPCallerSourceHeader, string(cc.AgentServerName))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("tools/list status = %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
		Error interface{} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal tools/list response: %v, body=%s", err, w.Body.String())
	}
	if resp.Error != nil {
		t.Fatalf("tools/list json-rpc error: %v", resp.Error)
	}
	names := make(map[string]bool, len(resp.Result.Tools))
	for _, tool := range resp.Result.Tools {
		names[tool.Name] = true
	}
	return names
}

func jsonRPC(t *testing.T, id int, method string, params map[string]interface{}) string {
	t.Helper()
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		payload["params"] = params
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal jsonrpc: %v", err)
	}
	return string(b)
}
