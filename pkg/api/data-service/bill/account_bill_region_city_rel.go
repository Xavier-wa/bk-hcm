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
	"hcm/pkg/api/core"
	corebill "hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
)

// BatchBillRegionCityRelCreateReq batch create request for account bill region city rel.
type BatchBillRegionCityRelCreateReq struct {
	Items []BillRegionCityRelCreateReq `json:"items" validate:"required,min=1,max=100,dive,required"`
}

// Validate validates the request.
func (r *BatchBillRegionCityRelCreateReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	for _, item := range r.Items {
		if err := item.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// BillRegionCityRelCreateReq create request for account bill region city rel.
type BillRegionCityRelCreateReq struct {
	Region string        `json:"region" validate:"required,max=128"`
	Vendor enumor.Vendor `json:"vendor" validate:"required"`
	CityID int32         `json:"city_id" validate:"required"`
}

// Validate validates the request.
func (r *BillRegionCityRelCreateReq) Validate() error {
	if err := r.Vendor.Validate(); err != nil {
		return err
	}
	return validator.Validate.Struct(r)
}

// BillRegionCityRelListReq list request for account bill region city rel.
type BillRegionCityRelListReq = core.ListReq

// BillRegionCityRelListResult list result for account bill region city rel.
type BillRegionCityRelListResult = core.ListResultT[*corebill.AccountBillRegionCityRel]

// BillRegionCityRelUpdateReq update request for account bill region city rel.
type BillRegionCityRelUpdateReq struct {
	ID     string        `json:"id" validate:"required"`
	Region string        `json:"region" validate:"omitempty,max=128"`
	Vendor enumor.Vendor `json:"vendor" validate:"omitempty"`
	CityID *int32        `json:"city_id" validate:"omitempty"`
}

// Validate validates the request.
func (r *BillRegionCityRelUpdateReq) Validate() error {
	if r.Vendor != "" {
		if err := r.Vendor.Validate(); err != nil {
			return err
		}
	}

	return validator.Validate.Struct(r)
}
