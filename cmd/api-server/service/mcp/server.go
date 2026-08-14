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

// Package mcp 实现 api-server 的 MCP 协议编解码层，
// 在 trpc-mcp-go 之上提供工厂、HTTP handler 包装与中间件接入点。
//
// 本包不引入 A2A、agent-server 依赖，仅完成 JSON-RPC / streamable HTTP 的传输层处理。
// 具体业务工具的注册在 ingress / internal 子包中完成。
package mcp

import (
	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

// BuildBaseServer 构造一个用于 api-server 的 MCP server 基础实例。
//
// 该工厂统一以下默认行为，使得对外部提供服务的 ingress 与对内部提供服务的 internal 两条链路共享相同基线：
//
//   - WithServerPath("")：禁用 trpc-mcp-go 内置的 path 严格校验。
//     api-server 把 server.HTTPHandler() 挂载到 net/http.ServeMux 时由上层 mux
//     决定路径前缀（如 `/api/v1/mcp/servers/foo/mcp/` 或 `/api/v1/mcp/internal/hcm/mcp/`），
//     SDK 内部不应再做二次校验，否则会与子路径 / 路径变量冲突。
//   - WithStatelessMode(true)：每个 HTTP 请求生成临时 session，
//     不依赖客户端 Mcp-Session-Id header，与对外部提供服务的 OpenClaw / 第三方 client 的实际行为相符。
//   - WithServerLogger(newLoggerAdapter(component))：把 SDK 内部日志接入 hcm/pkg/logs。
//
// 调用方可通过 extraOpts 追加更多 ServerOption，
// 比如 ingress 会追加 WithHTTPContextFunc(identity.IdentityHTTPContextFunc)、
// internal 会追加 WithHTTPContextFunc(... 解析内部 header 的 fn ...) 等。
//
// 入参：
//   - name / version：MCP `initialize` 协商返回的 serverInfo。
//   - component：日志前缀，建议取值 "mcp/ingress"、"mcp/internal"。
//   - extraOpts：附加的 trpc-mcp-go ServerOption。
func BuildBaseServer(name, version, component string, extraOpts ...mcpsdk.ServerOption) *mcpsdk.Server {
	baseOpts := []mcpsdk.ServerOption{
		// 禁用 path 严格校验，详见上文说明。
		mcpsdk.WithServerPath(""),
		// 对外部提供服务的 / 对内部提供服务的都走 stateless。这里关闭的是 **MCP 传输层** 的 session
		// （`Mcp-Session-Id` header + SDK 内部 sessionManager），原因：
		//   - 上游 client（OpenClaw / agent-server LLM）实际不维持 Mcp-Session-Id；
		//   - 避免 SDK 按 session 缓存 SSE 通道导致内存泄漏；
		//   - 每次 tools/call 都是一次完整的 A2A streaming 调用，SSE 生命周期与
		//     单次 tools/call 对齐，不需要跨请求长连接。
		//
		// 注意：这并 **不** 影响连续对话能力。OpenClaw ↔ agent-server 的多轮上下文
		// 由 A2A 协议层的 `contextId`（业务对话 ID）维护，由 send_message 工具
		// inputSchema 直接透传给 A2AClient.StreamMessage，与 MCP 传输层 session 解耦。
		mcpsdk.WithStatelessMode(true),
		// 协议日志接入 hcm 全局日志。
		mcpsdk.WithServerLogger(newLoggerAdapter(component)),
	}
	baseOpts = append(baseOpts, extraOpts...)

	return mcpsdk.NewServer(name, version, baseOpts...)
}
