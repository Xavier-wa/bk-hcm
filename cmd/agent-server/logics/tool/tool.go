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
}

// BuildMCPToolSets constructs MCP ToolSet instances from the global configuration.
// Each config maps 1-to-1 to a mcp.ToolSet. Errors from any entry abort the whole build.
// BK application credentials for toolsets with Type == "bkaidev" are read from cc.AgentServer().Tools.BKAIDev.
func BuildMCPToolSets() (*MCPToolSet, error) {
	cfgs := cc.AgentServer().Tools.MCPToolSets

	sets := make([]tool.ToolSet, 0, len(cfgs))
	for _, cfg := range cfgs {
		ts, err := buildOneMCPToolSet(cfg)
		if err != nil {
			return nil, fmt.Errorf("toolset %q: %w", cfg.Name, err)
		}
		if cfg.RequireConfirm {
			ts = newConfirmToolSet(ts)
			logs.Infof("AGUI MCP toolset registered: name=%q type=%q transport=%q serverUrl=%q requireConfirm=true",
				cfg.Name, cfg.Type, cfg.Transport, cfg.ServerURL)
		} else {
			logs.Infof("AGUI MCP toolset registered: name=%q type=%q transport=%q serverUrl=%q",
				cfg.Name, cfg.Type, cfg.Transport, cfg.ServerURL)
		}
		sets = append(sets, ts)
	}
	return &MCPToolSet{TS: sets}, nil
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

	if !strings.EqualFold(strings.TrimSpace(cfg.Type), constant.MCPTypeBKAIDev) {
		return mcp.NewMCPToolSet(conn, opts...), nil
	}

	bkaidevCfg := cc.AgentServer().Tools.BKAIDev
	appCode := bkaidevCfg.AppCode
	appSecret := bkaidevCfg.AppSecret
	logs.Infof("AGUI MCP toolset %q: bkaidev auth hook registered (appCode=%q)", cfg.Name, appCode)
	opts = append(opts, mcp.WithMCPOptions(
		trpcmcp.WithHTTPBeforeRequest(func(ctx context.Context, req *http.Request) error {
			rid := rest.RidFromContext(ctx)
			ticket := auth.BKTicketFromContext(ctx)
			if ticket != "" {
				req.Header.Set(constant.BKGWAuthKey,
					auth.BKApiAuthHeaderValue(appCode, appSecret, auth.BKUsernameFromContext(ctx), ticket))
				logs.Infof("bkaidev MCP hook: injected bk_ticket auth header for %s %s, rid: %s",
					req.Method, req.URL.Path, rid)
				return nil
			}

			token := auth.AccessTokenFromContext(ctx)
			if token != "" {
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

	return mcp.NewMCPToolSet(conn, opts...), nil
}
