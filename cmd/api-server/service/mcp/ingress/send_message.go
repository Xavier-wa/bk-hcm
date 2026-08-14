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
	"errors"
	"fmt"
	"strings"

	"hcm/cmd/api-server/service/mcp/middleware"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/tools/util"

	"github.com/getkin/kin-openapi/openapi3"
	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

// buildSendMessageTool 根据 ingress 配置构造对外暴露的唯一聚合工具描述。
//
// 工具入参 schema：
//
//	{
//	  "text":       {"type": "string",  "description": "用户问题或操作意图"},
//	  "contextId":  {"type": "string",  "description": "会话上下文 ID，连续对话固定复用"},
//	  "bk_biz_id":  {"type": "integer", "description": "业务 ID，可选"},
//	  "model_name": {"type": "string",  "description": "模型名称，可选"},
//	  "confirm":    {"type": "object",  "description": "HITL 结构化确认（对齐 AG-UI resumeValue）"}
//	}
//
// `required: ["text"]`，其余字段可选。
//
// title 通过 ToolAnnotations.Title 暴露给 OpenClaw（用户可见名称），默认 "HCM Agent"。
func buildSendMessageTool(cfg cc.MCPIngressSetting) *mcpsdk.Tool {
	return mcpsdk.NewTool(
		cfg.AggregatedToolName,
		mcpsdk.WithDescription(
			"Send a natural-language message to the HCM agent. "+
				"Use this tool for any HCM-related question or operation intent. "+
				"Reuse the returned contextId in subsequent calls to keep a continuous conversation. "+
				"When a previous response includes _meta.confirm, call again with the same contextId "+
				"and a confirm payload to approve or cancel the pending operation."),
		mcpsdk.WithToolAnnotations(&mcpsdk.ToolAnnotations{
			Title: "HCM Agent",
			// HCM agent 工具会调用云资源接口，可能存在副作用，
			// 不能声明为 ReadOnlyHint=true。
			ReadOnlyHint:  mcpsdk.BoolPtr(false),
			OpenWorldHint: mcpsdk.BoolPtr(true),
		}),
		mcpsdk.WithString("text",
			mcpsdk.Description("用户问题或操作意图，必填"),
			mcpsdk.Required(),
		),
		mcpsdk.WithString("contextId",
			mcpsdk.Description("A2A 会话上下文 ID，连续对话固定复用；首轮可省略，由 HCM 自动生成"),
		),
		mcpsdk.WithInteger("bk_biz_id",
			mcpsdk.Description("业务 ID（cmdb biz id），可选"),
		),
		mcpsdk.WithString("model_name",
			mcpsdk.Description("模型名称，可选；为空时由 HCM agent 选择默认模型"),
		),
		mcpsdk.WithObject("confirm",
			mcpsdk.Description(
				"HITL 结构化确认（与 Web AG-UI resumeValue 对齐）。"+
					"上一轮 _meta.confirm 存在时必填：action=confirm 并回传 args=_meta.confirm.data；"+
					"或 action=cancel 取消。"),
			mcpsdk.Properties(openapi3.Schemas{
				"action": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type:        &openapi3.Types{openapi3.TypeString},
					Description: "confirm 放行执行；cancel 取消待确认操作",
					Enum:        []any{constant.MCPConfirmActionConfirm, constant.MCPConfirmActionCancel},
				}),
				"args": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type:        &openapi3.Types{openapi3.TypeObject},
					Description: "确认后的工具入参，通常原样回传上一轮 _meta.confirm.data；可按需微调后回传",
				}),
			}),
		),
	)
}

// errParamMissingText 是缺少必填参数 text 时返回的协议级错误，
// JSON-RPC 错误码使用 -32602 InvalidParams。
//
// 该错误**不**作为 CallToolResult.IsError=true 返回，而是作为 handler 第二个返回值
// （error），由 trpc-mcp-go SDK 包装成 JSON-RPC error response 给客户端。
var errParamMissingText = errors.New("missing required parameter: text")

// newSendMessageHandler 构造 send_message 工具的 toolHandler 闭包。
//
// 职责分层：
//   - **协议层（本函数）**：参数校验（text 必填、text 长度上限）、字段类型断言、
//     从 ctx 取 mcp_server_name；
//   - **业务层（BridgeHandler.SendMessage）**：A2A 调用、contextId 生成、流事件转换。
//
// 这样 ingress 包既能在 NoopBridgeHandler 下做协议层冒烟测试，
// 又能在生产路径下委派给 bridge 包做真正业务处理。
func newSendMessageHandler(cfg cc.MCPIngressSetting, bridge BridgeHandler,
	srv *mcpsdk.Server) func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {

	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		// 协议层参数校验。
		text, _ := req.Params.Arguments["text"].(string)
		if text == "" {
			return nil, errParamMissingText
		}
		if cfg.TextMaxLength > 0 && len(text) > cfg.TextMaxLength {
			return nil, fmt.Errorf(
				"parameter text length %d exceeds limit %d",
				len(text), cfg.TextMaxLength)
		}

		confirm, err := parseConfirmArg(req.Params.Arguments["confirm"])
		if err != nil {
			return nil, err
		}

		smReq := &SendMessageRequest{
			Text:          text,
			ContextID:     stringArg(req.Params.Arguments, "contextId"),
			BkBizID:       int64Arg(req.Params.Arguments, "bk_biz_id"),
			ModelName:     stringArg(req.Params.Arguments, "model_name"),
			Confirm:       confirm,
			MCPServerName: middleware.MCPServerNameFromCtx(ctx),
		}

		// progressToken：仅用于支持 cancel 反查 (progressToken → A2A taskId)，
		// 不参与 notifications/progress 发送（progress 通知由 SDK 自动注入的
		// mcp.GetNotificationSender(ctx) 处理，详见 cancel_token.go 顶部注释）。
		//
		// 优先从 JSON-RPC 中间件写入的 ctx 取；兜底再尝试 req.Params.Meta
		// （未来 SDK 修复 manager_tools.go:252 时该字段自然可用，无需改动）。
		if pt, ok := progressTokenFromCtx(ctx); ok {
			smReq.ProgressToken = pt
		} else if req.Params.Meta != nil {
			smReq.ProgressToken = req.Params.Meta.ProgressToken
		}

		rid := ""
		if kt, ok := middleware.KitFromCtx(ctx); ok {
			rid = kt.Rid
		}
		logs.Infof("ingress: tools/call send_message: mcp_server_name: %s, text_len: %d, contextId: %q, "+
			"has_progress_token: %t, has_confirm: %t, rid: %s", smReq.MCPServerName, len(smReq.Text),
			smReq.ContextID, smReq.ProgressToken != nil, smReq.Confirm != nil, rid)

		return bridge.SendMessage(ctx, smReq, srv)
	}
}

// parseConfirmArg 解析 send_message.confirm 对象。缺失时返回 (nil, nil)。
func parseConfirmArg(raw interface{}) (*ConfirmRequest, error) {
	if raw == nil {
		return nil, nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("parameter confirm must be an object")
	}
	action := strings.TrimSpace(stringArg(m, "action"))
	if action == "" {
		return nil, fmt.Errorf("parameter confirm.action is required")
	}
	switch action {
	case constant.MCPConfirmActionConfirm, constant.MCPConfirmActionCancel:
	default:
		return nil, fmt.Errorf("parameter confirm.action must be %q or %q",
			constant.MCPConfirmActionConfirm, constant.MCPConfirmActionCancel)
	}

	confirm := &ConfirmRequest{Action: action}
	if argsRaw, exists := m["args"]; exists && argsRaw != nil {
		args, ok := argsRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("parameter confirm.args must be an object")
		}
		confirm.Args = args
	}
	return confirm, nil
}

// stringArg 从 arguments 中提取字符串字段，缺失或类型不匹配时返回空串。
func stringArg(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

// int64Arg 从 arguments 中提取整数字段。
//
// JSON 反序列化的数字默认为 float64，因此需要按 float64→int64 转换；
// 同时兼容直接为 int / int64 / json.Number 等情况。
// 缺失或不可识别时返回 0。
func int64Arg(args map[string]interface{}, key string) int64 {
	v, ok := args[key]
	if !ok {
		return 0
	}

	n, err := util.GetInt64ByInterface(v)
	if err != nil {
		return 0
	}
	return n
}
