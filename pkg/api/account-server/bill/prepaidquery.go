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
	"hcm/pkg/api/core"
	billcore "hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"

	"github.com/shopspring/decimal"
)

// PrepaidItemListReq 预付费账单列表查询请求。
type PrepaidItemListReq = core.ListWithoutFieldReq

// PrepaidItemListResult 预付费账单列表查询结果。
type PrepaidItemListResult struct {
	// Count 总数，仅 page.count=true 时有值
	Count uint64 `json:"count"`
	// Details 列表项，核算派生值由后端实时计算不落库
	Details []*PrepaidItemResult `json:"details"`
}

// PrepaidItemResult 预付费账单列表项，在主表字段之上附加实时派生的核算信息。
type PrepaidItemResult struct {
	*billcore.PrepaidItem `json:",inline"`
	// AccountingState 核算状态，由已推送调增条目数与调增条目总数派生
	AccountingState billcore.PrepaidAccountingState `json:"accounting_state"`
	// AccountedCost 累计核算金额，为已推送调增条目的分摊金额之和，不含调减
	AccountedCost decimal.Decimal `json:"accounted_cost"`
	// AccountedRMBCost 累计核算金额的人民币值
	AccountedRMBCost decimal.Decimal `json:"accounted_rmb_cost"`
}

// PrepaidSplitItemListResult 预付费账单月度分摊行列表查询结果。
type PrepaidSplitItemListResult struct {
	// Count 总数
	Count uint64 `json:"count"`
	// Details 月度分摊行
	Details []*PrepaidSplitItemResult `json:"details"`
}

// PrepaidSplitItemResult 月度分摊行，行级核算状态按该条 push_status 单条判定。
type PrepaidSplitItemResult struct {
	// AdjustmentID 调账编号
	AdjustmentID string `json:"adjustment_id"`
	// BillYear 账期年份
	BillYear int `json:"bill_year"`
	// BillMonth 账期月份
	BillMonth int `json:"bill_month"`
	// Accounted 行级核算状态：该条 push_status=pushed 即已核算
	Accounted bool `json:"accounted"`
	// Type 调账类型
	Type enumor.BillAdjustmentType `json:"type"`
	// Cost 调账金额
	Cost decimal.Decimal `json:"cost"`
	// RMBCost 调账金额的人民币值
	RMBCost decimal.Decimal `json:"rmb_cost"`
	// Currency 币种
	Currency enumor.CurrencyCode `json:"currency"`
	// ResClass 调账资源类别
	ResClass enumor.BillAdjustmentResClass `json:"res_class"`
	// ResSubClass 调账资源子类
	ResSubClass string `json:"res_sub_class"`
	// PushStatus OBS 推送状态
	PushStatus enumor.BillAdjustmentPushStatus `json:"push_status"`
	// SettleState 定账状态
	SettleState enumor.BillSettleState `json:"settle_state"`
	// Memo 备注
	Memo *string `json:"memo"`
}
