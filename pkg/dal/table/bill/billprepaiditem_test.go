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
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
	cvt "hcm/pkg/tools/converter"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// TestPrepaidItemTimeFieldsEnterSQLAsIs 三个业务时间字段必须以原样时间串进入 SQL，
// 即「写进去什么格式，库里就是什么格式」。
func TestPrepaidItemTimeFieldsEnterSQLAsIs(t *testing.T) {
	item := &AccountBillPrepaidItem{
		UsageStartAt: cvt.ValToPtr("2026-08-01 00:00:00"),
		UsageEndAt:   cvt.ValToPtr("2026-10-31 23:59:59"),
		OrderAt:      "2026-07-15 10:00:00",
	}

	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(item, utils.NewFieldOptions())
	assert.NoError(t, err)

	for column, want := range map[string]string{
		"usage_start_at": "2026-08-01 00:00:00",
		"usage_end_at":   "2026-10-31 23:59:59",
		"order_at":       "2026-07-15 10:00:00",
	} {
		assert.Contains(t, setExpr, column+" = :"+column)
		assert.Equal(t, want, timeValueOf(toUpdate[column]))
	}
}

// TestPrepaidItemEmptyUsageTimeStillEntersUpdate 覆盖重推清空的关键前提：
// 使用时间指向空串时仍须进入 SET 子句，若退化成非指针字段会被 blank 规则跳过，库里旧值就永远清不掉。
func TestPrepaidItemEmptyUsageTimeStillEntersUpdate(t *testing.T) {
	item := &AccountBillPrepaidItem{
		UsageStartAt: cvt.ValToPtr(""),
		UsageEndAt:   cvt.ValToPtr(""),
		OrderAt:      "2026-07-15 10:00:00",
	}

	setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(item, utils.NewFieldOptions())
	assert.NoError(t, err)

	for _, column := range []string{"usage_start_at", "usage_end_at"} {
		assert.Contains(t, setExpr, column+" = :"+column)
		assert.Equal(t, "", timeValueOf(toUpdate[column]))
	}
}

// TestPrepaidItemNilUsageTimeSkipsUpdate 指针为 nil 表示本次不更新该列，须落在 blank 分支被跳过。
func TestPrepaidItemNilUsageTimeSkipsUpdate(t *testing.T) {
	item := &AccountBillPrepaidItem{OrderAt: "2026-07-15 10:00:00"}

	setExpr, _, err := utils.RearrangeSQLDataWithOption(item, utils.NewFieldOptions())
	assert.NoError(t, err)

	assert.NotContains(t, setExpr, "usage_start_at")
	assert.NotContains(t, setExpr, "usage_end_at")
}

// TestPrepaidItemInsertValidateOnlyRequiresOrderAt 使用起止时间非必填，缺失不拦；
// 订单时间是账期归属依据，缺失必须在写库前被拒。
func TestPrepaidItemInsertValidateOnlyRequiresOrderAt(t *testing.T) {
	item := newValidPrepaidItem()
	item.UsageStartAt, item.UsageEndAt = nil, nil
	assert.NoError(t, item.InsertValidate())

	item = newValidPrepaidItem()
	item.OrderAt = ""
	assert.Error(t, item.InsertValidate())
}

// newValidPrepaidItem 构造一条通过 InsertValidate 的最小合法记录。
func newValidPrepaidItem() *AccountBillPrepaidItem {
	cost := &types.Decimal{Decimal: decimal.NewFromInt(100)}

	return &AccountBillPrepaidItem{
		UUID:          "prepaid-uuid-1",
		OrderYear:     2026,
		OrderMonth:    7,
		Vendor:        enumor.TCloud,
		RootAccountID: "root-1",
		MainAccountID: "main-1",
		UsageStartAt:  cvt.ValToPtr("2026-08-01 00:00:00"),
		UsageEndAt:    cvt.ValToPtr("2026-10-31 23:59:59"),
		OrderAt:       "2026-07-15 10:00:00",
		Currency:      enumor.CurrencyRMB,
		Cost:          cost,
		RMBCost:       cost,
		SettleState:   enumor.BillSettleStateUnsettled,
	}
}

// timeValueOf 取出进入 SQL 的时间列值，指针与值两种承载方式都归一为时间串。
func timeValueOf(value interface{}) string {
	if ptr, ok := value.(*string); ok {
		return cvt.PtrToVal(ptr)
	}

	return value.(string)
}
