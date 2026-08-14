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

package a2apass

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/serviced"
)

func TestBuildPassthroughHandler_ProxyBodyAndHeaders(t *testing.T) {
	requestBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"message/stream"}`)
	var gotBody []byte
	var gotPath string
	var gotCallerSource string
	var gotUser string

	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body failed: %v", err)
		}
		gotPath = r.URL.Path
		gotCallerSource = r.Header.Get(constant.MCPCallerSourceHeader)
		gotUser = r.Header.Get(constant.UserKey)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(gotBody)
	}))
	defer agent.Close()

	handler := buildPassthroughHandlerWithDiscovery(cc.A2APassthroughSetting{},
		&fakeDiscovery{servers: []string{agent.URL}})
	req := httptest.NewRequest(http.MethodPost, constant.A2ABasePathDefault+constant.A2AJSONRPCSubPath,
		bytes.NewReader(requestBody))
	withValidIdentityHeaders(req)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if !bytes.Equal(gotBody, requestBody) {
		t.Fatalf("proxied body = %s, want %s", string(gotBody), string(requestBody))
	}
	if !bytes.Equal(w.Body.Bytes(), requestBody) {
		t.Fatalf("response body = %s, want %s", w.Body.String(), string(requestBody))
	}
	if gotPath != constant.A2ABasePathDefault+constant.A2AJSONRPCSubPath {
		t.Fatalf("backend path = %q", gotPath)
	}
	if gotCallerSource != string(cc.APIServerName) {
		t.Fatalf("caller source = %q, want %q", gotCallerSource, cc.APIServerName)
	}
	if gotUser != "alice" {
		t.Fatalf("user header = %q, want alice", gotUser)
	}
}

func TestBuildPassthroughHandler_PreservesSSEStreaming(t *testing.T) {
	firstFrameWritten := make(chan struct{})
	releaseSecondFrame := make(chan struct{})

	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer is not flusher")
		}
		_, _ = w.Write([]byte("data: first\n\n"))
		flusher.Flush()
		close(firstFrameWritten)
		<-releaseSecondFrame
		_, _ = w.Write([]byte("data: second\n\n"))
		flusher.Flush()
	}))
	defer agent.Close()

	handler := buildPassthroughHandlerWithDiscovery(cc.A2APassthroughSetting{FlushIntervalMS: -1},
		&fakeDiscovery{servers: []string{agent.URL}})
	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	req, err := http.NewRequest(http.MethodGet, proxyServer.URL+constant.A2ABasePathDefault+constant.A2AJSONRPCSubPath, nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	withValidIdentityHeaders(req)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request failed: %v", err)
	}
	defer resp.Body.Close()

	select {
	case <-firstFrameWritten:
	case <-time.After(time.Second):
		t.Fatal("backend did not write first frame")
	}

	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read first SSE line failed: %v", err)
	}
	if line != "data: first\n" {
		t.Fatalf("first line = %q", line)
	}

	close(releaseSecondFrame)
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read frame separator failed: %v", err)
	}
	if line != "\n" {
		t.Fatalf("separator = %q", line)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read second SSE line failed: %v", err)
	}
	if line != "data: second\n" {
		t.Fatalf("second line = %q", line)
	}
}

func TestBuildPassthroughHandler_RejectsMissingIdentity(t *testing.T) {
	called := false
	agent := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer agent.Close()

	handler := buildPassthroughHandlerWithDiscovery(cc.A2APassthroughSetting{},
		&fakeDiscovery{servers: []string{agent.URL}})
	req := httptest.NewRequest(http.MethodPost, constant.A2ABasePathDefault+constant.A2AJSONRPCSubPath,
		strings.NewReader("{}"))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if called {
		t.Fatal("backend should not be called when identity is missing")
	}
}

func TestRegisterMountsPaths(t *testing.T) {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer agent.Close()

	mux := http.NewServeMux()
	err := Register(mux, cc.A2APassthroughSetting{Enable: true}, &fakeServicedDiscover{servers: []string{agent.URL}})
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}

	for _, path := range []string{
		constant.A2ABasePathDefault + constant.A2AJSONRPCSubPath,
		constant.A2ABasePathDefault + constant.A2AWellKnownAgentCardPath,
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		withValidIdentityHeaders(req)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("path %s status = %d, body=%s", path, w.Code, w.Body.String())
		}
		if w.Body.String() != path {
			t.Fatalf("path %s response = %q", path, w.Body.String())
		}
	}
}

func TestRegisterDisabledAndNilMux(t *testing.T) {
	if err := Register(nil, cc.A2APassthroughSetting{}, nil); err == nil {
		t.Fatal("expected nil mux error")
	}

	mux := http.NewServeMux()
	if err := Register(mux, cc.A2APassthroughSetting{Enable: false}, nil); err != nil {
		t.Fatalf("disabled Register error: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, constant.A2ABasePathDefault+constant.A2AJSONRPCSubPath, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestSelectAgentServer(t *testing.T) {
	u, err := selectAgentServer(&fakeDiscovery{servers: []string{"127.0.0.1:8080"}})
	if err != nil {
		t.Fatalf("selectAgentServer error: %v", err)
	}
	if u.String() != "http://127.0.0.1:8080" {
		t.Fatalf("url = %q", u.String())
	}
}

type fakeDiscovery struct {
	servers []string
	err     error
}

func (f *fakeDiscovery) GetServers() ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.servers, nil
}

type fakeServicedDiscover struct {
	servers []string
}

func (f *fakeServicedDiscover) Discover(cc.Name) ([]string, error) {
	return f.servers, nil
}

func (f *fakeServicedDiscover) Services() []cc.Name {
	return []cc.Name{cc.AgentServerName}
}

func (f *fakeServicedDiscover) ByLabels([]string) serviced.Discover {
	return f
}

func (f *fakeServicedDiscover) GetServiceAllNodeKeys(cc.Name) ([]string, error) {
	return nil, nil
}

func withValidIdentityHeaders(r *http.Request) {
	r.Header.Set(constant.UserKey, "alice")
	r.Header.Set(constant.AppCodeKey, constant.WebSourceAppCode)
	r.Header.Set(constant.RidKey, "rid-aaaaaaaaaaaaaaaa")
	r.Header.Set(constant.TenantIDKey, "default")
}
