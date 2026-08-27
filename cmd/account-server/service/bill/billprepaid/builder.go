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
	asbill "hcm/pkg/api/account-server/bill"
	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	cvt "hcm/pkg/tools/converter"

	"github.com/shopspring/decimal"
)

// prepaidBillDay 预付费派生调账固定落每月 1 日，账期粒度为月不区分具体天。
const prepaidBillDay = 1

// buildPrepaidItemCreateReq 组装预付费主表落库数据，账号维度取映射结果而非请求原值。
// 新建记录 settle_state 固定为 unsettled。
func buildPrepaidItemCreateReq(item *asbill.PrepaidItemSyncItem,
	mainAccount *mappedMainAccount) *dsbill.PrepaidItemCreateReq {

	return &dsbill.PrepaidItemCreateReq{
		UUID:               item.UUID,
		OrderYear:          item.OrderYear,
		OrderMonth:         item.OrderMonth,
		Vendor:             item.Vendor,
		RootAccountID:      mainAccount.RootAccountID,
		MainAccountID:      mainAccount.ID,
		RootAccountCloudID: mainAccount.RootAccountCloudID,
		MainAccountCloudID: mainAccount.CloudID,
		ProductID:          mainAccount.OpProductID,
		ResourceID:         item.ResourceID,
		InvoiceID:          item.InvoiceID,
		GPUType:            item.GPUType,
		DeviceNum:          item.DeviceNum,
		CardNum:            item.CardNum,
		ProductName:        item.ProductName,
		ProductSpec:        item.ProductSpec,
		Region:             item.Region,
		UsageStartAt:       item.UsageStartAt,
		UsageEndAt:         item.UsageEndAt,
		OrderAt:            item.OrderAt,
		Currency:           item.Currency,
		Cost:               item.Cost,
		RMBCost:            deriveRMBCost(item.Currency, item.Cost),
		SettleState:        enumor.BillSettleStateUnsettled,
	}
}

// buildAdjustmentItems 生成 N+1 条调账：N 条调增按分摊自然月分布，1 条调减固定落订单月份。
func buildAdjustmentItems(kt *kit.Kit, item *asbill.PrepaidItemSyncItem,
	mainAccount *mappedMainAccount) []dsbill.BillAdjustmentItemCreateReq {

	items := make([]dsbill.BillAdjustmentItemCreateReq, 0, len(item.SplitItems)+1)

	for _, split := range item.SplitItems {
		adjustment := newAdjustmentBase(kt, item, mainAccount)
		adjustment.BillYear = split.BillYear
		adjustment.BillMonth = split.BillMonth
		adjustment.Type = enumor.BillAdjustmentIncrease
		adjustment.Memo = split.Memo
		adjustment.Cost = split.Cost
		adjustment.RMBCost = deriveRMBCost(item.Currency, split.Cost)
		items = append(items, adjustment)
	}

	// 调减条目落订单月份而非当前自然月，保证一次性扣减与订单账期对齐。
	decrease := newAdjustmentBase(kt, item, mainAccount)
	decrease.BillYear = item.OrderYear
	decrease.BillMonth = item.OrderMonth
	decrease.Type = enumor.BillAdjustmentDecrease
	decrease.Cost = item.Cost
	decrease.RMBCost = deriveRMBCost(item.Currency, item.Cost)
	decrease.Memo = cvt.ValToPtr("预付费账单一次性扣减，uuid: " + item.UUID)
	items = append(items, decrease)

	return items
}

// newAdjustmentBase 构造预付费派生调账的公共字段。
// bk_biz_id 留空只填 product_id，以满足调账表「业务与运营产品二选一」的约束。
// 资源类别固定为 gpu_card、资源子类取该单的 gpu_type：预付费单据本身就是 GPU 卡的整单预购。
func newAdjustmentBase(kt *kit.Kit, item *asbill.PrepaidItemSyncItem,
	mainAccount *mappedMainAccount) dsbill.BillAdjustmentItemCreateReq {

	return dsbill.BillAdjustmentItemCreateReq{
		RootAccountID: mainAccount.RootAccountID,
		MainAccountID: mainAccount.ID,
		Vendor:        item.Vendor,
		ProductID:     mainAccount.OpProductID,
		BillDay:       prepaidBillDay,
		Operator:      kt.User,
		Currency:      item.Currency,
		ResClass:      enumor.BillAdjustmentResClassGpuCard,
		ResSubClass:   item.GPUType,
		State:         enumor.BillAdjustmentStateConfirmed,
		Source:        enumor.BillAdjustmentSourcePrepaid,
		PushStatus:    enumor.BillAdjustmentPushStatusUnpushed,
		SettleState:   enumor.BillSettleStateUnsettled,
	}
}

// deriveRMBCost 按币种派生人民币金额：本币即人民币时与 cost 同值，其他币种落 0。
// HCM 不做汇率换算，外币单据的人民币金额由账单汇总任务按当月平均汇率另行计算。
func deriveRMBCost(currency enumor.CurrencyCode, cost decimal.Decimal) decimal.Decimal {
	if currency == enumor.CurrencyRMB {
		return cost
	}

	return decimal.Zero
}
