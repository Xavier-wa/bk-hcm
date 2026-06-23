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

package mcp

import (
	"net/http"
	"runtime/debug"
	"time"

	"hcm/cmd/api-server/service/mcp/middleware"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/tools/uuid"

	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

// Middleware 是 net/http 风格的中间件类型：接收下游 handler，返回包装后的 handler。
//
// 例如 middleware.IdentityMiddleware 与 middleware.CallerSourceMiddleware 都符合该签名。
type Middleware func(http.Handler) http.Handler

// HandlerOption 配置 NewMCPHTTPHandler 的可选行为。
type HandlerOption func(*handlerOptions)

// handlerOptions 是 HandlerOption 内部累积状态。
type handlerOptions struct {
	middlewares   []Middleware
	disablePanic  bool
	disableReqLog bool
	logComponent  string
}

// WithMiddleware 在 server.HTTPHandler() 与 panic recovery / request log 之间
// 追加一层 net/http 中间件。多次调用按调用顺序栈式叠加：
// 最后一个 WithMiddleware 注册的位于最内层（紧邻 SDK handler），
// 最先注册的位于最外层（紧邻请求日志），形成标准洋葱模型。
func WithMiddleware(mw Middleware) HandlerOption {
	return func(o *handlerOptions) {
		if mw == nil {
			return
		}
		o.middlewares = append(o.middlewares, mw)
	}
}

// WithLogComponent 设置请求日志前缀（默认 "mcp"），便于在大量日志中筛选不同 MCP 链路。
// 建议取值如 "mcp/ingress"、"mcp/internal"、"mcp/a2a-passthrough"。
func WithLogComponent(name string) HandlerOption {
	return func(o *handlerOptions) { o.logComponent = name }
}

// NewMCPHTTPHandler 在 *mcpsdk.Server 之上叠加 api-server 标准中间件链，
// 返回可直接挂载到 net/http.ServeMux 的 http.Handler。
//
// 中间件分层（从外到内，request 路径方向）：
//
//	[panic recovery]
//	  └─ [request log]
//	       └─ [WithMiddleware 注入的链]
//	            └─ server.HTTPHandler()  ← trpc-mcp-go SDK
//
// 设计要点：
//
//   - SDK 本身已经接管 streamable HTTP（POST + GET + SSE flush）的语义；
//     本函数**不**消费 request body、**不**包装 ResponseWriter（除请求日志为
//     抓 status code 而做的最小包装），保留 SSE Flush 正确性。
//   - identity / caller-source 这类业务鉴权 SHALL 由调用方通过 WithMiddleware 注入，
//     使得同一 NewMCPHTTPHandler 既能服务外部（identity）也能服务内部（caller-source）。
func NewMCPHTTPHandler(server *mcpsdk.Server, opts ...HandlerOption) http.Handler {
	o := &handlerOptions{
		logComponent: "mcp",
	}
	for _, opt := range opts {
		opt(o)
	}

	var h http.Handler = server.HTTPHandler()

	// 从最后一个 WithMiddleware 开始包裹，使得"最先注册的中间件"位于最外层。
	for i := len(o.middlewares) - 1; i >= 0; i-- {
		h = o.middlewares[i](h)
	}

	if !o.disableReqLog {
		h = requestLogMiddleware(o.logComponent)(h)
	}
	if !o.disablePanic {
		h = panicRecoveryMiddleware(o.logComponent)(h)
	}

	return h
}

// requestLogMiddleware 打印每个 MCP 请求的开始 / 结束 / 状态码 / 耗时。
//
// 关键约束：
//   - **不**缓冲 response body（SSE 必须真正实时下发）。
//   - 通过 statusCapturingWriter 仅捕获 status code，并暴露 http.Flusher 接口。
//   - 不存在 rid 的请求（未经 identity 中间件）则补一个临时 rid 用于关联首尾日志。
func requestLogMiddleware(component string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(constant.RidKey)
			if rid == "" {
				rid = uuid.UUID()
			}

			start := time.Now()
			logs.Infof("[%s] request begin: method=%s, path=%s, remote=%s, rid: %s",
				component, r.Method, r.URL.Path, r.RemoteAddr, rid)

			sw := &statusCapturingWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			logs.Infof("[%s] request end: method=%s, path=%s, status=%d, "+
				"elapsed_ms=%d, rid: %s",
				component, r.Method, r.URL.Path, sw.status,
				time.Since(start).Milliseconds(), rid)
		})
	}
}

// panicRecoveryMiddleware 兜底捕获下游 handler 的 panic，记录 error 日志并返回 500。
//
// 关键约束：仅在 WriteHeader 尚未发生时返回 500；
// 若 panic 发生在 SSE 流中（response 已经开始写），只记日志不再写 body，避免破坏 SSE 帧。
func panicRecoveryMiddleware(component string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				rid := r.Header.Get(constant.RidKey)
				if kt, ok := middleware.KitFromCtx(r.Context()); ok && kt.Rid != "" {
					rid = kt.Rid
				}
				logs.Errorf("[%s] panic recovered: %v, path: %s\nstack:\n%s, rid: %s",
					component, rec, r.URL.Path, rid, debug.Stack(), rid)

				// statusCapturingWriter 在 WriteHeader 调用后 wroteHeader=true，
				// 此时不应该再写 status，避免 "http: superfluous response.WriteHeader"。
				if sw, ok := w.(*statusCapturingWriter); ok && sw.wroteHeader {
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// statusCapturingWriter 是仅捕获 status code 的 http.ResponseWriter 包装；
// 它实现了 http.Flusher 以保留 trpc-mcp-go SSE 写入路径的 Flush 行为。
type statusCapturingWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// WriteHeader 记录第一次 status，并下发给底层 ResponseWriter。
// 二次调用直接忽略（防止 SSE 路径下重复 WriteHeader 触发警告）。
func (w *statusCapturingWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

// Write 在尚未调用 WriteHeader 时隐式触发 200 标记，与 net/http 默认行为一致。
func (w *statusCapturingWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}

// Flush 透传 http.Flusher，保留 SSE 帧实时下发能力。
// 当底层 ResponseWriter 未实现 Flusher 时为 no-op。
func (w *statusCapturingWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// 编译期断言：statusCapturingWriter 必须实现 http.Flusher。
var _ http.Flusher = (*statusCapturingWriter)(nil)

// 编译期断言：中间件包导出的 IdentityMiddleware 必须符合 Middleware 类型，
// 防止上游不小心改签名导致 NewMCPHTTPHandler 编译失败。
var _ Middleware = middleware.IdentityMiddleware
