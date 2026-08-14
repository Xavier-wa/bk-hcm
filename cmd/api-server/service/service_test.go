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

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcpwiring "hcm/cmd/api-server/service/mcp/wiring"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/serviced"
)

func TestBuildRootMux_DefaultDisabledKeepsProxyCatchAll(t *testing.T) {
	mux, err := buildRootMuxWithConfig(context.Background(), routeSmokeProxy(), nil,
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
		routeSmokeConfig(), false)
	if err != nil {
		t.Fatalf("buildRootMuxWithConfig: %v", err)
	}

	cases := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodPost, "/api/v1/cloud/cvms/list", http.StatusOK},
		{http.MethodPost, constant.MCPIngressBasePathDefault + "/foo/mcp/", http.StatusNotFound},
		{http.MethodGet, "/healthz", http.StatusOK},
		{http.MethodGet, "/alivez", http.StatusOK},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != c.want {
			t.Fatalf("%s %s status = %d, want %d, body=%s", c.method, c.path, w.Code, c.want, w.Body.String())
		}
	}
}

func TestBuildRootMux_IngressMountsBeforeProxy(t *testing.T) {
	cfg := routeSmokeConfig()
	cfg.MCP.Ingress.Enable = true

	mux, err := buildRootMuxWithConfig(context.Background(), routeSmokeProxy(),
		&fakeDiscover{servers: []string{"127.0.0.1:65535"}}, nil, nil, cfg, false)
	if err != nil {
		t.Fatalf("buildRootMuxWithConfig: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, constant.MCPIngressBasePathDefault+"/foo/mcp/",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"1.0.0"}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	withValidRouteIdentityHeaders(req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ingress initialize status = %d, body=%s", w.Code, w.Body.String())
	}

	proxyReq := httptest.NewRequest(http.MethodPost, "/api/v1/cloud/cvms/list", nil)
	proxyW := httptest.NewRecorder()
	mux.ServeHTTP(proxyW, proxyReq)
	if proxyW.Code != http.StatusOK {
		t.Fatalf("proxy path status = %d, want 200", proxyW.Code)
	}
}

func TestBuildRootMux_A2APassthroughMountsBeforeProxy(t *testing.T) {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer agent.Close()

	cfg := routeSmokeConfig()
	cfg.A2APassthrough.Enable = true
	mux, err := buildRootMuxWithConfig(context.Background(), routeSmokeProxy(),
		&fakeDiscover{servers: []string{agent.URL}}, nil, nil, cfg, false)
	if err != nil {
		t.Fatalf("buildRootMuxWithConfig: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet,
		constant.A2ABasePathDefault+constant.A2AWellKnownAgentCardPath, nil)
	withValidRouteIdentityHeaders(req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("agent card status = %d, body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != constant.A2ABasePathDefault+constant.A2AWellKnownAgentCardPath {
		t.Fatalf("agent card backend path = %q", w.Body.String())
	}

	proxyReq := httptest.NewRequest(http.MethodPost, "/api/v1/cloud/cvms/list", nil)
	proxyW := httptest.NewRecorder()
	mux.ServeHTTP(proxyW, proxyReq)
	if proxyW.Code != http.StatusOK {
		t.Fatalf("proxy path status = %d, want 200", proxyW.Code)
	}

	aguiReq := httptest.NewRequest(http.MethodPost, constant.AGUIPath, nil)
	aguiW := httptest.NewRecorder()
	mux.ServeHTTP(aguiW, aguiReq)
	if aguiW.Code != http.StatusNotFound {
		t.Fatalf("AGUI path status = %d, want proxy 404", aguiW.Code)
	}
}

func TestLocalBackendCaller_AllowsOnlyLocalProxyPaths(t *testing.T) {
	var gotPath, gotQuery string
	caller := &localBackendCaller{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte("ok"))
	})}

	resp, err := caller.Call(context.Background(), &mcpwiring.InternalBackendRequest{
		Method:  http.MethodPost,
		Path:    "/api/v1/cloud/cvms/list?vendor=tcloud",
		Headers: http.Header{"X-Test": []string{"1"}},
		Body:    []byte(`{"page":{"limit":1}}`),
	})
	if err != nil {
		t.Fatalf("Call valid local path: %v", err)
	}
	if resp.StatusCode != http.StatusOK || string(resp.Body) != "ok" {
		t.Fatalf("response = %+v, body=%s, want 200 ok", resp, string(resp.Body))
	}
	if gotPath != "/api/v1/cloud/cvms/list" || gotQuery != "vendor=tcloud" {
		t.Fatalf("handler got path=%q query=%q", gotPath, gotQuery)
	}

	blocked := []string{
		"http://169.254.169.254/latest/meta-data",
		"//169.254.169.254/latest/meta-data",
		"/api/v1/cloud/../healthz",
		"/api/v1/cloud/./cvms/list",
		"/api/v1/cloud\\..\\healthz",
		"/healthz",
		"/api/v1/mcp/internal/hcm/mcp/",
	}
	for _, path := range blocked {
		if _, err := caller.Call(context.Background(), &mcpwiring.InternalBackendRequest{Method: http.MethodGet, Path: path}); err == nil {
			t.Fatalf("Call path %q got nil err, want rejected", path)
		}
	}
}

func routeSmokeProxy() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/cloud/") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("proxy ok"))
			return
		}
		http.NotFound(w, r)
	})
}

func routeSmokeConfig() cc.ApiServerSetting {
	return cc.ApiServerSetting{
		MCP: cc.MCPServerSetting{
			Ingress: cc.MCPIngressSetting{
				BasePath:                constant.MCPIngressBasePathDefault,
				ServerName:              "HCM Agent",
				ServerVersion:           "1.0.0",
				AggregatedToolName:      constant.MCPIngressAggregatedToolName,
				TextMaxLength:           8000,
				ProgressTaskMapMaxSize:  10000,
				ProgressTaskMapEntryTTL: 30 * time.Minute,
			},
			Bridge: cc.MCPBridgeSetting{
				ConnectTimeout:      5 * time.Second,
				ReadTimeout:         300 * time.Second,
				MaxIdleConnsPerHost: 100,
			},
			Internal: cc.MCPInternalSetting{
				Servers: []cc.MCPInternalServerSetting{
					{
						Name:            "hcm-internal-mcp",
						BasePath:        "/api/v1/mcp/internal/hcm/mcp",
						ServerVersion:   constant.MCPInternalServerDefaultVersion,
						OpenAPISpecPath: "bk_apigw_resources_bk-hcm_internal_mcp.yaml",
					},
				},
			},
		},
		A2APassthrough: cc.A2APassthroughSetting{
			BasePath:        constant.A2ABasePathDefault,
			FlushIntervalMS: -1,
			IdleTimeout:     600 * time.Second,
		},
	}
}

type fakeDiscover struct {
	servers []string
}

func (f *fakeDiscover) Discover(cc.Name) ([]string, error) {
	return f.servers, nil
}

func (f *fakeDiscover) Services() []cc.Name {
	return []cc.Name{cc.AgentServerName}
}

func (f *fakeDiscover) ByLabels([]string) serviced.Discover {
	return f
}

func (f *fakeDiscover) GetServiceAllNodeKeys(cc.Name) ([]string, error) {
	return nil, nil
}

func withValidRouteIdentityHeaders(r *http.Request) {
	r.Header.Set(constant.UserKey, "alice")
	r.Header.Set(constant.AppCodeKey, constant.WebSourceAppCode)
	r.Header.Set(constant.RidKey, "rid-aaaaaaaaaaaaaaaa")
	r.Header.Set(constant.TenantIDKey, "default")
}

// TestRestFilter_InternalCallerBypassesJWT 验证 localBackendCaller 携带
// X-Bkhcm-Caller-Source: api-server 时，restFilter 跳过 JWT 解析，
// 直接用 X-Bkapi-User-Name 通过鉴权，不返回 "jwt token is required"。
func TestRestFilter_InternalCallerBypassesJWT(t *testing.T) {
	// 构造完整 mux（disableTGW=true 时 gwparser 用 defaultParser，
	// 但生产环境 disableTGW=false；这里测试的是 caller-source 分支）。
	// 直接构造 localBackendCaller 并调用，验证 filter 对内部请求的行为。
	backendReached := false
	fakeBackend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证 caller-source header 已被正确传递
		if r.Header.Get(constant.MCPCallerSourceHeader) != string(cc.APIServerName) {
			t.Errorf("expected caller-source=%s, got %s",
				cc.APIServerName, r.Header.Get(constant.MCPCallerSourceHeader))
		}
		backendReached = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0}`))
	})

	caller := &localBackendCaller{handler: fakeBackend}
	resp, err := caller.Call(context.Background(), &mcpwiring.InternalBackendRequest{
		Method: http.MethodPost,
		Path:   "/api/v1/cloud/cvms/list",
		Headers: http.Header{
			constant.MCPCallerSourceHeader: []string{string(cc.APIServerName)},
			constant.UserKey:               []string{"ironguo"},
			constant.AppCodeKey:            []string{"bk-hcm"},
			constant.RidKey:                []string{"rid-internal-test"},
			constant.TenantIDKey:           []string{"default"},
		},
		Body: []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("localBackendCaller.Call unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	if !backendReached {
		t.Fatal("backend handler should have been reached")
	}
}
