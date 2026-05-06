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

package bill

import (
	"errors"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// AccountBillRegionCityRelColumns defines account_bill_region_city_rel's columns.
var AccountBillRegionCityRelColumns = utils.MergeColumns(nil, AccountBillRegionCityRelColumnDescriptor)

// AccountBillRegionCityRelColumnDescriptor is AccountBillRegionCityRel's column descriptors.
var AccountBillRegionCityRelColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "region", NamedC: "region", Type: enumor.String},
	{Column: "vendor", NamedC: "vendor", Type: enumor.String},
	{Column: "city_id", NamedC: "city_id", Type: enumor.Numeric},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// AccountBillRegionCityRel defines the account_bill_region_city_rel table structure.
type AccountBillRegionCityRel struct {
	// ID auto-generated ID
	ID string `db:"id" validate:"lte=64" json:"id"`
	// Region is the cloud vendor region identifier
	Region string `db:"region" validate:"lte=128" json:"region"`
	// Vendor is the cloud vendor
	Vendor enumor.Vendor `db:"vendor" json:"vendor"`
	// CityID is the city ID mapped from the region
	CityID int32 `db:"city_id" json:"city_id"`
	// Creator is the creator of the record
	Creator string `db:"creator" validate:"max=64" json:"creator"`
	// Reviser is the last updater of the record
	Reviser string `db:"reviser" validate:"max=64" json:"reviser"`
	// CreatedAt is the creation time
	CreatedAt types.Time `db:"created_at" json:"created_at"`
	// UpdatedAt is the last update time
	UpdatedAt types.Time `db:"updated_at" json:"updated_at"`
}

// TableName returns the table name.
func (r *AccountBillRegionCityRel) TableName() table.Name {
	return table.AccountBillRegionCityRelTable
}

// InsertValidate validates the record on insert.
func (r *AccountBillRegionCityRel) InsertValidate() error {
	if len(r.ID) == 0 {
		return errors.New("id is required")
	}
	if len(r.Region) == 0 {
		return errors.New("region is required")
	}

	if err := r.Vendor.Validate(); err != nil {
		return err
	}

	return validator.Validate.Struct(r)
}

// UpdateValidate validates the record on update.
func (r *AccountBillRegionCityRel) UpdateValidate() error {
	if len(r.ID) == 0 {
		return errors.New("id is required")
	}
	return validator.Validate.Struct(r)
}
