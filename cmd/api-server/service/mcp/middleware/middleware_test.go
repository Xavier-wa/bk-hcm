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

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"
)

// withValidHeaders 设置一组通过 gwparser.defaultParser 的最小合法 header。
// defaultParser 要求 User 非空且 Rid 长度在 16~50；AppCode 使用
// constant.WebSourceAppCode 是为了在测试环境下跳过 kit.Validate 中
// cc.TenantEnable() 这一项（cc.rt 在测试中未初始化，访问会 panic）。
func withValidHeaders(r *http.Request) {
	r.Header.Set(constant.UserKey, "alice")
	r.Header.Set(constant.AppCodeKey, constant.WebSourceAppCode)
	r.Header.Set(constant.RidKey, "rid-aaaaaaaaaaaaaaaa") // 20 字符，满足长度约束
	r.Header.Set(constant.TenantIDKey, "default")
}

func TestParseFromHeader_HappyPath(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/foo/mcp/", nil)
	withValidHeaders(r)

	kt, err := ParseFromHeader(r.Context(), r.Header)
	if err != nil {
		t.Fatalf("ParseFromHeader unexpected error: %v", err)
	}
	if kt.User != "alice" {
		t.Errorf("kt.User = %q, want alice", kt.User)
	}
	if kt.AppCode != constant.WebSourceAppCode {
		t.Errorf("kt.AppCode = %q, want %s", kt.AppCode, constant.WebSourceAppCode)
	}
	if kt.TenantID != "default" {
		t.Errorf("kt.TenantID = %q, want default", kt.TenantID)
	}
}

func TestParseFromHeader_MissingUser(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(constant.AppCodeKey, constant.WebSourceAppCode)
	r.Header.Set(constant.RidKey, "rid-aaaaaaaaaaaaaaaa")

	if _, err := ParseFromHeader(r.Context(), r.Header); err == nil {
		t.Error("expected error on missing UserKey, got nil")
	}
}

func TestWithKitRoundTrip(t *testing.T) {
	kt := &kit.Kit{User: "alice", Rid: "rid-xxxxxxxxxxxxxxxx"}
	ctx := WithKit(context.Background(), kt)

	got, ok := KitFromCtx(ctx)
	if !ok {
		t.Fatal("KitFromCtx returned ok=false after WithKit")
	}
	if got.User != "alice" {
		t.Errorf("got.User = %q, want alice", got.User)
	}
}

func TestWithKit_NilNoop(t *testing.T) {
	ctx := WithKit(context.Background(), nil)
	if _, ok := KitFromCtx(ctx); ok {
		t.Error("KitFromCtx should return ok=false when WithKit got nil kit")
	}
}

func TestMCPServerNameRoundTrip(t *testing.T) {
	ctx := WithMCPServerName(context.Background(), "bk-hcm-finops")
	if got := MCPServerNameFromCtx(ctx); got != "bk-hcm-finops" {
		t.Errorf("MCPServerNameFromCtx = %q, want bk-hcm-finops", got)
	}
}

func TestMCPServerName_EmptyNoop(t *testing.T) {
	ctx := WithMCPServerName(context.Background(), "")
	if got := MCPServerNameFromCtx(ctx); got != "" {
		t.Errorf("MCPServerNameFromCtx = %q, want empty", got)
	}
}

func TestIdentityHTTPContextFunc_OK(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	withValidHeaders(r)

	ctx := IdentityHTTPContextFunc(context.Background(), r)
	kt, ok := KitFromCtx(ctx)
	if !ok {
		t.Fatal("KitFromCtx returned ok=false; expected kit set by context func")
	}
	if kt.User != "alice" {
		t.Errorf("kt.User = %q, want alice", kt.User)
	}
}

func TestIdentityHTTPContextFunc_ParseFailReturnsOriginalCtx(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	// 故意不设 UserKey，触发 defaultParser 校验失败

	ctx := IdentityHTTPContextFunc(context.Background(), r)
	if _, ok := KitFromCtx(ctx); ok {
		t.Error("expected no kit in ctx when parse fails")
	}
}

func TestIdentityMiddleware_HappyPath(t *testing.T) {
	var seenUser string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		kt, ok := KitFromCtx(r.Context())
		if !ok {
			t.Fatal("downstream handler: kit not in ctx")
		}
		seenUser = kt.User
	})

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	withValidHeaders(r)
	w := httptest.NewRecorder()
	IdentityMiddleware(next).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if seenUser != "alice" {
		t.Errorf("downstream saw user %q, want alice", seenUser)
	}
}

func TestIdentityMiddleware_RejectsMissingUser(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(constant.AppCodeKey, constant.WebSourceAppCode)
	w := httptest.NewRecorder()
	IdentityMiddleware(next).ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
	if called {
		t.Error("downstream handler should NOT be called on identity parse failure")
	}
}

func TestRequireCallerSource(t *testing.T) {
	cases := []struct {
		name     string
		expected string
		got      string
		wantErr  bool
	}{
		{"empty expected disables check", "", "anything", false},
		{"match passes", "agent-server", "agent-server", false},
		{"mismatch rejects", "agent-server", "api-server", true},
		{"missing header rejects", "agent-server", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			if tc.got != "" {
				h.Set(constant.MCPCallerSourceHeader, tc.got)
			}
			err := RequireCallerSource(h, tc.expected)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
			if tc.wantErr && err != nil && !errors.Is(err, ErrCallerSourceMismatch) {
				t.Errorf("expected ErrCallerSourceMismatch, got %v", err)
			}
		})
	}
}

func TestCallerSourceMiddleware_Pass(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(constant.MCPCallerSourceHeader, "agent-server")
	w := httptest.NewRecorder()
	CallerSourceMiddleware("agent-server", next).ServeHTTP(w, r)

	if !called {
		t.Error("downstream handler should be called when caller source matches")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestCallerSourceMiddleware_Reject(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(constant.MCPCallerSourceHeader, "api-server")
	w := httptest.NewRecorder()
	CallerSourceMiddleware("agent-server", next).ServeHTTP(w, r)

	if called {
		t.Error("downstream handler should NOT be called on caller source mismatch")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestCallerSourceMiddleware_EmptyExpectedDisablesCheck(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})

	r := httptest.NewRequest(http.MethodPost, "/", nil) // 无任何 caller source header
	w := httptest.NewRecorder()
	CallerSourceMiddleware("", next).ServeHTTP(w, r)

	if !called {
		t.Error("downstream handler should be called when expected is empty")
	}
}

func TestInjectInternalHeaders_AllFields(t *testing.T) {
	kt := &kit.Kit{
		User:     "alice",
		AppCode:  "hcm-test",
		TenantID: "default",
		Rid:      "rid-xxxxxxxxxxxxxxxx",
	}
	h := http.Header{}
	InjectInternalHeaders(kt, h, "api-server")

	if got := h.Get(constant.UserKey); got != "alice" {
		t.Errorf("UserKey = %q, want alice", got)
	}
	if got := h.Get(constant.AppCodeKey); got != "hcm-test" {
		t.Errorf("AppCodeKey = %q, want hcm-test", got)
	}
	if got := h.Get(constant.TenantIDKey); got != "default" {
		t.Errorf("TenantIDKey = %q, want default", got)
	}
	if got := h.Get(constant.RidKey); got != "rid-xxxxxxxxxxxxxxxx" {
		t.Errorf("RidKey = %q, want rid-xxxxxxxxxxxxxxxx", got)
	}
	if got := h.Get(constant.MCPCallerSourceHeader); got != "api-server" {
		t.Errorf("MCPCallerSourceHeader = %q, want api-server", got)
	}
}

func TestInjectInternalHeaders_GenerateRidWhenMissing(t *testing.T) {
	kt := &kit.Kit{User: "alice", AppCode: "hcm-test"}
	h := http.Header{}
	InjectInternalHeaders(kt, h, "api-server")

	rid := h.Get(constant.RidKey)
	if rid == "" {
		t.Fatal("RidKey should be auto-generated when kit.Rid is empty")
	}
	if len(rid) < 8 {
		t.Errorf("auto-generated rid too short: %q", rid)
	}
}

func TestInjectInternalHeaders_OverwriteExistingHeaders(t *testing.T) {
	kt := &kit.Kit{User: "alice", Rid: "rid-xxxxxxxxxxxxxxxx"}
	h := http.Header{}
	h.Set(constant.UserKey, "evil-spoof")              // 上游伪造
	h.Set(constant.MCPCallerSourceHeader, "openclaw") // 上游伪造来源

	InjectInternalHeaders(kt, h, "api-server")

	if got := h.Get(constant.UserKey); got != "alice" {
		t.Errorf("UserKey should be overwritten to alice, got %q", got)
	}
	if got := h.Get(constant.MCPCallerSourceHeader); got != "api-server" {
		t.Errorf("MCPCallerSourceHeader should be overwritten to api-server, got %q", got)
	}
}

func TestInjectInternalHeaders_NilKtSetsRidAndCallerSource(t *testing.T) {
	h := http.Header{}
	InjectInternalHeaders(nil, h, "api-server")

	if h.Get(constant.RidKey) == "" {
		t.Error("RidKey should be auto-generated when kt is nil")
	}
	if got := h.Get(constant.MCPCallerSourceHeader); got != "api-server" {
		t.Errorf("MCPCallerSourceHeader = %q, want api-server", got)
	}
	if got := h.Get(constant.UserKey); got != "" {
		t.Errorf("UserKey should remain empty when kt is nil, got %q", got)
	}
}

func TestInjectInternalHeaders_NilDestNoop(t *testing.T) {
	// 仅验证不 panic。
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("InjectInternalHeaders with nil dest panicked: %v", r)
		}
	}()
	InjectInternalHeaders(&kit.Kit{User: "alice"}, nil, "api-server")
}

func TestInjectInternalHeaders_EmptyCallerSourceSkipsHeader(t *testing.T) {
	kt := &kit.Kit{User: "alice", Rid: "rid-xxxxxxxxxxxxxxxx"}
	h := http.Header{}
	InjectInternalHeaders(kt, h, "")
	if got := h.Get(constant.MCPCallerSourceHeader); got != "" {
		t.Errorf("empty callerSource should not set header, got %q", got)
	}
}

func TestWriteForbiddenContent(t *testing.T) {
	w := httptest.NewRecorder()
	writeForbidden(w, "test reason")

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(w.Body.String(), "test reason") {
		t.Errorf("body does not contain reason: %s", w.Body.String())
	}
}
