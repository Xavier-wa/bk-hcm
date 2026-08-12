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

package plan

import (
	"errors"
	"unicode/utf8"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/tools/times"
)

// OverwriteAppendResPlanTicketReq is the biz overwrite-append resource plan ticket request.
// 覆盖：按 overwrite_filter 匹配本地 res_plan_demand 转 cancel 条目；
// 追加：demands 转 add 条目；二者合并为同一主单。
type OverwriteAppendResPlanTicketReq struct {
	// Type is the root ticket type; required. enumeration values such as:
	// budget_declare/add/adjust/delete.
	Type            enumor.RPTicketType      `json:"type" validate:"required"`
	Overwrite       bool                     `json:"overwrite" validate:"omitempty"`
	OverwriteFilter *ResPlanOverwriteFilter  `json:"overwrite_filter" validate:"omitempty"`
	SkipItsm        bool                     `json:"skip_itsm" validate:"omitempty"`
	DemandClass     enumor.DemandClass       `json:"demand_class" validate:"omitempty"`
	Demands         []CreateResPlanDemandReq `json:"demands" validate:"omitempty"`
	Applicant       string                   `json:"applicant" validate:"required"`
	Remark          string                   `json:"remark" validate:"omitempty"`
}

// ResPlanOverwriteFilter is the overwrite filter of resource plan overwrite-append.
type ResPlanOverwriteFilter struct {
	ObsProjects      []enumor.ObsProject `json:"obs_projects" validate:"omitempty"`
	TechnicalClasses []string            `json:"technical_classes" validate:"omitempty"`
	ExpectTimeRange  *times.DateRange    `json:"expect_time_range" validate:"omitempty"`
}

// Validate whether OverwriteAppendResPlanTicketReq is valid.
func (r *OverwriteAppendResPlanTicketReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	if err := r.Type.ValidateRootTicketType(); err != nil {
		return err
	}

	// overwrite=false 且无追加明细，视为参数非法。
	if !r.Overwrite && len(r.Demands) == 0 {
		return errors.New("overwrite is false and demands is empty, at least one is required")
	}

	// overwrite=true 时 overwrite_filter 必填，其中 obs_projects 与 expect_time_range 必填。
	if r.Overwrite {
		if r.OverwriteFilter == nil {
			return errors.New("overwrite_filter is required when overwrite is true")
		}
		if err := r.OverwriteFilter.Validate(); err != nil {
			return err
		}
	}

	// 携带 demands 时 demand_class 必填，并逐条校验明细。
	if len(r.Demands) > 0 {
		if err := r.DemandClass.Validate(); err != nil {
			return err
		}
		for i := range r.Demands {
			if err := r.Demands[i].Validate(); err != nil {
				return err
			}
		}

		lenRemark := utf8.RuneCountInString(r.Remark)
		if lenRemark < 20 || lenRemark > 1024 {
			return errors.New("len remark should be >= 20 and <= 1024 when demands present")
		}
	}

	return nil
}

// Validate whether ResPlanOverwriteFilter is valid.
func (f *ResPlanOverwriteFilter) Validate() error {
	if err := validator.Validate.Struct(f); err != nil {
		return err
	}

	if len(f.ObsProjects) == 0 {
		return errors.New("overwrite_filter.obs_projects is required")
	}
	for _, obsProject := range f.ObsProjects {
		if err := obsProject.ValidateResPlan(); err != nil {
			return err
		}
	}

	if f.ExpectTimeRange == nil {
		return errors.New("overwrite_filter.expect_time_range is required")
	}
	if err := f.ExpectTimeRange.Validate(); err != nil {
		return err
	}

	return nil
}
