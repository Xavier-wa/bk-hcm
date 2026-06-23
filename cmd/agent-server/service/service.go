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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hcm/cmd/agent-server/logics"
	authlogic "hcm/cmd/agent-server/logics/auth"
	"hcm/cmd/agent-server/logics/prompt"
	"hcm/cmd/agent-server/logics/skill"
	"hcm/cmd/agent-server/service/a2a"
	aguievent "hcm/cmd/agent-server/service/agui-event"
	"hcm/cmd/agent-server/service/capability"
	configsvc "hcm/cmd/agent-server/service/config"
	"hcm/cmd/agent-server/service/memory"
	promptsvc "hcm/cmd/agent-server/service/prompt"
	"hcm/cmd/agent-server/service/session"
	skillsvc "hcm/cmd/agent-server/service/skill"
	"hcm/cmd/agent-server/types/readiness"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/cron"
	"hcm/pkg/cron/core"
	"hcm/pkg/handler"
	"hcm/pkg/iam/auth"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/metrics"
	"hcm/pkg/rest"
	restcli "hcm/pkg/rest/client"
	"hcm/pkg/runtime/shutdown"
	"hcm/pkg/serviced"
	"hcm/pkg/tools/ssl"
	"hcm/pkg/tools/uuid"

	"github.com/emicklei/go-restful/v3"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/server/agui"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/adapter"
	aguirunner "trpc.group/trpc-go/trpc-agent-go/server/agui/runner"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// Service do all the agent server's work
type Service struct {
	serve      *http.Server
	authorizer auth.Authorizer
	clientSet  *client.ClientSet
	resolver   *session.Resolver
	runTime    *logics.Runtime
	tasks      map[enumor.CronTask]core.Task
}

// NewService create a service instance.
func NewService(sd serviced.ServiceDiscover) (*Service, error) {
	tlsConfig, err := initTLSConfig()
	if err != nil {
		return nil, err
	}

	// Create authorizer for IAM permission checks.
	authorizer, err := auth.NewAuthorizer(sd, cc.AgentServer().Network.TLS)
	if err != nil {
		return nil, fmt.Errorf("create authorizer failed: %w", err)
	}

	apiClientSet, err := initAPIClient(tlsConfig, sd)
	if err != nil {
		return nil, err
	}

	rt, err := logics.New(apiClientSet)
	if err != nil {
		logs.Errorf("init runtime failed, err: %v", err)
		return nil, fmt.Errorf("init runtime: %v", err)
	}

	svc := &Service{
		authorizer: authorizer,
		clientSet:  apiClientSet,
		resolver:   session.NewResolver(apiClientSet.DataService()),
		runTime:    rt,
	}
	if err = svc.initCronTasks(); err != nil {
		return nil, err
	}

	return svc, nil
}

func (s *Service) initCronTasks() error {
	s.tasks = make(map[enumor.CronTask]core.Task)

	if err := cron.Init(context.Background(), metrics.Register()); err != nil {
		return fmt.Errorf("init cron: %w", err)
	}

	tasks := make([]core.Task, 0)
	skillSyncTask, err := skill.NewSyncCronTask(s.runTime.SkillSyncer())
	if err != nil {
		logs.Errorf("init skill sync cron task failed, err: %v", err)
		return err
	}
	if skillSyncTask != nil {
		s.tasks[enumor.CronTaskSyncAgentSkills] = skillSyncTask
		tasks = append(tasks, skillSyncTask)
	}

	promptTask, err := prompt.NewSyncCronTask(s.runTime.PromptSyncer())
	if err != nil {
		logs.Errorf("init prompt sync cron task failed, err: %v", err)
		return err
	}
	if promptTask != nil {
		s.tasks[enumor.CronTaskSyncAgentPrompts] = promptTask
		tasks = append(tasks, promptTask)
	}

	if err = cron.Register(tasks); err != nil {
		return fmt.Errorf("register skill sync cron: %w", err)
	}

	return nil
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

	// Mount A2A endpoint when enabled. A2A 与 AG-UI 完全独立：
	// 路径前缀虽然都在 /api/v1/agent 之下，但 A2A 走自己的中间件链
	// （mcpCallerOrigin → bkapi-context → readiness），不复用 AG-UI 的 session
	// resolver 与 IAM 鉴权，避免对现有链路造成影响。
	if err := s.mountA2A(root); err != nil {
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
		agui.WithPath(constant.AGUIPath),
		// Cancel endpoint: clients POST {threadId} to abort an in-progress run.
		agui.WithCancelEnabled(true),
		agui.WithCancelPath(constant.AGUICancelPath),
		// Increase post-run finalization timeout to allow events to be persisted
		// even when the request is canceled. This helps prevent incomplete event
		// sequences (e.g., TEXT_MESSAGE_CONTENT without TEXT_MESSAGE_START).
		agui.WithPostRunFinalizationTimeout(20 * time.Second),
		// 展示思考内容
		agui.WithReasoningContentEnabled(svcCfg.Model.DisplayReasoning),
		// 开启 graph interrupt 事件流，使 AGUI 前端能感知中断状态
		agui.WithGraphNodeInterruptActivityEnabled(true),
		agui.WithAGUIRunnerOptions(
			aguirunner.WithUserIDResolver(resolveAGUIUserID),
			aguirunner.WithRunOptionResolver(
				makeRunOptionResolver(s.runTime.CheckpointSaver(),
					svcCfg.AllowedModelNames(), s.runTime.DynamicToolFilter())),
			aguirunner.WithTranslatorFactory(aguievent.NewCustomTranslator),
			// Auto-cancel the LLM call when the SSE connection drops (client disconnects).
			// NOTE: When ctx ends, the request stops immediately, so recorded conversation events may be incomplete.
			// aguirunner.WithCancelOnContextDoneEnabled(true),
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
			// 开启会话历史消息快照
			agui.WithMessagesSnapshotEnabled(true),
			// 开启会话消息快照续传
			agui.WithMessagesSnapshotFollowEnabled(true),
			agui.WithFlushInterval(50*time.Millisecond),
			agui.WithMessagesSnapshotPath(constant.AGUIHistoryPath),
		)
	}

	aguiServer, err := agui.New(s.runTime.AGUIRunner, aguiOpts...)
	if err != nil {
		return fmt.Errorf("create AG-UI server failed: %v", err)
	}

	// /agui、/cancel 和 /history 在 session-code middleware 可用时进行包装，
	// 使得 session_code 在请求到达 AG-UI runner 前被解析为 thread_id。
	aguiHandler := s.sessionCodeMiddleware(aguiServer.Handler())
	// 添加权限校验中间件
	authMW := agentAuthMiddleware(s.authorizer)
	aguiHandler = authMW(aguiHandler)
	// 注入 BK 用户信息到 context，供 AGUI runner 使用
	aguiHandler = bkapiContextMiddleware(aguiHandler)
	// block AGUI until skill/prompt initial sync completes
	aguiHandler = readinessMiddleware(s.runTime.Readiness(), aguiHandler)
	// cancel 和 history 路径注册在 aguiServer 内部的 ServeMux 中，
	// 外部 mux 也必须单独挂载同一个 handler，才能将请求路由进去。
	mux.Handle(aguiServer.Path(), aguiHandler)
	mux.Handle(constant.AGUICancelPath, aguiHandler)
	if sessionSvc != nil {
		mux.Handle(constant.AGUIHistoryPath, aguiHandler)
	}

	return nil
}

// mountA2A mounts the A2A protocol endpoints onto the provided mux when enabled.
//
// 端点（默认 basePath="/api/v1/agent"）：
//   - POST /api/v1/agent/a2a                                 → JSON-RPC 入口
//   - GET  /api/v1/agent/.well-known/agent-card.json         → A2A v0.2.2 AgentCard
//   - GET  /api/v1/agent/.well-known/agent.json              → A2A 0.1.x 兼容 AgentCard
//
// 中间件链（从外到内）：
//
//	mcpCallerOrigin → bkapiContext → readiness → a2a.Handler
//
// 其中 mcpCallerOrigin 校验上游 X-Bkhcm-Caller-Source；bkapiContext 注入
// bk_username 等到 ctx，使 internal MCP toolset 能正确拼装下游请求头；
// readiness 保证 skill/prompt 完成首轮同步后才放行。
func (s *Service) mountA2A(mux *http.ServeMux) error {
	cfg := cc.AgentServer().A2A
	if !cfg.Enable {
		return nil
	}

	srv, err := a2a.New(cfg, s.runTime.AGUIRunner, cc.AgentServer().AGUI.Model.Stream)
	if err != nil {
		return fmt.Errorf("create A2A server failed: %v", err)
	}

	h := srv.Handler()
	h = readinessMiddleware(s.runTime.Readiness(), h)
	h = bkapiContextMiddleware(h)
	h = mcpCallerOriginMiddleware(cfg.EnforceCallerOrigin, h)

	srv.RegisterHandlers(mux, h)
	logs.Infof("a2a: endpoints mounted, jsonRPCPath=%s, cardPath=%s, legacyCardPath=%s, "+
		"enforceCallerOrigin=%v",
		srv.JSONRPCPath(), srv.AgentCardPath(), srv.AgentLegacyCardPath(),
		cfg.EnforceCallerOrigin)
	return nil
}

// mcpCallerOriginMiddleware 校验请求头 X-Bkhcm-Caller-Source 是否为 api-server。
//
// 行为：
//   - enforce=true：缺失或不匹配时直接返回 HTTP 403；
//   - enforce=false（默认）：仅记录 warn 日志便于联调期监控，不拦截。
//
// 该中间件仅作用于 A2A 入口，不会影响 AG-UI 链路。
func mcpCallerOriginMiddleware(enforce bool, next http.Handler) http.Handler {
	expected := string(cc.APIServerName)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get(constant.MCPCallerSourceHeader)
		if origin != expected {
			rid := r.Header.Get(constant.RidKey)
			if enforce {
				logs.Errorf("a2a: caller origin check failed, expect=%q got=%q, "+
					"path=%s, rid: %s", expected, origin, r.URL.Path, rid)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				rest.WriteResp(w, rest.NewBaseResp(errf.PermissionDenied,
					"caller origin is not allowed for A2A endpoint"))
				return
			}
			logs.Warnf("a2a: caller origin missing or mismatched, expect=%q got=%q, "+
				"path=%s, rid: %s (enforcement disabled)",
				expected, origin, r.URL.Path, rid)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Service) apiSet() *restful.Container {
	ws := new(restful.WebService)
	ws.Path("/api/v1/agent")
	ws.Produces(restful.MIME_JSON)

	c := &capability.Capability{
		WebService: ws,
		ClientSet:  s.clientSet,
		Authorizer: s.authorizer,
		RunTime:    s.runTime,
		Tasks:      s.tasks,
	}

	memory.InitService(c)
	session.InitService(c, s.resolver)
	configsvc.InitService(c)
	skillsvc.InitService(c)
	promptsvc.InitService(c)
	// 提供前端判断 Agent 是否就绪的接口（走 rest.Handler 统一封装 result/code/message/data）
	readinessH := rest.NewHandler()
	readinessH.Add("AgentReadiness", http.MethodGet, "/readiness", s.AgentReadiness)
	readinessH.Load(ws)

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

// sessionCodeMiddleware intercepts /agui, /cancel and /history requests, resolves sessionCode to threadId,
// generates a runId, rewrites the request body, and forwards to the downstream handler.
// For /agui requests, it also asynchronously increments session_content_count after SSE ends.
func (s *Service) sessionCodeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		sessionCode, reqMap, err := extractSessionCode(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		kt, err := kit.FromHeader(r.Context(), r.Header)
		if err != nil {
			logs.Errorf("sessionCodeMiddleware: build kit from header failed: %v", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		sessionMeta, httpStatus, err := s.resolveAndValidateSession(kt, sessionCode)
		if err != nil {
			http.Error(w, err.Error(), httpStatus)
			return
		}
		threadID := sessionMeta.ThreadID

		delete(reqMap, "sessionCode")
		reqMap["threadId"] = threadID
		reqMap["runId"] = uuid.UUID()
		// 将会话场景标签通过 forwardedProps 透传，供 Graph 首轮注入 StateKeySessionTag
		if sessionMeta.SessionTag != "" {
			logs.Infof("session tag exist, inject into forwardedProps, session_code: %s, tag: %s, rid: %s",
				sessionCode, sessionMeta.SessionTag, kt.Rid)
			injectForwardedSessionTag(reqMap, sessionMeta.SessionTag)
		}

		newBody, err := json.Marshal(reqMap)
		if err != nil {
			http.Error(w, "failed to rewrite request body", http.StatusInternalServerError)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(newBody))
		r.ContentLength = int64(len(newBody))

		isAGUI := strings.HasSuffix(r.URL.Path, "/agui")
		if isAGUI {
			go s.asyncIncrContentCount(kt, sessionCode, threadID, sessionMeta.SessionTag)
			if sessionMeta.BkBizID > 0 {
				r = r.WithContext(authlogic.WithBkBizID(r.Context(), sessionMeta.BkBizID))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// extractSessionCode parses the raw JSON body, validates and extracts the sessionCode field.
// Returns the sessionCode string and the full request map for body rewriting.
func extractSessionCode(body []byte) (string, map[string]interface{}, error) {
	var reqMap map[string]interface{}
	if err := json.Unmarshal(body, &reqMap); err != nil {
		return "", nil, errors.New("invalid JSON body")
	}

	sessionCodeVal, ok := reqMap["sessionCode"]
	if !ok {
		return "", nil, errors.New(`missing field "sessionCode"`)
	}

	sessionCode, ok := sessionCodeVal.(string)
	if !ok || strings.TrimSpace(sessionCode) == "" {
		return "", nil, errors.New(`"sessionCode" must be a non-empty string`)
	}

	return sessionCode, reqMap, nil
}

// resolveAndValidateSession resolves sessionCode to its metadata and verifies
// that the resolved user matches the authenticated user in kt.
// Returns the session metadata, an HTTP status code and an error on failure.
func (s *Service) resolveAndValidateSession(kt *kit.Kit, sessionCode string) (*session.SessionMeta, int, error) {
	sessionMeta, err := s.resolver.ResolveMeta(kt, sessionCode)
	if err != nil {
		logs.Errorf("resolve session code failed, session_code: %s, err: %v, rid: %s", sessionCode, err, kt.Rid)
		return nil, http.StatusBadRequest, errors.New("invalid session code")
	}

	if sessionMeta.User != kt.User {
		logs.Errorf("session code user mismatch, session_code: %s, session_user: %s, user: %s, rid: %s",
			sessionCode, sessionMeta.User, kt.User, kt.Rid)
		return nil, http.StatusForbidden, errors.New("permission denied")
	}

	return sessionMeta, http.StatusOK, nil
}

// asyncIncrContentCount asynchronously increments the session_content_count for the given sessionCode.
func (s *Service) asyncIncrContentCount(kt *kit.Kit, sessionCode string, threadID string,
	sessionTag enumor.IntentType) {

	ctx, cancel := context.WithTimeout(context.Background(), constant.SessionIncrContentCountTimeout)
	defer cancel()

	asyncKt := kt.NewSubKitWithCtx(ctx)
	req := &dsaiagent.IncrContentCountReq{SessionCode: sessionCode}
	if err := s.clientSet.DataService().Aiagent.Session.IncrContentCount(asyncKt, req); err != nil {
		logs.Errorf("async incr content count failed, session_code: %s, err: %v, rid: %s",
			sessionCode, err, asyncKt.Rid)
	}
	// Run 结束后对账：意图识别命中受支持场景时回写 session_tag
	s.reconcileSessionTag(asyncKt, sessionCode, threadID, sessionTag)
}

// injectForwardedSessionTag merges the session tag into the request body's forwardedProps map.
func injectForwardedSessionTag(reqMap map[string]interface{}, sessionTag enumor.IntentType) {
	fp, _ := reqMap["forwardedProps"].(map[string]interface{})
	if fp == nil {
		fp = make(map[string]interface{})
	}
	fp[constant.ForwardedPropSessionTag] = string(sessionTag)
	reqMap["forwardedProps"] = fp
}

// reconcileSessionTag writes back the scene tag recognised during the run when the
// session started without a tag. It reads the latest checkpoint for StateKeySessionTag,
// persists it via data-service, and refreshes the resolver cache.
// 该操作为尽力而为，失败仅记录 Warn 日志，不影响对话。
func (s *Service) reconcileSessionTag(kt *kit.Kit, sessionCode, threadID string, originalTag enumor.IntentType) {
	// TODO：目前会话标签不允许修改，所有有标签的会话不需要回写，只会写意图识别出来的场景
	// 未来需要支持修改标签时，需要修改这里
	if originalTag != "" {
		// 已绑定标签的会话无需回写
		logs.Infof("reconcile session tag: session already has tag, session_code: %s, tag: %s, rid: %s",
			sessionCode, originalTag, kt.Rid)
		return
	}

	saver := s.runTime.CheckpointSaver()
	if saver == nil {
		logs.Infof("reconcile session tag: checkpoint saver is nil, session_code: %s, rid: %s",
			sessionCode, kt.Rid)
		return
	}

	cm := graph.NewCheckpointManager(saver)
	tuple, err := cm.Latest(kt.Ctx, threadID, "")
	if err != nil {
		logs.Warnf("reconcile session tag: get latest checkpoint failed, thread: %s, err: %v, rid: %s",
			threadID, err, kt.Rid)
		return
	}
	if tuple == nil || tuple.Checkpoint == nil {
		return
	}

	var tag enumor.IntentType
	switch value := tuple.Checkpoint.ChannelValues[constant.StateKeySessionTag].(type) {
	case enumor.IntentType:
		tag = value
	case string:
		tag = enumor.IntentType(value)
	}
	if tag == "" {
		logs.Infof("reconcile session tag: checkpoint has no session tag, session_code: %s, rid: %s",
			sessionCode, kt.Rid)
		return
	}

	updateReq := &dsaiagent.UpdateAiagentSessionReq{
		ID:         threadID,
		Reviser:    kt.User,
		SessionTag: tag,
	}
	if err := s.clientSet.DataService().Aiagent.Session.Update(kt, updateReq); err != nil {
		logs.Warnf("reconcile session tag: write back failed, session_code: %s, tag: %s, err: %v, rid: %s",
			sessionCode, tag, err, kt.Rid)
		return
	}

	s.resolver.UpdateCachedSessionTag(kt, sessionCode, tag)
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
		rid := r.Header.Get(constant.RidKey)
		if rid == "" {
			rid = uuid.UUID()
			r.Header.Set(constant.RidKey, rid)
		}

		ctx := context.WithValue(r.Context(), constant.RidKey, rid)
		if username := r.Header.Get(constant.UserKey); username != "" {
			ctx = authlogic.WithBKUsername(ctx, username)
		}
		if ticket := r.Header.Get(constant.BKTicket); ticket != "" {
			ctx = authlogic.WithBKTicket(ctx, ticket)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// agentAuthMiddleware returns an HTTP middleware that verifies the caller has
// the AgentAssistant permission via IAM before forwarding the request.
func agentAuthMiddleware(authorizer auth.Authorizer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			kt, err := kit.FromHeader(r.Context(), r.Header)
			if err != nil {
				logs.Errorf("agent auth: build kit failed, err: %v", err)
				w.WriteHeader(http.StatusUnauthorized)
				rest.WriteResp(w, rest.NewBaseResp(errf.DoAuthorizeFailed, "invalid request identity"))
				return
			}

			if err := authorizer.AuthorizeWithPerm(kt, meta.ResourceAttribute{
				Basic: &meta.Basic{Type: meta.AgentAssistant, Action: meta.Find},
			}); err != nil {
				logs.Errorf("agent auth: permission denied, user: %s, err: %v, rid: %s", kt.User, err, kt.Rid)
				w.WriteHeader(http.StatusForbidden)
				rest.WriteResp(w, rest.NewBaseResp(errf.PermissionDenied, "no permission to access agent assistant"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// resolveAGUIUserID derives the session user identifier for an AG-UI run from
// the request context (populated by bkapiContextMiddleware from X-Bkapi-User-Name).
// Falls back to "anonymous" only when the header is absent (e.g. unauthenticated dev calls).
func resolveAGUIUserID(ctx context.Context, _ *adapter.RunAgentInput) (string, error) {
	if v := authlogic.BKUsernameFromContext(ctx); v != "" {
		return v, nil
	}
	return "anonymous", nil
}

// makeRunOptionResolver returns an AG-UI RunOptionResolver that handles model
// selection, dynamic tool filtering, and automatic checkpoint resume.
//
// Model selection: reads "modelName" from forwardedProps and translates it into
// agent.WithModelName. Rejects unknown models with an error.
//
// Tool filtering: when toolFilter is non-nil, injects agent.WithToolFilter so
// that each Run performs index-based tool retrieval.
func makeRunOptionResolver(saver graph.CheckpointSaver, allowedModels []string,
	toolFilter tool.FilterFunc) aguirunner.RunOptionResolver {

	allowed := make(map[string]struct{}, len(allowedModels))
	for _, m := range allowedModels {
		allowed[m] = struct{}{}
	}
	return func(ctx context.Context, input *adapter.RunAgentInput) ([]agent.RunOption, error) {
		var opts []agent.RunOption

		// Bind lineageID to threadID so checkpoints can be queried by thread.
		runtimeState := map[string]any{
			graph.CfgKeyLineageID: input.ThreadID,
		}

		// Inject bk_biz_id from session context when the session belongs to a business.
		bkBizID := authlogic.BkBizIDFromContext(ctx)
		if bkBizID > 0 {
			runtimeState[constant.SessionBkBizIDStateKey] = bkBizID
		}

		// Auto-detect interrupted checkpoint and prepare resume (HITL / fallback interrupt).
		// resume command 承载用户输入文本；forwardedProps 的结构化值写入独立 runtime-state key。
		var forwardedProps map[string]any
		if props, ok := input.ForwardedProps.(map[string]any); ok {
			forwardedProps = props
		}
		runtimeState = tryPrepareAutoResume(saver, ctx, input, runtimeState, forwardedProps)

		// ForwardedProps: model selection.
		if modelName, _ := forwardedProps["modelName"].(string); modelName != "" {
			modelName = strings.TrimSpace(modelName)
			if _, ok := allowed[modelName]; !ok {
				return nil, fmt.Errorf("model %q is not in the allowed models list", modelName)
			}
			opts = append(opts, agent.WithModelName(modelName))
		}

		// 注入会话场景标签：非空时写入 StateKeySessionTag，供 scene_dispatch 首轮直达
		if sessionTag, _ := forwardedProps[constant.ForwardedPropSessionTag].(string); sessionTag != "" {
			tag := enumor.IntentType(sessionTag)
			if tag.Validate() != nil {
				return nil, fmt.Errorf("session tag %q is not valid", sessionTag)
			}
			logs.Infof("session tag is exist in forwarded, tag is %s, type is %T, rid: %s", tag, tag,
				rest.RidFromContext(ctx))
			runtimeState[constant.StateKeySessionTag] = tag
		}

		opts = append(opts, agent.WithRuntimeState(runtimeState))

		// Dynamic tool filtering.
		if toolFilter != nil {
			opts = append(opts, agent.WithToolFilter(toolFilter))
		}

		return opts, nil
	}
}

// tryPrepareAutoResume checks if the thread has an interrupted checkpoint and, if
// so, injects the checkpointID and resume value into runtimeState so the graph
// continues from the interrupt point.
//
// resume command 始终承载最新的用户消息文本（非格式化、无法预期的自由输入）。
// forwardedProps 中的结构化内容（如选中的 account_id）则写入独立的
// StateKeyForwardedResumeValue，与用户输入区分开，供节点单独消费。
//
// NOTE: mergeInitialStateNonInternal skips keys starting with "_", so
// StateKeyCommand (processed by processResumeCommand) must be used instead of
// writing ResumeChannel directly.
func tryPrepareAutoResume(saver graph.CheckpointSaver, ctx context.Context, input *adapter.RunAgentInput,
	runtimeState map[string]any, forwardedProps map[string]any) map[string]any {

	rid := rest.RidFromContext(ctx)
	if saver == nil {
		return runtimeState
	}
	cm := graph.NewCheckpointManager(saver)
	tuple, err := cm.Latest(ctx, input.ThreadID, "")
	if err != nil {
		logs.Warnf("auto-resume: failed to get latest checkpoint for thread=%s: %v, rid: %s", input.ThreadID, err, rid)
		return runtimeState
	}

	if tuple == nil || tuple.Checkpoint == nil || !tuple.Checkpoint.IsInterrupted() {
		return runtimeState
	}

	runtimeState[graph.CfgKeyCheckpointID] = tuple.Checkpoint.ID

	// 前端通过 forwardedProps 传入的结构化数据（如选中的 account_id）写入独立的 runtime-state key。
	// resume command 始终承载用户自由输入文本，是非格式化、无法预期的内容，二者必须区分开，
	// 避免结构化内容覆盖用户输入。节点可按需从 StateKeyForwardedResumeValue 单独消费结构化值。
	forwardedResumeVal, hasForwarded := forwardedProps[constant.ForwardedPropResumeValue]
	if hasForwarded && forwardedResumeVal != nil {
		runtimeState[constant.StateKeyForwardedResumeValue] = forwardedResumeVal
		logs.Infof("auto-resume: stored forwardedProps resume value into runtime state %v, rid: %s",
			forwardedResumeVal, rid)
	}

	// resume command 来自最新的用户消息文本；即便前端通过 forwardedProps 传结构化数据，
	// 仍需设置 resume command 以驱动 graph 从中断点继续。
	var userInput string
	if len(input.Messages) > 0 {
		lastMsg := input.Messages[len(input.Messages)-1]
		if lastMsg.Role == "user" {
			userInput, _ = lastMsg.Content.(string)
		}
	}
	if userInput != "" {
		// NOTE: mergeInitialStateNonInternal skips keys starting with "_",
		// so we must use StateKeyCommand (processed by processResumeCommand)
		// instead of writing ResumeChannel directly.
		runtimeState[graph.StateKeyCommand] = graph.NewResumeCommand().WithResume(userInput)
		logs.Infof("auto-resume: set resume command from user input: %q, rid: %s", userInput, rid)
	}

	return runtimeState
}

// AgentReadiness handles GET /api/v1/agent/readiness.
// Unlike /healthz (which checks etcd), this reports skill/prompt initial sync status.
// Envelope is built by rest.Handler (respEntity / respErrorWithEntity), same as other APIs.
func (s *Service) AgentReadiness(cts *rest.Contexts) (interface{}, error) {
	rd := s.runTime.Readiness()
	data := readiness.AgentReadinessResp{
		SkillReady:  rd.SkillReady(),
		PromptReady: rd.PromptReady(),
		Ready:       rd.IsReady(),
	}
	if !data.Ready {
		return data, errf.New(errf.UnHealthy, "agent not ready: skill or prompt initial sync has not completed")
	}
	return data, nil
}

// readinessMiddleware wraps an http.Handler and returns 503 with a clear error
// message until readiness.IsReady() becomes true. The /healthz endpoint is
// intentionally NOT wrapped by this middleware (it lives on a separate mux path).
func readinessMiddleware(rd *logics.Readiness, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// agent还未就绪，则返回 UnHealthy
		if rd != nil && !rd.IsReady() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			rest.WriteResp(w, rest.NewBaseResp(errf.UnHealthy,
				"agent is not ready: initial sync of skill or prompt has not completed yet"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
