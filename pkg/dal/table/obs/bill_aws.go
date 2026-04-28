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

// OBSBillItemAwsColumns defines account_bill_summary's columns.
var OBSBillItemAwsColumns = utils.MergeColumns(nil, OBSBillItemAwsColumnDescriptor)

// OBSBillItemAwsColumnDescriptor is AwsBill's column descriptors.
var OBSBillItemAwsColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "main_account_id", NamedC: "main_account_id", Type: enumor.String},
	{Column: "bill_year", NamedC: "bill_year", Type: enumor.Numeric},
	{Column: "bill_month", NamedC: "bill_month", Type: enumor.Numeric},
	{Column: "vendor", NamedC: "vendor", Type: enumor.String},
	{Column: "set_index", NamedC: "set_index", Type: enumor.String},
	{Column: "yearMonth", NamedC: "yearMonth", Type: enumor.Numeric},
	{Column: "bill_payer_account_id", NamedC: "bill_payer_account_id", Type: enumor.String},
	{Column: "line_item_usage_account_id", NamedC: "line_item_usage_account_id", Type: enumor.String},
	{Column: "bill_invoice_id", NamedC: "bill_invoice_id", Type: enumor.String},
	{Column: "bill_billing_entity", NamedC: "bill_billing_entity", Type: enumor.String},
	{Column: "line_item_product_code", NamedC: "line_item_product_code", Type: enumor.String},
	{Column: "product_product_family", NamedC: "product_product_family", Type: enumor.String},
	{Column: "product_product_name", NamedC: "product_product_name", Type: enumor.String},
	{Column: "line_item_usage_type", NamedC: "line_item_usage_type", Type: enumor.String},
	{Column: "product_instance_type", NamedC: "product_instance_type", Type: enumor.String},
	{Column: "product_region", NamedC: "product_region", Type: enumor.String},
	{Column: "product_location", NamedC: "product_location", Type: enumor.String},
	{Column: "line_item_resource_id", NamedC: "line_item_resource_id", Type: enumor.String},
	{Column: "pricing_term", NamedC: "pricing_term", Type: enumor.String},
	{Column: "line_item_line_item_type", NamedC: "line_item_line_item_type", Type: enumor.String},
	{Column: "line_item_line_item_description", NamedC: "line_item_line_item_description", Type: enumor.String},
	{Column: "line_item_usage_start_date", NamedC: "line_item_usage_start_date", Type: enumor.String},
	{Column: "line_item_usage_end_date", NamedC: "line_item_usage_end_date", Type: enumor.String},
	{Column: "line_item_usage_amount", NamedC: "line_item_usage_amount", Type: enumor.String},
	{Column: "pricing_unit", NamedC: "pricing_unit", Type: enumor.String},
	{Column: "pricing_public_on_demand_rate", NamedC: "pricing_public_on_demand_rate", Type: enumor.String},
	{Column: "line_item_unblended_rate", NamedC: "line_item_unblended_rate", Type: enumor.String},
	{Column: "line_item_net_unblended_rate", NamedC: "line_item_net_unblended_rate", Type: enumor.String},
	{Column: "savings_plan_savings_plan_rate", NamedC: "savings_plan_savings_plan_rate", Type: enumor.String},
	{Column: "pricing_public_on_demand_cost", NamedC: "pricing_public_on_demand_cost", Type: enumor.String},
	{Column: "line_item_unblended_cost", NamedC: "line_item_unblended_cost", Type: enumor.String},
	{Column: "line_item_net_unblended_cost", NamedC: "line_item_net_unblended_cost", Type: enumor.String},
	{Column: "savings_plan_savings_plan_effective_cost", NamedC: "savings_plan_savings_plan_effective_cost",
		Type: enumor.String},
	{Column: "savings_plan_savings_plan_net_effective_cost", NamedC: "savings_plan_savings_plan_net_effective_cost",
		Type: enumor.String},
	{Column: "reservation_effective_cost", NamedC: "reservation_effective_cost", Type: enumor.String},
	{Column: "reservation_net_effective_cost", NamedC: "reservation_net_effective_cost", Type: enumor.String},
	{Column: "line_item_currency_code", NamedC: "line_item_currency_code", Type: enumor.String},
	{Column: "line_item_operation", NamedC: "line_item_operation", Type: enumor.String},
	{Column: "rate", NamedC: "rate", Type: enumor.Numeric},
	{Column: "cost", NamedC: "cost", Type: enumor.Numeric},
	{Column: "productid", NamedC: "productid", Type: enumor.Numeric},
	{Column: "linked_accountname", NamedC: "linked_accountname", Type: enumor.String},
	{Column: "region", NamedC: "region", Type: enumor.Numeric},
	{Column: "discount_private_rate_discount", NamedC: "discount_private_rate_discount", Type: enumor.String},
	{Column: "discount_edp_discount", NamedC: "discount_edp_discount", Type: enumor.String},
	{Column: "memo", NamedC: "memo", Type: enumor.String},
	{Column: "CityId", NamedC: "CityId", Type: enumor.Numeric},
	{Column: "ResClassId", NamedC: "ResClassId", Type: enumor.Numeric},
}

// OBSBillItemAws aws bill item
type OBSBillItemAws struct {
	ID            string `db:"id" validate:"lte=64" json:"id"`
	SetIndex      string `db:"set_index" json:"set_index"`
	MainAccountID string `db:"main_account_id" json:"main_account_id"`
	BillYear      int64  `db:"bill_year" json:"bill_year"`
	BillMonth     int64  `db:"bill_month" json:"bill_month"`
	Vendor        string `db:"vendor" json:"vendor"`

	YearMonth                              int32          `db:"yearMonth" json:"yearMonth"`
	BillPayerAccountID                     string         `json:"bill_payer_account_id" db:"bill_payer_account_id"`
	LineItemUsageAccountID                 string         `json:"line_item_usage_account_id" db:"line_item_usage_account_id"`
	BillInvoiceID                          string         `json:"bill_invoice_id" db:"bill_invoice_id"`
	BillBillingEntity                      string         `json:"bill_billing_entity" db:"bill_billing_entity"`
	LineItemProductCode                    string         `json:"line_item_product_code" db:"line_item_product_code"`
	ProductProductFamily                   string         `json:"product_product_family" db:"product_product_family"`
	ProductProductName                     string         `json:"product_product_name" db:"product_product_name"`
	LineItemUsageType                      string         `json:"line_item_usage_type" db:"line_item_usage_type"`
	ProductInstanceType                    string         `json:"product_instance_type" db:"product_instance_type"`
	ProductRegion                          string         `json:"product_region" db:"product_region"`
	ProductLocation                        string         `json:"product_location" db:"product_location"`
	LineItemResourceID                     string         `json:"line_item_resource_id" db:"line_item_resource_id"`
	PricingTerm                            string         `json:"pricing_term" db:"pricing_term"`
	LineItemLineItemType                   string         `json:"line_item_line_item_type" db:"line_item_line_item_type"`
	LineItemLineItemDescription            string         `json:"line_item_line_item_description" db:"line_item_line_item_description"`
	LineItemUsageStartDate                 string         `json:"line_item_usage_start_date" db:"line_item_usage_start_date"`
	LineItemUsageEndDate                   string         `json:"line_item_usage_end_date" db:"line_item_usage_end_date"`
	LineItemUsageAmount                    string         `json:"line_item_usage_amount" db:"line_item_usage_amount"`
	PricingUnit                            string         `json:"pricing_unit" db:"pricing_unit"`
	PricingPublicOnDemandRate              string         `json:"pricing_public_on_demand_rate" db:"pricing_public_on_demand_rate"`
	LineItemUnblendedRate                  string         `json:"line_item_unblended_rate" db:"line_item_unblended_rate"`
	LineItemNetUnblendedRate               string         `json:"line_item_net_unblended_rate" db:"line_item_net_unblended_rate"`
	SavingsPlanSavingsPlanRate             string         `json:"savings_plan_savings_plan_rate" db:"savings_plan_savings_plan_rate"`
	PricingPublicOnDemandCost              string         `json:"pricing_public_on_demand_cost" db:"pricing_public_on_demand_cost"`
	LineItemUnblendedCost                  string         `json:"line_item_unblended_cost" db:"line_item_unblended_cost"`
	LineItemNetUnblendedCost               string         `json:"line_item_net_unblended_cost" db:"line_item_net_unblended_cost"`
	SavingsPlanSavingsPlanEffectiveCost    string         `json:"savings_plan_savings_plan_effective_cost" db:"savings_plan_savings_plan_effective_cost"`
	SavingsPlanSavingsPlanNetEffectiveCost string         `json:"savings_plan_savings_plan_net_effective_cost" db:"savings_plan_savings_plan_net_effective_cost"`
	ReservationEffectiveCost               string         `json:"reservation_effective_cost" db:"reservation_effective_cost"`
	ReservationNetEffectiveCost            string         `json:"reservation_net_effective_cost" db:"reservation_net_effective_cost"`
	LineItemCurrencyCode                   string         `json:"line_item_currency_code" db:"line_item_currency_code"`
	LineItemOperation                      string         `json:"line_item_operation" db:"line_item_operation"`
	Rate                                   float64        `json:"rate" db:"rate"`
	Cost                                   *types.Decimal `json:"cost" db:"cost"`
	ProductID                              int32          `json:"productid" db:"productid"`
	LinkedAccountName                      string         `json:"linked_accountname" db:"linked_accountname"`
	Region                                 int32          `json:"region" db:"region"`
	DiscountPrivateRateDiscount            string         `json:"discount_private_rate_discount" db:"discount_private_rate_discount"`
	DiscountEDPDiscount                    string         `json:"discount_edp_discount" db:"discount_edp_discount"`
	Memo                                   string         `json:"memo" db:"memo"`
	CityId                                 int32          `json:"CityId" db:"CityId"`
	ResClassId                             int32          `json:"ResClassId" db:"ResClassId"`
}

// TableName 返回月度汇总账单表名
func (bih *OBSBillItemAws) TableName() table.Name {
	return table.OBSBillAwsItemTable
}

// InsertValidate validate account bill summary on insert
func (bih *OBSBillItemAws) InsertValidate() error {
	if len(bih.ID) == 0 {
		return errors.New("id is required")
	}
	if err := validator.Validate.Struct(bih); err != nil {
		return err
	}
	return nil
}
