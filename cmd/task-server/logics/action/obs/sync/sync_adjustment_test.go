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

	"hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"

	"github.com/shopspring/decimal"
)

// TestSplitAdjustmentResSubClass 校验调账资源子类按资源类别分发到 OBS 的两列。
func TestSplitAdjustmentResSubClass(t *testing.T) {
	tests := []struct {
		name         string
		resClass     enumor.BillAdjustmentResClass
		resSubClass  string
		wantGpuCard  string
		wantAPIBrand string
	}{
		{"gpu_card 写卡型列", enumor.BillAdjustmentResClassGpuCard, "H100", "H100", ""},
		{"gpu_api 写模型厂商列", enumor.BillAdjustmentResClassGpuAPI, "gemini", "", "gemini"},
		{"gpu_other 两列均空", enumor.BillAdjustmentResClassGpuOther, "", "", ""},
		{"cpu 两列均空", enumor.BillAdjustmentResClassCPU, "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gpuCard, apiBrand := splitAdjustmentResSubClass(tt.resClass, tt.resSubClass)
			if gpuCard != tt.wantGpuCard || apiBrand != tt.wantAPIBrand {
				t.Fatalf("splitAdjustmentResSubClass() = (%s, %s), want (%s, %s)",
					gpuCard, apiBrand, tt.wantGpuCard, tt.wantAPIBrand)
			}
		})
	}
}

// TestAdjustmentOBSResClassID 校验四类调账在三个云厂商下落到的 OBS 资源分类 ID。
func TestAdjustmentOBSResClassID(t *testing.T) {
	tests := []struct {
		name     string
		vendor   enumor.Vendor
		resClass enumor.BillAdjustmentResClass
		want     int32
	}{
		{"aws gpu_api 落 6799", enumor.Aws, enumor.BillAdjustmentResClassGpuAPI,
			int32(enumor.OBSResClassIDAwsAPI)},
		{"aws gpu_card 落 GPU", enumor.Aws, enumor.BillAdjustmentResClassGpuCard,
			int32(enumor.OBSResClassIDAwsGPU)},
		{"aws cpu 落 CPU", enumor.Aws, enumor.BillAdjustmentResClassCPU,
			int32(enumor.OBSResClassIDAwsCPU)},
		{"gcp gpu_other 落 6312", enumor.Gcp, enumor.BillAdjustmentResClassGpuOther,
			int32(enumor.OBSResClassIDGcpGPU)},
		{"gcp gpu_api 落 API", enumor.Gcp, enumor.BillAdjustmentResClassGpuAPI,
			int32(enumor.OBSResClassIDGcpAPI)},
		{"华为云 gpu_api 回落 6315", enumor.HuaWei, enumor.BillAdjustmentResClassGpuAPI,
			int32(enumor.OBSResClassIDHuaweiGPU)},
		{"华为云 cpu 落 CPU", enumor.HuaWei, enumor.BillAdjustmentResClassCPU,
			int32(enumor.OBSResClassIDHuaweiCPU)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enumor.GetOBSResClassIDByType(tt.vendor, tt.resClass.IsGPU(), tt.resClass.IsAPI())
			if got != tt.want {
				t.Fatalf("GetOBSResClassIDByType(%s, %s) = %d, want %d", tt.vendor, tt.resClass, got, tt.want)
			}
		})
	}
}

func TestGetAdjustmentCost(t *testing.T) {
	tests := []struct {
		name    string
		adj     *bill.AdjustmentItem
		vendor  enumor.Vendor
		want    decimal.Decimal
		wantErr bool
	}{
		{
			name: "increase with positive cost",
			adj: &bill.AdjustmentItem{
				Type: enumor.BillAdjustmentIncrease,
				Cost: decimal.NewFromInt(100),
			},
			vendor: enumor.Azure,
			want:   decimal.NewFromInt(100),
		},
		{
			name: "decrease with positive cost",
			adj: &bill.AdjustmentItem{
				Type: enumor.BillAdjustmentDecrease,
				Cost: decimal.NewFromInt(100),
			},
			vendor: enumor.HuaWei,
			want:   decimal.NewFromInt(-100),
		},
		{
			name: "increase with negative cost",
			adj: &bill.AdjustmentItem{
				Type: enumor.BillAdjustmentIncrease,
				Cost: decimal.NewFromInt(-100),
			},
			vendor: enumor.Gcp,
			want:   decimal.NewFromInt(-100),
		},
		{
			name: "decrease with negative cost",
			adj: &bill.AdjustmentItem{
				Type: enumor.BillAdjustmentDecrease,
				Cost: decimal.NewFromInt(-100),
			},
			vendor: enumor.Zenlayer,
			want:   decimal.NewFromInt(100),
		},
		{
			name: "invalid adjustment type",
			adj: &bill.AdjustmentItem{
				Type: enumor.BillAdjustmentType("invalid"),
				Cost: decimal.NewFromInt(100),
			},
			vendor:  enumor.Aws,
			want:    decimal.Zero,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getAdjustmentCost(kit.New(), tt.adj, tt.vendor)
			if (err != nil) != tt.wantErr {
				t.Fatalf("getAdjustmentCost() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("getAdjustmentCost() got = %s, want %s", got.String(), tt.want.String())
			}
		})
	}
}
