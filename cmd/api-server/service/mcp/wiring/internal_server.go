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

// Package wiring exposes MCP subpackage wiring helpers for api-server startup.
package wiring

import (
	"context"
	"net/http"

	internalmcp "hcm/cmd/api-server/service/mcp/internal"
	"hcm/pkg/cc"
)

// InternalBackendRequest captures an internal MCP backend request.
type InternalBackendRequest = internalmcp.BackendRequest

// InternalBackendResponse captures an internal MCP backend response.
type InternalBackendResponse = internalmcp.BackendResponse

// InternalDispatcher dispatches internal MCP tools/call requests.
type InternalDispatcher = internalmcp.Dispatcher

// RegisterInternalServers mounts all configured southbound internal HCM MCP servers.
func RegisterInternalServers(ctx context.Context, mux *http.ServeMux, cfg cc.MCPInternalSetting) (
	[]*InternalDispatcher, error) {

	return internalmcp.RegisterAll(ctx, mux, cfg)
}
