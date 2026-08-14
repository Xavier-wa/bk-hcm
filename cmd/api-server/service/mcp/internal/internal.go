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
	"net/http"
	"strings"

	mcphttp "hcm/cmd/api-server/service/mcp"
	"hcm/cmd/api-server/service/mcp/middleware"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/uuid"

	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

const loggerComponent = "mcp/internal"

// RegisterAll mounts all configured southbound internal HCM MCP servers.
func RegisterAll(ctx context.Context, mux *http.ServeMux, cfg cc.MCPInternalSetting) ([]*Dispatcher, error) {
	if mux == nil {
		return nil, errors.New("internal mcp Register: mux is nil")
	}
	cfg = normalizeInternalSetting(cfg)
	if !cfg.Enable {
		logs.Infof("internal mcp: mcp.internal.enable=false, skip mounting internal MCP path")
		return nil, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	dispatchers := make([]*Dispatcher, 0, len(cfg.Servers))
	for _, serverCfg := range cfg.Servers {
		dispatcher, err := registerOne(ctx, mux, serverCfg, cfg.EnforceCallerSource)
		if err != nil {
			return nil, err
		}
		dispatchers = append(dispatchers, dispatcher)
	}
	return dispatchers, nil
}

func registerOne(ctx context.Context, mux *http.ServeMux, cfg cc.MCPInternalServerSetting,
	enforceCallerSource bool) (*Dispatcher, error) {

	dispatcher := NewDispatcher(cfg)
	srv := BuildInternalServer(cfg, dispatcher)
	if err := loadAndRegisterTools(cfg, srv, dispatcher); err != nil {
		return nil, err
	}

	expectedCallerSource := ""
	if enforceCallerSource {
		expectedCallerSource = string(cc.AgentServerName)
	}
	httpHandler := mcphttp.NewMCPHTTPHandler(srv,
		mcphttp.WithLogComponent(loggerComponent),
		mcphttp.WithMiddleware(func(next http.Handler) http.Handler {
			return middleware.CallerSourceMiddleware(expectedCallerSource, next)
		}),
		mcphttp.WithMiddleware(InternalIdentityMiddleware),
	)

	pattern := internalPathPattern(cfg.BasePath)
	mux.Handle(pattern, httpHandler)
	logs.Infof("internal mcp: mounted internal MCP server at %s, server=%s/%s",
		pattern, cfg.Name, cfg.ServerVersion)
	return dispatcher, nil
}

// BuildInternalServer constructs the southbound internal MCP server.
func BuildInternalServer(cfg cc.MCPInternalServerSetting, dispatcher *Dispatcher) *mcpsdk.Server {
	cfg = normalizeInternalServerSetting(cfg)
	if dispatcher == nil {
		dispatcher = NewDispatcher(cfg)
	}

	return mcphttp.BuildBaseServer(
		cfg.Name,
		cfg.ServerVersion,
		loggerComponent,
		mcpsdk.WithHTTPContextFunc(InternalIdentityHTTPContextFunc),
	)
}

// loadAndRegisterTools parses the OpenAPI yaml, merges InternalOnlyTools, and
// registers every resulting tool on srv + dispatcher.
//
// 合并规则：(OpenAPI loader) ∪ InternalOnlyTools，同名时 InternalOnlyTools 覆盖。
func loadAndRegisterTools(cfg cc.MCPInternalServerSetting, srv *mcpsdk.Server, dispatcher *Dispatcher) error {
	if srv == nil || dispatcher == nil {
		return errors.New("server or dispatcher is nil")
	}

	loaded, err := LoadOpenAPISpec(cfg.OpenAPISpecPath, cfg.IncludeOperationIDs)
	if err != nil {
		return fmt.Errorf("load openapi spec: %w", err)
	}

	backends := make(map[string]ToolBackend, len(loaded.Backends))
	tools := make([]*mcpsdk.Tool, 0, len(loaded.Tools)+len(cfg.InternalOnlyTools))
	taken := make(map[string]bool, len(loaded.Tools))
	for _, tool := range loaded.Tools {
		if tool == nil || tool.Name == "" {
			continue
		}
		tools = append(tools, tool)
		backends[tool.Name] = loaded.Backends[tool.Name]
		taken[tool.Name] = true
	}

	for _, cfgTool := range cfg.InternalOnlyTools {
		tool, backend, err := internalOnlyToolFromCfg(cfgTool)
		if err != nil {
			logs.Warnf("internal mcp: skip invalid internalOnlyTool, err: %v", err)
			continue
		}
		if taken[tool.Name] {
			// 覆盖 OpenAPI 中的同名工具：从已收集的 slice 中移除旧条目。
			tools = removeToolByName(tools, tool.Name)
			logs.Infof("internal mcp: internalOnlyTool %q overrides OpenAPI definition", tool.Name)
		}
		tools = append(tools, tool)
		backends[tool.Name] = backend
		taken[tool.Name] = true
	}

	for _, tool := range tools {
		srv.RegisterTool(tool, dispatcher.Dispatch)
	}
	dispatcher.SetBackends(backends)

	logs.Infof("internal mcp: registered %d tools (openapi=%d, internalOnly=%d, openapiSpec=%s)",
		len(tools), len(loaded.Tools), len(cfg.InternalOnlyTools), cfg.OpenAPISpecPath)
	return nil
}

func removeToolByName(tools []*mcpsdk.Tool, name string) []*mcpsdk.Tool {
	out := tools[:0]
	for _, t := range tools {
		if t != nil && t.Name != name {
			out = append(out, t)
		}
	}
	return out
}

// InternalIdentityHTTPContextFunc writes trusted internal identity headers to ctx.
func InternalIdentityHTTPContextFunc(ctx context.Context, r *http.Request) context.Context {
	kt := kitFromInternalHeader(ctx, r.Header)
	if kt == nil {
		return ctx
	}
	return middleware.WithKit(ctx, kt)
}

// InternalIdentityMiddleware validates X-Bkapi-User-Name and writes kit to ctx.
func InternalIdentityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kt := kitFromInternalHeader(r.Context(), r.Header)
		if kt == nil || strings.TrimSpace(kt.User) == "" {
			logs.Warnf("internal mcp: missing bk_username, remote=%s, path=%s",
				r.RemoteAddr, r.URL.Path)
			writeForbidden(w, "bk_username is required")
			return
		}
		next.ServeHTTP(w, r.WithContext(middleware.WithKit(r.Context(), kt)))
	})
}

func kitFromInternalHeader(ctx context.Context, h http.Header) *kit.Kit {
	user := strings.TrimSpace(h.Get(constant.UserKey))
	if user == "" {
		return nil
	}

	rid := strings.TrimSpace(h.Get(constant.RidKey))
	if rid == "" {
		rid = uuid.UUID()
	}
	return &kit.Kit{
		User:     user,
		Rid:      rid,
		AppCode:  h.Get(constant.AppCodeKey),
		TenantID: h.Get(constant.TenantIDKey),
		Ctx:      context.WithValue(ctx, constant.RidKey, rid),
	}
}

func internalPathPattern(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = constant.MCPInternalBasePathDefault
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}
	return basePath
}

func writeForbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	body, _ := json.Marshal(errf.New(errf.PermissionDenied, msg))
	_, _ = w.Write(body)
}

// normalizeInternalSetting applies defaults to MCPInternalSetting fields used
// inside the internal MCP package; mirrors cc.MCPInternalSetting.trySetDefault
// so tests can construct cfg with minimal values.
func normalizeInternalSetting(cfg cc.MCPInternalSetting) cc.MCPInternalSetting {
	for i := range cfg.Servers {
		cfg.Servers[i] = normalizeInternalServerSetting(cfg.Servers[i])
	}
	return cfg
}

func normalizeInternalServerSetting(cfg cc.MCPInternalServerSetting) cc.MCPInternalServerSetting {
	if strings.TrimSpace(cfg.ServerVersion) == "" {
		cfg.ServerVersion = constant.MCPInternalServerDefaultVersion
	}
	return cfg
}
