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

package ingress

import (
	"hcm/cmd/api-server/service/mcp"
	"hcm/cmd/api-server/service/mcp/middleware"
	"hcm/pkg/cc"

	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

// loggerComponent 是 ingress server / handler 的日志前缀。
const loggerComponent = "mcp/ingress"

// BuildIngressServer 构造对外部提供服务的 MCP ingress 的 *mcpsdk.Server 实例。
//
// 行为：
//   - 复用 mcp.BuildBaseServer 工厂的统一基线（stateless、空 ServerPath、hcm 日志）；
//   - 追加 identity HTTP context func，把蓝鲸网关 JWT 解析出的 *kit.Kit 写入 ctx；
//   - 注册唯一聚合工具 send_message（schema 见 buildSendMessageTool）；
//   - 注册 notifications/cancelled 处理器，把取消请求委托给 BridgeHandler.Cancel。
//
// 注意：所有 mcp_server_name 路径变量共享**同一个** Server 实例，
// 这保证了"任意路径下 tools/list 完全一致"的需求（任务 4.5）；
// mcp_server_name 通过 pathMiddleware 写入 ctx，供日志 / metrics 区分。
//
// 当 bridge 为 nil 时使用 NoopBridgeHandler 作为兜底，便于配置 ingress.enable=true
// 但 bridge 包尚未注入时的灰度部署 / 单元测试。
func BuildIngressServer(cfg cc.MCPIngressSetting, bridge BridgeHandler) *mcpsdk.Server {
	if bridge == nil {
		bridge = NoopBridgeHandler{}
	}

	srv := mcp.BuildBaseServer(
		cfg.ServerName,
		cfg.ServerVersion,
		loggerComponent,
		mcpsdk.WithHTTPContextFunc(middleware.IdentityHTTPContextFunc),
		// JSON-RPC 层中间件：兜底解析 `_meta.progressToken` 写入 ctx，
		// 见 progress_token.go 注释中关于 trpc-mcp-go v0.0.14 限制的说明。
		mcpsdk.WithMiddleware(progressTokenMiddleware),
	)

	tool := buildSendMessageTool(cfg)
	srv.RegisterTool(tool, newSendMessageHandler(cfg, bridge, srv))

	srv.RegisterNotificationHandler(notificationsCancelledMethod,
		newCancelledNotificationHandler(bridge))

	return srv
}
