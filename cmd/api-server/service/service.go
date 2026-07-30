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

// Package service ...
package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"hcm/cmd/api-server/service/a2apass"
	"hcm/cmd/api-server/service/mcp/bridge"
	"hcm/cmd/api-server/service/mcp/ingress"
	mcpwiring "hcm/cmd/api-server/service/mcp/wiring"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/handler"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/rest/client"
	"hcm/pkg/runtime/shutdown"
	"hcm/pkg/serviced"
	"hcm/pkg/tools/ssl"
)

// Service do all the api server's work
type Service struct {
	proxy *proxy
	dis   serviced.Discover
}

// NewService create a service instance.
func NewService(dis serviced.Discover) (*Service, error) {
	network := cc.ApiServer().Network

	var tlsConfig *ssl.TLSConfig
	if network.TLS.Enable() {
		tlsConfig = &ssl.TLSConfig{
			InsecureSkipVerify: network.TLS.InsecureSkipVerify,
			CertFile:           network.TLS.CertFile,
			KeyFile:            network.TLS.KeyFile,
			CAFile:             network.TLS.CAFile,
			Password:           network.TLS.Password,
		}
	}

	cli, err := client.NewClient(tlsConfig)
	if err != nil {
		return nil, err
	}

	p, err := newProxy(dis, cli)
	if err != nil {
		return nil, err
	}

	return &Service{
		proxy: p,
		dis:   dis,
	}, nil
}

// ListenAndServeRest listen and serve the restful server
func (s *Service) ListenAndServeRest() error {

	root, err := s.buildRootMux(context.Background())
	if err != nil {
		return err
	}

	network := cc.ApiServer().Network
	server := &http.Server{
		Addr:    net.JoinHostPort(network.BindIP, strconv.FormatUint(uint64(network.Port), 10)),
		Handler: root,
	}

	if network.TLS.Enable() {
		tls := network.TLS
		tlsC, err := ssl.ClientTLSConfVerify(tls.InsecureSkipVerify, tls.CAFile, tls.CertFile, tls.KeyFile,
			tls.Password)
		if err != nil {
			return fmt.Errorf("init restful tls config failed, err: %v", err)
		}

		server.TLSConfig = tlsC
	}

	logs.Infof("listen restful server on %s with secure(%v) now.", server.Addr, network.TLS.Enable())

	go func() {
		notifier := shutdown.AddNotifier()
		select {
		case <-notifier.Signal:
			defer notifier.Done()

			logs.Infof("start shutdown restful server gracefully...")

			ctx, cancel := context.WithTimeout(context.TODO(), 20*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				logs.Errorf("shutdown restful server failed, err: %v", err)
				return
			}

			logs.Infof("shutdown restful server success...")
		}
	}()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logs.Errorf("serve restful server failed, err: %v", err)
			shutdown.SignalShutdownGracefully()
		}
	}()

	return nil
}

func (s *Service) buildRootMux(ctx context.Context) (*http.ServeMux, error) {
	if s == nil || s.proxy == nil {
		return nil, errors.New("api-server service or proxy is nil")
	}
	return s.buildRootMuxWithProxy(ctx, s.proxy.apiSet())
}

func (s *Service) buildRootMuxWithProxy(ctx context.Context, proxyHandler http.Handler) (*http.ServeMux, error) {
	return buildRootMuxWithConfig(ctx, proxyHandler, s.dis, s.Healthz, s.Alivez, cc.ApiServer(), true)
}

func buildRootMuxWithConfig(ctx context.Context, proxyHandler http.Handler, dis serviced.Discover,
	healthz http.HandlerFunc, alivez http.HandlerFunc, apiCfg cc.ApiServerSetting, registerCommon bool) (
	*http.ServeMux, error) {

	if proxyHandler == nil {
		return nil, errors.New("api-server proxy handler is nil")
	}

	root := http.NewServeMux()

	if apiCfg.MCP.Ingress.Enable {
		bridgeHandler, err := bridge.NewHandler(apiCfg.MCP.Bridge, apiCfg.MCP.Ingress, dis)
		if err != nil {
			return nil, fmt.Errorf("initialize MCP ingress bridge failed, err: %v", err)
		}
		if err = ingress.Register(root, apiCfg.MCP.Ingress, bridgeHandler); err != nil {
			return nil, fmt.Errorf("register MCP ingress routes failed, err: %v", err)
		}
	}

	var internalDispatchers []*mcpwiring.InternalDispatcher
	if apiCfg.MCP.Internal.Enable {
		dispatchers, err := mcpwiring.RegisterInternalServers(ctx, root, apiCfg.MCP.Internal)
		if err != nil {
			return nil, fmt.Errorf("register internal MCP routes failed, err: %v", err)
		}
		internalDispatchers = dispatchers
	}

	if apiCfg.A2APassthrough.Enable {
		if err := a2apass.Register(root, apiCfg.A2APassthrough, dis); err != nil {
			return nil, fmt.Errorf("register A2A passthrough routes failed, err: %v", err)
		}
	}

	if healthz != nil {
		root.HandleFunc("/healthz", healthz)
	}
	if alivez != nil {
		root.HandleFunc("/alivez", alivez)
	}
	root.HandleFunc("/", proxyHandler.ServeHTTP)
	if registerCommon {
		handler.SetCommonHandler(root)
	}

	for _, dispatcher := range internalDispatchers {
		if dispatcher != nil {
			dispatcher.SetBackendCaller(&localBackendCaller{handler: root}, 0)
		}
	}
	logRouteSwitches(apiCfg)

	return root, nil
}

func logRouteSwitches(apiCfg cc.ApiServerSetting) {
	paths := make([]string, 0, len(apiCfg.MCP.Internal.Servers))
	for _, server := range apiCfg.MCP.Internal.Servers {
		paths = append(paths, strings.TrimRight(server.BasePath, "/"))
	}
	internalPath := strings.Join(paths, ",")
	logs.Infof("api-server optional route status: mcp.ingress.enable=%v path=%s/{mcp_server_name}/mcp/, "+
		"mcp.internal.enable=%v path=%s/, a2aPassthrough.enable=%v path=%s%s and %s/.well-known/, rid: %s",
		apiCfg.MCP.Ingress.Enable, strings.TrimRight(apiCfg.MCP.Ingress.BasePath, "/"),
		apiCfg.MCP.Internal.Enable, internalPath,
		apiCfg.A2APassthrough.Enable, strings.TrimRight(apiCfg.A2APassthrough.BasePath, "/"),
		constant.A2AJSONRPCSubPath, strings.TrimRight(apiCfg.A2APassthrough.BasePath, "/"),
		"bootstrap")
}

const (
	localBackendHost       = "api-server.local"
	localBackendRemoteAddr = "127.0.0.1:0"
)

type localBackendCaller struct {
	handler http.Handler
}

func (c *localBackendCaller) Call(ctx context.Context, req *mcpwiring.InternalBackendRequest) (
	*mcpwiring.InternalBackendResponse, error) {

	if c == nil || c.handler == nil {
		return nil, errors.New("local backend caller is not initialized")
	}
	if req == nil {
		return nil, errors.New("local backend request is nil")
	}
	if strings.TrimSpace(req.Method) == "" {
		return nil, errors.New("local backend request method is empty")
	}
	localURL, err := newLocalBackendURL(req.Path)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// RequestURI 必须与 URL 保持一致，否则 proxy.Do 里 r.RequestURI 为空，
	// 导致转发到下游服务时路径丢失（变成 "http://host" 而无路径）。
	requestURI := localURL.Path
	if localURL.RawQuery != "" {
		requestURI += "?" + localURL.RawQuery
	}
	httpReq := (&http.Request{
		Method:        req.Method,
		URL:           localURL,
		RequestURI:    requestURI,
		Host:          localBackendHost,
		Header:        req.Headers.Clone(),
		Body:          io.NopCloser(bytes.NewReader(req.Body)),
		ContentLength: int64(len(req.Body)),
		RemoteAddr:    localBackendRemoteAddr,
	}).WithContext(ctx)

	recorder := newLocalBackendResponseRecorder()
	c.handler.ServeHTTP(recorder, httpReq)

	return &mcpwiring.InternalBackendResponse{
		StatusCode: recorder.status(),
		Body:       recorder.body.Bytes(),
	}, nil
}

func newLocalBackendURL(pathWithQuery string) (*url.URL, error) {
	pathWithQuery = strings.TrimSpace(pathWithQuery)
	if pathWithQuery == "" {
		return nil, errors.New("local backend request path is empty")
	}
	if strings.Contains(pathWithQuery, "\\") {
		return nil, fmt.Errorf("local backend request path must not contain backslash: %q", pathWithQuery)
	}
	if strings.HasPrefix(pathWithQuery, "//") || !strings.HasPrefix(pathWithQuery, "/") {
		return nil, fmt.Errorf("local backend request path must be an absolute local path: %q", pathWithQuery)
	}

	parsed, err := url.ParseRequestURI(pathWithQuery)
	if err != nil {
		return nil, fmt.Errorf("parse local backend request path %q failed: %w", pathWithQuery, err)
	}
	if parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil {
		return nil, fmt.Errorf("local backend request path must not include scheme or host: %q", pathWithQuery)
	}
	if parsed.Path == "" || parsed.Path[0] != '/' || strings.HasPrefix(parsed.Path, "//") {
		return nil, fmt.Errorf("local backend request path must be an absolute local path: %q", pathWithQuery)
	}
	if strings.Contains(parsed.Path, "/../") || strings.HasSuffix(parsed.Path, "/..") ||
		strings.Contains(parsed.Path, "/./") || strings.HasSuffix(parsed.Path, "/.") {
		return nil, fmt.Errorf("local backend request path must not contain dot segments: %q", pathWithQuery)
	}
	if !isAllowedLocalBackendPath(parsed.Path) {
		return nil, fmt.Errorf("local backend request path is not allowed: %q", parsed.Path)
	}

	return &url.URL{Path: parsed.Path, RawQuery: parsed.RawQuery}, nil
}

func isAllowedLocalBackendPath(p string) bool {
	for _, prefix := range []string{"/api/v1/cloud/", "/api/v1/woa/", "/api/v1/account/"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

type localBackendResponseRecorder struct {
	header      http.Header
	body        bytes.Buffer
	code        int
	wroteHeader bool
}

func newLocalBackendResponseRecorder() *localBackendResponseRecorder {
	return &localBackendResponseRecorder{
		header: make(http.Header),
	}
}

func (r *localBackendResponseRecorder) Header() http.Header {
	return r.header
}

func (r *localBackendResponseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.body.Write(b)
}

func (r *localBackendResponseRecorder) WriteHeader(statusCode int) {
	if r.wroteHeader {
		return
	}
	r.code = statusCode
	r.wroteHeader = true
}

func (r *localBackendResponseRecorder) status() int {
	if !r.wroteHeader {
		return http.StatusOK
	}
	return r.code
}

// Healthz service health check.
func (s *Service) Healthz(w http.ResponseWriter, r *http.Request) {
	if shutdown.IsShuttingDown() {
		logs.Errorf("service healthz check failed, current service is shutting down")
		w.WriteHeader(http.StatusServiceUnavailable)
		rest.WriteResp(w, rest.NewBaseResp(errf.UnHealthy, "current service is shutting down"))
		return
	}

	if err := serviced.Healthz(r.Context(), cc.ApiServer().Service); err != nil {
		logs.Errorf("serviced healthz check failed, err: %v", err)
		rest.WriteResp(w, rest.NewBaseResp(errf.UnHealthy, "serviced healthz error, "+err.Error()))
		return
	}

	rest.WriteResp(w, rest.NewBaseResp(errf.OK, "healthy"))
	return
}

// Alivez simply returns OK to indicate the service is alive.
func (s *Service) Alivez(w http.ResponseWriter, r *http.Request) {
	if shutdown.IsShuttingDown() {
		logs.Errorf("service %s alivez check failed, current service is shutting down", cc.ServiceName())
		w.WriteHeader(http.StatusServiceUnavailable)
		rest.WriteResp(w, rest.NewBaseResp(errf.UnHealthy,
			fmt.Sprintf("service %s is shutting down", cc.ServiceName())))
		return
	}

	rest.WriteResp(w, rest.NewBaseResp(errf.OK, "alive"))
	return
}
