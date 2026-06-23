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
	"context"

	"hcm/cmd/api-server/service/mcp/middleware"
	"hcm/pkg/logs"

	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

// notificationsCancelledMethod 是 MCP 协议规定的取消通知方法名。
// trpc-mcp-go v0.0.14 没有提供常量，按 schema 字面量使用。
const notificationsCancelledMethod = "notifications/cancelled"

// newCancelledNotificationHandler 构造 notifications/cancelled 处理器。
//
// MCP 协议约定：客户端可通过该通知请求服务端取消一个正在执行的 tools/call。
// 通知的 params 包含 requestId 字段，对应 client 在 tools/call 中通过
// `_meta.progressToken` 传入的同名值（MCP spec 写作 progressToken == requestId）。
//
// 实现：
//  1. 从 notification.Params.AdditionalFields 取出 requestId（trpc-mcp-go 的
//     NotificationParams 自定义 UnmarshalJSON 会把非 _meta 字段写到 AdditionalFields）；
//  2. 委派给 BridgeHandler.Cancel；
//  3. 不抛错——取消是 best-effort，找不到对应 task 时静默返回。
func newCancelledNotificationHandler(bridge BridgeHandler) mcpsdk.ServerNotificationHandler {
	return func(ctx context.Context, notification *mcpsdk.JSONRPCNotification) error {
		var progressToken interface{}
		if notification != nil {
			if v, ok := notification.Params.AdditionalFields["requestId"]; ok {
				progressToken = v
			}
		}

		rid := ""
		if kt, ok := middleware.KitFromCtx(ctx); ok {
			rid = kt.Rid
		}

		if progressToken == nil {
			logs.Warnf("ingress: notifications/cancelled missing requestId, ignored, rid: %s", rid)
			return nil
		}

		logs.Infof("ingress: notifications/cancelled received, requestId=%v, rid: %s", progressToken, rid)
		bridge.Cancel(ctx, progressToken)
		return nil
	}
}
