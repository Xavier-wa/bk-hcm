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

package ingress

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"hcm/cmd/api-server/service/mcp/middleware"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"

	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

// 确保 mcpsdk 被使用（防止未来重构时未使用 import 警告）。
var _ = mcpsdk.NewTextResult

// testCfg 构造单测用的 MCPIngressSetting，与 pkg/cc/api_mcp.go::trySetDefault 默认值对齐。
// 这里手填而非调用 trySetDefault，是因为该方法是 pkg/cc 包内私有，外部无法触达。
func testCfg() cc.MCPIngressSetting {
	return cc.MCPIngressSetting{
		Enable:                  true,
		BasePath:                constant.MCPIngressBasePathDefault,
		ServerName:              "hcm-agent",
		ServerVersion:           "1.0.0",
		AggregatedToolName:      constant.MCPIngressAggregatedToolName,
		TextMaxLength:           8000,
		ProgressTaskMapMaxSize:  10000,
		ProgressTaskMapEntryTTL: 30 * time.Minute,
	}
}

// recordingBridge 用于在测试中观测 BridgeHandler 调用。
type recordingBridge struct {
	mu         sync.Mutex
	sendCalls  []*SendMessageRequest
	cancelArgs []interface{}
	respText   string
	isErr      bool
}

func (b *recordingBridge) SendMessage(_ context.Context, req *SendMessageRequest,
	_ *mcpsdk.Server) (*mcpsdk.CallToolResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sendCalls = append(b.sendCalls, req)
	if b.isErr {
		return mcpsdk.NewErrorResult(b.respText), nil
	}
	return mcpsdk.NewTextResult(b.respText), nil
}

func (b *recordingBridge) Cancel(_ context.Context, progressToken interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cancelArgs = append(b.cancelArgs, progressToken)
}

// jsonRPC 构造一个标准 MCP JSON-RPC 请求体。
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

// validIdentityHeaders 给请求加上能通过 gwparser.defaultParser 校验的最小 header 集合。
// 与 middleware/middleware_test.go 用法一致：AppCode = WebSourceAppCode 跳过 cc.TenantEnable 分支。
func validIdentityHeaders(r *http.Request) {
	r.Header.Set(constant.UserKey, "alice")
	r.Header.Set(constant.AppCodeKey, constant.WebSourceAppCode)
	r.Header.Set(constant.RidKey, "rid-aaaaaaaaaaaaaaaa")
	r.Header.Set(constant.TenantIDKey, "default")
}

// do 发送一次 MCP 请求到 ingress 挂载的 mux 上。
func do(t *testing.T, mux *http.ServeMux, path, body string) (status int, resp map[string]interface{}, raw string) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	validIdentityHeaders(r)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	b, _ := io.ReadAll(w.Body)
	raw = string(b)
	_ = json.Unmarshal(b, &resp)
	return w.Code, resp, raw
}

// setupMux 构造一个挂载好 ingress 的 ServeMux。
func setupMux(t *testing.T, bridge BridgeHandler) (*http.ServeMux, cc.MCPIngressSetting) {
	t.Helper()
	cfg := testCfg()
	mux := http.NewServeMux()
	if err := Register(mux, cfg, bridge); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return mux, cfg
}

func TestRegister_DisabledNoMount(t *testing.T) {
	mux := http.NewServeMux()
	cfg := cc.MCPIngressSetting{Enable: false}
	if err := Register(mux, cfg, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	// 任意路径应该 404，因为 mux 没挂载任何 handler。
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/foo/mcp/", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (mux not mounted)", w.Code)
	}
}

func TestRegister_NilMuxReturnsError(t *testing.T) {
	cfg := testCfg()
	if err := Register(nil, cfg, nil); err == nil {
		t.Error("expected error when mux is nil")
	}
}

func TestToolsList_OnlySendMessage(t *testing.T) {
	mux, _ := setupMux(t, &recordingBridge{respText: "ok"})

	status, resp, raw := do(t, mux,
		"/api/v1/mcp/servers/foo/mcp/",
		jsonRPC(t, 1, "tools/list", nil))
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, raw)
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result not a map: %s", raw)
	}
	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("result.tools not an array: %s", raw)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d: %s", len(tools), raw)
	}
	tool, _ := tools[0].(map[string]interface{})
	if tool["name"] != "send_message" {
		t.Errorf("tool.name = %v, want send_message", tool["name"])
	}
	annotations, _ := tool["annotations"].(map[string]interface{})
	if annotations == nil || annotations["title"] != "HCM Agent" {
		t.Errorf("annotations.title = %v, want HCM Agent", annotations)
	}
	schema, _ := tool["inputSchema"].(map[string]interface{})
	required, _ := schema["required"].([]interface{})
	if len(required) != 1 || required[0] != "text" {
		t.Errorf("required = %v, want [text]", required)
	}
	// 关键属性必须存在
	props, _ := schema["properties"].(map[string]interface{})
	for _, key := range []string{"text", "contextId", "bk_biz_id", "model_name"} {
		if _, ok := props[key]; !ok {
			t.Errorf("inputSchema.properties missing %q", key)
		}
	}
}

func TestToolsList_IdenticalAcrossMCPServerNames(t *testing.T) {
	// 任务 4.5 核心：两个不同的 {mcp_server_name} 路径返回的 tools/list 必须**完全相同**。
	mux, _ := setupMux(t, &recordingBridge{respText: "ok"})

	_, respFoo, _ := do(t, mux,
		"/api/v1/mcp/servers/foo/mcp/",
		jsonRPC(t, 1, "tools/list", nil))
	_, respBar, _ := do(t, mux,
		"/api/v1/mcp/servers/bar-with-dash/mcp/",
		jsonRPC(t, 1, "tools/list", nil))

	if !reflect.DeepEqual(respFoo["result"], respBar["result"]) {
		t.Errorf("tools/list differs between mcp_server_name=foo and bar:\nfoo: %v\nbar: %v",
			respFoo["result"], respBar["result"])
	}
}

func TestToolsCall_SendMessageDispatchesToBridge(t *testing.T) {
	bridge := &recordingBridge{respText: "hello from bridge"}
	mux, _ := setupMux(t, bridge)

	body := jsonRPC(t, 2, "tools/call", map[string]interface{}{
		"name": "send_message",
		"arguments": map[string]interface{}{
			"text":       "查询主机列表",
			"contextId":  "ctx-existing",
			"bk_biz_id":  100,
			"model_name": "hcm-default",
		},
		"_meta": map[string]interface{}{
			"progressToken": "pt-123",
		},
	})

	status, resp, raw := do(t, mux, "/api/v1/mcp/servers/foo/mcp/", body)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body: %s", status, raw)
	}

	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if len(bridge.sendCalls) != 1 {
		t.Fatalf("bridge.SendMessage call count = %d, want 1", len(bridge.sendCalls))
	}
	got := bridge.sendCalls[0]
	if got.Text != "查询主机列表" {
		t.Errorf("Text = %q, want 查询主机列表", got.Text)
	}
	if got.ContextID != "ctx-existing" {
		t.Errorf("ContextID = %q, want ctx-existing", got.ContextID)
	}
	if got.BkBizID != 100 {
		t.Errorf("BkBizID = %d, want 100", got.BkBizID)
	}
	if got.ModelName != "hcm-default" {
		t.Errorf("ModelName = %q, want hcm-default", got.ModelName)
	}
	if got.MCPServerName != "foo" {
		t.Errorf("MCPServerName = %q, want foo (extracted from path)", got.MCPServerName)
	}
	if got.ProgressToken != "pt-123" {
		t.Errorf("ProgressToken = %v, want pt-123", got.ProgressToken)
	}

	// 响应内容应该来自 bridge 的回执
	result, _ := resp["result"].(map[string]interface{})
	contents, _ := result["content"].([]interface{})
	first, _ := contents[0].(map[string]interface{})
	if first["text"] != "hello from bridge" {
		t.Errorf("response text = %v, want 'hello from bridge'", first["text"])
	}
}

func TestToolsCall_MissingTextReturnsInvalidParams(t *testing.T) {
	bridge := &recordingBridge{respText: "should not be called"}
	mux, _ := setupMux(t, bridge)

	body := jsonRPC(t, 3, "tools/call", map[string]interface{}{
		"name":      "send_message",
		"arguments": map[string]interface{}{}, // 缺 text
	})
	status, resp, raw := do(t, mux, "/api/v1/mcp/servers/foo/mcp/", body)
	if status != http.StatusOK {
		// 协议层错误仍走 HTTP 200 + JSON-RPC error
		t.Fatalf("status = %d, body: %s", status, raw)
	}

	// 应当返回 JSON-RPC error 或 CallToolResult.IsError=true（trpc-mcp-go 把 tool handler 返回的 error 包成 -32603）
	// 任意一种都说明协议层成功拦截。
	if _, hasErr := resp["error"]; hasErr {
		// 路径 1：error 对象（旧版 SDK 行为）
	} else if result, ok := resp["result"].(map[string]interface{}); ok {
		// 路径 2：CallToolResult 形式 (isError=true)
		if isErr, _ := result["isError"].(bool); !isErr {
			t.Errorf("expected isError=true on missing text, got: %s", raw)
		}
	} else {
		t.Errorf("expected JSON-RPC error or isError=true, got: %s", raw)
	}

	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if len(bridge.sendCalls) != 0 {
		t.Errorf("bridge.SendMessage should NOT be called on missing text, got %d calls",
			len(bridge.sendCalls))
	}
}

func TestToolsCall_TextLengthExceeded(t *testing.T) {
	bridge := &recordingBridge{respText: "should not be called"}

	cfg := testCfg()
	cfg.TextMaxLength = 5

	mux := http.NewServeMux()
	if err := Register(mux, cfg, bridge); err != nil {
		t.Fatalf("Register: %v", err)
	}

	body := jsonRPC(t, 4, "tools/call", map[string]interface{}{
		"name": "send_message",
		"arguments": map[string]interface{}{
			"text": "1234567890", // 长度 10 > 5
		},
	})
	_, _, raw := do(t, mux, "/api/v1/mcp/servers/foo/mcp/", body)

	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if len(bridge.sendCalls) != 0 {
		t.Errorf("bridge.SendMessage should NOT be called on text overflow, got %d calls, body: %s",
			len(bridge.sendCalls), raw)
	}
}

func TestMissingIdentityHeader_Returns403(t *testing.T) {
	// IdentityMiddleware 在缺少 UserKey 时直接返回 403，不会调到 bridge。
	bridge := &recordingBridge{respText: "x"}
	mux, _ := setupMux(t, bridge)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/foo/mcp/",
		strings.NewReader(jsonRPC(t, 1, "tools/list", nil)))
	r.Header.Set("Content-Type", "application/json")
	// 故意不调 validIdentityHeaders
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestNotificationsCancelled_DispatchesToBridge(t *testing.T) {
	bridge := &recordingBridge{respText: "ok"}
	mux, _ := setupMux(t, bridge)

	// MCP notification：没有 id 字段。
	body := `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"pt-abc"}}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/foo/mcp/",
		strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	validIdentityHeaders(r)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	// notification 不期望返回 JSON-RPC response（trpc-mcp-go 返回 202/200 空体）。
	if w.Code >= 400 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}

	// 给后台 dispatch 留一点时间，但实际是同步调用。
	time.Sleep(10 * time.Millisecond)

	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if len(bridge.cancelArgs) != 1 {
		t.Fatalf("bridge.Cancel call count = %d, want 1", len(bridge.cancelArgs))
	}
	if bridge.cancelArgs[0] != "pt-abc" {
		t.Errorf("cancelArgs[0] = %v, want pt-abc", bridge.cancelArgs[0])
	}
}

func TestNotificationsCancelled_MissingRequestIdIgnored(t *testing.T) {
	bridge := &recordingBridge{respText: "ok"}
	mux, _ := setupMux(t, bridge)

	body := `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{}}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/foo/mcp/",
		strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	validIdentityHeaders(r)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code >= 400 {
		t.Fatalf("status = %d", w.Code)
	}

	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if len(bridge.cancelArgs) != 0 {
		t.Errorf("bridge.Cancel should NOT be called when requestId missing, got %d calls",
			len(bridge.cancelArgs))
	}
}

func TestPathMiddleware_ExtractsMCPServerName(t *testing.T) {
	// 单测 pathMiddleware 本身：直接构造请求并断言 ctx 中 mcp_server_name。
	var seenName string
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seenName = middleware.MCPServerNameFromCtx(r.Context())
	})

	// 直接构造 ServeMux 来让 PathValue 工作
	mux := http.NewServeMux()
	mux.Handle("/mcp/servers/{mcp_server_name}/mcp/", pathMiddleware(inner))

	r := httptest.NewRequest(http.MethodPost, "/mcp/servers/finops/mcp/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if seenName != "finops" {
		t.Errorf("seenName = %q, want finops", seenName)
	}
}

func TestNoopBridge_SafeToCall(t *testing.T) {
	b := NoopBridgeHandler{}
	res, err := b.SendMessage(context.Background(),
		&SendMessageRequest{Text: "x", MCPServerName: "foo"}, nil)
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if !res.IsError {
		t.Error("NoopBridge SendMessage should return IsError=true")
	}
	// Cancel 不抛错也不 panic
	b.Cancel(context.Background(), "pt-x")
}

func TestProgressTokenMiddleware_NonToolsCallPassthrough(t *testing.T) {
	// 非 tools/call 方法不应触碰 ctx。
	called := false
	next := func(ctx context.Context, _ *mcpsdk.JSONRPCRequest) (mcpsdk.JSONRPCMessage, error) {
		called = true
		if _, ok := progressTokenFromCtx(ctx); ok {
			t.Error("non-tools/call should not set progressToken in ctx")
		}
		return nil, nil
	}
	wrapped := progressTokenMiddleware(next)
	_, _ = wrapped(context.Background(), &mcpsdk.JSONRPCRequest{
		Request: mcpsdk.Request{Method: "ping"},
		Params:  map[string]interface{}{"_meta": map[string]interface{}{"progressToken": "x"}},
	})
	if !called {
		t.Error("next handler should be called")
	}
}

func TestProgressTokenMiddleware_NilProgressTokenIgnored(t *testing.T) {
	next := func(ctx context.Context, _ *mcpsdk.JSONRPCRequest) (mcpsdk.JSONRPCMessage, error) {
		if _, ok := progressTokenFromCtx(ctx); ok {
			t.Error("nil progressToken should not be written to ctx")
		}
		return nil, nil
	}
	wrapped := progressTokenMiddleware(next)
	_, _ = wrapped(context.Background(), &mcpsdk.JSONRPCRequest{
		Request: mcpsdk.Request{Method: mcpsdk.MethodToolsCall},
		Params: map[string]interface{}{
			"_meta": map[string]interface{}{"progressToken": nil},
		},
	})
}

func TestPathPattern(t *testing.T) {
	cases := []struct {
		basePath string
		want     string
	}{
		{"/api/v1/mcp/servers", "/api/v1/mcp/servers/{mcp_server_name}/mcp/"},
		{"/api/v1/mcp/servers/", "/api/v1/mcp/servers/{mcp_server_name}/mcp/"},
		{"/", "/{mcp_server_name}/mcp/"},
	}
	for _, c := range cases {
		got := pathPattern(c.basePath)
		if got != c.want {
			t.Errorf("pathPattern(%q) = %q, want %q", c.basePath, got, c.want)
		}
	}
}
