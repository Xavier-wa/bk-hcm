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
	"hcm/pkg/criteria/enumor"

	"github.com/shopspring/decimal"
)

// PrepaidItem 预付费账单主数据。
type PrepaidItem struct {
	ID                 string                 `json:"id"`
	UUID               string                 `json:"uuid"`
	OrderYear          int                    `json:"order_year"`
	OrderMonth         int                    `json:"order_month"`
	Vendor             enumor.Vendor          `json:"vendor"`
	RootAccountID      string                 `json:"root_account_id"`
	MainAccountID      string                 `json:"main_account_id"`
	RootAccountCloudID string                 `json:"root_account_cloud_id"`
	MainAccountCloudID string                 `json:"main_account_cloud_id"`
	ProductID          int64                  `json:"product_id"`
	ResourceID         string                 `json:"resource_id"`
	InvoiceID          string                 `json:"invoice_id"`
	GPUType            string                 `json:"gpu_type"`
	DeviceNum          int                    `json:"device_num"`
	CardNum            int                    `json:"card_num"`
	ProductName        string                 `json:"product_name"`
	ProductSpec        string                 `json:"product_spec"`
	Region             string                 `json:"region"`
	UsageStartAt       string                 `json:"usage_start_at"`
	UsageEndAt         string                 `json:"usage_end_at"`
	OrderAt            string                 `json:"order_at"`
	Currency           enumor.CurrencyCode    `json:"currency"`
	Cost               decimal.Decimal        `json:"cost"`
	RMBCost            decimal.Decimal        `json:"rmb_cost"`
	SettleState        enumor.BillSettleState `json:"settle_state"`
	Creator            string                 `json:"creator"`
	Reviser            string                 `json:"reviser"`
	CreatedAt          string                 `json:"created_at"`
	UpdatedAt          string                 `json:"updated_at"`
}

// PrepaidAccountingState 预付费账单的核算状态，按条目数口径由 push_status 实时派生，不落库。
type PrepaidAccountingState string

const (
	// PrepaidAccountingStatePending 待核算
	PrepaidAccountingStatePending PrepaidAccountingState = "pending"
	// PrepaidAccountingStateAccounting 核算中
	PrepaidAccountingStateAccounting PrepaidAccountingState = "accounting"
	// PrepaidAccountingStateAccounted 已完成
	PrepaidAccountingStateAccounted PrepaidAccountingState = "accounted"
)

// DerivePrepaidAccountingState 按条目数口径派生核算状态。
// pushedNum 为该单下 push_status=pushed 且 type=increase 的条目数，
// increaseNum 为该单下 type=increase 的条目总数。
func DerivePrepaidAccountingState(pushedNum, increaseNum int) PrepaidAccountingState {
	if pushedNum <= 0 {
		return PrepaidAccountingStatePending
	}
	if pushedNum < increaseNum {
		return PrepaidAccountingStateAccounting
	}
	return PrepaidAccountingStateAccounted
}
