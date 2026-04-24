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

	"github.com/shopspring/decimal"
)

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
			got, err := getAdjustmentCost(tt.adj, tt.vendor)
			if (err != nil) != tt.wantErr {
				t.Fatalf("getAdjustmentCost() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("getAdjustmentCost() got = %s, want %s", got.String(), tt.want.String())
			}
		})
	}
}
