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

package billadjustmentitem

import (
	"testing"

	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	cvt "hcm/pkg/tools/converter"

	"github.com/stretchr/testify/assert"
)

// TestStateUpdateReqRequiresFilter 更新范围统一由 filter 表达，缺失 filter 必须被挡在 handler 之外。
func TestStateUpdateReqRequiresFilter(t *testing.T) {
	req := &dsbill.BillAdjustmentItemStateUpdateReq{
		PushStatus: enumor.BillAdjustmentPushStatusPushed,
	}

	assert.Error(t, req.Validate())
}

// TestBuildStateUpdatePlanByIDs 按 ID 集合打点：过滤条件收敛到 id IN，且更新模型必须能过按 filter 更新的校验。
func TestBuildStateUpdatePlanByIDs(t *testing.T) {
	req := &dsbill.BillAdjustmentItemStateUpdateReq{
		Filter:         tools.ContainersExpression("id", []string{"adj-1", "adj-2"}),
		PushStatus:     enumor.BillAdjustmentPushStatusPushed,
		PushFailReason: cvt.ValToPtr(""),
	}
	assert.NoError(t, req.Validate())

	whereExpr, whereValue, err := req.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	assert.NoError(t, err)
	assert.Contains(t, whereExpr, "id IN")
	assert.NotEmpty(t, whereValue)

	updateData := buildStateUpdateData(req)

	// 关键断言：批量打点没有单一 ID，模型必须能过不依赖 ID 的 UpdateValidate。
	assert.NoError(t, updateData.UpdateValidate())
	assert.Empty(t, updateData.ID)
	assert.Equal(t, enumor.BillAdjustmentPushStatusPushed, updateData.PushStatus)
	assert.Equal(t, cvt.ValToPtr(""), updateData.PushFailReason)
}

// TestBuildStateUpdatePlanByPeriodFilter 按账期打点：过滤条件不含 id，模型同样要过校验。
func TestBuildStateUpdatePlanByPeriodFilter(t *testing.T) {
	req := &dsbill.BillAdjustmentItemStateUpdateReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("vendor", enumor.TCloud),
			tools.RuleEqual("bill_year", 2026),
			tools.RuleEqual("bill_month", 8),
			tools.RuleEqual("state", enumor.BillAdjustmentStateConfirmed),
		),
		PushStatus:     enumor.BillAdjustmentPushStatusPushing,
		PushFailReason: cvt.ValToPtr(""),
	}
	assert.NoError(t, req.Validate())

	whereExpr, _, err := req.Filter.SQLWhereExpr(tools.DefaultSqlWhereOption)
	assert.NoError(t, err)
	assert.Contains(t, whereExpr, "bill_month")
	assert.NotContains(t, whereExpr, "id IN")

	updateData := buildStateUpdateData(req)

	assert.NoError(t, updateData.UpdateValidate())
	assert.Equal(t, enumor.BillAdjustmentPushStatusPushing, updateData.PushStatus)
	assert.Equal(t, cvt.ValToPtr(""), updateData.PushFailReason)
}

// TestBuildStateUpdatePlanKeepsPushFailReason 定账任务只传 SettleState，
// 此时 push_fail_reason 必须保持 nil，否则会擦掉停留 failed 等人工介入的失败原因。
func TestBuildStateUpdatePlanKeepsPushFailReason(t *testing.T) {
	req := &dsbill.BillAdjustmentItemStateUpdateReq{
		Filter:      tools.ContainersExpression("id", []string{"adj-1"}),
		SettleState: enumor.BillSettleStateSettled,
	}
	assert.NoError(t, req.Validate())

	updateData := buildStateUpdateData(req)

	assert.Nil(t, updateData.PushFailReason)
	assert.Equal(t, enumor.BillSettleStateSettled, updateData.SettleState)
	assert.NoError(t, updateData.UpdateValidate())
}

// TestBuildStateUpdatePlanRejectsInvalidState 非法状态枚举必须被模型校验挡住，避免刷进整批数据。
func TestBuildStateUpdatePlanRejectsInvalidState(t *testing.T) {
	req := &dsbill.BillAdjustmentItemStateUpdateReq{
		Filter:     tools.ContainersExpression("id", []string{"adj-1"}),
		PushStatus: enumor.BillAdjustmentPushStatus("unknown"),
	}

	assert.Error(t, buildStateUpdateData(req).UpdateValidate())
}
