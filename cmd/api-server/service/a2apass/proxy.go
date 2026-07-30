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

// Package a2apass provides api-server northbound A2A passthrough reverse proxy.
package a2apass

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"hcm/cmd/api-server/service/mcp/middleware"
	"hcm/pkg/cc"
	"hcm/pkg/client/discovery"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"
)

const (
	defaultFlushIntervalMS = -1
	defaultIdleTimeout     = 600 * time.Second
)

type serverDiscovery interface {
	GetServers() ([]string, error)
}

// BuildPassthroughHandler builds the A2A passthrough handler with identity middleware.
func BuildPassthroughHandler(cfg cc.A2APassthroughSetting, dis serviced.Discover) http.Handler {
	if dis == nil {
		return buildPassthroughHandlerWithDiscovery(cfg,
			errDiscovery{err: fmt.Errorf("a2apass: serviced discover is nil")})
	}
	return buildPassthroughHandlerWithDiscovery(cfg, discovery.NewAPIDiscovery(cc.AgentServerName, dis))
}

func buildPassthroughHandlerWithDiscovery(cfg cc.A2APassthroughSetting, dis serverDiscovery) http.Handler {
	cfg = normalizeSetting(cfg)
	proxy := newReverseProxy(cfg, dis)
	return middleware.IdentityMiddleware(proxy)
}

func newReverseProxy(cfg cc.A2APassthroughSetting, dis serverDiscovery) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			target, err := selectAgentServer(dis)
			if err != nil {
				ctx := context.WithValue(req.Context(), proxyErrorCtxKey{}, err)
				*req = *req.WithContext(ctx)
				return
			}

			kt, _ := middleware.KitFromCtx(req.Context())
			middleware.InjectInternalHeaders(kt, req.Header, string(cc.APIServerName))

			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = joinTargetPath(target.Path, req.URL.Path)
			if target.RawQuery == "" || req.URL.RawQuery == "" {
				req.URL.RawQuery = target.RawQuery + req.URL.RawQuery
			} else {
				req.URL.RawQuery = target.RawQuery + "&" + req.URL.RawQuery
			}

			req.Host = target.Host
		},
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			IdleConnTimeout:       cfg.IdleTimeout,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
		FlushInterval: time.Duration(cfg.FlushIntervalMS) * time.Millisecond,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if storedErr, ok := r.Context().Value(proxyErrorCtxKey{}).(error); ok && storedErr != nil {
				err = storedErr
			}
			rid := ""
			if kt, ok := middleware.KitFromCtx(r.Context()); ok && kt != nil {
				rid = kt.Rid
			}
			logs.Errorf("a2apass: reverse proxy failed, err: %v, rid: %s", err, rid)
			http.Error(w, "a2a passthrough failed", http.StatusBadGateway)
		},
	}
}

type proxyErrorCtxKey struct{}

type errDiscovery struct {
	err error
}

func (d errDiscovery) GetServers() ([]string, error) {
	return nil, d.err
}

func normalizeSetting(cfg cc.A2APassthroughSetting) cc.A2APassthroughSetting {
	if strings.TrimSpace(cfg.BasePath) == "" {
		cfg.BasePath = constant.A2ABasePathDefault
	}
	if cfg.FlushIntervalMS == 0 {
		cfg.FlushIntervalMS = defaultFlushIntervalMS
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = defaultIdleTimeout
	}
	return cfg
}

func selectAgentServer(dis serverDiscovery) (*url.URL, error) {
	if dis == nil {
		return nil, fmt.Errorf("a2apass: discovery is nil")
	}
	servers, err := dis.GetServers()
	if err != nil {
		return nil, fmt.Errorf("a2apass: get agent-server instances failed: %w", err)
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("a2apass: no agent-server instance available")
	}

	addr := strings.TrimSpace(servers[0])
	if addr == "" {
		return nil, fmt.Errorf("a2apass: empty agent-server address")
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	target, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("a2apass: parse agent-server address %q failed: %w", addr, err)
	}
	return target, nil
}

func joinTargetPath(basePath, reqPath string) string {
	if basePath == "" || basePath == "/" {
		return reqPath
	}
	return strings.TrimRight(basePath, "/") + "/" + strings.TrimLeft(reqPath, "/")
}
