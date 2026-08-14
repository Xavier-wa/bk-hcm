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

// Package ingress 对外部提供服务的 MCP ingress
package ingress

import (
	"errors"
	"net/http"

	"hcm/cmd/api-server/service/mcp"
	"hcm/cmd/api-server/service/mcp/middleware"
	"hcm/pkg/cc"
	"hcm/pkg/logs"
)

// Register 把对外部提供服务的 MCP ingress handler 挂载到指定 ServeMux 上。
//
// 路径模式：`<cfg.BasePath>/{mcp_server_name}/mcp/`
// 示例：`/api/v1/mcp/servers/{mcp_server_name}/mcp/`
//
// 所有 mcp_server_name 共享同一个 *mcpsdk.Server 实例，因此 tools/list 在
// 任意 mcp_server_name 下都返回完全相同的工具集合（仅 send_message）。
//
// 中间件链（外→内）：
//
//	mcp.NewMCPHTTPHandler 自带的 panic recovery + request log
//	  └─ identity.IdentityMiddleware（对外部提供服务的必备：解析蓝鲸网关 JWT，失败 403）
//	       └─ pathMiddleware（提取 {mcp_server_name} 写入 ctx）
//	            └─ trpc-mcp-go server.HTTPHandler()
//
// 当 cfg.Enable=false 时本函数静默返回 nil，不挂载任何路径，
// 与既有 proxy 链路零干扰。
func Register(mux *http.ServeMux, cfg cc.MCPIngressSetting, bridge BridgeHandler) error {
	if mux == nil {
		return errors.New("ingress.Register: mux is nil")
	}
	if !cfg.Enable {
		logs.Infof("ingress: mcp.ingress.enable=false, skip mounting MCP ingress paths")
		return nil
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	srv := BuildIngressServer(cfg, bridge)

	httpHandler := mcp.NewMCPHTTPHandler(srv,
		mcp.WithLogComponent(loggerComponent),
		mcp.WithMiddleware(middleware.IdentityMiddleware),
		mcp.WithMiddleware(pathMiddleware),
	)

	pattern := pathPattern(cfg.BasePath)
	mux.Handle(pattern, httpHandler)

	logs.Infof("ingress: mounted MCP ingress at %s, server=%s/%s, tool=%s",
		pattern, cfg.ServerName, cfg.ServerVersion, cfg.AggregatedToolName)
	return nil
}
