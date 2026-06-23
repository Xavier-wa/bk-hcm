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

package hitl

import (
	"context"
	"encoding/json"
	"fmt"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// humanConfirmHandler 处理 LLM 主动发起的 human_confirm 工具：向用户提问（可携带选项），
// 并把答案回灌到对话中。
type humanConfirmHandler struct{}

// NewHumanConfirmHandler 创建 human_confirm handler。
func NewHumanConfirmHandler() Handler {
	return &humanConfirmHandler{}
}

// ToolName 返回 human_confirm 工具名。
func (h *humanConfirmHandler) ToolName() string {
	return constant.HumanConfirmToolName
}

// EventKind 将中断路由到 hitl.interrupt 前端事件。
func (h *humanConfirmHandler) EventKind() string {
	return constant.HITLInterruptKey
}

// BuildPayload 从工具调用入参构造 question/options payload。
func (h *humanConfirmHandler) BuildPayload(ctx context.Context, tc *model.ToolCall) (any, error) {
	rid := rest.RidFromContext(ctx)
	var args HumanConfirmArgs
	if err := json.Unmarshal(tc.Function.Arguments, &args); err != nil {
		logs.Errorf("human confirm handler: parse args failed, err: %v, rid: %s", err, rid)
		return nil, fmt.Errorf("parse human_confirm args: %w", err)
	}
	logs.Infof("human confirm handler: question=%s, options=%v, rid: %s", args.Question, args.Options, rid)
	return map[string]any{
		"question": args.Question,
		"options":  args.Options,
	}, nil
}

// OnResume 将用户的选择作为工具结果和一条 user 消息追加，然后在 llm 节点继续。
func (h *humanConfirmHandler) OnResume(
	ctx context.Context, tc *model.ToolCall, resumeValue any) (ResumeResult, error) {

	rid := rest.RidFromContext(ctx)
	userChoice, ok := resumeValue.(string)
	if !ok {
		logs.Errorf("human confirm handler: invalid resume value type, expected string, got %T, rid: %s",
			resumeValue, rid)
		return ResumeResult{}, fmt.Errorf("invalid resume value type, expected string, got %T", resumeValue)
	}
	logs.Infof("human confirm handler: userChoice=%s, rid: %s", userChoice, rid)

	return ResumeResult{
		Next: enumor.CvmApplyNodeLLM,
		AppendMessages: []model.Message{
			{Role: model.RoleTool, ToolID: tc.ID, Content: userChoice},
			{Role: model.RoleUser, Content: userChoice},
		},
	}, nil
}

// HumanConfirmArgs 定义 human_confirm 工具的参数。
// LLM 在需要用户确认或选择时调用该工具。
type HumanConfirmArgs struct {
	// Question 是要展示给用户的问题。
	Question string `json:"question"`
	// Options 是供用户选择的选项列表（可选）。
	Options []string `json:"options,omitempty"`
}

// HumanConfirmTool 返回 human_confirm 的工具声明。
// 这是一个没有实际执行逻辑的纯声明型工具，中断逻辑由 HITL 节点处理。
func HumanConfirmTool() *tool.Declaration {
	return &tool.Declaration{
		Name:        constant.HumanConfirmToolName,
		Description: "请求用户确认或选择，当需要用户在多个选项中做出选择时使用",
		InputSchema: &tool.Schema{
			Type: "object",
			Properties: map[string]*tool.Schema{
				"question": {
					Type:        "string",
					Description: "向用户展示的问题",
				},
				"options": {
					Type:        "array",
					Description: "选项列表（可选）",
					Items: &tool.Schema{
						Type: "string",
					},
				},
			},
			Required: []string{"question"},
		},
	}
}
