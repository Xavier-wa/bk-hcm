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

package bill

import (
	"testing"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const prepaidItemSyncMaxNum = 100

// newValidSyncItem 构造一个全部硬校验均通过的基线订单，各反例用例在其上做单点篡改。
func newValidSyncItem() PrepaidItemSyncItem {
	return PrepaidItemSyncItem{
		UUID:               "prepaid-uuid-0001",
		OrderYear:          2026,
		OrderMonth:         7,
		Vendor:             enumor.Aws,
		RootAccountCloudID: "210987654321",
		MainAccountCloudID: "123456789012",
		GPUType:            "H100",
		DeviceNum:          2,
		CardNum:            16,
		ProductName:        "SageMaker Training Plan",
		Region:             "us-east-1",
		UsageStartAt:       "2026-08-01 00:00:00",
		UsageEndAt:         "2026-10-31 23:59:59",
		OrderAt:            "2026-07-15 10:00:00",
		Currency:           enumor.CurrencyUSD,
		Cost:               decimal.NewFromInt(3000),
		SplitItems: []PrepaidSplitItem{
			{BillYear: 2026, BillMonth: 8, Cost: decimal.NewFromInt(1000)},
			{BillYear: 2026, BillMonth: 9, Cost: decimal.NewFromInt(1000)},
			{BillYear: 2026, BillMonth: 10, Cost: decimal.NewFromInt(1000)},
		},
	}
}

// newValidSyncReq 构造只含一条基线订单的批量请求。
func newValidSyncReq() *PrepaidItemSyncReq {
	return &PrepaidItemSyncReq{Items: []PrepaidItemSyncItem{newValidSyncItem()}}
}

// TestPrepaidItemSyncReqValidatePass 合法批量请求校验通过。
func TestPrepaidItemSyncReqValidatePass(t *testing.T) {
	assert.NoError(t, newValidSyncReq().Validate())
}

// TestPrepaidItemSyncItemValidateHardCheck 逐一验证单个订单的硬校验反例。
func TestPrepaidItemSyncItemValidateHardCheck(t *testing.T) {
	tests := []struct {
		name    string
		tamper  func(item *PrepaidItemSyncItem)
		keyword string
	}{
		{
			name:    "illegal currency",
			tamper:  func(item *PrepaidItemSyncItem) { item.Currency = enumor.CurrencyCode("JPY") },
			keyword: "currency",
		},
		{
			name:    "illegal vendor",
			tamper:  func(item *PrepaidItemSyncItem) { item.Vendor = enumor.Vendor("not-exist-vendor") },
			keyword: "vendor",
		},
		{
			name:    "gpu type missing",
			tamper:  func(item *PrepaidItemSyncItem) { item.GPUType = "" },
			keyword: "gpu_type",
		},
		{
			name:    "root account cloud id missing",
			tamper:  func(item *PrepaidItemSyncItem) { item.RootAccountCloudID = "" },
			keyword: "root_account_cloud_id",
		},
		{
			name:    "main account cloud id missing",
			tamper:  func(item *PrepaidItemSyncItem) { item.MainAccountCloudID = "" },
			keyword: "main_account_cloud_id",
		},
		{
			name:    "usage start not before end",
			tamper:  func(item *PrepaidItemSyncItem) { item.UsageStartAt = "2026-11-01 00:00:00" },
			keyword: "usage_start_at",
		},
		{
			name:    "order at missing",
			tamper:  func(item *PrepaidItemSyncItem) { item.OrderAt = "" },
			keyword: "order_at",
		},
		{
			name:    "order at year mismatch",
			tamper:  func(item *PrepaidItemSyncItem) { item.OrderAt = "2025-07-15 10:00:00" },
			keyword: "order_at",
		},
		{
			name:    "order at month mismatch",
			tamper:  func(item *PrepaidItemSyncItem) { item.OrderAt = "2026-08-15 10:00:00" },
			keyword: "order_at",
		},
		{
			name:    "split bill month out of range",
			tamper:  func(item *PrepaidItemSyncItem) { item.SplitItems[0].BillMonth = 13 },
			keyword: "bill_month",
		},
		{
			name:    "cost not positive",
			tamper:  func(item *PrepaidItemSyncItem) { item.Cost = decimal.Zero },
			keyword: "cost",
		},
		{
			name:    "card num negative",
			tamper:  func(item *PrepaidItemSyncItem) { item.CardNum = -1 },
			keyword: "card_num",
		},
		{
			name:    "device num negative",
			tamper:  func(item *PrepaidItemSyncItem) { item.DeviceNum = -1 },
			keyword: "device_num",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := newValidSyncItem()
			tt.tamper(&item)

			err := item.Validate()
			assert.Error(t, err)
			assert.Equal(t, errf.InvalidParameter, errf.Error(err).Code)
			assert.Contains(t, err.Error(), tt.keyword)
		})
	}
}

// TestPrepaidItemSyncItemSplitItemsNoMaxNum 分摊明细不设条数上限，超过 36 条只要合计等于总价即通过。
func TestPrepaidItemSyncItemSplitItemsNoMaxNum(t *testing.T) {
	item := newValidSyncItem()
	item.SplitItems = make([]PrepaidSplitItem, 0, 37)
	for i := 0; i < 37; i++ {
		item.SplitItems = append(item.SplitItems,
			PrepaidSplitItem{BillYear: 2026, BillMonth: 8, Cost: decimal.NewFromInt(1)})
	}
	item.Cost = decimal.NewFromInt(37)

	assert.NoError(t, item.Validate())
}

// TestPrepaidItemSyncItemUsageTimeOptional 使用起止时间为非必填，缺失时不再约束分摊月份区间。
func TestPrepaidItemSyncItemUsageTimeOptional(t *testing.T) {
	item := newValidSyncItem()
	item.UsageStartAt, item.UsageEndAt = "", ""

	assert.NoError(t, item.Validate())
}

// TestPrepaidItemSyncItemSplitMonthOutsideUsageRange 分摊月份落在使用区间外不再被拒。
func TestPrepaidItemSyncItemSplitMonthOutsideUsageRange(t *testing.T) {
	item := newValidSyncItem()
	item.SplitItems[0].BillMonth = 12

	assert.NoError(t, item.Validate())
}

// TestPrepaidItemSyncItemSplitCostNotRequiredPositive 单条分摊金额不要求为正，合计等于总价即通过。
func TestPrepaidItemSyncItemSplitCostNotRequiredPositive(t *testing.T) {
	item := newValidSyncItem()
	item.SplitItems[0].Cost = decimal.NewFromInt(-100)
	item.SplitItems[1].Cost = decimal.Zero
	item.SplitItems[2].Cost = decimal.NewFromInt(3100)

	assert.NoError(t, item.Validate())
}

// TestPrepaidItemSyncItemSplitCostNotBalanced 分摊合计与总价不一致时拒绝并指明两个金额。
func TestPrepaidItemSyncItemSplitCostNotBalanced(t *testing.T) {
	item := newValidSyncItem()
	item.SplitItems[2].Cost = decimal.RequireFromString("999.99")

	err := item.Validate()
	assert.Error(t, err)
	assert.Equal(t, errf.InvalidParameter, errf.Error(err).Code)
	assert.Contains(t, err.Error(), "2999.99")
	assert.Contains(t, err.Error(), "3000")
}

// TestPrepaidItemSyncReqDuplicatedUniqueKey 批内唯一键重复时整批拒绝。
func TestPrepaidItemSyncReqDuplicatedUniqueKey(t *testing.T) {
	req := &PrepaidItemSyncReq{Items: []PrepaidItemSyncItem{newValidSyncItem(), newValidSyncItem()}}

	err := req.Validate()
	assert.Error(t, err)
	assert.Equal(t, errf.InvalidParameter, errf.Error(err).Code)
	assert.Contains(t, err.Error(), "duplicated")
}

// TestPrepaidItemSyncReqExceedMaxNum 单次批量超过 100 单在 validator 层即被拒。
func TestPrepaidItemSyncReqExceedMaxNum(t *testing.T) {
	req := &PrepaidItemSyncReq{Items: make([]PrepaidItemSyncItem, 0, prepaidItemSyncMaxNum+1)}
	for i := 0; i <= prepaidItemSyncMaxNum; i++ {
		req.Items = append(req.Items, newValidSyncItem())
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Equal(t, errf.InvalidParameter, errf.Error(err).Code)
	assert.Contains(t, err.Error(), "max")
}

// TestPrepaidItemSyncItemUniqueKey 唯一键按 uuid + 订单年月拼接，月份补零对齐。
func TestPrepaidItemSyncItemUniqueKey(t *testing.T) {
	item := newValidSyncItem()

	assert.Equal(t, "prepaid-uuid-0001/2026-07", item.UniqueKey())
}

// TestCurrencyCodeValidate 覆盖 AC-C03：CurrencyCode.Validate 新增方法的正反用例。
func TestCurrencyCodeValidate(t *testing.T) {
	assert.NoError(t, enumor.CurrencyCNY.Validate())
	assert.NoError(t, enumor.CurrencyUSD.Validate())
	assert.Error(t, enumor.CurrencyCode("JPY").Validate())
}

func TestPrepaidItemDeleteReq_Validate(t *testing.T) {
	testCases := []struct {
		name    string
		req     PrepaidItemDeleteReq
		wantErr bool
	}{
		{name: "valid order month", req: PrepaidItemDeleteReq{OrderYear: 2026, OrderMonth: 7}},
		{name: "optional cloud ids", req: PrepaidItemDeleteReq{OrderYear: 2026, OrderMonth: 7,
			MainAccountCloudIDs: []string{"123456789012", "210987654321"}}},
		{name: "empty cloud id in list", req: PrepaidItemDeleteReq{OrderYear: 2026, OrderMonth: 7,
			MainAccountCloudIDs: []string{""}}, wantErr: true},
		{name: "month zero", req: PrepaidItemDeleteReq{OrderYear: 2026, OrderMonth: 0}, wantErr: true},
		{name: "month thirteen", req: PrepaidItemDeleteReq{OrderYear: 2026, OrderMonth: 13}, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if tc.wantErr {
				require.Error(t, err)
				assert.Equal(t, errf.InvalidParameter, errf.Error(err).Code)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// TestValidatePrepaidCurrency 覆盖币种与二级账号所属 summary root 一致性比对函数。
func TestValidatePrepaidCurrency(t *testing.T) {
	assert.NoError(t, ValidatePrepaidCurrency(enumor.CurrencyUSD, enumor.CurrencyUSD))

	err := ValidatePrepaidCurrency(enumor.CurrencyUSD, enumor.CurrencyCNY)
	assert.Error(t, err)
	assert.Equal(t, errf.InvalidParameter, errf.Error(err).Code)
}

// TestValidatePrepaidRootAccountCloudID 覆盖请求一级账号云上 ID 与映射结果的一致性比对。
func TestValidatePrepaidRootAccountCloudID(t *testing.T) {
	assert.NoError(t, ValidatePrepaidRootAccountCloudID("210987654321", "210987654321"))

	err := ValidatePrepaidRootAccountCloudID("wrong-root", "210987654321")
	assert.Error(t, err)
	assert.Equal(t, errf.InvalidParameter, errf.Error(err).Code)
	assert.Contains(t, err.Error(), "root_account_cloud_id")
}
