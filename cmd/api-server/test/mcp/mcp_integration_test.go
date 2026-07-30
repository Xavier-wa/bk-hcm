/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"hcm/cmd/api-server/service/mcp/bridge"
	"hcm/cmd/api-server/service/mcp/ingress"
	mcpwiring "hcm/cmd/api-server/service/mcp/wiring"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/serviced"

	"trpc.group/trpc-go/trpc-a2a-go/protocol"
)

const integrationSpec = `openapi: 3.0.1
paths:
  /api/v1/cloud/cvms/list:
    post:
      operationId: list_cvms
      description: 查询 CVM 列表
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [page]
              properties:
                page:
                  type: object
                filter:
                  type: object
      x-bk-apigateway-resource:
        backend:
          method: post
          path: "{env.url_path_prefix}/api/v1/cloud/cvms/list"
`

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

func TestNorthboundMCPIngressToFakeA2AAgent(t *testing.T) {
	var gotA2APath string
	var gotCallerSource string
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotA2APath = r.URL.Path
		gotCallerSource = r.Header.Get(constant.MCPCallerSourceHeader)

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		writeSSE(t, w, statusEvent("task-1", "ctx-1", protocol.TaskStateWorking, "working", false))
		writeSSE(t, w, artifactEvent("task-1", "ctx-1", "hello from fake agent"))
		writeSSE(t, w, statusEvent("task-1", "ctx-1", protocol.TaskStateCompleted, "done", true))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer agent.Close()

	cfg := ingressCfg()
	bridgeHandler, err := bridge.NewHandler(cc.MCPBridgeSetting{}, cfg, &fakeDiscover{servers: []string{agent.URL}})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	mux := http.NewServeMux()
	if err := ingress.Register(mux, cfg, bridgeHandler); err != nil {
		t.Fatalf("ingress.Register: %v", err)
	}
	gateway := fakeGatewayMCPProxy(mux)

	resp := doGatewayMCP(t, gateway, "/api/v2/mcp-servers/foo/mcp/", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "send_message",
			"arguments": map[string]interface{}{
				"text":      "hello",
				"contextId": "ctx-1",
			},
		},
	})

	if gotA2APath != constant.A2ABasePathDefault+constant.A2AJSONRPCSubPath {
		t.Fatalf("A2A path = %q", gotA2APath)
	}
	if gotCallerSource != string(cc.APIServerName) {
		t.Fatalf("caller source = %q, want api-server", gotCallerSource)
	}
	if text := mcpResultText(resp); !strings.Contains(text, "hello from fake agent") {
		t.Fatalf("MCP result text = %q", text)
	}
}

func TestSouthboundInternalMCPToFakeBackend(t *testing.T) {
	mux := http.NewServeMux()
	cfg := internalCfg(t)
	dispatchers, err := mcpwiring.RegisterInternalServers(context.Background(), mux, cfg)
	if err != nil {
		t.Fatalf("RegisterInternalServers: %v", err)
	}
	if len(dispatchers) != 1 {
		t.Fatalf("dispatchers count = %d, want 1", len(dispatchers))
	}
	dispatcher := dispatchers[0]
	caller := &recordingBackendCaller{body: []byte(`{"code":0,"data":{"count":1}}`)}
	dispatcher.SetBackendCaller(caller, 0)

	resp := doInternalMCP(t, mux, "/api/v1/mcp/internal/hcm-resource/mcp/", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "list_cvms",
			"arguments": map[string]interface{}{
				"page": map[string]interface{}{"limit": 10},
			},
		},
	})

	if caller.path != "/api/v1/cloud/cvms/list" {
		t.Fatalf("backend path = %q", caller.path)
	}
	if caller.user != "alice" {
		t.Fatalf("backend user = %q", caller.user)
	}
	if text := mcpResultText(resp); !strings.Contains(text, `"count":1`) {
		t.Fatalf("MCP result text = %q", text)
	}
}

func TestMCPFakeBackendPerformanceBudget(t *testing.T) {
	cfg := ingressCfg()
	mux := http.NewServeMux()
	if err := ingress.Register(mux, cfg, ingress.NoopBridgeHandler{}); err != nil {
		t.Fatalf("ingress.Register: %v", err)
	}
	gateway := fakeGatewayMCPProxy(mux)

	toolsListLatencies := runConcurrent(100, func() {
		_ = doGatewayMCP(t, gateway, "/api/v2/mcp-servers/foo/mcp/", map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/list",
		})
	})
	if p99 := percentile(toolsListLatencies, 0.99); p99 > 100*time.Millisecond {
		t.Fatalf("tools/list p99 = %s, want <= 100ms", p99)
	}

	callLatencies := runConcurrent(100, func() {
		_ = doGatewayMCP(t, gateway, "/api/v2/mcp-servers/foo/mcp/", map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "send_message",
				"arguments": map[string]interface{}{
					"text":      "hello",
					"contextId": "ctx-perf",
				},
			},
		})
	})
	if p99 := percentile(callLatencies, 0.99); p99 > 3*time.Second {
		t.Fatalf("tools/call p99 = %s, want <= 3s", p99)
	}
}

func ingressCfg() cc.MCPIngressSetting {
	return cc.MCPIngressSetting{
		Enable:                  true,
		BasePath:                constant.MCPIngressBasePathDefault,
		ServerName:              "HCM Agent",
		ServerVersion:           "1.0.0",
		AggregatedToolName:      constant.MCPIngressAggregatedToolName,
		TextMaxLength:           8000,
		ProgressTaskMapMaxSize:  100,
		ProgressTaskMapEntryTTL: time.Minute,
	}
}

func internalCfg(t *testing.T) cc.MCPInternalSetting {
	t.Helper()
	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(integrationSpec), 0644); err != nil {
		t.Fatalf("write openapi spec: %v", err)
	}
	return cc.MCPInternalSetting{
		Enable:              true,
		EnforceCallerSource: true,
		Servers: []cc.MCPInternalServerSetting{
			{
				Name:            "hcm-resource-mcp",
				BasePath:        "/api/v1/mcp/internal/hcm-resource/mcp",
				ServerVersion:   constant.MCPInternalServerDefaultVersion,
				OpenAPISpecPath: specPath,
			},
		},
	}
}

type recordingBackendCaller struct {
	method string
	path   string
	user   string
	body   []byte
}

func (c *recordingBackendCaller) Call(_ context.Context, req *mcpwiring.InternalBackendRequest) (
	*mcpwiring.InternalBackendResponse, error) {

	c.method = req.Method
	c.path = req.Path
	c.user = req.Headers.Get(constant.UserKey)
	return &mcpwiring.InternalBackendResponse{StatusCode: http.StatusOK, Body: c.body}, nil
}

func fakeGatewayMCPProxy(apiServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Bkapi-JWT") != "" {
			http.Error(w, "OpenClaw should not send X-Bkapi-JWT", http.StatusBadRequest)
			return
		}

		const publicPrefix = "/api/v2/mcp-servers/"
		const suffix = "/mcp/"
		if !strings.HasPrefix(r.URL.Path, publicPrefix) || !strings.HasSuffix(r.URL.Path, suffix) {
			http.NotFound(w, r)
			return
		}

		serverName := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, publicPrefix), suffix)
		internalReq := r.Clone(r.Context())
		internalReq.URL.Path = constant.MCPIngressBasePathDefault + "/" + serverName + suffix
		validIdentityHeaders(internalReq)
		apiServer.ServeHTTP(w, internalReq)
	})
}

func doGatewayMCP(t *testing.T, gateway http.Handler, path string, payload map[string]interface{}) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(mustJSON(t, payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return doJSON(t, gateway, req)
}

func doInternalMCP(t *testing.T, mux http.Handler, path string, payload map[string]interface{}) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(mustJSON(t, payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set(constant.UserKey, "alice")
	req.Header.Set(constant.RidKey, "rid-internal")
	req.Header.Set(constant.MCPCallerSourceHeader, string(cc.AgentServerName))
	return doJSON(t, mux, req)
}

func doJSON(t *testing.T, mux http.Handler, req *http.Request) map[string]interface{} {
	t.Helper()
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("%s %s status = %d, body=%s", req.Method, req.URL.Path, w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response %q: %v", w.Body.String(), err)
	}
	if resp["error"] != nil {
		t.Fatalf("json-rpc error: %v", resp["error"])
	}
	return resp
}

func mcpResultText(resp map[string]interface{}) string {
	result, _ := resp["result"].(map[string]interface{})
	content, _ := result["content"].([]interface{})
	if len(content) == 0 {
		return ""
	}
	first, _ := content[0].(map[string]interface{})
	text, _ := first["text"].(string)
	return text
}

func validIdentityHeaders(r *http.Request) {
	r.Header.Set(constant.UserKey, "alice")
	r.Header.Set(constant.AppCodeKey, constant.WebSourceAppCode)
	r.Header.Set(constant.RidKey, "rid-integration-01")
	r.Header.Set(constant.TenantIDKey, "default")
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

func writeSSE(t *testing.T, w io.Writer, event protocol.StreamingMessageEvent) {
	t.Helper()
	b, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal sse event: %v", err)
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
		t.Fatalf("write sse: %v", err)
	}
}

func statusEvent(taskID, contextID string, state protocol.TaskState, text string,
	final bool) protocol.StreamingMessageEvent {

	return protocol.StreamingMessageEvent{Result: &protocol.TaskStatusUpdateEvent{
		TaskID:    taskID,
		ContextID: contextID,
		Kind:      protocol.KindTaskStatusUpdate,
		Final:     final,
		Status: protocol.TaskStatus{
			State:   state,
			Message: textMessage(text),
		},
	}}
}

func artifactEvent(taskID, contextID, text string) protocol.StreamingMessageEvent {
	return protocol.StreamingMessageEvent{Result: &protocol.TaskArtifactUpdateEvent{
		TaskID:    taskID,
		ContextID: contextID,
		Kind:      protocol.KindTaskArtifactUpdate,
		Artifact: protocol.Artifact{
			ArtifactID: "artifact-1",
			Parts:      []protocol.Part{protocol.NewTextPart(text)},
		},
	}}
}

func textMessage(text string) *protocol.Message {
	msg := protocol.NewMessage(protocol.MessageRoleAgent, []protocol.Part{protocol.NewTextPart(text)})
	return &msg
}

func runConcurrent(n int, fn func()) []time.Duration {
	var wg sync.WaitGroup
	start := make(chan struct{})
	latencies := make([]time.Duration, n)
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			begin := time.Now()
			fn()
			latencies[i] = time.Since(begin)
		}()
	}
	close(start)
	wg.Wait()
	return latencies
}

func percentile(latencies []time.Duration, p float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	idx := int(float64(len(latencies)-1) * p)
	return latencies[idx]
}
