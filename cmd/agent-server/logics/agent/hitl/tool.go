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
	"hcm/pkg/criteria/constant"

	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// HumanConfirmArgs defines the parameters for the human_confirm tool.
// This tool is called by LLM when it needs user confirmation or choice.
type HumanConfirmArgs struct {
	// Question is the question to display to the user.
	Question string `json:"question"`
	// Options is the list of options for the user to choose from (optional).
	Options []string `json:"options,omitempty"`
}

// HumanConfirmTool returns the tool declaration for human_confirm.
// This is a pure declaration tool with no actual execution logic.
// The HITL node handles the interrupt logic.
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
