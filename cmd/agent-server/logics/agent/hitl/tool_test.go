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
	"encoding/json"
	"testing"

	"hcm/pkg/criteria/constant"
)

// TestHumanConfirmTool_Declaration 测试 HumanConfirmTool 返回的声明是否正确。
func TestHumanConfirmTool_Declaration(t *testing.T) {
	decl := HumanConfirmTool()

	// 验证工具名称
	if decl.Name != constant.HumanConfirmToolName {
		t.Errorf("HumanConfirmTool().Name = %q, want %q", decl.Name, constant.HumanConfirmToolName)
	}

	// 验证工具描述非空
	if decl.Description == "" {
		t.Error("HumanConfirmTool().Description should not be empty")
	}

	// 验证 InputSchema 不为 nil
	if decl.InputSchema == nil {
		t.Fatal("HumanConfirmTool().InputSchema should not be nil")
	}

	// 验证 InputSchema 类型
	if decl.InputSchema.Type != "object" {
		t.Errorf("HumanConfirmTool().InputSchema.Type = %q, want %q", decl.InputSchema.Type, "object")
	}

	// 验证 question 属性存在且为 string 类型
	questionProp, ok := decl.InputSchema.Properties["question"]
	if !ok {
		t.Fatal("HumanConfirmTool().InputSchema.Properties should contain 'question'")
	}
	if questionProp.Type != "string" {
		t.Errorf("HumanConfirmTool().InputSchema.Properties['question'].Type = %q, want %q", questionProp.Type, "string")
	}

	// 验证 options 属性存在且为 array 类型
	optionsProp, ok := decl.InputSchema.Properties["options"]
	if !ok {
		t.Fatal("HumanConfirmTool().InputSchema.Properties should contain 'options'")
	}
	if optionsProp.Type != "array" {
		t.Errorf("HumanConfirmTool().InputSchema.Properties['options'].Type = %q, want %q", optionsProp.Type, "array")
	}

	// 验证 question 为必填项
	found := false
	for _, req := range decl.InputSchema.Required {
		if req == "question" {
			found = true
			break
		}
	}
	if !found {
		t.Error("HumanConfirmTool().InputSchema.Required should contain 'question'")
	}

	// 验证 options 不是必填项（optional）
	for _, req := range decl.InputSchema.Required {
		if req == "options" {
			t.Error("HumanConfirmTool().InputSchema.Required should NOT contain 'options' (it's optional)")
		}
	}
}

// TestHumanConfirmArgs_JSONSerialization 测试 HumanConfirmArgs 的 JSON 序列化/反序列化。
func TestHumanConfirmArgs_JSONSerialization(t *testing.T) {
	tests := []struct {
		name     string
		input    HumanConfirmArgs
		wantJSON string
	}{
		{
			name: "仅有 question",
			input: HumanConfirmArgs{
				Question: "是否继续操作？",
			},
			wantJSON: `{"question":"是否继续操作？"}`,
		},
		{
			name: "question 和 options",
			input: HumanConfirmArgs{
				Question: "请选择方案",
				Options:  []string{"方案A", "方案B"},
			},
			wantJSON: `{"question":"请选择方案","options":["方案A","方案B"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(data) != tt.wantJSON {
				t.Errorf("json.Marshal() = %s, want %s", string(data), tt.wantJSON)
			}
		})
	}
}

// TestHumanConfirmArgs_JSONDeserialization 测试 HumanConfirmArgs 从 JSON 反序列化。
func TestHumanConfirmArgs_JSONDeserialization(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		want    HumanConfirmArgs
		wantErr bool
	}{
		{
			name:    "仅有 question",
			jsonStr: `{"question":"即将删除3台主机，是否继续？"}`,
			want: HumanConfirmArgs{
				Question: "即将删除3台主机，是否继续？",
				Options:  nil,
			},
		},
		{
			name:    "question 和 options",
			jsonStr: `{"question":"请选择配置方案","options":["高性能型","经济型"]}`,
			want: HumanConfirmArgs{
				Question: "请选择配置方案",
				Options:  []string{"高性能型", "经济型"},
			},
		},
		{
			name:    "空的 options 数组",
			jsonStr: `{"question":"确认操作","options":[]}`,
			want: HumanConfirmArgs{
				Question: "确认操作",
				Options:  []string{},
			},
		},
		{
			name:    "无效 JSON",
			jsonStr: `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got HumanConfirmArgs
			err := json.Unmarshal([]byte(tt.jsonStr), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Question != tt.want.Question {
				t.Errorf("HumanConfirmArgs.Question = %q, want %q", got.Question, tt.want.Question)
			}
			if len(got.Options) != len(tt.want.Options) {
				t.Errorf("len(HumanConfirmArgs.Options) = %d, want %d", len(got.Options), len(tt.want.Options))
				return
			}
			for i, opt := range got.Options {
				if opt != tt.want.Options[i] {
					t.Errorf("HumanConfirmArgs.Options[%d] = %q, want %q", i, opt, tt.want.Options[i])
				}
			}
		})
	}
}
