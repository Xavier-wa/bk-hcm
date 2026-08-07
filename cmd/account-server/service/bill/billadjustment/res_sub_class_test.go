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

package billadjustment

import (
	"reflect"
	"testing"

	"hcm/pkg/api/account-server/bill"
	billcore "hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	cvt "hcm/pkg/tools/converter"
)

// newTestResSubClassSource 构造配置存在状态下的清单来源。
func newTestResSubClassSource() *resSubClassSource {
	return &resSubClassSource{
		awsGpuInstanceTypes: map[string]string{
			"p5.48xlarge":  "H100",
			"p5e.48xlarge": "H100",
			"p4d.24xlarge": "A100",
			"g5.xlarge":    "",
		},
		gcpGpuInstancePrefixes: map[string]string{
			"a4x-": "GB200",
			"a3-":  "H100",
		},
	}
}

// TestListGpuCards 覆盖三个云厂商在配置存在、配置缺失两种状态下的卡型清单。
func TestListGpuCards(t *testing.T) {
	gcpL1 := enumor.ListGcpGpuCardL1()

	tests := []struct {
		name   string
		vendor enumor.Vendor
		source *resSubClassSource
		want   []string
	}{
		{
			name:   "aws 配置存在，取 value 集合并去重排序",
			vendor: enumor.Aws,
			source: newTestResSubClassSource(),
			want:   []string{"A100", "H100"},
		},
		{
			name:   "aws 配置缺失，返回空列表",
			vendor: enumor.Aws,
			source: &resSubClassSource{},
			want:   []string{},
		},
		{
			name:   "aws 配置解析失败降级为空映射，返回空列表",
			vendor: enumor.Aws,
			source: &resSubClassSource{awsGpuInstanceTypes: map[string]string{}},
			want:   []string{},
		},
		{
			name:   "gcp 配置缺失时仍返回一级卡型清单",
			vendor: enumor.Gcp,
			source: &resSubClassSource{},
			want:   sortedStrings(gcpL1),
		},
		{
			name:   "gcp 配置存在时一级清单并上配置 value",
			vendor: enumor.Gcp,
			source: newTestResSubClassSource(),
			want:   sortedStrings(append(append([]string{}, gcpL1...), "GB200")),
		},
		{
			name:   "华为云无卡型来源，配置存在也返回空",
			vendor: enumor.HuaWei,
			source: newTestResSubClassSource(),
			want:   []string{},
		},
		{
			name:   "source 为 nil 时不 panic",
			vendor: enumor.Aws,
			source: nil,
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := listGpuCards(tt.vendor, tt.source)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("listGpuCards(%s) = %v, want %v", tt.vendor, got, tt.want)
			}
		})
	}
}

// TestListGpuCardsAwsNotContainGcpOnlyCard AWS 清单不应含仅 GCP 侧存在的卡型。
func TestListGpuCardsAwsNotContainGcpOnlyCard(t *testing.T) {
	awsCards := listGpuCards(enumor.Aws, newTestResSubClassSource())
	if isResSubClassInOptions("TPU", awsCards) {
		t.Errorf("aws gpu card list should not contain gcp only card TPU, got %v", awsCards)
	}
}

// TestListAPIBrands 覆盖三个云厂商的模型厂商清单，华为云为空。
func TestListAPIBrands(t *testing.T) {
	want := []string{"claude", "gemini", "jina", "kimi"}
	for _, vendor := range []enumor.Vendor{enumor.Aws, enumor.Gcp} {
		if got := listAPIBrands(vendor); !reflect.DeepEqual(got, want) {
			t.Errorf("listAPIBrands(%s) = %v, want %v", vendor, got, want)
		}
	}
	if got := listAPIBrands(enumor.HuaWei); len(got) != 0 {
		t.Errorf("listAPIBrands(huawei) = %v, want empty", got)
	}
}

// TestParseGpuCardConfigValue 配置解析失败时降级为空映射且不返回错误。
func TestParseGpuCardConfigValue(t *testing.T) {
	kt := kit.New()

	got := parseGpuCardConfigValue(kt, "aws_gpu_instance_types", types.JsonField(`{"p5.48xlarge":"H100"}`))
	if !reflect.DeepEqual(got, map[string]string{"p5.48xlarge": "H100"}) {
		t.Errorf("parse valid config = %v", got)
	}

	broken := parseGpuCardConfigValue(kt, "aws_gpu_instance_types", types.JsonField(`["H100"]`))
	if len(broken) != 0 {
		t.Errorf("parse broken config = %v, want empty map", broken)
	}
}

// TestValidateResSubClass 覆盖必填、必须为空、取值域越界与大小写严格比对。
func TestValidateResSubClass(t *testing.T) {
	source := newTestResSubClassSource()

	tests := []struct {
		name        string
		vendor      enumor.Vendor
		resClass    enumor.BillAdjustmentResClass
		resSubClass string
		wantErr     bool
	}{
		{"cpu 为空通过", enumor.Aws, enumor.BillAdjustmentResClassCPU, "", false},
		{"gpu_other 为空通过", enumor.Aws, enumor.BillAdjustmentResClassGpuOther, "", false},
		{"cpu 带子类被拒", enumor.Aws, enumor.BillAdjustmentResClassCPU, "H100", true},
		{"gpu_other 带子类被拒", enumor.Gcp, enumor.BillAdjustmentResClassGpuOther, "gemini", true},
		{"gpu_card 为空被拒", enumor.Aws, enumor.BillAdjustmentResClassGpuCard, "", true},
		{"gpu_api 为空被拒", enumor.Gcp, enumor.BillAdjustmentResClassGpuAPI, "", true},
		{"aws gpu_card 命中配置通过", enumor.Aws, enumor.BillAdjustmentResClassGpuCard, "H100", false},
		{"aws gpu_card 大小写不一致被拒", enumor.Aws, enumor.BillAdjustmentResClassGpuCard, "h100", true},
		{"aws gpu_card 跨厂商越界被拒", enumor.Aws, enumor.BillAdjustmentResClassGpuCard, "GB200", true},
		{"gcp gpu_card 命中一级清单通过", enumor.Gcp, enumor.BillAdjustmentResClassGpuCard, "TPU", false},
		{"gcp gpu_card 命中配置 value 通过", enumor.Gcp, enumor.BillAdjustmentResClassGpuCard, "GB200", false},
		{"gpu_card 与 api 厂商错配被拒", enumor.Gcp, enumor.BillAdjustmentResClassGpuCard, "gemini", true},
		{"gpu_api 与卡型错配被拒", enumor.Aws, enumor.BillAdjustmentResClassGpuAPI, "H100", true},
		{"aws gpu_api 命中通过", enumor.Aws, enumor.BillAdjustmentResClassGpuAPI, "gemini", false},
		{"gpu_api 大小写不一致被拒", enumor.Gcp, enumor.BillAdjustmentResClassGpuAPI, "Gemini", true},
		{"华为云 gpu_card 一律被拒", enumor.HuaWei, enumor.BillAdjustmentResClassGpuCard, "H100", true},
		{"华为云 gpu_api 一律被拒", enumor.HuaWei, enumor.BillAdjustmentResClassGpuAPI, "gemini", true},
		{"华为云 cpu 通过", enumor.HuaWei, enumor.BillAdjustmentResClassCPU, "", false},
		{"华为云 gpu_other 通过", enumor.HuaWei, enumor.BillAdjustmentResClassGpuOther, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateResSubClass(tt.vendor, tt.resClass, tt.resSubClass, source)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateResSubClass() err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && errf.Error(err).Code != errf.InvalidParameter {
				t.Errorf("validateResSubClass() err code = %d, want %d", errf.Error(err).Code, errf.InvalidParameter)
			}
		})
	}
}

// TestMergeUpdateResSubClass 更新请求叠加到记录现值：未传沿用现值、显式置空生效。
func TestMergeUpdateResSubClass(t *testing.T) {
	record := &billcore.AdjustmentItem{
		Vendor:      enumor.Aws,
		ResClass:    enumor.BillAdjustmentResClassGpuCard,
		ResSubClass: "H100",
	}

	tests := []struct {
		name            string
		req             *bill.BillAdjustmentItemUpdateReq
		wantResClass    enumor.BillAdjustmentResClass
		wantResSubClass string
	}{
		{
			name:            "两者均未传，沿用记录现值",
			req:             &bill.BillAdjustmentItemUpdateReq{},
			wantResClass:    enumor.BillAdjustmentResClassGpuCard,
			wantResSubClass: "H100",
		},
		{
			name:            "只改子类",
			req:             &bill.BillAdjustmentItemUpdateReq{ResSubClass: cvt.ValToPtr("A100")},
			wantResClass:    enumor.BillAdjustmentResClassGpuCard,
			wantResSubClass: "A100",
		},
		{
			name:            "改类别但不显式置空，沿用现值供校验拦截",
			req:             &bill.BillAdjustmentItemUpdateReq{ResClass: enumor.BillAdjustmentResClassCPU},
			wantResClass:    enumor.BillAdjustmentResClassCPU,
			wantResSubClass: "H100",
		},
		{
			name: "改类别并显式置空",
			req: &bill.BillAdjustmentItemUpdateReq{
				ResClass:    enumor.BillAdjustmentResClassGpuOther,
				ResSubClass: cvt.ValToPtr(""),
			},
			wantResClass:    enumor.BillAdjustmentResClassGpuOther,
			wantResSubClass: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resClass, resSubClass := mergeUpdateResSubClass(record, tt.req)
			if resClass != tt.wantResClass || resSubClass != tt.wantResSubClass {
				t.Errorf("mergeUpdateResSubClass() = (%s, %s), want (%s, %s)",
					resClass, resSubClass, tt.wantResClass, tt.wantResSubClass)
			}
		})
	}
}

// TestValidateUpdateResSubClassCombination 更新路径按记录厂商校验，未显式置空的降级请求被拒。
func TestValidateUpdateResSubClassCombination(t *testing.T) {
	source := newTestResSubClassSource()
	record := &billcore.AdjustmentItem{
		Vendor:      enumor.Aws,
		ResClass:    enumor.BillAdjustmentResClassGpuCard,
		ResSubClass: "H100",
	}

	// 改为 cpu 但未显式置空，合并后子类仍为 H100，应被拒
	resClass, resSubClass := mergeUpdateResSubClass(record,
		&bill.BillAdjustmentItemUpdateReq{ResClass: enumor.BillAdjustmentResClassCPU})
	if err := validateResSubClass(record.Vendor, resClass, resSubClass, source); err == nil {
		t.Error("change res_class to cpu without clearing res_sub_class should be rejected")
	}

	// 改为 cpu 并显式置空，应通过
	resClass, resSubClass = mergeUpdateResSubClass(record, &bill.BillAdjustmentItemUpdateReq{
		ResClass:    enumor.BillAdjustmentResClassCPU,
		ResSubClass: cvt.ValToPtr(""),
	})
	if err := validateResSubClass(record.Vendor, resClass, resSubClass, source); err != nil {
		t.Errorf("change res_class to cpu with explicit clear should pass, err: %v", err)
	}

	// 记录厂商为 AWS，只改子类为仅 GCP 侧卡型，应被拒
	resClass, resSubClass = mergeUpdateResSubClass(record,
		&bill.BillAdjustmentItemUpdateReq{ResSubClass: cvt.ValToPtr("GB200")})
	if err := validateResSubClass(record.Vendor, resClass, resSubClass, source); err == nil {
		t.Error("res_sub_class out of vendor option list should be rejected")
	}
}

// TestIsResSubClassVendorSupported 仅 AWS/GCP/华为云在资源子类支持范围内。
func TestIsResSubClassVendorSupported(t *testing.T) {
	supported := []enumor.Vendor{enumor.Aws, enumor.Gcp, enumor.HuaWei}
	for _, vendor := range supported {
		if !isResSubClassVendorSupported(vendor) {
			t.Errorf("vendor %s should be supported", vendor)
		}
	}
	for _, vendor := range []enumor.Vendor{enumor.TCloud, enumor.Azure, ""} {
		if isResSubClassVendorSupported(vendor) {
			t.Errorf("vendor %s should not be supported", vendor)
		}
	}
}

// sortedStrings 返回去重排序后的副本，供期望值构造使用。
func sortedStrings(items []string) []string {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	return sortedKeys(set)
}
