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

package billprepaid

import (
	"testing"

	asbill "hcm/pkg/api/account-server/bill"
	billcore "hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// newAdjustment 构造一条调账条目，简化各聚合用例的样板代码。
func newAdjustment(id, sourceID string, adjType enumor.BillAdjustmentType,
	pushStatus enumor.BillAdjustmentPushStatus, cost int64) *billcore.AdjustmentItem {

	return &billcore.AdjustmentItem{
		ID:         id,
		SourceID:   sourceID,
		Source:     enumor.BillAdjustmentSourcePrepaid,
		Type:       adjType,
		PushStatus: pushStatus,
		Cost:       decimal.NewFromInt(cost),
		RMBCost:    decimal.NewFromInt(cost * 7),
	}
}

func TestStatAdjustmentExcludeDecrease(t *testing.T) {
	items := []*billcore.AdjustmentItem{
		newAdjustment("a1", "p1", enumor.BillAdjustmentIncrease, enumor.BillAdjustmentPushStatusPushed, 100),
		newAdjustment("a2", "p1", enumor.BillAdjustmentIncrease, enumor.BillAdjustmentPushStatusUnpushed, 100),
		newAdjustment("a3", "p1", enumor.BillAdjustmentIncrease, enumor.BillAdjustmentPushStatusUnpushed, 100),
		// 调减条目既不进分子也不进分母，更不计入累计核算金额。
		newAdjustment("a4", "p1", enumor.BillAdjustmentDecrease, enumor.BillAdjustmentPushStatusPushed, 300),
	}

	stat := statAdjustmentBySourceID(items)["p1"]

	assert.Equal(t, 3, stat.IncreaseNum)
	assert.Equal(t, 1, stat.PushedIncreaseNum)
	assert.True(t, stat.AccountedCost.Equal(decimal.NewFromInt(100)))
	assert.True(t, stat.AccountedRMBCost.Equal(decimal.NewFromInt(700)))
}

func TestStatAdjustmentGroupBySourceID(t *testing.T) {
	items := []*billcore.AdjustmentItem{
		newAdjustment("a1", "p1", enumor.BillAdjustmentIncrease, enumor.BillAdjustmentPushStatusPushed, 100),
		newAdjustment("a2", "p2", enumor.BillAdjustmentIncrease, enumor.BillAdjustmentPushStatusPushed, 200),
		newAdjustment("a3", "p2", enumor.BillAdjustmentIncrease, enumor.BillAdjustmentPushStatusPushed, 200),
	}

	statMap := statAdjustmentBySourceID(items)

	assert.Len(t, statMap, 2)
	assert.Equal(t, 1, statMap["p1"].PushedIncreaseNum)
	assert.Equal(t, 2, statMap["p2"].PushedIncreaseNum)
	assert.True(t, statMap["p2"].AccountedCost.Equal(decimal.NewFromInt(400)))
}

func TestStatAdjustmentFailedNotAccounted(t *testing.T) {
	items := []*billcore.AdjustmentItem{
		newAdjustment("a1", "p1", enumor.BillAdjustmentIncrease, enumor.BillAdjustmentPushStatusFailed, 100),
		newAdjustment("a2", "p1", enumor.BillAdjustmentIncrease, enumor.BillAdjustmentPushStatusPushing, 100),
	}

	stat := statAdjustmentBySourceID(items)["p1"]

	assert.Equal(t, 2, stat.IncreaseNum)
	assert.Equal(t, 0, stat.PushedIncreaseNum)
	assert.True(t, stat.AccountedCost.IsZero())
}

func TestDerivePrepaidAccountingState(t *testing.T) {
	tests := []struct {
		name      string
		pushedNum int
		totalNum  int
		want      billcore.PrepaidAccountingState
	}{
		{name: "P=0 待核算", pushedNum: 0, totalNum: 3, want: billcore.PrepaidAccountingStatePending},
		{name: "0<P<N 核算中", pushedNum: 1, totalNum: 3, want: billcore.PrepaidAccountingStateAccounting},
		{name: "P=N 已完成", pushedNum: 3, totalNum: 3, want: billcore.PrepaidAccountingStateAccounted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, billcore.DerivePrepaidAccountingState(tt.pushedNum, tt.totalNum))
		})
	}
}

// newSyncItem 构造一个 3 个分摊月的同步订单，供 N+1 生成用例复用。
func newSyncItem() *asbill.PrepaidItemSyncItem {
	return &asbill.PrepaidItemSyncItem{
		UUID:       "prepaid-uuid-0001",
		OrderYear:  2026,
		OrderMonth: 7,
		Vendor:     enumor.Aws,
		GPUType:    "H100",
		Currency:   enumor.CurrencyUSD,
		Cost:       decimal.NewFromInt(3000),
		SplitItems: []asbill.PrepaidSplitItem{
			{BillYear: 2026, BillMonth: 8, Cost: decimal.NewFromInt(1000)},
			{BillYear: 2026, BillMonth: 9, Cost: decimal.NewFromInt(1000)},
			{BillYear: 2026, BillMonth: 10, Cost: decimal.NewFromInt(1000)},
		},
	}
}

func TestBuildAdjustmentItemsShape(t *testing.T) {
	syncItem := newSyncItem()
	account := &mappedMainAccount{ID: "ma-001", RootAccountID: "ra-001", CloudID: "123456", OpProductID: 66}

	items := buildAdjustmentItems(&kit.Kit{User: "syncer"}, syncItem, account)

	assert.Len(t, items, len(syncItem.SplitItems)+1)

	for idx, split := range syncItem.SplitItems {
		item := items[idx]
		assert.Equal(t, enumor.BillAdjustmentIncrease, item.Type)
		assert.Equal(t, split.BillYear, item.BillYear)
		assert.Equal(t, split.BillMonth, item.BillMonth)
		assert.True(t, item.Cost.Equal(split.Cost))
		assert.Equal(t, prepaidBillDay, item.BillDay)
		assert.Equal(t, enumor.BillAdjustmentStateConfirmed, item.State)
		assert.Equal(t, enumor.BillAdjustmentSourcePrepaid, item.Source)
		assert.Equal(t, enumor.BillAdjustmentPushStatusUnpushed, item.PushStatus)
		assert.Equal(t, enumor.BillSettleStateUnsettled, item.SettleState)
		// bk_biz_id 必须留空，只填 product_id，以满足调账表二选一约束。
		assert.Zero(t, item.BkBizID)
		assert.Equal(t, account.OpProductID, item.ProductID)
	}
}

// TestBuildAdjustmentItemsResClassFixed 资源类别固定 gpu_card、资源子类取 gpu_type，N+1 条全部一致。
func TestBuildAdjustmentItemsResClassFixed(t *testing.T) {
	syncItem := newSyncItem()

	items := buildAdjustmentItems(&kit.Kit{User: "syncer"},
		syncItem, &mappedMainAccount{ID: "ma-001", RootAccountID: "ra-001"})

	for _, item := range items {
		assert.Equal(t, enumor.BillAdjustmentResClassGpuCard, item.ResClass)
		assert.Equal(t, syncItem.GPUType, item.ResSubClass)
	}
}

// TestBuildAdjustmentItemsRMBCostByCurrency 人民币金额按币种派生：外币落 0，人民币与 cost 同值。
func TestBuildAdjustmentItemsRMBCostByCurrency(t *testing.T) {
	account := &mappedMainAccount{ID: "ma-001", RootAccountID: "ra-001"}

	usdItems := buildAdjustmentItems(&kit.Kit{User: "syncer"}, newSyncItem(), account)
	for _, item := range usdItems {
		assert.True(t, item.RMBCost.IsZero())
	}

	cnyItem := newSyncItem()
	cnyItem.Currency = enumor.CurrencyCNY
	cnyItems := buildAdjustmentItems(&kit.Kit{User: "syncer"}, cnyItem, account)
	for _, item := range cnyItems {
		assert.True(t, item.RMBCost.Equal(item.Cost))
	}
}

func TestBuildAdjustmentItemsDecreaseOnOrderMonth(t *testing.T) {
	syncItem := newSyncItem()
	account := &mappedMainAccount{ID: "ma-001", RootAccountID: "ra-001", OpProductID: 66}

	items := buildAdjustmentItems(&kit.Kit{User: "syncer"}, syncItem, account)
	decrease := items[len(items)-1]

	assert.Equal(t, enumor.BillAdjustmentDecrease, decrease.Type)
	// 调减落订单月份而非当前自然月，也不是任一分摊月。
	assert.Equal(t, syncItem.OrderYear, decrease.BillYear)
	assert.Equal(t, syncItem.OrderMonth, decrease.BillMonth)
	// 金额一律传正数，正负语义由 type 承载。
	assert.True(t, decrease.Cost.Equal(syncItem.Cost))
	assert.True(t, decrease.Cost.IsPositive())
}

func TestBuildAdjustmentItemsIncreaseSumEqualsDecrease(t *testing.T) {
	items := buildAdjustmentItems(&kit.Kit{User: "syncer"},
		newSyncItem(), &mappedMainAccount{ID: "ma-001", RootAccountID: "ra-001"})

	sum := decimal.Zero
	for _, item := range items {
		if item.Type == enumor.BillAdjustmentIncrease {
			sum = sum.Add(item.Cost)
		}
	}

	assert.True(t, sum.Equal(items[len(items)-1].Cost))
}

func TestBuildPrepaidItemCreateReqUsesMappedAccount(t *testing.T) {
	syncItem := newSyncItem()
	syncItem.MainAccountCloudID = "raw-cloud-id"
	syncItem.RootAccountCloudID = "raw-root-cloud-id"
	account := &mappedMainAccount{
		ID:                 "ma-001",
		RootAccountID:      "ra-001",
		RootAccountCloudID: "mapped-root-cloud-id",
		CloudID:            "mapped-cloud-id",
		OpProductID:        66,
	}

	dsReq := buildPrepaidItemCreateReq(syncItem, account)

	assert.Equal(t, account.ID, dsReq.MainAccountID)
	assert.Equal(t, account.RootAccountID, dsReq.RootAccountID)
	// 账号维度取映射结果而非请求原值，避免调用方传错云上 ID 时落库脏数据。
	assert.Equal(t, account.CloudID, dsReq.MainAccountCloudID)
	assert.Equal(t, account.RootAccountCloudID, dsReq.RootAccountCloudID)
	assert.Equal(t, account.OpProductID, dsReq.ProductID)
	assert.Equal(t, enumor.BillSettleStateUnsettled, dsReq.SettleState)
}

// TestBuildPrepaidItemCreateReqKeepsTimeAsIs 时间列是 varchar，落库串必须与请求原样一致：
// 该格式无时区偏移且定长，字典序即时间序，范围查询直接依赖这一点。
func TestBuildPrepaidItemCreateReqKeepsTimeAsIs(t *testing.T) {
	syncItem := newSyncItem()
	syncItem.UsageStartAt = "2026-08-01 00:00:00"
	syncItem.OrderAt = "2026-07-15 10:00:00"

	dsReq := buildPrepaidItemCreateReq(syncItem, &mappedMainAccount{ID: "ma-001", RootAccountID: "ra-001"})

	assert.Equal(t, "2026-08-01 00:00:00", dsReq.UsageStartAt)
	assert.Equal(t, "2026-07-15 10:00:00", dsReq.OrderAt)
	assert.Empty(t, dsReq.UsageEndAt)
}
