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

// Package tool ...
package tool

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hcm/cmd/agent-server/logics/auth"
	"hcm/cmd/agent-server/logics/logger"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/mcp"
	trpcmcp "trpc.group/trpc-go/trpc-mcp-go"
)

// MCPToolSet is a collection of tool sets.
type MCPToolSet struct {
	// TS holds the configured MCP tool sets.
	TS []tool.ToolSet
	// scenes stores the scene tags for each ToolSet in TS, indexed in parallel.
	// An empty slice at a given index means the toolset applies to all scenes.
	scenes [][]string
}

// BuildMCPToolSets constructs MCP ToolSet instances from the global configuration.
// Each config maps 1-to-1 to a mcp.ToolSet. Errors from any entry abort the whole build.
// BK application credentials for toolsets with Type == "bkaidev" are read from cc.AgentServer().Tools.BKAIDev.
// gateSkip 为受工程门禁守护的工具名集合，从泛化确认中排除，避免双重确认。
func BuildMCPToolSets(gateSkip map[string]struct{}) (*MCPToolSet, error) {
	cfgs := cc.AgentServer().Tools.MCPToolSets

	sets := make([]tool.ToolSet, 0, len(cfgs))
	scenesList := make([][]string, 0, len(cfgs))
	for _, cfg := range cfgs {
		ts, err := buildOneMCPToolSet(cfg)
		if err != nil {
			return nil, fmt.Errorf("toolset %q: %w", cfg.Name, err)
		}
		if cfg.RequireConfirm {
			ts = newConfirmToolSet(ts, gateSkip)
			logs.Infof("AGUI MCP toolset registered: name=%q type=%q transport=%q serverUrl=%q requireConfirm=true",
				cfg.Name, cfg.Type, cfg.Transport, cfg.ServerURL)
		} else {
			logs.Infof("AGUI MCP toolset registered: name=%q type=%q transport=%q serverUrl=%q",
				cfg.Name, cfg.Type, cfg.Transport, cfg.ServerURL)
		}
		sets = append(sets, ts)
		scns := make([]string, 0, len(cfg.Scenes))
		for _, sc := range cfg.Scenes {
			scns = append(scns, string(sc))
		}
		scenesList = append(scenesList, scns)
	}
	return &MCPToolSet{TS: sets, scenes: scenesList}, nil
}

// FilterByScene returns a new MCPToolSet containing only toolsets whose scenes list
// is empty (applies to all scenes) or contains the specified scene.
// The original MCPToolSet is not modified.
func (s *MCPToolSet) FilterByScene(scene string) *MCPToolSet {
	filtered := &MCPToolSet{}
	for i, ts := range s.TS {
		var scns []string
		if i < len(s.scenes) {
			scns = s.scenes[i]
		}
		// Empty scenes means the toolset applies to all scenes.
		if len(scns) == 0 {
			filtered.TS = append(filtered.TS, ts)
			filtered.scenes = append(filtered.scenes, scns)
			continue
		}
		for _, sc := range scns {
			if sc == scene {
				filtered.TS = append(filtered.TS, ts)
				filtered.scenes = append(filtered.scenes, scns)
				break
			}
		}
	}
	return filtered
}

// Close closes the MCPToolSet.
func (s *MCPToolSet) Close() error {
	for _, ts := range s.TS {
		if err := ts.Close(); err != nil {
			return err
		}
	}
	return nil
}

// buildOneMCPToolSet constructs a single MCP ToolSet from the given config.
// When cfg.Type is "bkaidev" and the global BKAIDev config is non-nil, a per-request
// X-Bkapi-Authorization header is injected using BKAIDev credentials and
// the bk_ticket resolved from the request context at call time.
func buildOneMCPToolSet(cfg cc.AgentMCPToolSet) (tool.ToolSet, error) {
	conn := mcp.ConnectionConfig{
		Transport: strings.TrimSpace(cfg.Transport),
		ServerURL: strings.TrimSpace(cfg.ServerURL),
		Headers:   cfg.Headers,
		Command:   strings.TrimSpace(cfg.Command),
		Args:      cfg.Args,
	}

	if raw := strings.TrimSpace(cfg.Timeout); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			conn.Timeout = d
		}
	}

	opts := make([]mcp.ToolSetOption, 0, 4)
	if name := strings.TrimSpace(cfg.Name); name != "" {
		opts = append(opts, mcp.WithName(name))
	}

	if f := cfg.Filter; f != nil && len(f.Names) > 0 {
		var filterFunc tool.FilterFunc
		if f.Mode == enumor.MCPFilterModeExclude {
			filterFunc = tool.NewExcludeToolNamesFilter(f.Names...)
		} else {
			filterFunc = tool.NewIncludeToolNamesFilter(f.Names...)
		}
		opts = append(opts, mcp.WithToolFilterFunc(filterFunc))
	}

	if r := cfg.Reconnect; r != nil && r.Enabled {
		attempts := r.MaxAttempts
		if attempts <= 0 {
			attempts = 3
		}
		opts = append(opts, mcp.WithSessionReconnect(attempts))
	}

	// Log HTTP >=400 response bodies: trpc-mcp-go does not attach body to errors on non-200.
	// See trpcmcp streamable_client.send(). Disable via env AGENT_SERVER_MCP_HTTP_LOG_ERROR_BODY=0.
	opts = append(opts, mcp.WithMCPOptions(
		trpcmcp.WithHTTPReqHandler(logger.NewMCPHTTPLoggingHandler(trpcmcp.NewDefaultHTTPReqHandler(), cfg.Name)),
	))

	// 按 cfg.Type 分派认证 / 身份注入 hook：
	//   - bkaidev  : 注入 X-Bkapi-Authorization（bk_ticket 优先，access_token 兜底）
	//   - internal : 仅注入 X-Bkapi-User-Name，用于 agent-server LLM → api-server 内置 HCM MCP 等内网直连场景
	//   - 其他     : 不注入任何 header，保持默认透传行为
	switch cfg.Type.Normalize() {
	case constant.MCPTypeBKAIDev:
		opts = appendBKAIDevAuthHook(opts, cfg.Name)
	case constant.MCPTypeInternal:
		opts = appendInternalAuthHook(opts, cfg.Name)
	}

	return mcp.NewMCPToolSet(conn, opts...), nil
}

// appendBKAIDevAuthHook 注入 bkaidev 类型 MCP toolset 的鉴权 header。
// 优先使用 ctx 中的 bk_ticket（基于 appCode/appSecret 签名），否则回退到 access_token。
func appendBKAIDevAuthHook(opts []mcp.ToolSetOption, name string) []mcp.ToolSetOption {
	bkaidevCfg := cc.AgentServer().Tools.BKAIDev
	appCode := bkaidevCfg.AppCode
	appSecret := bkaidevCfg.AppSecret
	logs.Infof("AGUI MCP toolset %q: bkaidev auth hook registered (appCode=%q)", name, appCode)

	return append(opts, mcp.WithMCPOptions(
		trpcmcp.WithHTTPBeforeRequest(func(ctx context.Context, req *http.Request) error {
			rid := rest.RidFromContext(ctx)
			if ticket := auth.BKTicketFromContext(ctx); ticket != "" {
				req.Header.Set(constant.BKGWAuthKey,
					auth.BKApiAuthHeaderValue(appCode, appSecret, auth.BKUsernameFromContext(ctx), ticket))
				logs.Infof("bkaidev MCP hook: injected bk_ticket auth header for %s %s, rid: %s",
					req.Method, req.URL.Path, rid)
				return nil
			}
			if token := auth.AccessTokenFromContext(ctx); token != "" {
				req.Header.Set(constant.BKGWAuthKey, auth.AccessTokenAuthHeaderValue(token))
				logs.Infof("bkaidev MCP hook: injected access_token auth header for %s %s, rid: %s",
					req.Method, req.URL.Path, rid)
				return nil
			}
			logs.Warnf("bkaidev MCP hook: no bk_ticket or access_token in context for %s %s, "+
				"skipping auth header injection, rid: %s", req.Method, req.URL.Path, rid)
			return nil
		}),
	))
}

// appendInternalAuthHook 注入 internal 类型 MCP toolset 的身份 header。
// 仅写入 X-Bkapi-User-Name（来自 ctx 中的 bk_username），不读取 / 不注入
// X-Bkapi-Authorization、bk_ticket、access_token；适用于 agent-server LLM →
// api-server 内置 HCM MCP 等内网直连场景。
func appendInternalAuthHook(opts []mcp.ToolSetOption, name string) []mcp.ToolSetOption {
	logs.Infof("A2A MCP toolset %q: internal auth hook registered (bk_username only)", name)
	return append(opts, mcp.WithMCPOptions(
		trpcmcp.WithHTTPBeforeRequest(func(ctx context.Context, req *http.Request) error {
			rid := rest.RidFromContext(ctx)
			username := auth.BKUsernameFromContext(ctx)
			if username == "" {
				logs.Warnf("internal MCP hook: no bk_username in context for %s %s, "+
					"calling without identity, rid: %s", req.Method, req.URL.Path, rid)
				return nil
			}
			req.Header.Set(constant.UserKey, username)
			logs.V(4).Infof("internal MCP hook: injected X-Bkapi-User-Name=%s for %s %s, rid: %s",
				username, req.Method, req.URL.Path, rid)
			return nil
		}),
	))
}
