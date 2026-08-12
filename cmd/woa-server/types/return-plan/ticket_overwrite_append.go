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

package returnplan

import (
	"errors"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/tools/times"
)

// OverwriteAppendReturnPlanTicketReq 覆盖追加退回计划请求。
// overwrite=true 时按 overwrite_filter 覆盖（删除）CRP 已有退回计划；
// return_details 非空时追加新的退回计划明细；二者可同时进行。
type OverwriteAppendReturnPlanTicketReq struct {
	// Overwrite 覆盖开关，为 true 时按 OverwriteFilter 删除已有退回计划。
	Overwrite bool `json:"overwrite" validate:"omitempty"`
	// OverwriteFilter 覆盖筛选条件，Overwrite 为 true 时必填。
	OverwriteFilter *ReturnPlanOverwriteFilter `json:"overwrite_filter" validate:"omitempty"`
	// ReturnDetails 追加的退回计划明细列表，仅覆盖不新增时可为空。
	ReturnDetails []AppendReturnPlanDetail `json:"return_details" validate:"omitempty,dive"`
	// Applicant 提单人，作为 CRP 提单人，用于覆盖当前调用账号（如 admin）。
	Applicant string `json:"applicant" validate:"required"`
	// Remark 说明。
	Remark string `json:"remark" validate:"omitempty"`
}

// ReturnPlanOverwriteFilter 退回计划覆盖筛选条件。
type ReturnPlanOverwriteFilter struct {
	// ObsProjects OBS 项目类型列表。
	ObsProjects []enumor.ObsProject `json:"obs_projects" validate:"omitempty"`
	// TechnicalClasses 技术分类列表，不传时筛选全部。
	TechnicalClasses []string `json:"technical_classes" validate:"omitempty"`
	// PlanTimeRange 计划退回时间范围，按预计退回时间筛选待删除退回计划，格式 YYYY-MM-DD。
	PlanTimeRange *times.DateRange `json:"plan_time_range" validate:"omitempty"`
}

// AppendReturnPlanDetail 追加的退回计划明细。
// InstanceModel+CvmAmount 与 InstanceType+CoreTypeName+CoreAmount 二选一。
type AppendReturnPlanDetail struct {
	// ObsProject OBS 项目类型。
	ObsProject enumor.ObsProject `json:"obs_project" validate:"required"`
	// PlanTime 计划退回时间，格式 YYYY-MM-DD。CRP 要求不早于当前时间 + 35 天。
	PlanTime string `json:"plan_time" validate:"required"`
	// ResourcePoolName 资源池（自研池/公有池），为空时默认自研池。
	ResourcePoolName string `json:"resource_pool_name" validate:"omitempty"`
	// RegionID 地区/城市 ID。
	RegionID string `json:"region_id" validate:"required"`
	// ZoneID 可用区 ID。
	ZoneID string `json:"zone_id" validate:"omitempty"`
	// InstanceModel 实例规格，与 InstanceType 二选一，与 CvmAmount 搭配使用。
	InstanceModel string `json:"instance_model" validate:"omitempty"`
	// CvmAmount 退回实例数，传 InstanceModel 时必填。
	CvmAmount int64 `json:"cvm_amount" validate:"omitempty"`
	// InstanceType 实例类型，与 InstanceModel 二选一，与 CoreTypeName、CoreAmount 搭配使用。
	InstanceType string `json:"instance_type" validate:"omitempty"`
	// CoreTypeName 核心类型，传 InstanceType 时必填。
	CoreTypeName string `json:"core_type_name" validate:"omitempty"`
	// CoreAmount 退回核心数，传 InstanceType 时必填。
	CoreAmount int64 `json:"core_amount" validate:"omitempty"`
	// ReturnReasonClass 退回原因大类，为空时使用默认值「成本优化&利用率提升」。
	ReturnReasonClass string `json:"return_reason_class" validate:"omitempty"`
	// Desc 备注。
	Desc string `json:"desc" validate:"omitempty"`
}

// Validate validates OverwriteAppendReturnPlanTicketReq.
func (r *OverwriteAppendReturnPlanTicketReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	// overwrite=false 且无追加明细，视为参数非法。
	if !r.Overwrite && len(r.ReturnDetails) == 0 {
		return errors.New("overwrite is false and return_details is empty")
	}

	if r.Overwrite {
		if r.OverwriteFilter == nil {
			return errors.New("overwrite_filter is required when overwrite is true")
		}
		if err := r.OverwriteFilter.Validate(); err != nil {
			return err
		}
	}

	for i := range r.ReturnDetails {
		if err := r.ReturnDetails[i].Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validate validates ReturnPlanOverwriteFilter.
func (f *ReturnPlanOverwriteFilter) Validate() error {
	if len(f.ObsProjects) == 0 {
		return errors.New("obs_projects is required in overwrite_filter")
	}
	for _, obsProject := range f.ObsProjects {
		if err := obsProject.Validate(); err != nil {
			return err
		}
	}

	if f.PlanTimeRange == nil {
		return errors.New("plan_time_range is required in overwrite_filter")
	}
	return f.PlanTimeRange.Validate()
}

// Validate validates AppendReturnPlanDetail.
func (d *AppendReturnPlanDetail) Validate() error {
	if err := d.ObsProject.Validate(); err != nil {
		return err
	}

	// InstanceModel+CvmAmount 与 InstanceType+CoreTypeName+CoreAmount 二选一。
	byInstanceModel := d.InstanceModel != ""
	byInstanceType := d.InstanceType != ""
	if byInstanceModel == byInstanceType {
		return errors.New("instance_model and instance_type must be set exactly one")
	}

	if byInstanceModel {
		if d.CvmAmount <= 0 {
			return errors.New("cvm_amount should be > 0 when instance_model is set")
		}
	} else {
		if d.CoreTypeName == "" {
			return errors.New("core_type_name is required when instance_type is set")
		}
		if d.CoreAmount <= 0 {
			return errors.New("core_amount should be > 0 when instance_type is set")
		}
	}

	return nil
}

// OverwriteAppendReturnPlanTicketResp 覆盖追加退回计划响应。
type OverwriteAppendReturnPlanTicketResp struct {
	// ID 退回计划主单 ID。
	ID string `json:"id"`
}
