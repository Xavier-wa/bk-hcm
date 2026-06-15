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

// Package a2a 提供 agent-server 的 A2A 协议服务端实现。
//
// 本包将 A2A JSON-RPC 入口与 AgentCard 元数据端点挂载到外层 ServeMux，
// 内部复用 trpc-agent-go/server/a2a 封装的 messageProcessor，并继续使用与
// AG-UI 链路同一份 runner.Runner。
//
// A2A 协议的 contextId 在 trpc-agent-go/server/a2a 内部会直接作为 runner.Run
// 的 sessionID（即 threadId）传入，因此本包无需再做 contextId → threadId 的额外转换。
package a2a

import (
	"fmt"
	"net/http"
	"strings"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"

	a2aserver "trpc.group/trpc-go/trpc-a2a-go/server"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	agoa2a "trpc.group/trpc-go/trpc-agent-go/server/a2a"
)

// Server 持有 A2A handler 及其路由元数据。
type Server struct {
	// handler 是 trpc-a2a-go server 暴露的 net/http.Handler，
	// 内部按 jsonRPCEndpoint / agentCardPath / oldAgentCardPath 路由请求。
	handler http.Handler

	// jsonRPCPath 是对外 JSON-RPC 路径（例如 "/api/v1/agent/a2a"）。
	jsonRPCPath string
	// agentCardPath 是 A2A v0.2.2 AgentCard 发现路径。
	agentCardPath string
	// agentLegacyCardPath 是 A2A 0.1.x 兼容路径（agent.json）。
	agentLegacyCardPath string
}

// New 构建一个 A2A Server，使用传入的 runner.Runner（与 AG-UI 链路共享同一实例）。
//
// 调用方负责：
//   - 在挂载到 ServeMux 前先做 Enable 判断（cfg.Enable=false 时直接跳过本函数）；
//   - 在 mux 上分别注册 server.JSONRPCPath() / AgentCardPath() / AgentLegacyCardPath()。
func New(cfg cc.A2ASetting, r runner.Runner, streaming bool) (*Server, error) {
	if r == nil {
		return nil, fmt.Errorf("a2a.New: runner is nil")
	}

	basePath := normalizeBasePath(cfg.BasePath)
	jsonRPCPath := basePath + constant.A2AJSONRPCSubPath

	card := buildAgentCard(cfg, streaming)
	logs.Infof("a2a: building server, basePath=%s, jsonRPCPath=%s, "+
		"cardName=%q, cardVersion=%q, skills=%d",
		basePath, jsonRPCPath, card.Name, card.Version, len(card.Skills))

	a2aSrv, err := agoa2a.New(
		agoa2a.WithRunner(r),
		agoa2a.WithAgentCard(card),
		// 显式声明 X-Bkapi-User-Name 作为 A2A user ID 来源；
		// 上游 mcpCallerOriginMiddleware 已将 bk_username 注入 header，
		// trpc-a2a-go 的默认 authProvider 会读取并填进 ctx 中的 auth.User.ID，
		// 再被 messageProcessor 当作 runner.Run 的 userID。
		agoa2a.WithUserIDHeader(constant.UserKey),
		// 优先级：WithBasePath 先把 jsonRPCEndpoint 设为 basePath+"/"，
		// 再用 WithJSONRPCEndpoint 单独覆盖成 basePath+"/a2a"，
		// 保留 AgentCard 路径仍位于 basePath/.well-known/* 下。
		agoa2a.WithExtraA2AOptions(
			a2aserver.WithBasePath(basePath),
			a2aserver.WithJSONRPCEndpoint(jsonRPCPath),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("a2a.New: build a2a server failed: %w", err)
	}

	return &Server{
		handler:             a2aSrv.Handler(),
		jsonRPCPath:         jsonRPCPath,
		agentCardPath:       basePath + constant.A2AWellKnownAgentCardPath,
		agentLegacyCardPath: basePath + constant.A2AWellKnownAgentLegacyPath,
	}, nil
}

// Handler 返回 A2A 内部 ServeMux。它根据请求路径分发到 JSON-RPC 或 AgentCard 处理函数。
func (s *Server) Handler() http.Handler { return s.handler }

// JSONRPCPath 返回 A2A JSON-RPC 入口路径，外层 ServeMux 必须在此路径上注册 Server.Handler()。
func (s *Server) JSONRPCPath() string { return s.jsonRPCPath }

// AgentCardPath 返回 A2A v0.2.2 AgentCard 发现路径。
func (s *Server) AgentCardPath() string { return s.agentCardPath }

// AgentLegacyCardPath 返回兼容 A2A 0.1.x 的 AgentCard 发现路径。
func (s *Server) AgentLegacyCardPath() string { return s.agentLegacyCardPath }

// RegisterHandlers 将 A2A 三个对外路径挂载到外层 mux。
// 所有路径都共用同一个 inner ServeMux handler；调用方负责在挂载前组合中间件。
func (s *Server) RegisterHandlers(mux *http.ServeMux, h http.Handler) {
	mux.Handle(s.jsonRPCPath, h)
	mux.Handle(s.agentCardPath, h)
	mux.Handle(s.agentLegacyCardPath, h)
}

// normalizeBasePath 把 basePath 规范化为以 "/" 开头、不以 "/" 结尾的形式。
// 空字符串或 "/" 退化为默认值。
func normalizeBasePath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" || basePath == "/" {
		return constant.A2ABasePathDefault
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	return strings.TrimSuffix(basePath, "/")
}
