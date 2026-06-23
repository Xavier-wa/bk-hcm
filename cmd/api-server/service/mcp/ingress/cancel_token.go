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

	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

const (
	// jsonRPCMetaKey 是 JSON-RPC params 中 _meta 字段的键名。
	jsonRPCMetaKey = "_meta"
	// progressTokenKey 是 _meta 中 progressToken 字段的键名。
	progressTokenKey = "progressToken"
)

// progressTokenCtxKey 是 ctx 内部 key 的私有类型，防止跨包 key 冲突。
type progressTokenCtxKey struct{}

// withProgressToken 把 MCP `_meta.progressToken` 写入 ctx。
func withProgressToken(ctx context.Context, token interface{}) context.Context {
	if token == nil {
		return ctx
	}
	return context.WithValue(ctx, progressTokenCtxKey{}, token)
}

// progressTokenFromCtx 从 ctx 中取出 progressToken。
// 第二个返回值表示是否存在（区分 "没设" 和 "设了 nil"）。
func progressTokenFromCtx(ctx context.Context) (interface{}, bool) {
	v := ctx.Value(progressTokenCtxKey{})
	if v == nil {
		return nil, false
	}
	return v, true
}

// progressTokenMiddleware 是一个 JSON-RPC 层中间件，专门为 `tools/call` 方法
// 提取请求 `params._meta.progressToken` 并写入 ctx。
//
// 用途——支持取消（notifications/cancelled）：
//   - MCP 规范规定 notifications/cancelled.params.requestId == 原 tools/call 的 progressToken；
//   - 业务层 bridge 维护 progressToken → A2A taskId 的映射 (ProgressTaskMap)，
//     收到 cancel 通知时按 progressToken 反查 taskId 并调用 A2AClient.CancelTask。
//   - 因此 send_message handler 必须在请求进入时拿到 progressToken 写入 map。
//
// 关于 progress 通知（notifications/progress）：
//   - 不依赖该 token；业务直接调用 mcp.GetNotificationSender(ctx).SendProgress(...)
//     即可，由 SDK 在 SSE 模式下自动注入的 sseNotificationSender 负责推送。
//
// 背景——SDK bug：
//   - trpc-mcp-go ≤ v0.0.16 的 toolManager.handleCallTool 故意丢弃 progressToken
//     （manager_tools.go:252 `_ = progressToken`），导致 *CallToolRequest.Params.Meta
//     永远拿不到 progressToken。本中间件在更上层 (JSON-RPC) 兜底解析。
//   - 若未来 SDK 修复，移除本中间件并改用 req.Params.Meta 即可，handler 侧已做 fallback。
//
// 该中间件只关心 `tools/call`，其它 method 直接透传，零开销。
func progressTokenMiddleware(next mcpsdk.HandlerFunc) mcpsdk.HandlerFunc {
	return func(ctx context.Context, req *mcpsdk.JSONRPCRequest) (mcpsdk.JSONRPCMessage, error) {
		if req == nil || req.Method != mcpsdk.MethodToolsCall {
			return next(ctx, req)
		}

		paramsMap, ok := req.Params.(map[string]interface{})
		if !ok {
			return next(ctx, req)
		}
		metaMap, ok := paramsMap[jsonRPCMetaKey].(map[string]interface{})
		if !ok {
			return next(ctx, req)
		}
		if token, exists := metaMap[progressTokenKey]; exists && token != nil {
			ctx = withProgressToken(ctx, token)
		}
		return next(ctx, req)
	}
}
