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
	"strings"
	"testing"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/types"
	tablebill "hcm/pkg/dal/table/bill"
	"hcm/pkg/dal/table/utils"
	cvt "hcm/pkg/tools/converter"

	"github.com/stretchr/testify/assert"
)

// TestAdjustmentItemBlankedFields 守护可置空字段的写入语义：nil 不进 SET、指向零值才清空。
// 同时守护「可置空字段必须是指针类型」这一前提：RearrangeSQLDataWithOption 对可置空字段
// 会调用 reflect.Value.IsNil 判空，字段一旦退回非指针类型就会 panic 而不是报错。
func TestAdjustmentItemBlankedFields(t *testing.T) {
	testCases := []struct {
		name       string
		updateData *tablebill.AccountBillAdjustmentItem
		wantUpdate bool
	}{
		{
			name: "nil push fail reason is left untouched",
			updateData: &tablebill.AccountBillAdjustmentItem{
				PushStatus: enumor.BillAdjustmentPushStatusPushing,
			},
			wantUpdate: false,
		},
		{
			name: "blank push fail reason is cleared",
			updateData: &tablebill.AccountBillAdjustmentItem{
				PushStatus:     enumor.BillAdjustmentPushStatusPushed,
				PushFailReason: cvt.ValToPtr(""),
			},
			wantUpdate: true,
		},
		{
			name: "non blank push fail reason is written",
			updateData: &tablebill.AccountBillAdjustmentItem{
				PushStatus:     enumor.BillAdjustmentPushStatusFailed,
				PushFailReason: cvt.ValToPtr("obs push flow canceled"),
			},
			wantUpdate: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := utils.NewFieldOptions().AddIgnoredFields(types.DefaultIgnoredFields...).
				AddBlankedFields(adjustmentItemBlankedFields...)

			setExpr, toUpdate, err := utils.RearrangeSQLDataWithOption(tc.updateData, opts)
			assert.NoError(t, err)

			_, exist := toUpdate["push_fail_reason"]
			assert.Equal(t, tc.wantUpdate, exist)
			assert.Equal(t, tc.wantUpdate, strings.Contains(setExpr, "push_fail_reason = :push_fail_reason"))
		})
	}
}
