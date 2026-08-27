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
	"errors"

	"hcm/pkg/api/core"
	"hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"

	"github.com/shopspring/decimal"
)

// PrepaidItemCreateReq 预付费账单主表创建请求。
type PrepaidItemCreateReq struct {
	UUID               string                 `json:"uuid" validate:"required,max=255"`
	OrderYear          int                    `json:"order_year" validate:"required"`
	OrderMonth         int                    `json:"order_month" validate:"required,min=1,max=12"`
	Vendor             enumor.Vendor          `json:"vendor" validate:"required"`
	RootAccountID      string                 `json:"root_account_id" validate:"required,max=64"`
	MainAccountID      string                 `json:"main_account_id" validate:"required,max=64"`
	RootAccountCloudID string                 `json:"root_account_cloud_id" validate:"omitempty,max=255"`
	MainAccountCloudID string                 `json:"main_account_cloud_id" validate:"omitempty,max=255"`
	ProductID          int64                  `json:"product_id"`
	ResourceID         string                 `json:"resource_id" validate:"omitempty,max=255"`
	InvoiceID          string                 `json:"invoice_id" validate:"omitempty,max=255"`
	GPUType            string                 `json:"gpu_type" validate:"omitempty,max=64"`
	DeviceNum          int                    `json:"device_num"`
	CardNum            int                    `json:"card_num"`
	ProductName        string                 `json:"product_name" validate:"omitempty,max=255"`
	ProductSpec        string                 `json:"product_spec" validate:"omitempty,max=255"`
	Region             string                 `json:"region" validate:"omitempty,max=255"`
	UsageStartAt       string                 `json:"usage_start_at" validate:"omitempty"`
	UsageEndAt         string                 `json:"usage_end_at" validate:"omitempty"`
	OrderAt            string                 `json:"order_at" validate:"required"`
	Currency           enumor.CurrencyCode    `json:"currency" validate:"required"`
	Cost               decimal.Decimal        `json:"cost"`
	RMBCost            decimal.Decimal        `json:"rmb_cost"`
	SettleState        enumor.BillSettleState `json:"settle_state" validate:"required"`
}

// Validate 校验预付费账单主表创建请求。
func (req *PrepaidItemCreateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if err := req.Vendor.Validate(); err != nil {
		return err
	}
	if err := req.Currency.Validate(); err != nil {
		return err
	}

	return req.SettleState.Validate()
}

// PrepaidItemUpdateReq 预付费账单主表更新请求。
type PrepaidItemUpdateReq struct {
	IDs         []string               `json:"ids" validate:"required,min=1"`
	SettleState enumor.BillSettleState `json:"settle_state" validate:"omitempty"`
}

// Validate 校验预付费账单主表更新请求。
func (req *PrepaidItemUpdateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if len(req.SettleState) == 0 {
		return errors.New("nothing to update")
	}

	return req.SettleState.Validate()
}

// PrepaidItemListReq 预付费账单主表查询请求。
type PrepaidItemListReq = core.ListReq

// PrepaidItemListResult 预付费账单主表查询结果。
type PrepaidItemListResult = core.ListResultT[*bill.PrepaidItem]

// PrepaidItemSyncReq 是预付费写入的请求。
type PrepaidItemSyncReq struct {
	// Item 预付费账单主数据
	Item *PrepaidItemCreateReq `json:"item" validate:"required"`
	// AdjustmentItems 本次要重建的 N+1 条调账
	AdjustmentItems []BillAdjustmentItemCreateReq `json:"adjustment_items" validate:"required,min=1,dive"`
}

// Validate ...
func (req *PrepaidItemSyncReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	return req.Item.Validate()
}

// PrepaidItemSyncResult 是单事务编排的结果。
type PrepaidItemSyncResult struct {
	// ID 预付费账单 ID，覆盖重推时与首次写入保持一致
	ID string `json:"id"`
	// Created 本次是否为首次写入，false 表示覆盖重推
	Created bool `json:"created"`
	// AdjustmentIDs 本次新建的调账 ID 列表
	AdjustmentIDs []string `json:"adjustment_ids"`
	// DeletedAdjustmentIDs 本次被物理删除的旧调账 ID 列表，首次写入为空
	DeletedAdjustmentIDs []string `json:"deleted_adjustment_ids"`
}
