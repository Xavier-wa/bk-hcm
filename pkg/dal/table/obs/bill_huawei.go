/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
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

// Package obs ...
package obs

import (
	"errors"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// OBSBillItemHuaweiColumns defines account_bill_summary's columns.
var OBSBillItemHuaweiColumns = utils.MergeColumns(nil, OBSBillItemHuaweiColumnDescriptor)

// OBSBillItemHuaweiColumnDescriptor is AwsBill's column descriptors.
var OBSBillItemHuaweiColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "main_account_id", NamedC: "main_account_id", Type: enumor.String},
	{Column: "bill_year", NamedC: "bill_year", Type: enumor.Numeric},
	{Column: "bill_month", NamedC: "bill_month", Type: enumor.Numeric},
	{Column: "vendor", NamedC: "vendor", Type: enumor.String},
	{Column: "set_index", NamedC: "set_index", Type: enumor.String},
	{Column: "effective_time", NamedC: "effective_time", Type: enumor.String},
	{Column: "expire_time", NamedC: "expire_time", Type: enumor.String},
	{Column: "product_id", NamedC: "product_id", Type: enumor.String},
	{Column: "product_name", NamedC: "product_name", Type: enumor.String},
	{Column: "order_id", NamedC: "order_id", Type: enumor.String},
	{Column: "amount", NamedC: "amount", Type: enumor.String},
	{Column: "measure_id", NamedC: "measure_id", Type: enumor.String},
	{Column: "usage_type", NamedC: "usage_type", Type: enumor.String},
	{Column: "usages", NamedC: "usages", Type: enumor.String},
	{Column: "usage_measure_id", NamedC: "usage_measure_id", Type: enumor.String},
	{Column: "free_resource_usage", NamedC: "free_resource_usage", Type: enumor.String},
	{Column: "free_resource_measure_id", NamedC: "free_resource_measure_id", Type: enumor.String},
	{Column: "cloud_service_type", NamedC: "cloud_service_type", Type: enumor.String},
	{Column: "region", NamedC: "region", Type: enumor.String},
	{Column: "resource_type", NamedC: "resource_type", Type: enumor.String},
	{Column: "charge_mode", NamedC: "charge_mode", Type: enumor.String},
	{Column: "resource_tag", NamedC: "resource_tag", Type: enumor.String},
	{Column: "resource_name", NamedC: "resource_name", Type: enumor.String},
	{Column: "resource_id", NamedC: "resource_id", Type: enumor.String},
	{Column: "bill_type", NamedC: "bill_type", Type: enumor.String},
	{Column: "enterprise_project_id", NamedC: "enterprise_project_id", Type: enumor.String},
	{Column: "period_type", NamedC: "period_type", Type: enumor.String},
	{Column: "spot", NamedC: "spot", Type: enumor.String},
	{Column: "ri_usage", NamedC: "ri_usage", Type: enumor.String},
	{Column: "ri_usage_measure_id", NamedC: "ri_usage_measure_id", Type: enumor.String},
	{Column: "official_amount", NamedC: "official_amount", Type: enumor.String},
	{Column: "discount_amount", NamedC: "discount_amount", Type: enumor.String},
	{Column: "cash_amount", NamedC: "cash_amount", Type: enumor.String},
	{Column: "credit_amount", NamedC: "credit_amount", Type: enumor.String},
	{Column: "coupon_amount", NamedC: "coupon_amount", Type: enumor.String},
	{Column: "flexipurchase_coupon_amount", NamedC: "flexipurchase_coupon_amount", Type: enumor.String},
	{Column: "bonus_amount", NamedC: "bonus_amount", Type: enumor.String},
	{Column: "debt_amount", NamedC: "debt_amount", Type: enumor.String},
	{Column: "adjustment_amount", NamedC: "adjustment_amount", Type: enumor.String},
	{Column: "spec_size", NamedC: "spec_size", Type: enumor.String},
	{Column: "spec_size_measure_id", NamedC: "spec_size_measure_id", Type: enumor.String},
	{Column: "account_name", NamedC: "account_name", Type: enumor.String},
	{Column: "productid", NamedC: "productid", Type: enumor.Numeric},
	{Column: "account_type", NamedC: "account_type", Type: enumor.String},
	{Column: "yearMonth", NamedC: "yearMonth", Type: enumor.Numeric},
	{Column: "fetchTime", NamedC: "fetchTime", Type: enumor.String},
	{Column: "total_count", NamedC: "total_count", Type: enumor.Numeric},
	{Column: "rate", NamedC: "rate", Type: enumor.Numeric},
	{Column: "real_cost", NamedC: "real_cost", Type: enumor.Numeric},
	{Column: "CityId", NamedC: "CityId", Type: enumor.Numeric},
	{Column: "ResClassId", NamedC: "ResClassId", Type: enumor.Numeric},
}

// OBSBillItemHuawei huawei bill item
type OBSBillItemHuawei struct {
	ID                        string         `db:"id" validate:"lte=64" json:"id"`
	SetIndex                  string         `db:"set_index" json:"set_index"`
	MainAccountID             string         `db:"main_account_id" json:"main_account_id"`
	BillYear                  int64          `db:"bill_year" json:"bill_year"`
	BillMonth                 int64          `db:"bill_month" json:"bill_month"`
	Vendor                    string         `db:"vendor" json:"vendor"`
	EffectiveTime             string         `db:"effective_time" json:"effective_time"`
	ExpireTime                string         `db:"expire_time" json:"expire_time"`
	ProductID                 string         `db:"product_id" json:"product_id"`
	ProductName               string         `db:"product_name" json:"product_name"`
	OrderID                   string         `db:"order_id" json:"order_id"`
	Amount                    string         `db:"amount" json:"amount"`
	MeasureID                 string         `db:"measure_id" json:"measure_id"`
	UsageType                 string         `db:"usage_type" json:"usage_type"`
	Usages                    string         `db:"usages" json:"usages"`
	UsageMeasureID            string         `db:"usage_measure_id" json:"usage_measure_id"`
	FreeResourceUsage         string         `db:"free_resource_usage" json:"free_resource_usage"`
	FreeResourceMeasureID     string         `db:"free_resource_measure_id" json:"free_resource_measure_id"`
	CloudServiceType          string         `db:"cloud_service_type" json:"cloud_service_type"`
	Region                    string         `db:"region" json:"region"`
	ResourceType              string         `db:"resource_type" json:"resource_type"`
	ChargeMode                string         `db:"charge_mode" json:"charge_mode"`
	ResourceTag               string         `db:"resource_tag" json:"resource_tag"`
	ResourceName              string         `db:"resource_name" json:"resource_name"`
	ResourceID                string         `db:"resource_id" json:"resource_id"`
	BillType                  string         `db:"bill_type" json:"bill_type"`
	EnterpriseProjectID       string         `db:"enterprise_project_id" json:"enterprise_project_id"`
	PeriodType                string         `db:"period_type" json:"period_type"`
	Spot                      string         `db:"spot" json:"spot"`
	RiUsage                   string         `db:"ri_usage" json:"ri_usage"`
	RiUsageMeasureID          string         `db:"ri_usage_measure_id" json:"ri_usage_measure_id"`
	OfficialAmount            string         `db:"official_amount" json:"official_amount"`
	DiscountAmount            string         `db:"discount_amount" json:"discount_amount"`
	CashAmount                string         `db:"cash_amount" json:"cash_amount"`
	CreditAmount              string         `db:"credit_amount" json:"credit_amount"`
	CouponAmount              string         `db:"coupon_amount" json:"coupon_amount"`
	FlexipurchaseCouponAmount string         `db:"flexipurchase_coupon_amount" json:"flexipurchase_coupon_amount"`
	StoredCardAmount          string         `db:"stored_card_amount" json:"stored_card_amount"`
	BonusAmount               string         `db:"bonus_amount" json:"bonus_amount"`
	DebtAmount                string         `db:"debt_amount" json:"debt_amount"`
	AdjustmentAmount          string         `db:"adjustment_amount" json:"adjustment_amount"`
	SpecSize                  string         `db:"spec_size" json:"spec_size"`
	SpecSizeMeasureID         string         `db:"spec_size_measure_id" json:"spec_size_measure_id"`
	AccountName               string         `db:"account_name" json:"account_name"`
	ProductId                 int32          `db:"productid" json:"productid"`
	AccountType               string         `db:"account_type" json:"account_type"`
	YearMonth                 int32          `db:"yearMonth" json:"yearMonth"`
	FetchTime                 string         `db:"fetchTime" json:"fetchTime"`
	TotalCount                int32          `db:"total_count" json:"total_count"`
	Rate                      float64        `db:"rate" json:"rate"`
	RealCost                  *types.Decimal `db:"real_cost"  json:"real_cost"`
	CityId                    int32          `db:"CityId" json:"CityId"`
	ResClassId                int32          `db:"ResClassId" json:"ResClassId"`
}

// TableName 返回月度汇总账单表名
func (bih *OBSBillItemHuawei) TableName() table.Name {
	return table.OBSBillHuaweiItemTable
}

// InsertValidate validate account bill summary on insert
func (bih *OBSBillItemHuawei) InsertValidate() error {
	if len(bih.ID) == 0 {
		return errors.New("id is required")
	}
	if err := validator.Validate.Struct(bih); err != nil {
		return err
	}
	return nil
}
