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
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	mcpmetrics "hcm/cmd/api-server/service/mcp/metrics"
	"hcm/cmd/api-server/service/mcp/middleware"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/converter"

	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

const defaultBackendCallTimeout = 60 * time.Second

// BackendCaller invokes a local backend on behalf of an internal MCP tool call.
//
// The dispatcher hands off the final HTTP request shape (method/path/body/headers)
// to a BackendCaller, allowing production code to use api-server self-call and
// tests to use an in-memory mock.
type BackendCaller interface {
	Call(ctx context.Context, req *BackendRequest) (*BackendResponse, error)
}

// BackendRequest captures everything dispatcher needs to talk to a local backend.
type BackendRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

// BackendResponse is the trimmed-down result returned by BackendCaller.
type BackendResponse struct {
	StatusCode int
	Body       []byte
}

// Dispatcher routes southbound tools/call requests to local backends.
//
// Dispatcher 不再依赖蓝鲸网关 MCP-proxy；所有工具均通过 BackendCaller 走 api-server
// 自身 mux（self-call）调用 cloud/woa/account-server 等内部服务。
type Dispatcher struct {
	cfg         cc.MCPInternalServerSetting
	serverName  string
	caller      BackendCaller
	callTimeout time.Duration

	mu       sync.RWMutex
	backends map[string]ToolBackend
}

// NewDispatcher creates a Dispatcher with a default in-process self-call caller.
//
// The default caller is a stub that returns an error result until task 8 wires
// the api-server outer mux into the dispatcher (so we can avoid an extra TCP
// round-trip for southbound tool calls).
func NewDispatcher(cfg cc.MCPInternalServerSetting) *Dispatcher {
	return NewDispatcherWithCaller(cfg, &notImplementedCaller{}, defaultBackendCallTimeout)
}

// NewDispatcherWithCaller creates a Dispatcher with a custom backend caller.
func NewDispatcherWithCaller(cfg cc.MCPInternalServerSetting, caller BackendCaller,
	callTimeout time.Duration) *Dispatcher {

	cfg = normalizeInternalServerSetting(cfg)
	if caller == nil {
		caller = &notImplementedCaller{}
	}
	if callTimeout <= 0 {
		callTimeout = defaultBackendCallTimeout
	}
	return &Dispatcher{
		cfg:         cfg,
		serverName:  cfg.Name,
		caller:      caller,
		callTimeout: callTimeout,
		backends:    make(map[string]ToolBackend),
	}
}

// SetBackendCaller replaces the backend caller used by Dispatch.
func (d *Dispatcher) SetBackendCaller(caller BackendCaller, callTimeout time.Duration) {
	if d == nil {
		return
	}
	if caller == nil {
		caller = &notImplementedCaller{}
	}
	if callTimeout <= 0 {
		callTimeout = defaultBackendCallTimeout
	}

	d.mu.Lock()
	d.caller = caller
	d.callTimeout = callTimeout
	d.mu.Unlock()
}

// SetBackends replaces the tool→backend mapping used by Dispatch.
func (d *Dispatcher) SetBackends(backends map[string]ToolBackend) {
	if d == nil {
		return
	}
	next := make(map[string]ToolBackend, len(backends))
	for name, b := range backends {
		next[name] = b
	}

	d.mu.Lock()
	d.backends = next
	d.mu.Unlock()
}

// Dispatch handles a southbound MCP tools/call request.
func (d *Dispatcher) Dispatch(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	start := time.Now()
	toolName := ""
	if req != nil {
		toolName = req.Params.Name
	}
	status := mcpmetrics.StatusError
	defer func() {
		mcpmetrics.IncToolsCall(toolName, d.metricServerName(), status)
		mcpmetrics.ObserveToolsCallDuration(toolName, d.metricServerName(), time.Since(start).Seconds())
	}()

	if d == nil {
		return nil, errors.New("internal mcp dispatcher is nil")
	}
	if req == nil || strings.TrimSpace(req.Params.Name) == "" {
		status = mcpmetrics.StatusInvalidArg
		return nil, errors.New("missing tool name")
	}

	backend, ok := d.backendOf(req.Params.Name)
	if !ok {
		status = mcpmetrics.StatusInvalidArg
		return mcpsdk.NewErrorResult(fmt.Sprintf("unknown internal MCP tool: %s", req.Params.Name)), nil
	}

	result, err := d.callBackend(ctx, req, backend)
	if err != nil {
		return nil, err
	}
	if result != nil && !result.IsError {
		status = mcpmetrics.StatusSuccess
	}
	return result, nil
}

func (d *Dispatcher) backendOf(name string) (ToolBackend, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	backend, ok := d.backends[name]
	return backend, ok
}

func (d *Dispatcher) backendCaller() (BackendCaller, time.Duration) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.caller, d.callTimeout
}

func (d *Dispatcher) callBackend(ctx context.Context, req *mcpsdk.CallToolRequest, backend ToolBackend) (
	*mcpsdk.CallToolResult, error) {

	kt, rid := kitFromCtx(ctx)
	if kt == nil || strings.TrimSpace(kt.User) == "" {
		return mcpsdk.NewErrorResult("missing bk_username for internal MCP backend call"), nil
	}

	args := req.Params.Arguments
	if args == nil {
		args = map[string]interface{}{}
	}
	finalPath, body, err := buildBackendCallShape(backend, args)
	if err != nil {
		logs.Errorf("internal mcp: build backend shape failed, tool=%s, err: %v, rid: %s",
			req.Params.Name, err, rid)
		return mcpsdk.NewErrorResult(fmt.Sprintf("build backend shape failed: %v", err)), nil
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	headers.Set(constant.UserKey, kt.User)
	// 内部自调用是进程内可信路径，AppCode 直接用服务自身名称填充，
	// 避免上游（如 agent-server）未传递 X-Bkapi-App-Code 时 defaultParser 校验失败。
	appCode := strings.TrimSpace(kt.AppCode)
	if appCode == "" {
		appCode = string(cc.APIServerName)
	}
	headers.Set(constant.AppCodeKey, appCode)
	if strings.TrimSpace(kt.TenantID) != "" {
		headers.Set(constant.TenantIDKey, kt.TenantID)
	}
	if strings.TrimSpace(kt.Rid) != "" {
		headers.Set(constant.RidKey, kt.Rid)
	}
	headers.Set(constant.MCPCallerSourceHeader, string(cc.APIServerName))

	caller, callTimeout := d.backendCaller()
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	resp, err := caller.Call(ctx, &BackendRequest{
		Method:  backend.Method,
		Path:    finalPath,
		Headers: headers,
		Body:    body,
	})
	if err != nil {
		if isTimeoutErr(err) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			logs.Errorf("internal mcp: backend call timeout, tool=%s, err: %v, rid: %s",
				req.Params.Name, err, rid)
			return mcpsdk.NewErrorResult("后端调用超时"), nil
		}
		logs.Errorf("internal mcp: backend call failed, tool=%s, err: %v, rid: %s",
			req.Params.Name, err, rid)
		return nil, err
	}
	if resp == nil {
		return mcpsdk.NewErrorResult("backend returned nil response"), nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		logs.Errorf("internal mcp: backend returned bad status, tool=%s, status=%d, rid: %s",
			req.Params.Name, resp.StatusCode, rid)
		return mcpsdk.NewErrorResult(
			fmt.Sprintf("backend returned status %d: %s", resp.StatusCode, truncateBytes(resp.Body, 512)),
		), nil
	}
	return mcpsdk.NewTextResult(string(resp.Body)), nil
}

func (d *Dispatcher) metricServerName() string {
	if d == nil || strings.TrimSpace(d.serverName) == "" {
		return constant.MCPInternalDefaultServerName
	}
	return d.serverName
}

// buildBackendCallShape returns the final HTTP path (path-template variables
// replaced from args) and the JSON body (args minus the path variables).
//
// 支持两种参数格式：
//  1. 分组格式（与蓝鲸网关 MCP 一致）：args 中包含 "path_param" 和/或 "body_param" 键。
//     path 变量从 path_param 中取，body 直接使用 body_param 的内容。
//  2. 平铺格式（向后兼容 internalOnlyTools）：args 中直接包含各参数，
//     path 变量从顶层 args 中取，剩余参数作为 body。
func buildBackendCallShape(backend ToolBackend, args map[string]interface{}) (string, []byte, error) {
	// 检测是否为分组格式（存在 path_param 或 body_param 键）。
	_, hasPathParam := args["path_param"]
	_, hasBodyParam := args["body_param"]
	if hasPathParam || hasBodyParam {
		return buildBackendCallShapeGrouped(backend, args)
	}
	return buildBackendCallShapeFlat(backend, args)
}

// buildBackendCallShapeGrouped 处理分组格式（path_param / body_param）。
func buildBackendCallShapeGrouped(backend ToolBackend, args map[string]interface{}) (string, []byte, error) {
	finalPath := backend.Path
	pathVars := pathTemplateVars(backend.Path)

	// 从 path_param 中提取 path 变量。
	pathParams, _ := args["path_param"].(map[string]interface{})
	if pathParams == nil {
		pathParams = map[string]interface{}{}
	}
	for _, name := range pathVars {
		val, ok := pathParams[name]
		if !ok {
			return "", nil, fmt.Errorf("path variable %q missing in path_param", name)
		}
		finalPath = strings.ReplaceAll(finalPath, "{"+name+"}", converter.FormatPlainString(val))
	}

	var body []byte
	if backend.Method != http.MethodGet && backend.Method != http.MethodDelete {
		bodyParam, _ := args["body_param"].(map[string]interface{})
		if bodyParam == nil {
			bodyParam = map[string]interface{}{}
		}
		b, err := json.Marshal(bodyParam)
		if err != nil {
			return "", nil, err
		}
		body = b
	}
	return finalPath, body, nil
}

// buildBackendCallShapeFlat 处理平铺格式（向后兼容 internalOnlyTools）。
func buildBackendCallShapeFlat(backend ToolBackend, args map[string]interface{}) (string, []byte, error) {
	finalPath := backend.Path
	pathVars := pathTemplateVars(backend.Path)

	leftover := make(map[string]interface{}, len(args))
	for k, v := range args {
		leftover[k] = v
	}
	for _, name := range pathVars {
		val, ok := args[name]
		if !ok {
			return "", nil, fmt.Errorf("path variable %q missing in arguments", name)
		}
		finalPath = strings.ReplaceAll(finalPath, "{"+name+"}", converter.FormatPlainString(val))
		delete(leftover, name)
	}

	var body []byte
	if backend.Method != http.MethodGet && backend.Method != http.MethodDelete {
		b, err := json.Marshal(leftover)
		if err != nil {
			return "", nil, err
		}
		body = b
	}
	return finalPath, body, nil
}

// pathTemplateVars returns the list of {var} placeholders in a path template
// in left-to-right order.
func pathTemplateVars(p string) []string {
	var out []string
	for {
		open := strings.Index(p, "{")
		if open < 0 {
			return out
		}
		close := strings.Index(p[open:], "}")
		if close < 0 {
			return out
		}
		name := p[open+1 : open+close]
		out = append(out, name)
		p = p[open+close+1:]
	}
}

func kitFromCtx(ctx context.Context) (*kit.Kit, string) {
	kt, ok := middleware.KitFromCtx(ctx)
	if !ok || kt == nil {
		return nil, "unknown"
	}
	rid := kt.Rid
	if strings.TrimSpace(rid) == "" {
		rid = "unknown"
	}
	return kt, rid
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func truncateBytes(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}

// notImplementedCaller is the default BackendCaller until task 8 wires the
// outer mux self-call. It always returns a structured error so misconfigured
// deployments surface loudly instead of silently dropping tool calls.
type notImplementedCaller struct{}

// Call implements BackendCaller.
func (notImplementedCaller) Call(_ context.Context, req *BackendRequest) (*BackendResponse, error) {
	logs.Warnf("internal mcp: backend caller not wired yet, method=%s, path=%s",
		req.Method, req.Path)
	return &BackendResponse{
		StatusCode: http.StatusNotImplemented,
		Body: []byte(fmt.Sprintf(
			`{"error":"backend caller not wired; method=%s, path=%s"}`, req.Method, req.Path)),
	}, nil
}
