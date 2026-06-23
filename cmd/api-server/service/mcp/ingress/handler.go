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

// SendMessageRequest 是 BridgeHandler.SendMessage 的入参，
// 由 ingress 层负责从 MCP CallToolRequest 中解析得到，业务层无需关心 MCP 协议细节。
//
// 字段对齐 design.md 中 send_message 工具 inputSchema。
type SendMessageRequest struct {
	// Text 是用户问题或操作意图，必填。
	Text string
	// ContextID 是 A2A 业务对话上下文 ID。
	// 空字符串表示首轮对话，由 BridgeHandler 实现负责生成（如 ctx-<uuid>）。
	ContextID string
	// BkBizID 是业务 ID，可选；0 表示未提供。
	BkBizID int64
	// ModelName 是模型名称，可选。
	ModelName string
	// ProgressToken 是 MCP 客户端在 _meta.progressToken 字段中携带的进度令牌，
	// BridgeHandler 用它把 A2A 流式事件转换为 notifications/progress，
	// 并在 notifications/cancelled 时反向查找对应的 A2A taskId。
	// nil 表示客户端不订阅进度通知（如 OpenClaw 一次性同步调用）。
	ProgressToken interface{}
	// MCPServerName 是请求路径上的 mcp_server_name 路径变量，
	// 用于多路径统一日志 / metrics 标签。
	MCPServerName string
}

// BridgeHandler 抽象 MCP→A2A 桥接业务层，使得 ingress 协议层与具体业务实现解耦。
//
// 任务 4 提供 NoopBridgeHandler 作为占位（返回固定 stub 响应），保证协议层可单独
// 编译 / 测试 / 部署；任务 5 在 bridge 包中给出真正基于 trpc-a2a-go A2AClient
// 的实现，并在启动期通过 Register 注入。
type BridgeHandler interface {
	// SendMessage 处理一次 send_message 工具调用。
	//
	// 参数：
	//   - ctx 已经包含从 HTTP header 解析的 *kit.Kit、mcp_server_name 等上下文；
	//   - req 是 ingress 协议层完成参数校验后的请求对象；
	//   - srv 是当前 MCP server，用于业务层通过 srv.SendNotification 推送
	//     notifications/progress。
	//
	// 返回 *CallToolResult：
	//   - 业务成功：IsError=false + Content（文本回答），可在 Meta 中附 contextId / taskId；
	//   - 业务失败：IsError=true + 错误描述（用 mcpsdk.NewErrorResult 构造）。
	//
	// 注意：协议层错误（缺 text、JSON 解析失败）由 ingress 层在调用本方法**前**用
	// JSON-RPC -32602 InvalidParams 返回，不会进入到本方法。
	SendMessage(ctx context.Context, req *SendMessageRequest,
		srv *mcpsdk.Server) (*mcpsdk.CallToolResult, error)

	// Cancel 在收到 MCP notifications/cancelled 时调用。
	// progressToken 即客户端 notifications/cancelled 通知中的 requestId 字段。
	//
	// 该方法 SHALL NOT 返回错误：取消是 best-effort 操作，
	// 找不到对应的 in-flight task 时应记 warn 日志后静默返回。
	Cancel(ctx context.Context, progressToken interface{})
}

// NoopBridgeHandler 是 BridgeHandler 的"什么都不做"占位实现，用于：
//   - 配置 mcp.ingress.enable=true 但 bridge 包尚未注入 handler 时的生产兜底，
//     避免空指针 panic（见 BuildIngressServer）；
//   - 协议层单元测试中作为轻量 stub。
//
// 接收到 SendMessage 时直接返回 IsError=true 的 stub 响应；
// 接收到 Cancel 时记 warn 日志。
type NoopBridgeHandler struct{}

// SendMessage 实现 BridgeHandler。返回 IsError=true 的 stub CallToolResult，
// 让上游客户端能识别到桥接未就绪而不是协议错误。
func (NoopBridgeHandler) SendMessage(ctx context.Context, req *SendMessageRequest,
	_ *mcpsdk.Server) (*mcpsdk.CallToolResult, error) {

	rid := ""
	if kt, ok := middleware.KitFromCtx(ctx); ok {
		rid = kt.Rid
	}
	logs.Warnf("ingress: NoopBridgeHandler.SendMessage called, text_len=%d, "+
		"mcp_server_name=%s, rid: %s", len(req.Text), req.MCPServerName, rid)
	return mcpsdk.NewErrorResult("ingress bridge handler not wired yet; " +
		"please ensure cmd/api-server/service/mcp/bridge has been registered"), nil
}

// Cancel 实现 BridgeHandler，仅记 warn 日志。
func (NoopBridgeHandler) Cancel(ctx context.Context, progressToken interface{}) {
	rid := ""
	if kt, ok := middleware.KitFromCtx(ctx); ok {
		rid = kt.Rid
	}
	logs.Warnf("ingress: NoopBridgeHandler.Cancel called, progressToken=%v, rid: %s", progressToken, rid)
}
