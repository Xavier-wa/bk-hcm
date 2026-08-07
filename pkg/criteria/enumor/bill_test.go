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

// TestBillAdjustmentResClassValidate 验证调账资源类别四值枚举。
func TestBillAdjustmentResClassValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   BillAdjustmentResClass
		wantErr string
	}{
		{"cpu 合法", BillAdjustmentResClassCPU, ""},
		{"gpu_card 合法", BillAdjustmentResClassGpuCard, ""},
		{"gpu_api 合法", BillAdjustmentResClassGpuAPI, ""},
		{"gpu_other 合法", BillAdjustmentResClassGpuOther, ""},
		{"已下线的 gpu 非法", "gpu", "unsupported bill adjustment res class: gpu"},
		{"空值非法", "", "unsupported bill adjustment res class: "},
		{"未知值非法", "tpu", "unsupported bill adjustment res class: tpu"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate(%q) = %v, want nil", tt.input, err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Errorf("Validate(%q) = %v, want %q", tt.input, err, tt.wantErr)
			}
		})
	}
}

// TestBillAdjustmentResClassFlags 验证由资源类别推出的 GPU / API / 子类必填三个标志。
func TestBillAdjustmentResClassFlags(t *testing.T) {
	tests := []struct {
		input           BillAdjustmentResClass
		isGPU           bool
		isAPI           bool
		needResSubClass bool
	}{
		{BillAdjustmentResClassCPU, false, false, false},
		{BillAdjustmentResClassGpuCard, true, false, true},
		{BillAdjustmentResClassGpuAPI, true, true, true},
		{BillAdjustmentResClassGpuOther, true, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.IsGPU(); got != tt.isGPU {
				t.Errorf("IsGPU(%q) = %v, want %v", tt.input, got, tt.isGPU)
			}
			if got := tt.input.IsAPI(); got != tt.isAPI {
				t.Errorf("IsAPI(%q) = %v, want %v", tt.input, got, tt.isAPI)
			}
			if got := tt.input.NeedResSubClass(); got != tt.needResSubClass {
				t.Errorf("NeedResSubClass(%q) = %v, want %v", tt.input, got, tt.needResSubClass)
			}
		})
	}
}

// TestListGcpGpuCardL1 验证一级卡型清单的内容与去重。
func TestListGcpGpuCardL1(t *testing.T) {
	cards := ListGcpGpuCardL1()

	if len(cards) != len(gcpGpuCardL1Keywords) {
		t.Errorf("ListGcpGpuCardL1() returns %d cards, want %d", len(cards), len(gcpGpuCardL1Keywords))
	}

	seen := make(map[string]struct{}, len(cards))
	for _, card := range cards {
		if _, ok := seen[card]; ok {
			t.Errorf("ListGcpGpuCardL1() returns duplicated card %q", card)
		}
		seen[card] = struct{}{}
	}

	// tpu7x 的短卡型名以代码为准取 TPU
	for _, want := range []string{"H200", "H100", "A100", "RTX6000PRO", "L4", "TPU", "V100", "P100", "P4", "K80"} {
		if _, ok := seen[want]; !ok {
			t.Errorf("ListGcpGpuCardL1() misses card %q", want)
		}
	}
}

// TestListBillAdjustmentAPIBrands 验证调账模型厂商清单固定为归并后的四值。
func TestListBillAdjustmentAPIBrands(t *testing.T) {
	brands := ListBillAdjustmentAPIBrands()

	want := []string{"claude", "gemini", "jina", "kimi"}
	if len(brands) != len(want) {
		t.Fatalf("ListBillAdjustmentAPIBrands() = %v, want %v", brands, want)
	}
	for i, brand := range brands {
		if brand != want[i] {
			t.Errorf("ListBillAdjustmentAPIBrands()[%d] = %q, want %q", i, brand, want[i])
		}
	}

	// veo/imagen/lyria 在上报侧已归并为 gemini，不得出现在调账下拉中
	for _, brand := range brands {
		switch BillItemAIFlag(brand) {
		case BillItemAIFlagVeo, BillItemAIFlagImagen, BillItemAIFlagLyria:
			t.Errorf("ListBillAdjustmentAPIBrands() must not contain merged brand %q", brand)
		}
	}
}

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
		{"TPU7x 命中", "TPU7x running in Americas", "TPU"},
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
