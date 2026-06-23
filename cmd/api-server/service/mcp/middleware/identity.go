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

// Package middleware 提供 api-server MCP / A2A 路径共用的中间件与上下文工具。
//
// 包内函数同时面向两种调用风格：
//   - 纯函数（ParseFromHeader / RequireCallerSource）—— 给 trpc-mcp-go.WithHTTPContextFunc 用，
//     由 MCP server 在解析完 HTTP 头后调用，把结果写入 ctx；
//   - http.Handler 包装器（IdentityMiddleware / CallerSourceMiddleware）—— 给 A2A passthrough
//     等基于 net/http 的链路用。
//
// 与现有 proxy 链路完全隔离：本包**不**调用 cmd/api-server/service/filter.go 中的
// peekRequest，避免破坏 streamable HTTP / SSE 流式语义。
package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/gwparser"
)

// ctxKey 是 ctx 内部 key 的私有类型，防止 ctx 键冲突。
type ctxKey int

const (
	ctxKeyKit ctxKey = iota
	ctxKeyMCPServerName
)

// ParseFromHeader 复用 gwparser.Parse 解析请求 header，得到 *kit.Kit。
//
// 复用项目既有 jwt 解析逻辑，确保身份解析行为与 api-server proxy 链路完全一致。
// 当 disableJWT=true（开发环境）时退化为读取 X-Bkapi-User-Name 等 header 直填。
func ParseFromHeader(ctx context.Context, h http.Header) (*kit.Kit, error) {
	return gwparser.Parse(ctx, h)
}

// WithKit 把 *kit.Kit 写入 ctx，下游 tool handler 可通过 KitFromCtx 取回。
func WithKit(ctx context.Context, kt *kit.Kit) context.Context {
	if kt == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyKit, kt)
}

// KitFromCtx 从 ctx 中取出 *kit.Kit。
// 当 ctx 中不存在 kit 时返回 (nil, false)，调用方需自行处理。
func KitFromCtx(ctx context.Context) (*kit.Kit, bool) {
	kt, ok := ctx.Value(ctxKeyKit).(*kit.Kit)
	if !ok || kt == nil {
		return nil, false
	}
	return kt, true
}

// WithMCPServerName 把 MCP 对外部提供服务的 ingress 路径中的 {mcp_server_name} 写入 ctx，
// 供日志、metrics 标签使用。
func WithMCPServerName(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyMCPServerName, name)
}

// MCPServerNameFromCtx 从 ctx 中取出 mcp_server_name，未设置时返回空字符串。
func MCPServerNameFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyMCPServerName).(string); ok {
		return v
	}
	return ""
}

// IdentityHTTPContextFunc 适配 trpc-mcp-go.WithHTTPContextFunc 签名：
// 在 HTTP request 进入 MCP server 时解析身份并写入 ctx。
//
// 解析失败时仍返回原 ctx（不写 kit），由下游 tool handler 在取 kit 时按需返回错误。
// 这避免了在 ctx func 中无法返回 HTTP 错误的语义难题；MCP 入口的鉴权门面
// 由 IdentityMiddleware 在 HTTP 层提前拦截。
func IdentityHTTPContextFunc(ctx context.Context, r *http.Request) context.Context {
	kt, err := ParseFromHeader(ctx, r.Header)
	if err != nil {
		rid := r.Header.Get(constant.RidKey)
		logs.Warnf("parse identity from header failed in mcp context func, "+
			"downstream tool will see no kit, err: %v, rid: %s", err, rid)
		return ctx
	}
	ctx = WithKit(ctx, kt)
	return ctx
}

// IdentityMiddleware 返回一个 http.Handler 包装器，在进入下游 handler 前解析
// 请求 header 中的身份信息并写入 ctx；解析失败时直接以 HTTP 403 拒绝请求。
//
// 不会消费 / 缓冲 request body，保留 streamable HTTP 流式语义。
//
// 使用场景：A2A passthrough 反向代理入口。
func IdentityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kt, err := ParseFromHeader(r.Context(), r.Header)
		if err != nil {
			rid := r.Header.Get(constant.RidKey)
			logs.Warnf("identity middleware: parse header failed, rid: %s, err: %v", rid, err)
			writeForbidden(w, "identity parse failed")
			return
		}

		ctx := WithKit(r.Context(), kt)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// writeForbidden 以 errf 风格写出 HTTP 403 响应体。
func writeForbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	body, err := json.Marshal(errf.New(errf.PermissionDenied, msg))
	if err != nil {
		logs.Errorf("writeForbidden: marshal error response failed, err: %v", err)
		body = []byte(`{"code":403,"message":"permission denied"}`)
	}
	_, _ = w.Write(body)
}
