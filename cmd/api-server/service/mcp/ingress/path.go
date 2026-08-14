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

// Package ingress 实现对外部提供服务的 MCP ingress：
// 把 OpenClaw 通过蓝鲸网关发来的 MCP streamable HTTP 请求转换为对 agent-server
// 的 A2A 调用，并把 A2A 流式事件回译为 MCP notifications/progress + CallToolResult。
//
// 本包**不**直接依赖 trpc-a2a-go：业务桥接通过 BridgeHandler 接口注入，便于：
//   - 任务 4 实现协议层（工具 / 通知占位、路径挂载），用 NoopBridgeHandler 跑通；
//   - 任务 5 在 cmd/api-server/service/mcp/bridge/ 中提供真正基于 A2AClient 的实现。
package ingress

import (
	"net/http"
	"strings"

	"hcm/cmd/api-server/service/mcp/middleware"
)

// pathParamName 是对外部提供服务的 ingress 路径中的 mcp_server_name 占位符变量名，
// 用于 net/http.ServeMux（Go 1.22+）路径模式 `{name}` 提取。
const pathParamName = "mcp_server_name"

// pathPattern 根据 basePath 生成 net/http.ServeMux 路径模式。
//
// 输入示例：basePath = "/api/v1/mcp/servers"
// 输出：    "/api/v1/mcp/servers/{mcp_server_name}/mcp/"
//
// 路径末尾的 `/` 是 streamable HTTP 入口约定（OpenClaw / 蓝鲸网关都按 trailing slash
// 配置），不允许去掉。
func pathPattern(basePath string) string {
	base := strings.TrimRight(basePath, "/")
	return base + "/{" + pathParamName + "}/mcp/"
}

// pathMiddleware 把 net/http ServeMux 提取出的 {mcp_server_name} 写入 ctx，
// 让下游 tool handler 与日志能拿到原始路径变量。
//
// trpc-mcp-go SDK 内部不识别该变量，因此通过 ctx 旁路传递。
func pathMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue(pathParamName)
		if name != "" {
			ctx := middleware.WithMCPServerName(r.Context(), name)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}
