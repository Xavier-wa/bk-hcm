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
	"hcm/pkg/dal/table/utils"
)

// OBSBillItemGcpColumns defines account_bill_summary's columns.
var OBSBillItemGcpColumns = utils.MergeColumns(nil, OBSBillItemGcpColumnDescriptor)

// OBSBillItemGcpColumnDescriptor is AwsBill's column descriptors.
var OBSBillItemGcpColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "main_account_id", NamedC: "main_account_id", Type: enumor.String},
	{Column: "bill_year", NamedC: "bill_year", Type: enumor.Numeric},
	{Column: "bill_month", NamedC: "bill_month", Type: enumor.Numeric},
	{Column: "vendor", NamedC: "vendor", Type: enumor.String},
	{Column: "set_index", NamedC: "set_index", Type: enumor.String},
	{Column: "BillingAccountId", NamedC: "BillingAccountId", Type: enumor.String},
	{Column: "ServiceId", NamedC: "ServiceId", Type: enumor.String},
	{Column: "ServiceDescription", NamedC: "ServiceDescription", Type: enumor.String},
	{Column: "SkuId", NamedC: "SkuId", Type: enumor.String},
	{Column: "SkuDescription", NamedC: "SkuDescription", Type: enumor.String},
	{Column: "UsageStartTime", NamedC: "UsageStartTime", Type: enumor.String},
	{Column: "UsageEndTime", NamedC: "UsageEndTime", Type: enumor.String},
	{Column: "ProjectId", NamedC: "ProjectId", Type: enumor.String},
	{Column: "ProjectName", NamedC: "ProjectName", Type: enumor.String},
	{Column: "Cost", NamedC: "Cost", Type: enumor.Numeric},
	{Column: "Currency", NamedC: "Currency", Type: enumor.String},
	{Column: "CurrencyConversionRate", NamedC: "CurrencyConversionRate", Type: enumor.Numeric},
	{Column: "UsageAmount", NamedC: "UsageAmount", Type: enumor.Numeric},
	{Column: "UsageUnit", NamedC: "UsageUnit", Type: enumor.String},
	{Column: "CreditsAmount", NamedC: "CreditsAmount", Type: enumor.String},
	{Column: "ExportTime", NamedC: "ExportTime", Type: enumor.String},
	{Column: "location", NamedC: "location", Type: enumor.String},
	{Column: "country", NamedC: "country", Type: enumor.String},
	{Column: "region", NamedC: "region", Type: enumor.String},
	{Column: "zone", NamedC: "zone", Type: enumor.String},
	{Column: "ProductId", NamedC: "ProductId", Type: enumor.Numeric},
	{Column: "YearMonth", NamedC: "YearMonth", Type: enumor.Numeric},
	{Column: "FetchTime", NamedC: "FetchTime", Type: enumor.String},
	{Column: "Rate", NamedC: "Rate", Type: enumor.Numeric},
	{Column: "RealCost", NamedC: "RealCost", Type: enumor.Numeric},
	{Column: "ReturnCost", NamedC: "ReturnCost", Type: enumor.Numeric},
	{Column: "DispatchProjectId", NamedC: "DispatchProjectId", Type: enumor.String},
	{Column: "CityId", NamedC: "CityId", Type: enumor.Numeric},
	{Column: "ResClassId", NamedC: "ResClassId", Type: enumor.Numeric},
}

// OBSBillItemGcp huawei bill item
type OBSBillItemGcp struct {
	ID                     string  `db:"id" validate:"lte=64" json:"id"`
	SetIndex               string  `db:"set_index" json:"set_index"`
	MainAccountID          string  `db:"main_account_id" json:"main_account_id"`
	BillYear               int64   `db:"bill_year" json:"bill_year"`
	BillMonth              int64   `db:"bill_month" json:"bill_month"`
	Vendor                 string  `db:"vendor" json:"vendor"`
	BillingAccountId       string  `db:"BillingAccountId" json:"BillingAccountId"`
	ServiceId              string  `db:"ServiceId" json:"ServiceId"`
	ServiceDescription     string  `db:"ServiceDescription" json:"ServiceDescription"`
	SkuId                  string  `db:"SkuId" json:"SkuId"`
	SkuDescription         string  `db:"SkuDescription" json:"SkuDescription"`
	UsageStartTime         string  `db:"UsageStartTime" json:"UsageStartTime"`
	UsageEndTime           string  `db:"UsageEndTime" json:"UsageEndTime"`
	ProjectId              string  `db:"ProjectId" json:"ProjectId"`
	ProjectName            string  `db:"ProjectName" json:"ProjectName"`
	Cost                   float64 `db:"Cost" json:"Cost"`
	Currency               string  `db:"Currency" json:"Currency"`
	CurrencyConversionRate float64 `db:"CurrencyConversionRate" json:"CurrencyConversionRate"`
	UsageAmount            float64 `db:"UsageAmount" json:"UsageAmount"`
	UsageUnit              string  `db:"UsageUnit" json:"UsageUnit"`
	CreditsAmount          string  `db:"CreditsAmount" json:"CreditsAmount"`
	ExportTime             string  `db:"ExportTime" json:"ExportTime"`
	Location               string  `db:"location" json:"location"`
	Country                string  `db:"country" json:"country"`
	Region                 string  `db:"region" json:"region"`
	Zone                   string  `db:"zone" json:"zone"`
	ProductId              int32   `db:"ProductId" json:"ProductId"`
	YearMonth              int32   `db:"YearMonth" json:"YearMonth"`
	FetchTime              string  `db:"FetchTime" json:"FetchTime"`
	Rate                   float64 `db:"Rate" json:"Rate"`
	RealCost               float64 `db:"RealCost"  json:"RealCost"`
	ReturnCost             float64 `db:"ReturnCost" json:"ReturnCost"`
	DispatchProjectId      string  `db:"DispatchProjectId" json:"DispatchProjectId"`
	CityId                 int32   `db:"CityId" json:"CityId"`
	ResClassId             int32   `db:"ResClassId" json:"ResClassId"`
}

// TableName 返回月度汇总账单表名
func (bih *OBSBillItemGcp) TableName() table.Name {
	return table.OBSBillGcpItemTable
}

// InsertValidate validate account bill summary on insert
func (bih *OBSBillItemGcp) InsertValidate() error {
	if len(bih.ID) == 0 {
		return errors.New("id is required")
	}
	if err := validator.Validate.Struct(bih); err != nil {
		return err
	}
	return nil
}
