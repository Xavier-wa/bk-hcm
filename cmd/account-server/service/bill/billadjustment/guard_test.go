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
	"testing"

	billcore "hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"

	"github.com/stretchr/testify/assert"
)

// manualConfirmed 构造一条人工录入、已确认、未推送、未定账的调账，是本次放开编辑删除的目标形态。
func manualConfirmed(id string) *billcore.AdjustmentItem {
	return &billcore.AdjustmentItem{
		ID:          id,
		State:       enumor.BillAdjustmentStateConfirmed,
		Source:      enumor.BillAdjustmentSourceManual,
		PushStatus:  enumor.BillAdjustmentPushStatusUnpushed,
		SettleState: enumor.BillSettleStateUnsettled,
	}
}

func TestGuardSourceManual(t *testing.T) {
	assert.NoError(t, guardSourceManual([]*billcore.AdjustmentItem{manualConfirmed("a1")}))

	prepaid := manualConfirmed("a2")
	prepaid.Source = enumor.BillAdjustmentSourcePrepaid
	err := guardSourceManual([]*billcore.AdjustmentItem{manualConfirmed("a1"), prepaid})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "a2")
	// 整批原子拒绝：错误信息只列被拒项，但调用方据此整批放弃，不允许部分成功。
	assert.NotContains(t, err.Error(), "a1")
}

func TestGuardNotPushingOrSettled(t *testing.T) {
	assert.NoError(t, guardNotPushingOrSettled([]*billcore.AdjustmentItem{manualConfirmed("a1")}))

	pushing := manualConfirmed("a2")
	pushing.PushStatus = enumor.BillAdjustmentPushStatusPushing
	err := guardNotPushingOrSettled([]*billcore.AdjustmentItem{pushing})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pushing")

	settled := manualConfirmed("a3")
	settled.SettleState = enumor.BillSettleStateSettled
	err = guardNotPushingOrSettled([]*billcore.AdjustmentItem{settled})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "settled")
}

func TestGuardNotPushingOrSettledAllowFailed(t *testing.T) {
	// failed 是可重推的中间态，不应被守卫拦下。
	failed := manualConfirmed("a1")
	failed.PushStatus = enumor.BillAdjustmentPushStatusFailed

	assert.NoError(t, guardNotPushingOrSettled([]*billcore.AdjustmentItem{failed}))
}

func TestGuardNotConfirmed(t *testing.T) {
	// 批量确认路径行为不变：已确认仍被拒绝，防止重复确认。
	err := guardNotConfirmed([]*billcore.AdjustmentItem{manualConfirmed("a1")})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "confirmed")

	unconfirmed := manualConfirmed("a2")
	unconfirmed.State = enumor.BillAdjustmentStateUnconfirmed
	assert.NoError(t, guardNotConfirmed([]*billcore.AdjustmentItem{unconfirmed}))
}

func TestEditGuardCombinationAllowsConfirmedManual(t *testing.T) {
	// 编辑删除路径的守卫组合里没有 guardNotConfirmed，
	// 因此人工录入的已确认调账在非推送中且非已定账时可以改，这是本次的行为放开点。
	items := []*billcore.AdjustmentItem{manualConfirmed("a1")}

	assert.NoError(t, guardSourceManual(items))
	assert.NoError(t, guardNotPushingOrSettled(items))
}

func TestCollectRejectedPreservesOrder(t *testing.T) {
	items := []*billcore.AdjustmentItem{manualConfirmed("a1"), manualConfirmed("a2"), manualConfirmed("a3")}
	items[0].SettleState = enumor.BillSettleStateSettled
	items[2].SettleState = enumor.BillSettleStateSettled

	rejected := collectRejected(items, func(item *billcore.AdjustmentItem) bool {
		return item.SettleState == enumor.BillSettleStateSettled
	})

	assert.Equal(t, []string{"a1", "a3"}, rejected)
}
