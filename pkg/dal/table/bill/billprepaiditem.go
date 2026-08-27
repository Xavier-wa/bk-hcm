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

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
	cvt "hcm/pkg/tools/converter"
)

// AccountBillPrepaidItemColumns defines account_bill_prepaid_item's columns.
var AccountBillPrepaidItemColumns = utils.MergeColumns(nil, AccountBillPrepaidItemColumnDescriptor)

// AccountBillPrepaidItemColumnDescriptor is account_bill_prepaid_item's column descriptors.
var AccountBillPrepaidItemColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "uuid", NamedC: "uuid", Type: enumor.String},
	{Column: "order_year", NamedC: "order_year", Type: enumor.Numeric},
	{Column: "order_month", NamedC: "order_month", Type: enumor.Numeric},
	{Column: "vendor", NamedC: "vendor", Type: enumor.String},
	{Column: "root_account_id", NamedC: "root_account_id", Type: enumor.String},
	{Column: "main_account_id", NamedC: "main_account_id", Type: enumor.String},
	{Column: "root_account_cloud_id", NamedC: "root_account_cloud_id", Type: enumor.String},
	{Column: "main_account_cloud_id", NamedC: "main_account_cloud_id", Type: enumor.String},
	{Column: "product_id", NamedC: "product_id", Type: enumor.Numeric},
	{Column: "resource_id", NamedC: "resource_id", Type: enumor.String},
	{Column: "invoice_id", NamedC: "invoice_id", Type: enumor.String},
	{Column: "gpu_type", NamedC: "gpu_type", Type: enumor.String},
	{Column: "device_num", NamedC: "device_num", Type: enumor.Numeric},
	{Column: "card_num", NamedC: "card_num", Type: enumor.Numeric},
	{Column: "product_name", NamedC: "product_name", Type: enumor.String},
	{Column: "product_spec", NamedC: "product_spec", Type: enumor.String},
	{Column: "region", NamedC: "region", Type: enumor.String},
	{Column: "usage_start_at", NamedC: "usage_start_at", Type: enumor.String},
	{Column: "usage_end_at", NamedC: "usage_end_at", Type: enumor.String},
	{Column: "order_at", NamedC: "order_at", Type: enumor.String},
	{Column: "currency", NamedC: "currency", Type: enumor.String},
	{Column: "cost", NamedC: "cost", Type: enumor.Numeric},
	{Column: "rmb_cost", NamedC: "rmb_cost", Type: enumor.Numeric},
	{Column: "settle_state", NamedC: "settle_state", Type: enumor.String},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// AccountBillPrepaidItem 预付费账单主数据。
type AccountBillPrepaidItem struct {
	// ID 预付费账单 ID
	ID string `db:"id" validate:"lte=64" json:"id"`
	// UUID 上游系统外部唯一标识，与订单月份共同构成业务唯一键
	UUID string `db:"uuid" validate:"lte=255" json:"uuid"`
	// OrderYear 订单年份
	OrderYear int `db:"order_year" json:"order_year"`
	// OrderMonth 订单月份
	OrderMonth int `db:"order_month" json:"order_month"`
	// Vendor 云厂商
	Vendor enumor.Vendor `db:"vendor" json:"vendor"`
	// RootAccountID 一级账号 ID
	RootAccountID string `db:"root_account_id" validate:"lte=64" json:"root_account_id"`
	// MainAccountID 二级账号 ID
	MainAccountID string `db:"main_account_id" validate:"lte=64" json:"main_account_id"`
	// RootAccountCloudID 一级账号云上 ID
	RootAccountCloudID string `db:"root_account_cloud_id" validate:"lte=255" json:"root_account_cloud_id"`
	// MainAccountCloudID 二级账号云上 ID
	MainAccountCloudID string `db:"main_account_cloud_id" validate:"lte=255" json:"main_account_cloud_id"`
	// ProductID 运营产品 ID，取二级账号的 op_product_id
	ProductID int64 `db:"product_id" json:"product_id"`
	// ResourceID 资源 ID
	ResourceID string `db:"resource_id" validate:"lte=255" json:"resource_id"`
	// InvoiceID 发票 ID
	InvoiceID string `db:"invoice_id" validate:"lte=255" json:"invoice_id"`
	// GPUType GPU 型号
	GPUType string `db:"gpu_type" validate:"lte=64" json:"gpu_type"`
	// DeviceNum 数量（台）
	DeviceNum int `db:"device_num" json:"device_num"`
	// CardNum 数量（卡）
	CardNum int `db:"card_num" json:"card_num"`
	// ProductName 产品名称
	ProductName string `db:"product_name" validate:"lte=255" json:"product_name"`
	// ProductSpec 产品规格
	ProductSpec string `db:"product_spec" validate:"lte=255" json:"product_spec"`
	// Region 地域
	Region string `db:"region" validate:"lte=255" json:"region"`
	// UsageStartAt 使用开始时间，格式 constant.DateTimeLayout。
	UsageStartAt *string `db:"usage_start_at" validate:"omitempty,max=64" json:"usage_start_at"`
	// UsageEndAt 使用结束时间，格式 constant.DateTimeLayout
	UsageEndAt *string `db:"usage_end_at" validate:"omitempty,max=64" json:"usage_end_at"`
	// OrderAt 订单时间，格式 constant.DateTimeLayout
	OrderAt string `db:"order_at" validate:"omitempty,max=64" json:"order_at"`
	// Currency 币种
	Currency enumor.CurrencyCode `db:"currency" json:"currency"`
	// Cost 优惠后总价
	Cost *types.Decimal `db:"cost" json:"cost"`
	// RMBCost 优惠后总价的人民币金额
	RMBCost *types.Decimal `db:"rmb_cost" json:"rmb_cost"`
	// SettleState 定账状态
	SettleState enumor.BillSettleState `db:"settle_state" json:"settle_state"`
	// Creator 创建者
	Creator string `db:"creator" validate:"max=64" json:"creator"`
	// Reviser 更新者
	Reviser string `db:"reviser" validate:"max=64" json:"reviser"`
	// CreatedAt 创建时间
	CreatedAt types.Time `db:"created_at" validate:"isdefault" json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt types.Time `db:"updated_at" validate:"isdefault" json:"updated_at"`
}

// TableName 返回预付费账单主表名
func (a *AccountBillPrepaidItem) TableName() table.Name {
	return table.AccountBillPrepaidItemTable
}

// InsertValidate validate account bill prepaid item on insert
func (a *AccountBillPrepaidItem) InsertValidate() error {
	if len(a.UUID) == 0 {
		return errors.New("uuid is required")
	}
	if a.OrderYear == 0 {
		return errors.New("order_year is required")
	}
	if a.OrderMonth < 1 || a.OrderMonth > 12 {
		return errors.New("order_month must between 1 and 12")
	}
	if len(a.Vendor) == 0 {
		return errors.New("vendor is required")
	}
	if len(a.RootAccountID) == 0 {
		return errors.New("root_account_id is required")
	}
	if len(a.MainAccountID) == 0 {
		return errors.New("main_account_id is required")
	}
	if len(a.Currency) == 0 {
		return errors.New("currency is required")
	}
	if cvt.PtrToVal(a.Cost).IsZero() {
		return errors.New("cost is required")
	}
	if len(a.SettleState) == 0 {
		return errors.New("settle_state is required")
	}
	if len(a.OrderAt) == 0 {
		return errors.New("order_at is required")
	}

	return validator.Validate.Struct(a)
}

// UpdateValidate validate account bill prepaid item on update
func (a *AccountBillPrepaidItem) UpdateValidate() error {
	if len(a.SettleState) != 0 {
		if err := a.SettleState.Validate(); err != nil {
			return err
		}
	}

	return validator.Validate.Struct(a)
}
