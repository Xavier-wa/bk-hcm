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

package billprepaiditem

import (
	"testing"

	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	cvt "hcm/pkg/tools/converter"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// fullAdjustmentCreateReq 构造一条 account-server 实际下发形态的调账创建请求。
func fullAdjustmentCreateReq() *dsbill.BillAdjustmentItemCreateReq {
	return &dsbill.BillAdjustmentItemCreateReq{
		RootAccountID: "root-1",
		MainAccountID: "main-1",
		Vendor:        enumor.TCloud,
		ProductID:     10001,
		BillYear:      2026,
		BillMonth:     8,
		BillDay:       1,
		Type:          enumor.BillAdjustmentIncrease,
		ResClass:      enumor.BillAdjustmentResClassGpuCard,
		ResSubClass:   "A100",
		Operator:      "syncer",
		Currency:      enumor.CurrencyUSD,
		Cost:          decimal.NewFromInt(100),
		RMBCost:       decimal.NewFromInt(720),
		State:         enumor.BillAdjustmentStateConfirmed,
	}
}

// TestBuildAdjustmentModelPassesInsertValidate 字段完整性门禁：
// 转换结果必须能通过调账表的 InsertValidate，否则整组调账在 BulkInsert 前就被拒、事务回滚，
// 写入接口 100% 失败。后续给调账表加必填字段时，本用例会直接把漏拷暴露出来。
func TestBuildAdjustmentModelPassesInsertValidate(t *testing.T) {
	model := buildAdjustmentModel(&kit.Kit{User: "syncer"}, "prepaid-1", fullAdjustmentCreateReq())

	assert.NoError(t, model.InsertValidate())
}

// TestBuildAdjustmentModelCopiesBillPeriod 账期三字段必须跨服务完整拷贝，bill_day 是此前漏拷的字段。
func TestBuildAdjustmentModelCopiesBillPeriod(t *testing.T) {
	req := fullAdjustmentCreateReq()
	req.BillYear, req.BillMonth, req.BillDay = 2025, 12, 1

	model := buildAdjustmentModel(&kit.Kit{User: "syncer"}, "prepaid-1", req)

	assert.Equal(t, 2025, model.BillYear)
	assert.Equal(t, 12, model.BillMonth)
	assert.Equal(t, 1, model.BillDay)
}

// TestBuildAdjustmentModelFixesPrepaidFields 预付费路径的固定字段不取请求值，避免调用方越权指定。
func TestBuildAdjustmentModelFixesPrepaidFields(t *testing.T) {
	req := fullAdjustmentCreateReq()
	req.Source = enumor.BillAdjustmentSourceManual
	req.SourceID = "forged"
	req.PushStatus = enumor.BillAdjustmentPushStatusPushed
	req.SettleState = enumor.BillSettleStateSettled

	model := buildAdjustmentModel(&kit.Kit{User: "syncer"}, "prepaid-1", req)

	assert.Equal(t, enumor.BillAdjustmentSourcePrepaid, model.Source)
	assert.Equal(t, "prepaid-1", model.SourceID)
	assert.Equal(t, enumor.BillAdjustmentPushStatusUnpushed, model.PushStatus)
	assert.Equal(t, cvt.ValToPtr(""), model.PushFailReason)
	assert.Equal(t, enumor.BillSettleStateUnsettled, model.SettleState)
	assert.Equal(t, "syncer", model.Creator)
}

// TestBuildAdjustmentModelForcesConfirmedState state 恒为 confirmed，与请求传值无关。
// 定账任务「只捞 state=confirmed」的口径依赖这条不变量：若派生调账落成 unconfirmed，
// 它将永远不会被定账，settle_state 长期停在 unsettled。
func TestBuildAdjustmentModelForcesConfirmedState(t *testing.T) {
	for _, state := range []enumor.BillAdjustmentState{
		enumor.BillAdjustmentStateUnconfirmed,
		enumor.BillAdjustmentStateConfirmed,
		"",
	} {
		req := fullAdjustmentCreateReq()
		req.State = state

		model := buildAdjustmentModel(&kit.Kit{User: "syncer"}, "prepaid-1", req)

		assert.Equal(t, enumor.BillAdjustmentStateConfirmed, model.State)
		assert.NoError(t, model.InsertValidate())
	}
}

// TestBuildAdjustmentModelMissingBillDayRejected 反向验证上面的完整性断言不是空转：
// 缺 bill_day 时 InsertValidate 必须报错。
func TestBuildAdjustmentModelMissingBillDayRejected(t *testing.T) {
	req := fullAdjustmentCreateReq()
	req.BillDay = 0

	model := buildAdjustmentModel(&kit.Kit{User: "syncer"}, "prepaid-1", req)

	err := model.InsertValidate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bill_day")
}
