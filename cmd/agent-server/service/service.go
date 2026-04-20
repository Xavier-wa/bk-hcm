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

// Package service builds and owns the HTTP server for agent-server.
// It mounts the AG-UI endpoint, and any Channel HTTP ingress handlers
// onto a standard net/http ServeMux.
package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hcm/cmd/agent-server/logics"
	"hcm/cmd/agent-server/service/capability"
	"hcm/cmd/agent-server/service/memory"
	"hcm/cmd/agent-server/service/session"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/handler"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	restcli "hcm/pkg/rest/client"
	"hcm/pkg/runtime/shutdown"
	"hcm/pkg/serviced"
	"hcm/pkg/tools/ssl"
	"hcm/pkg/tools/uuid"

	"github.com/emicklei/go-restful/v3"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/server/agui"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/adapter"
	aguirunner "trpc.group/trpc-go/trpc-agent-go/server/agui/runner"
)

const (
	// headerBKTicket is the incoming request header that carries the BK login ticket.
	// The value is forwarded to the downstream BK API gateway as bk_ticket.
	// For MCP toolsets of type "bkaidev", this same value is used to construct
	// the X-Bkapi-Authorization header on each MCP request.
	headerBKTicket = "X-Bk-Ticket"

	// aguiPath is the HTTP path for the AG-UI endpoint.
	aguiPath = "/api/v1/agent/agui"
	// aguiCancelPath is the HTTP path for the AG-UI cancel endpoint.
	aguiCancelPath = "/api/v1/agent/cancel"
	// aguiHistoryPath is the HTTP path for the AG-UI history (MessagesSnapshot) endpoint.
	aguiHistoryPath = "/api/v1/agent/history"
)

// Service do all the agent server's work
type Service struct {
	serve     *http.Server
	clientSet *client.ClientSet
	runTime   *logics.Runtime
}

// NewService create a service instance.
func NewService(sd serviced.ServiceDiscover) (*Service, error) {
	tlsConfig, err := initTLSConfig()
	if err != nil {
		return nil, err
	}

	apiClientSet, err := initAPIClient(tlsConfig, sd)
	if err != nil {
		return nil, err
	}

	rt, err := logics.New()
	if err != nil {
		logs.Errorf("init runtime failed, err: %v", err)
		return nil, fmt.Errorf("init runtime: %v", err)
	}

	return &Service{
		clientSet: apiClientSet,
		runTime:   rt,
	}, nil
}

// initTLSConfig 初始化TLS配置
func initTLSConfig() (*ssl.TLSConfig, error) {
	tls := cc.AgentServer().Network.TLS
	if !tls.Enable() {
		return nil, nil
	}

	return &ssl.TLSConfig{
		InsecureSkipVerify: tls.InsecureSkipVerify,
		CertFile:           tls.CertFile,
		KeyFile:            tls.KeyFile,
		CAFile:             tls.CAFile,
		Password:           tls.Password,
	}, nil
}

// initAPIClient 初始化API客户端
func initAPIClient(tlsConfig *ssl.TLSConfig, dis serviced.ServiceDiscover) (*client.ClientSet, error) {
	restCli, err := restcli.NewClient(tlsConfig)
	if err != nil {
		return nil, err
	}
	return client.NewClientSet(restCli, dis), nil
}

// ListenAndServeRest listen and serve the restful server
func (s *Service) ListenAndServeRest() error {
	root := http.NewServeMux()

	// Mount AG-UI endpoint when enabled.
	if err := s.mountAGUI(root); err != nil {
		return err
	}

	root.HandleFunc("/", s.apiSet().ServeHTTP)
	root.HandleFunc("/healthz", s.Healthz)
	root.HandleFunc("/alivez", s.Alivez)
	handler.SetCommonHandler(root)

	network := cc.AgentServer().Network
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
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logs.Errorf("serve restful server failed, err: %v", err)
			shutdown.SignalShutdownGracefully()
		}
	}()

	s.serve = server

	return nil
}

// mountAGUI mounts the AG-UI endpoint onto the provided mux when enabled.
func (s *Service) mountAGUI(mux *http.ServeMux) error {
	svcCfg := cc.AgentServer().AGUI
	if !svcCfg.Enable {
		return nil
	}

	//   - "/api/v1/agent/cancel" → AG-UI cancel handler (only when opt.EnableAGUI is true)
	//   - "/api/v1/agent/history" → AG-UI history handler (only when session backend is configured)
	//   - "/api/v1/agent/sessions/{thread_id}/context-stats" → session context stats (only when session is configured)
	//   - "POST /api/v1/agent/memory" → add a memory entry (only when memory backend is configured)
	//   - "GET /api/v1/agent/memory" → list / search memory entries (only when memory backend is configured)
	//   - "DELETE /api/v1/agent/memory/{memory_id}" → delete a memory entry (only when memory backend is configured)
	//   - "DELETE /api/v1/agent/memory" → clear all memory entries (only when memory backend is configured)
	aguiOpts := []agui.Option{
		agui.WithPath(aguiPath),
		// Cancel endpoint: clients POST {threadId} to abort an in-progress run.
		agui.WithCancelEnabled(true),
		agui.WithCancelPath(aguiCancelPath),
		agui.WithAGUIRunnerOptions(
			aguirunner.WithUserIDResolver(resolveAGUIUserID),
			aguirunner.WithRunOptionResolver(makeModelRunOptionResolver(svcCfg.AllowedModels)),
			// Auto-cancel the LLM call when the SSE connection drops (client disconnects).
			aguirunner.WithCancelOnContextDoneEnabled(true),
		),
	}

	if svcCfg.AppName != "" {
		aguiOpts = append(aguiOpts, agui.WithAppName(svcCfg.AppName))
	}

	// History endpoint requires a MySQL session backend; enable it automatically
	// when one is configured so no extra config flag is needed.
	sessionSvc := s.runTime.SessionSvc()
	if sessionSvc != nil {
		aguiOpts = append(aguiOpts,
			agui.WithSessionService(sessionSvc),
			agui.WithMessagesSnapshotEnabled(true),
			agui.WithMessagesSnapshotPath(aguiHistoryPath),
		)
	}

	aguiServer, err := agui.New(s.runTime.AGUIRunner, aguiOpts...)
	if err != nil {
		return fmt.Errorf("create AG-UI server failed: %v", err)
	}

	// cancel 和 history 路径注册在 aguiServer 内部的 ServeMux 中，
	// 外部 mux 也必须单独挂载同一个 handler，才能将请求路由进去。
	mux.Handle(aguiServer.Path(), aguiServer.Handler())
	mux.Handle(aguiCancelPath, aguiServer.Handler())
	if sessionSvc != nil {
		mux.Handle(aguiHistoryPath, aguiServer.Handler())
	}

	return nil
}

func (s *Service) apiSet() *restful.Container {
	ws := new(restful.WebService)
	ws.Path("/api/v1/agent")
	ws.Produces(restful.MIME_JSON)

	c := &capability.Capability{
		WebService: ws,
		ClientSet:  s.clientSet,
	}

	appName := cc.AgentServer().AGUI.AppName
	memory.InitService(c, s.runTime.MemorySvc(), appName)
	session.InitService(c, s.runTime.SessionSvc(), appName)

	return restful.NewContainer().Add(c.WebService)
}

// Healthz check whether the service is healthy.
func (s *Service) Healthz(w http.ResponseWriter, r *http.Request) {
	if shutdown.IsShuttingDown() {
		logs.Errorf("service healthz check failed, current service is shutting down")
		w.WriteHeader(http.StatusServiceUnavailable)
		rest.WriteResp(w, rest.NewBaseResp(errf.UnHealthy, "current service is shutting down"))
		return
	}

	if err := serviced.Healthz(r.Context(), cc.AgentServer().Service); err != nil {
		logs.Errorf("etcd healthz check failed, err: %v", err)
		rest.WriteResp(w, rest.NewBaseResp(errf.UnHealthy, "etcd healthz error, "+err.Error()))
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

// bkapiContextMiddleware extracts BK auth parameters from the incoming HTTP
// request headers and injects them into the request context so that the
// downstream LLM client middleware and MCP toolsets of type "bkaidev" can include
// them in the X-Bkapi-Authorization header when calling the BK API gateway.
//
// Recognised headers:
//   - X-Bkapi-User-Name (constant.UserKey) → logics.WithBKUsername
//   - X-Bk-Ticket                          → logics.WithBKTicket
func bkapiContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(constant.RidKey) == "" {
			r.Header.Set(constant.RidKey, uuid.UUID())
		}

		ctx := r.Context()
		if username := r.Header.Get(constant.UserKey); username != "" {
			ctx = logics.WithBKUsername(ctx, username)
		}
		if ticket := r.Header.Get(headerBKTicket); ticket != "" {
			ctx = logics.WithBKTicket(ctx, ticket)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveAGUIUserID derives the session user identifier for an AG-UI run from
// the request context (populated by bkapiContextMiddleware from X-Bkapi-User-Name).
// Falls back to "anonymous" only when the header is absent (e.g. unauthenticated dev calls).
func resolveAGUIUserID(ctx context.Context, _ *adapter.RunAgentInput) (string, error) {
	if v := logics.BKUsernameFromContext(ctx); v != "" {
		return v, nil
	}
	return "anonymous", nil
}

// makeModelRunOptionResolver returns an AG-UI RunOptionResolver that reads the
// "modelName" field from forwardedProps and translates it into an agent.WithModelName
// RunOption. If the requested model is not in the allowed list the run is rejected
// with an error so the client receives a clear HTTP 500 / RunError event instead of
// silently falling back to the default model.
//
// allowedModels is the effective list already computed by runtime.New (config or
// platform defaults); it is never empty when this resolver is called.
func makeModelRunOptionResolver(allowedModels []string) aguirunner.RunOptionResolver {
	allowed := make(map[string]struct{}, len(allowedModels))
	for _, m := range allowedModels {
		allowed[m] = struct{}{}
	}
	return func(_ context.Context, input *adapter.RunAgentInput) ([]agent.RunOption, error) {
		props, ok := input.ForwardedProps.(map[string]any)
		if !ok {
			return nil, nil
		}
		modelName, _ := props["modelName"].(string)
		if modelName = strings.TrimSpace(modelName); modelName == "" {
			return nil, nil
		}
		if _, ok := allowed[modelName]; !ok {
			return nil, fmt.Errorf("model %q is not in the allowed models list", modelName)
		}
		return []agent.RunOption{agent.WithModelName(modelName)}, nil
	}
}
