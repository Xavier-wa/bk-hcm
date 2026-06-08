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

package tcloud

import (
	"testing"

	"hcm/pkg/criteria/constant"
)

func TestChangeArchitecture(t *testing.T) {
	tests := []struct {
		name         string
		architecture *string
		want         string
		description  string // 添加描述字段，说明测试目的
	}{
		// ========== 空值处理 ==========
		{
			name:         "nil architecture returns x86",
			architecture: nil,
			want:         constant.X86,
			description:  "当架构信息缺失时，默认返回 x86 架构",
		},
		{
			name:         "empty string returns as is",
			architecture: stringPtr(""),
			want:         "",
			description:  "空字符串应原样返回，与 nil 行为不同",
		},

		// ========== ARM 架构转换 ==========
		{
			name:         "arm architecture returns arm64",
			architecture: stringPtr("arm"),
			want:         constant.Arm64,
			description:  "腾讯云返回的 'arm' 需要转换为标准的 'arm64'",
		},
		{
			name:         "ARM uppercase returns as is (case sensitive)",
			architecture: stringPtr("ARM"),
			want:         "ARM",
			description:  "大写 'ARM' 不会被转换，验证大小写敏感性",
		},
		{
			name:         "arm64 returns as is",
			architecture: stringPtr("arm64"),
			want:         "arm64",
			description:  "已经是 arm64 的不需要转换",
		},
		{
			name:         "aarch64 returns as is",
			architecture: stringPtr("aarch64"),
			want:         "aarch64",
			description:  "aarch64 是 arm64 的别名，保持原样",
		},

		// ========== x86 架构相关 ==========
		{
			name:         "x86_64 architecture returns as is",
			architecture: stringPtr("x86_64"),
			want:         "x86_64",
			description:  "标准 x86_64 架构保持原样",
		},
		{
			name:         "i386 architecture returns as is",
			architecture: stringPtr("i386"),
			want:         "i386",
			description:  "32位 x86 架构保持原样",
		},
		{
			name:         "i686 architecture returns as is",
			architecture: stringPtr("i686"),
			want:         "i686",
			description:  "i686 架构保持原样",
		},
		{
			name:         "x86 returns as is",
			architecture: stringPtr("x86"),
			want:         "x86",
			description:  "x86 架构保持原样",
		},

		// ========== 边界条件和异常值 ==========
		{
			name:         "whitespace only returns as is",
			architecture: stringPtr("   "),
			want:         "   ",
			description:  "纯空格字符串原样返回，不做 trim 处理",
		},
		{
			name:         "unknown architecture returns as is",
			architecture: stringPtr("unknown_arch"),
			want:         "unknown_arch",
			description:  "未知架构类型原样返回，便于排查问题",
		},
		{
			name:         "mips architecture returns as is",
			architecture: stringPtr("mips"),
			want:         "mips",
			description:  "其他架构类型（如 mips）原样返回",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := changeArchitecture(tt.architecture)
			if got != tt.want {
				t.Errorf("changeArchitecture() = %q, want %q, description: %s",
					got, tt.want, tt.description)
			}
		})
	}
}

// TestChangeArchitectureConstants 验证使用的常量值
// 确保常量定义变化时测试能够捕获
func TestChangeArchitectureConstants(t *testing.T) {
	// 验证常量值符合预期
	// 如果 constant.X86 或 constant.Arm64 的值发生变化，测试会失败
	if constant.X86 != "x86_64" {
		t.Errorf("constant.X86 expected 'x86_64', got %q", constant.X86)
	}
	if constant.Arm64 != "arm64" {
		t.Errorf("constant.Arm64 expected 'arm64', got %q", constant.Arm64)
	}
}

// TestChangeArchitectureNilVsEmpty 明确 nil 和空字符串的行为差异
// 这是一个重要的边界条件，需要明确记录
func TestChangeArchitectureNilVsEmpty(t *testing.T) {
	nilResult := changeArchitecture(nil)
	emptyResult := changeArchitecture(stringPtr(""))

	// nil 返回默认值 x86
	if nilResult != constant.X86 {
		t.Errorf("nil input should return %q, got %q", constant.X86, nilResult)
	}

	// 空字符串返回空字符串（不是默认值）
	if emptyResult != "" {
		t.Errorf("empty string input should return empty string, got %q", emptyResult)
	}

	// 明确两者行为不同
	if nilResult == emptyResult {
		t.Error("nil and empty string should have different behavior")
	}
}

// TestChangeArchitectureCaseSensitivity 验证大小写敏感性
// 确保只有小写 "arm" 会被转换
func TestChangeArchitectureCaseSensitivity(t *testing.T) {
	armVariants := []struct {
		input    string
		expected string
	}{
		{"arm", constant.Arm64}, // 只有这个会被转换
		{"ARM", "ARM"},          // 大写不转换
		{"Arm", "Arm"},          // 首字母大写不转换
		{"aRm", "aRm"},          // 混合大小写不转换
		{"arm64", "arm64"},      // arm64 不需要转换
		{"ARM64", "ARM64"},      // 大写 ARM64 不转换
	}

	for _, tc := range armVariants {
		result := changeArchitecture(stringPtr(tc.input))
		if result != tc.expected {
			t.Errorf("changeArchitecture(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

// stringPtr is a helper function to create a pointer to a string
func stringPtr(s string) *string {
	return &s
}
