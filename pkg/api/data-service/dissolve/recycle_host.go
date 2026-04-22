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

// Package dissolve defines dissolve related data-service API protocol.
package dissolve

import (
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/criteria/validator"
	hostdefine "hcm/pkg/dal/table/dissolve/host"
	"hcm/pkg/runtime/filter"
)

// BatchCreateRecycleHostReq batch create recycle host request.
type BatchCreateRecycleHostReq struct {
	Hosts []RecycleHostCreateReq `json:"hosts" validate:"required,max=100"`
}

// Validate BatchCreateRecycleHostReq.
func (c *BatchCreateRecycleHostReq) Validate() error {
	if len(c.Hosts) == 0 || len(c.Hosts) > 100 {
		return errf.Newf(errf.InvalidParameter, "hosts count should between 1 and 100")
	}
	for _, item := range c.Hosts {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// RecycleHostCreateReq single create request.
type RecycleHostCreateReq struct {
	AssetID      string               `json:"asset_id" validate:"required"`
	InnerIP      string               `json:"inner_ip" validate:"required"`
	Module       string               `json:"module" validate:"required"`
	AbolishPhase enumor.AbolishPhase  `json:"abolish_phase" validate:"required"`
	ProjectName  string               `json:"project_name" validate:"required"`
}

// Validate RecycleHostCreateReq.
func (c *RecycleHostCreateReq) Validate() error {
	return validator.Validate.Struct(c)
}

// RecycleHostListReq list recycle host request.
type RecycleHostListReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
	Page   *core.BasePage     `json:"page" validate:"required"`
	Fields []string           `json:"fields" validate:"omitempty"`
}

// Validate RecycleHostListReq.
func (req *RecycleHostListReq) Validate() error {
	return validator.Validate.Struct(req)
}

// RecycleHostListResult list recycle host result.
type RecycleHostListResult = core.ListResultT[hostdefine.RecycleHostTable]

// BatchUpdateRecycleHostReq batch update recycle host request.
type BatchUpdateRecycleHostReq struct {
	Filter *filter.Expression     `json:"filter" validate:"required"`
	Data   *RecycleHostUpdateData `json:"data" validate:"required"`
}

// Validate BatchUpdateRecycleHostReq.
func (c *BatchUpdateRecycleHostReq) Validate() error {
	if c.Filter == nil {
		return errf.New(errf.InvalidParameter, "filter is required")
	}
	if c.Data == nil {
		return errf.New(errf.InvalidParameter, "data is required")
	}
	return validator.Validate.Struct(c)
}

// RecycleHostUpdateData update data fields.
type RecycleHostUpdateData struct {
	AbolishPhase *enumor.AbolishPhase `json:"abolish_phase"`
	ProjectName  *string              `json:"project_name"`
	Module       *string              `json:"module"`
}

// BatchDeleteRecycleHostReq batch delete recycle host request.
type BatchDeleteRecycleHostReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
}

// Validate BatchDeleteRecycleHostReq.
func (d *BatchDeleteRecycleHostReq) Validate() error {
	if d.Filter == nil {
		return errf.New(errf.InvalidParameter, "filter is required")
	}
	return nil
}
