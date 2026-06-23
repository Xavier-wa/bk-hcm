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

package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// withoutPanicRecovery 关闭最外层 panic recovery，仅供单测使用。
func withoutPanicRecovery() HandlerOption {
	return func(o *handlerOptions) { o.disablePanic = true }
}

// withoutRequestLog 关闭请求级日志，仅供单测使用，避免日志干扰断言。
func withoutRequestLog() HandlerOption {
	return func(o *handlerOptions) { o.disableReqLog = true }
}

// buildTestHandler 构造一个最小化的 MCP HTTP handler：无业务工具注册，
// 默认开启 panic recovery + request log（与生产路径一致），用以测试
// SDK 默认协议行为与本包包装层的边界行为。
func buildTestHandler(t *testing.T, opts ...HandlerOption) http.Handler {
	t.Helper()
	srv := BuildBaseServer("hcm-mcp-test", "0.0.0-test", "mcp/test")
	return NewMCPHTTPHandler(srv, opts...)
}

// doJSONRPC 发送一次 JSON-RPC over HTTP 请求并解码响应。
func doJSONRPC(t *testing.T, h http.Handler, body string) (status int, resp map[string]interface{}, raw string) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/foo/mcp/",
		strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	respBytes, _ := io.ReadAll(w.Body)
	raw = string(respBytes)

	// 非 JSON 响应（如非法 JSON 路径返回纯文本错误）允许 resp = nil。
	_ = json.Unmarshal(respBytes, &resp)
	return w.Code, resp, raw
}

func TestInitialize_ReturnsServerInfo(t *testing.T) {
	h := buildTestHandler(t)

	body := `{
		"jsonrpc":"2.0",
		"id":1,
		"method":"initialize",
		"params":{
			"protocolVersion":"2025-03-26",
			"clientInfo":{"name":"unit-test","version":"0.0"},
			"capabilities":{}
		}
	}`

	status, resp, raw := doJSONRPC(t, h, body)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, raw)
	}
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc field = %v, want 2.0", resp["jsonrpc"])
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result not a map: %s", raw)
	}
	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("result.serverInfo not a map: %s", raw)
	}
	if serverInfo["name"] != "hcm-mcp-test" {
		t.Errorf("serverInfo.name = %v, want hcm-mcp-test", serverInfo["name"])
	}
	if serverInfo["version"] != "0.0.0-test" {
		t.Errorf("serverInfo.version = %v, want 0.0.0-test", serverInfo["version"])
	}
	if result["protocolVersion"] == nil {
		t.Error("result.protocolVersion should be present")
	}
}

func TestPing_ReturnsEmptyResult(t *testing.T) {
	h := buildTestHandler(t)
	status, resp, raw := doJSONRPC(t, h,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`)

	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, raw)
	}
	if _, hasResult := resp["result"]; !hasResult {
		t.Errorf("expected result field on ping response, got: %s", raw)
	}
	if _, hasError := resp["error"]; hasError {
		t.Errorf("unexpected error field on ping response: %s", raw)
	}
}

func TestToolsList_DefaultEmpty(t *testing.T) {
	h := buildTestHandler(t)
	status, resp, raw := doJSONRPC(t, h,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)

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
	if len(tools) != 0 {
		t.Errorf("expected empty tools list, got %d items: %s", len(tools), raw)
	}
}

func TestPromptsList_DefaultEmpty(t *testing.T) {
	h := buildTestHandler(t)
	status, resp, raw := doJSONRPC(t, h,
		`{"jsonrpc":"2.0","id":4,"method":"prompts/list"}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, raw)
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result not a map: %s", raw)
	}
	prompts, ok := result["prompts"].([]interface{})
	if !ok {
		t.Fatalf("result.prompts not an array: %s", raw)
	}
	if len(prompts) != 0 {
		t.Errorf("expected empty prompts list, got %d items: %s", len(prompts), raw)
	}
}

func TestResourcesList_DefaultEmpty(t *testing.T) {
	h := buildTestHandler(t)
	status, resp, raw := doJSONRPC(t, h,
		`{"jsonrpc":"2.0","id":5,"method":"resources/list"}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, raw)
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result not a map: %s", raw)
	}
	resources, ok := result["resources"].([]interface{})
	if !ok {
		t.Fatalf("result.resources not an array: %s", raw)
	}
	if len(resources) != 0 {
		t.Errorf("expected empty resources list, got %d items: %s", len(resources), raw)
	}
}

func TestUnknownMethod_ReturnsMethodNotFound(t *testing.T) {
	h := buildTestHandler(t)
	status, resp, raw := doJSONRPC(t, h,
		`{"jsonrpc":"2.0","id":99,"method":"this/does/not/exist"}`)

	if status != http.StatusOK {
		t.Fatalf("JSON-RPC method-not-found should still be HTTP 200, got %d, body: %s",
			status, raw)
	}
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object, got: %s", raw)
	}
	// JSON 数字反序列化为 float64
	if code, _ := errObj["code"].(float64); int(code) != -32601 {
		t.Errorf("error.code = %v, want -32601", errObj["code"])
	}
}

func TestInvalidJSON_ReturnsHTTP400(t *testing.T) {
	h := buildTestHandler(t)
	// 故意发非法 JSON。trpc-mcp-go transport 层在 json.Decoder.Decode 失败时
	// 直接返回 HTTP 400 + 纯文本错误，而不是 JSON-RPC -32700。
	// 我们的中间件层不应吞掉该响应。
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/foo/mcp/",
		strings.NewReader(`{not-json`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body: %s", w.Code, w.Body.String())
	}
}

func TestNonPostMethod_NotAllowed(t *testing.T) {
	h := buildTestHandler(t)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/mcp/servers/foo/mcp/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT status = %d, want 405", w.Code)
	}
}

func TestWithMiddleware_AppliedInOrder(t *testing.T) {
	// 验证 WithMiddleware 注入的中间件确实生效，且按"先注册先执行"顺序。
	order := []string{}
	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw1-after")
		})
	}
	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw2-after")
		})
	}

	h := buildTestHandler(t,
		WithMiddleware(mw1), // 外层
		WithMiddleware(mw2), // 内层
		withoutRequestLog(),
		withoutPanicRecovery(),
	)

	_, _, _ = doJSONRPC(t, h, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)

	want := []string{"mw1-before", "mw2-before", "mw2-after", "mw1-after"}
	if len(order) != len(want) {
		t.Fatalf("call order length = %d, want %d: %v", len(order), len(want), order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %s, want %s; full: %v", i, order[i], want[i], order)
		}
	}
}

func TestNilMiddlewareIgnored(t *testing.T) {
	// 防御性测试：WithMiddleware(nil) 不应 panic 也不应破坏 chain。
	h := buildTestHandler(t, WithMiddleware(nil))
	status, _, raw := doJSONRPC(t, h, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	if status != http.StatusOK {
		t.Errorf("status = %d, body: %s", status, raw)
	}
}

func TestPanicRecovery_Returns500(t *testing.T) {
	// 注入一个一定会 panic 的中间件，验证最外层 panic recovery 兜底 500 而不是
	// 让 panic 冒泡到 httptest（默认会让 connection abort）。
	panicMw := func(_ http.Handler) http.Handler {
		return http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			panic("test panic from middleware")
		})
	}
	h := buildTestHandler(t, WithMiddleware(panicMw))

	r := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/foo/mcp/",
		bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	// recoveryMiddleware 会捕获 panic 而不是把它抛回到这里。
	h.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestPanicRecovery_DisabledLetsPanicPropagate(t *testing.T) {
	// 验证 WithoutPanicRecovery 选项确实关闭兜底，把 panic 抛出。
	panicMw := func(_ http.Handler) http.Handler {
		return http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			panic("should propagate")
		})
	}
	h := buildTestHandler(t,
		WithMiddleware(panicMw),
		withoutPanicRecovery(),
		withoutRequestLog(), // 减少噪音
	)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to propagate with WithoutPanicRecovery, got nil")
		}
	}()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{}`)))
	h.ServeHTTP(httptest.NewRecorder(), r)
}

func TestStatusCapturingWriter_Flush(t *testing.T) {
	// 单测最小验证 statusCapturingWriter 透传 Flush 与不重复 WriteHeader。
	inner := httptest.NewRecorder()
	w := &statusCapturingWriter{ResponseWriter: inner, status: http.StatusOK}

	w.WriteHeader(http.StatusAccepted)
	w.WriteHeader(http.StatusTeapot) // 二次调用应被忽略

	if w.status != http.StatusAccepted {
		t.Errorf("w.status = %d, want %d", w.status, http.StatusAccepted)
	}
	if inner.Code != http.StatusAccepted {
		t.Errorf("inner.Code = %d, want %d (二次 WriteHeader 不应覆盖)",
			inner.Code, http.StatusAccepted)
	}
	if !w.wroteHeader {
		t.Error("wroteHeader should be true after WriteHeader call")
	}

	// Flush no-panic：httptest.ResponseRecorder 实现了 http.Flusher
	w.Flush()
}

func TestLoggerAdapterDoesNotPanicOnUsage(t *testing.T) {
	// 端到端确认 logger 适配器在各级别下都能被 trpc-mcp-go 安全调用。
	// SDK 在 initialize 处理路径会通过 Logger 输出 info 日志，不应崩溃。
	h := buildTestHandler(t)
	doJSONRPC(t, h, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"x","version":"y"},"capabilities":{}}}`)
}
