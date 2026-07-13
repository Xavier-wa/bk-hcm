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

package enumor

import (
	"testing"
)

// TestIsAIBillItem ...
func TestIsAIBillItem(t *testing.T) {
	// 定义测试用例表
	tests := []struct {
		name     string // 测试用例名称
		input    string // 输入字符串
		expected bool   // 预期结果
	}{
		{"Contains Gemini AI", "This is about Gemini AI", true},
		{"Contains claude", "claude is from anthropic", true},
		{"Invalid aagemini", "aagemini is not valid", false},
		{"Invalid geminiaa", "geminiaa is not valid", false},
		{"Valid with underscore prefix", "aa_gemini is valid", true},
		{"Valid with underscore suffix", "gemini_aa is valid", true},
		{"Invalid combined word", "GeminiClaude is not valid", false},
		{"Uppercase GEMINI", "GEMINI is valid", true},
		{"Uppercase CLAUDE", "CLAUDE is valid", true},
		{"No target words", "No target words here", false},
		{"Multiple target words", "Check gemini and claude", true},
		{"With hyphen", "gemini-claude", true},
		{"With slash", "gemini/claude", true},
		{"With number suffix", "gemini6", true},
		{"With number prefix", "6gemini", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行被测函数
			got := IsAIBillItem(tt.input)
			// 验证结果
			if got != tt.expected {
				t.Errorf("ContainsTargetWords(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestMatchAPIBrandName ...
func TestMatchAPIBrandName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"单一品牌 claude", "this is claude api", "claude"},
		{"单一品牌 kimi", "kimi model usage", "kimi"},
		{"单一品牌 jina", "jina embedding", "jina"},
		{"单一品牌 gemini", "gemini pro", "gemini"},
		{"大小写混合 Gemini", "Gemini Pro", "gemini"},
		{"大写 VEO 归并 gemini", "VEO video", "gemini"},
		{"veo 归并 gemini", "use veo now", "gemini"},
		{"imagen 归并 gemini", "imagen image", "gemini"},
		{"lyria 归并 gemini", "lyria music", "gemini"},
		{"多命中取首个 gemini", "gemini and claude", "gemini"},
		{"多命中取首个 claude", "claude before gemini", "claude"},
		{"未命中返回空", "no ai keyword here", ""},
		{"子串不误判 claudexx", "claudexx is not valid", ""},
		{"子串不误判 aagemini", "aagemini invalid", ""},
		{"带前缀下划线命中", "_HCM_AI_Claude", "claude"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchAPIBrandName(tt.input)
			if got != tt.expected {
				t.Errorf("MatchAPIBrandName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestMatchGcpGpuCardByKeyword 验证 GCP L1 显式卡型关键词识别。
func TestMatchGcpGpuCardByKeyword(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"H200 命中", "H200 141GB GPU attached to DWS", "H200"},
		{"H100 命中", "A3 High Instance with H100", "H100"},
		{"A100 命中", "Nvidia Tesla A100 GPU running in Americas", "A100"},
		{"L4 命中", "Nvidia L4 GPU running in Frankfurt", "L4"},
		{"L4 大小写混合", "nvidia l4 gpu", "L4"},
		{"RTX Pro 6000 命中 RTX6000PRO", "NVIDIA RTX Pro 6000 GPU", "RTX6000PRO"},
		{"RTX 6000 96GB 命中 RTX6000PRO", "RTX 6000 96GB running in Delhi", "RTX6000PRO"},
		{"TPU7x 命中", "TPU7x running in Americas", "TPU7x"},
		{"V100 命中", "Nvidia Tesla V100 GPU", "V100"},
		{"P100 命中", "Nvidia Tesla P100 GPU", "P100"},
		{"P4 命中", "Nvidia Tesla P4 GPU", "P4"},
		{"K80 命中", "Nvidia Tesla K80 GPU", "K80"},
		{"T4 本期不识别", "Nvidia T4 GPU running in Frankfurt", ""},
		{"A1000 不误命中 A100", "GPU model A1000 series", ""},
		{"P40 不误命中 P4", "Nvidia Tesla P40 GPU", ""},
		{"随机串不误命中 L4", "model xl4y mix", ""},
		{"非 GPU 描述留空", "N1 Predefined Instance Core running in Americas", ""},
		{"先 H200 后命中", "H200 and A100 mixed", "H200"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchGcpGpuCardByKeyword(tt.input)
			if got != tt.expected {
				t.Errorf("MatchGcpGpuCardByKeyword(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
