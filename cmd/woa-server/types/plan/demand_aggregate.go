/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
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
	"slices"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/tools/times"

	"github.com/shopspring/decimal"
)

// ListResPlanDemandWithDeviceTypesReq is list resource plan demand with device types request.
type ListResPlanDemandWithDeviceTypesReq struct {
	BkBizID         int64                 `json:"-"`
	ObsProjects     []enumor.ObsProject   `json:"obs_projects" validate:"omitempty,max=100"`
	CoreTypes       []enumor.CoreType     `json:"core_types" validate:"omitempty,max=100"`
	DeviceFamilies  []string              `json:"device_families" validate:"omitempty,max=100"`
	DeviceClasses   []string              `json:"device_classes" validate:"omitempty,max=100"`
	DeviceTypes     []string              `json:"device_types" validate:"omitempty,max=100"`
	RegionIDs       []string              `json:"region_ids" validate:"omitempty,max=100"`
	PlanTypes       []enumor.PlanType     `json:"plan_types" validate:"omitempty,max=100"`
	CpuCores        []int64               `json:"cpu_cores" validate:"omitempty,max=100"`
	Memories        []int64               `json:"memories" validate:"omitempty,max=100"`
	ExpiringOnly    bool                  `json:"expiring_only" validate:"omitempty"`
	ExpectTimeRange *times.DateRange      `json:"expect_time_range" validate:"required"`
	Statuses        []enumor.DemandStatus `json:"statuses" validate:"omitempty,max=5"`
	Page            *core.BasePage        `json:"page" validate:"required"`
}

// Validate whether ListResPlanDemandWithDeviceTypesReq is valid.
func (r ListResPlanDemandWithDeviceTypesReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	for _, projectName := range r.ObsProjects {
		if err := projectName.ValidateResPlan(); err != nil {
			return err
		}
	}

	for _, planType := range r.PlanTypes {
		if err := planType.Validate(); err != nil {
			return err
		}
	}

	if r.ExpectTimeRange != nil {
		if err := r.ExpectTimeRange.Validate(); err != nil {
			return err
		}
	}

	for _, status := range r.Statuses {
		if err := status.Validate(); err != nil {
			return err
		}
	}

	if r.Page != nil {
		if err := r.Page.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// CheckObsProjects check whether obs project contains.
func (r ListResPlanDemandWithDeviceTypesReq) CheckObsProjects(obsProject enumor.ObsProject) bool {
	if len(r.ObsProjects) > 0 {
		return slices.Contains(r.ObsProjects, obsProject)
	}
	return true
}

// CheckCoreTypes check whether core type contains.
func (r ListResPlanDemandWithDeviceTypesReq) CheckCoreTypes(coreType enumor.CoreType) bool {
	if len(r.CoreTypes) > 0 {
		return slices.Contains(r.CoreTypes, coreType)
	}
	return true
}

// CheckDeviceFamilies check whether device family contains.
func (r ListResPlanDemandWithDeviceTypesReq) CheckDeviceFamilies(deviceFamily string) bool {
	if len(r.DeviceFamilies) > 0 {
		return slices.Contains(r.DeviceFamilies, deviceFamily)
	}
	return true
}

// CheckDeviceClasses check whether device class contains.
func (r ListResPlanDemandWithDeviceTypesReq) CheckDeviceClasses(deviceClass string) bool {
	if len(r.DeviceClasses) > 0 {
		return slices.Contains(r.DeviceClasses, deviceClass)
	}
	return true
}

// CheckDeviceTypes check whether device type contains.
func (r ListResPlanDemandWithDeviceTypesReq) CheckDeviceTypes(deviceType string) bool {
	if len(r.DeviceTypes) > 0 {
		return slices.Contains(r.DeviceTypes, deviceType)
	}
	return true
}

// CheckRegionIDs check whether region id contains.
func (r ListResPlanDemandWithDeviceTypesReq) CheckRegionIDs(regionID string) bool {
	if len(r.RegionIDs) > 0 {
		return slices.Contains(r.RegionIDs, regionID)
	}
	return true
}

// CheckPlanTypes check whether plan type contains.
func (r ListResPlanDemandWithDeviceTypesReq) CheckPlanTypes(planType enumor.PlanType) bool {
	if len(r.PlanTypes) > 0 {
		return slices.Contains(r.PlanTypes, planType)
	}
	return true
}

// ListResPlanDemandWithDeviceTypesResp is list resource plan demand with device types response.
type ListResPlanDemandWithDeviceTypesResp struct {
	Count   uint64                                  `json:"count"`
	Details []*ListResPlanDemandWithDeviceTypesItem `json:"details"`
}

// ListResPlanDemandWithDeviceTypesItem is list resource plan demand with device types detail item.
type ListResPlanDemandWithDeviceTypesItem struct {
	ListResPlanDemandItemBase

	// === 预测需求主数据 ===
	DemandIDs      []string             `json:"demand_ids"`
	BkBizID        int64                `json:"bk_biz_id"`
	BkBizName      string               `json:"bk_biz_name"`
	DemandResType  enumor.DemandResType `json:"demand_res_type"`
	RegionID       string               `json:"region_id"`
	RegionName     string               `json:"region_name"`
	ZoneID         string               `json:"zone_id"`
	ZoneName       string               `json:"zone_name"`
	PlanType       enumor.PlanType      `json:"plan_type"`
	ObsProject     enumor.ObsProject    `json:"obs_project"`
	TechnicalClass string               `json:"technical_class"`
	DeviceFamily   string               `json:"device_family"`
	CoreType       enumor.CoreType      `json:"core_type"`
	DiskType       enumor.DiskType      `json:"disk_type"`
	DiskTypeName   string               `json:"disk_type_name"`
	DiskIO         int64                `json:"disk_io"`

	// === 聚合阶段中间计算字段（不返回给前端，展开前会被清除） ===
	OriginalDeviceType string          `json:"-"`
	TotalOS            decimal.Decimal `json:"-"`
	AppliedOS          decimal.Decimal `json:"-"`
	RemainedOS         decimal.Decimal `json:"-"`
	TotalMemory        int64           `json:"-"`
	AppliedMemory      int64           `json:"-"`
	RemainedMemory     int64           `json:"-"`
	TotalDiskSize      int64           `json:"-"`
	RemainedDiskSize   int64           `json:"-"`

	// === 机型明细（展开后的字段） ===
	DeviceType       string          `json:"device_type"`
	IsOriginal       bool            `json:"is_original"`
	DeviceTypeClass  string          `json:"device_type_class"`
	DeviceClass      string          `json:"device_class"`
	CpuCore          int64           `json:"cpu_core"`
	Memory           int64           `json:"memory"`
	DetailTotalOS    decimal.Decimal `json:"total_os"`
	DetailAppliedOS  decimal.Decimal `json:"applied_os"`
	DetailRemainedOS decimal.Decimal `json:"remained_os"`
}
