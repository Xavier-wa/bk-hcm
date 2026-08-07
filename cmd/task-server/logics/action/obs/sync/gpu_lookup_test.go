/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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

package sync

import (
	"testing"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/dal/table/types"
)

// TestParseAwsGpuInstanceTypes 验证「实例类型 → 卡型」JSON 对象配置解析。
func TestParseAwsGpuInstanceTypes(t *testing.T) {
	tests := []struct {
		name        string
		configValue types.JsonField
		wantLen     int
		wantErr     bool
	}{
		{
			name:        "合法对象格式",
			configValue: `{"g5.12xlarge":"A10G","g6.xlarge":"L4"}`,
			wantLen:     2,
			wantErr:     false,
		},
		{
			name:        "空对象",
			configValue: `{}`,
			wantLen:     0,
			wantErr:     false,
		},
		{
			name:        "旧数组格式解析失败",
			configValue: `["g5.12xlarge","g6.xlarge"]`,
			wantErr:     true,
		},
		{
			name:        "非法 JSON 解析失败",
			configValue: `not a json`,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAwsGpuInstanceTypes(tt.configValue)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseAwsGpuInstanceTypes() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("parseAwsGpuInstanceTypes() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

// TestIsAwsGPU 验证 AWS GPU 判定逻辑（对象映射改造后语义不变）。
func TestIsAwsGPU(t *testing.T) {
	awsGpuMap := map[string]string{
		"g5.12xlarge": "A10G",
		"g6.xlarge":   "L4",
	}

	tests := []struct {
		name                string
		lineItemProductCode string
		productInstanceType string
		hcProductName       string
		expected            bool
	}{
		{"SageMaker 判为 GPU", constant.AmazonSageMaker, "", "", true},
		{"实例类型命中映射", "AmazonEC2", "g5.12xlarge", "", true},
		{"AI 前缀判为 GPU", "AmazonEC2", "t3.micro", constant.BillItemAIPrefix + "claude", true},
		{"非 GPU", "AmazonEC2", "t3.micro", "normal product", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAwsGPU(tt.lineItemProductCode, tt.productInstanceType, tt.hcProductName, awsGpuMap)
			if got != tt.expected {
				t.Errorf("isAwsGPU() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestLookupAwsGpuCardCategory 验证 AWS 卡型查询：普通命中/未命中，以及 SageMaker 带后缀去除后匹配。
func TestLookupAwsGpuCardCategory(t *testing.T) {
	awsGpuMap := map[string]string{
		"g5.12xlarge":      "A10G",
		"g6.xlarge":        "L4",
		"ml.p5en.48xlarge": "H200",
		"ml.g5.12xlarge":   "A10G",
		"p6-b200.48xlarge": "B200",
	}

	tests := []struct {
		name                string
		productProductName  string
		productInstanceType string
		expected            string
	}{
		{"普通命中 A10G", "Amazon Elastic Compute Cloud", "g5.12xlarge", "A10G"},
		{"普通未命中留空", "Amazon Elastic Compute Cloud", "t3.micro", ""},
		{"非 SageMaker 不去后缀（命中失败留空）", "Amazon Elastic Compute Cloud", "ml.p5en.48xlarge-cluster", ""},
		{"SageMaker 去 trainingplanunused 后缀", constant.AmazonSageMakerProductName, "ml.p5en.48xlarge-trainingplanunused", "H200"},
		{"SageMaker 去 trainingplaninuse 后缀", constant.AmazonSageMakerProductName, "ml.p5en.48xlarge-trainingplaninuse", "H200"},
		{"SageMaker 去 cluster 后缀", constant.AmazonSageMakerProductName, "ml.p5en.48xlarge-cluster", "H200"},
		{"SageMaker 去 Notebook 后缀但非 GPU 留空", constant.AmazonSageMakerProductName, "ml.t3.medium-Notebook", ""},
		{"SageMaker 命中 A10G", constant.AmazonSageMakerProductName, "ml.g5.12xlarge-Cluster", "A10G"},
		{"SageMaker 保留机型内部连字符", constant.AmazonSageMakerProductName, "p6-b200.48xlarge-cluster", "B200"},
		{"SageMaker 无后缀按原值匹配", constant.AmazonSageMakerProductName, "ml.p5en.48xlarge", "H200"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lookupAwsGpuCardCategory(tt.productProductName, tt.productInstanceType, awsGpuMap)
			if got != tt.expected {
				t.Errorf("lookupAwsGpuCardCategory() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestResolveGcpAPIBrandName 验证 GCP 品牌识别：优先 HcProductName，兜底 SkuDescription；AIDeduct 走 sku。
func TestResolveGcpAPIBrandName(t *testing.T) {
	tests := []struct {
		name           string
		hcProductName  string
		skuDescription string
		expected       string
	}{
		{"HcProductName 命中优先", constant.BillItemAIPrefix + "claude", "gemini sku", "claude"},
		{"HcProductName 未命中兜底 SkuDescription", "credit item", "gemini api credit", "gemini"},
		{"两者均未命中返回空", "normal product", "normal sku", ""},
		{"兜底命中 veo 归并 gemini", "credit item", "veo usage", "gemini"},
		{"AIDeduct 从 SkuDescription 识别品牌", constant.GcpAIDeductProductCode, "claude api usage", "claude"},
		{"AIDeduct 无品牌不强制 API", constant.GcpAIDeductProductCode, "normal sku", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveGcpAPIBrandName(tt.hcProductName, tt.skuDescription)
			if got != tt.expected {
				t.Errorf("resolveGcpAPIBrandName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestIsAIDeductBillItem 验证 AI 扣减产品标识识别。
func TestIsAIDeductBillItem(t *testing.T) {
	tests := []struct {
		name          string
		hcProductCode string
		hcProductName string
		expected      bool
	}{
		{"code 为 AIDeduct", constant.AwsAIDeductProductCode, "other", true},
		{"name 为 AIDeduct", "other", constant.AwsAIDeductProductCode, true},
		{"非扣减", "AmazonEC2", "AmazonEC2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAIDeductBillItem(tt.hcProductCode, tt.hcProductName); got != tt.expected {
				t.Errorf("isAIDeductBillItem() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestHcProductNameForGPUClass 验证 AI 扣减恢复 AI 前缀占位。
func TestHcProductNameForGPUClass(t *testing.T) {
	if got := hcProductNameForGPUClass(constant.AwsAIDeductProductCode, constant.AwsAIDeductProductCode); got != constant.BillItemAIPrefix {
		t.Errorf("hcProductNameForGPUClass(AIDeduct) = %q, want %q", got, constant.BillItemAIPrefix)
	}
	if got := hcProductNameForGPUClass("AmazonEC2", constant.BillItemAIPrefix+"claude"); got != constant.BillItemAIPrefix+"claude" {
		t.Errorf("hcProductNameForGPUClass(normal) = %q, want original", got)
	}
}

// TestResolveAwsAPIBrandName 验证 AWS 品牌识别：AIDeduct 从 extension 还原，不强制 API。
func TestResolveAwsAPIBrandName(t *testing.T) {
	tests := []struct {
		name                string
		hcProductCode       string
		hcProductName       string
		productProductName  string
		lineItemDescription string
		expected            string
	}{
		{
			name:               "原始账单仍用 HcProductName",
			hcProductCode:      "AmazonBedrock",
			hcProductName:      constant.BillItemAIPrefix + "claude",
			productProductName: "other",
			expected:           "claude",
		},
		{
			name:               "AIDeduct 从 ProductProductName 识别 claude",
			hcProductCode:      constant.AwsAIDeductProductCode,
			hcProductName:      constant.AwsAIDeductProductCode,
			productProductName: "Claude API on Marketplace",
			expected:           "claude",
		},
		{
			name:                "AIDeduct ProductName 未命中时用行描述",
			hcProductCode:       constant.AwsAIDeductProductCode,
			hcProductName:       constant.AwsAIDeductProductCode,
			productProductName:  "Amazon Elastic Compute Cloud",
			lineItemDescription: "kimi api tokens",
			expected:            "kimi",
		},
		{
			name:               "AIDeduct 无品牌不强制 API",
			hcProductCode:      constant.AwsAIDeductProductCode,
			hcProductName:      constant.AwsAIDeductProductCode,
			productProductName: "Amazon Elastic Compute Cloud",
			expected:           "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveAwsAPIBrandName(tt.hcProductCode, tt.hcProductName, tt.productProductName, tt.lineItemDescription)
			if got != tt.expected {
				t.Errorf("resolveAwsAPIBrandName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestAIDeductGPUClassification 验证 AIDeduct GPU 场景：无品牌时 isGPU=true，卡型可填充。
func TestAIDeductGPUClassification(t *testing.T) {
	awsGpuMap := map[string]string{"g5.12xlarge": "A10G"}

	hcName := hcProductNameForGPUClass(constant.AwsAIDeductProductCode, constant.AwsAIDeductProductCode)
	isGPU := isAwsGPU("AmazonEC2", "g5.12xlarge", hcName, awsGpuMap)
	if !isGPU {
		t.Fatalf("AIDeduct with GPU instance should be isGPU=true")
	}
	brand := resolveAwsAPIBrandName(constant.AwsAIDeductProductCode, constant.AwsAIDeductProductCode,
		"Amazon Elastic Compute Cloud", "")
	if brand != "" {
		t.Fatalf("AIDeduct GPU without brand keywords must not force API brand, got %q", brand)
	}
	card := lookupAwsGpuCardCategory("Amazon Elastic Compute Cloud", "g5.12xlarge", awsGpuMap)
	if card != "A10G" {
		t.Fatalf("GpuCardCategory = %q, want A10G", card)
	}

	// GCP：AIDeduct + 卡型 SKU，无品牌 → isGPU，不强制 API
	sku := "Nvidia L4 GPU running in Frankfurt"
	gcpCard := lookupGcpGpuCardCategory(sku, gcpGpuPrefixesForTest)
	gcpHcName := hcProductNameForGPUClass(constant.GcpAIDeductProductCode, constant.GcpAIDeductProductCode)
	if !isGcpGPU(gcpCard, sku, gcpHcName) {
		t.Fatalf("GCP AIDeduct with L4 sku should be isGPU=true")
	}
	if got := resolveGcpAPIBrandName(constant.GcpAIDeductProductCode, sku); got != "" {
		t.Fatalf("GCP AIDeduct L4 sku must not force API brand, got %q", got)
	}
}

// gcpGpuPrefixesForTest GCP L2 实例族前缀映射测试数据。
var gcpGpuPrefixesForTest = map[string]string{
	"A3Ultra":  "H200",
	"A3 Ultra": "H200",
	"A3":       "H100",
	"A2":       "A100",
	"G2":       "L4",
	"G4":       "RTX6000PRO",
}

// TestLookupGcpGpuCardCategory 验证 GCP 卡型识别：L1 显式关键词优先、L2 实例族前缀、A3 歧义最长前缀优先、配置缺失等。
func TestLookupGcpGpuCardCategory(t *testing.T) {
	tests := []struct {
		name           string
		skuDescription string
		prefixes       map[string]string
		expected       string
	}{
		// L1 显式关键词（最高优先级）
		{"L1 命中 L4", "Nvidia L4 GPU running in Frankfurt", gcpGpuPrefixesForTest, "L4"},
		{"L1 命中 H200", "H200 141GB GPU attached to DWS", gcpGpuPrefixesForTest, "H200"},
		{"L1 命中 RTX6000PRO", "NVIDIA RTX Pro 6000 GPU", gcpGpuPrefixesForTest, "RTX6000PRO"},
		// L2 实例族前缀
		{"L2 命中 A2 → A100", "A2 Instance Core running in Americas", gcpGpuPrefixesForTest, "A100"},
		{"L2 命中 G2 → L4", "G2 Instance Ram running in Frankfurt", gcpGpuPrefixesForTest, "L4"},
		{"L2 命中 G4 → RTX6000PRO", "G4 Instance Core running in Delhi", gcpGpuPrefixesForTest, "RTX6000PRO"},
		// A3 / A3Ultra 歧义：最长前缀优先
		{"L2 A3Ultra 连写命中 H200", "DWS Defined Duration A3Ultra Core", gcpGpuPrefixesForTest, "H200"},
		{"L2 A3 Ultra 带空格取最长 H200", "A3 Ultra Instance Ram running in Americas", gcpGpuPrefixesForTest, "H200"},
		{"L2 A3 非 Ultra 命中 H100", "A3 High Instance Core running in Americas", gcpGpuPrefixesForTest, "H100"},
		// L1 优先于 L2
		{"L1 优先：A3Ultra Core 含 H200 关键词", "A3Ultra H200 141GB GPU", gcpGpuPrefixesForTest, "H200"},
		// 本期不识别 T4
		{"T4 不识别留空", "Nvidia T4 GPU running in Frankfurt", gcpGpuPrefixesForTest, ""},
		// 配置缺失仅 L1 生效
		{"L2 配置缺失仅 L1：纯实例族前缀留空", "A2 Instance Core running in Americas", map[string]string{}, ""},
		{"L2 配置缺失仍命中 L1", "Nvidia L4 GPU", map[string]string{}, "L4"},
		{"L2 配置为 nil 仅 L1：纯实例族前缀留空", "G4 Instance Core", nil, ""},
		// 均未命中
		{"均未命中留空", "N1 Predefined Instance Core running in Americas", gcpGpuPrefixesForTest, ""},
		{"前缀词边界防误判（XG4Y 不命中 G4）", "Storage xg4y component", gcpGpuPrefixesForTest, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lookupGcpGpuCardCategory(tt.skuDescription, tt.prefixes)
			if got != tt.expected {
				t.Errorf("lookupGcpGpuCardCategory(%q) = %q, want %q", tt.skuDescription, got, tt.expected)
			}
		})
	}
}

// TestIsGcpGPU 验证改造后的 GCP GPU 判定：卡型命中、AI 前缀、AI 兜底为 true；仅含 calendar mode 为 false。
func TestIsGcpGPU(t *testing.T) {
	tests := []struct {
		name           string
		skuDescription string
		hcProductName  string
		expected       bool
	}{
		{"卡型 L1 命中判为 GPU", "Nvidia L4 GPU running in Frankfurt", "normal product", true},
		{"卡型 L2 命中判为 GPU", "A3Ultra Core running in Americas", "normal product", true},
		{"AI 前缀判为 GPU", "normal sku", constant.BillItemAIPrefix + "claude", true},
		{"AI 兜底（SkuDescription 命中）判为 GPU", "gemini api credit", "credit item", true},
		{"仅含 calendar mode 不再判为 GPU", "Some calendar mode reserved capacity", "normal product", false},
		{"非 GPU 非 AI 留 false", "N1 Predefined Instance Core", "normal product", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gpuCardCategory := lookupGcpGpuCardCategory(tt.skuDescription, gcpGpuPrefixesForTest)
			got := isGcpGPU(gpuCardCategory, tt.skuDescription, tt.hcProductName)
			if got != tt.expected {
				t.Errorf("isGcpGPU(%q, %q) = %v, want %v", tt.skuDescription, tt.hcProductName, got, tt.expected)
			}
		})
	}
}
