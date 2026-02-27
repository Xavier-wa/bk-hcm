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

// Package cvmapply ...
package cvmapply

import (
	tasktypes "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/criteria/validator"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
)

// BatchCreateZiyanCvmDeviceInfoReq batch create request
type BatchCreateZiyanCvmDeviceInfoReq struct {
	Devices []ZiyanCvmDeviceInfoCreateReq `json:"devices" validate:"required,max=100"`
}

// Validate ...
func (c *BatchCreateZiyanCvmDeviceInfoReq) Validate() error {
	if len(c.Devices) == 0 || len(c.Devices) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"devices count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.Devices {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmDeviceInfoCreateReq create request
type ZiyanCvmDeviceInfoCreateReq struct {
	OrderID          int64                  `json:"order_id" validate:"required"`
	SuborderID       string                 `json:"suborder_id" validate:"required,max=64"`
	GenerateID       string                 `json:"generate_id" validate:"omitempty"`
	BkBizID          int64                  `json:"bk_biz_id" validate:"required"`
	BkUsername       string                 `json:"bk_username" validate:"omitempty,max=64"`
	BkHostID         int64                  `json:"bk_host_id" validate:"omitempty"`
	IP               string                 `json:"ip" validate:"omitempty,max=64"`
	AssetID          string                 `json:"asset_id" validate:"omitempty,max=64"`
	InstanceID       string                 `json:"instance_id" validate:"max=64"`
	RequireType      enumor.RequireType     `json:"require_type" validate:"omitempty"`
	ResourceType     tasktypes.ResourceType `json:"resource_type" validate:"max=32"`
	DeviceType       string                 `json:"device_type" validate:"max=64"`
	Description      string                 `json:"description" validate:"omitempty"`
	Remark           string                 `json:"remark" validate:"omitempty"`
	ZoneName         string                 `json:"zone_name" validate:"max=128"`
	ZoneID           int64                  `json:"zone_id" validate:"omitempty"`
	CloudZone        string                 `json:"cloud_zone" validate:"omitempty"`
	CloudRegion      string                 `json:"cloud_region" validate:"omitempty"`
	ModuleName       string                 `json:"module_name" validate:"max=128"`
	RackID           string                 `json:"rack_id" validate:"max=64"`
	IsMatched        bool                   `json:"is_matched" validate:"omitempty"`
	IsChecked        bool                   `json:"is_checked" validate:"omitempty"`
	IsInited         bool                   `json:"is_inited" validate:"omitempty"`
	IsDelivered      bool                   `json:"is_delivered" validate:"omitempty"`
	Deliverer        string                 `json:"deliverer" validate:"max=64"`
	GenerateTaskID   string                 `json:"generate_task_id" validate:"max=128"`
	GenerateTaskLink string                 `json:"generate_task_link" validate:"max=512"`
	InitTaskID       string                 `json:"init_task_id" validate:"max=128"`
	InitTaskLink     string                 `json:"init_task_link" validate:"max=512"`
	IsManualMatched  bool                   `json:"is_manual_matched" validate:"omitempty"`
	OwnerIP          string                 `json:"owner_ip" validate:"omitempty"`
	// CreatedAt 原始创建时间（用于数据迁移，保留历史时间）
	CreatedAt types.Time `json:"created_at" validate:"omitempty"`
	UpdatedAt types.Time `json:"updated_at" validate:"omitempty"`
}

// Validate ...
func (c *ZiyanCvmDeviceInfoCreateReq) Validate() error {
	if len(c.IP) == 0 && len(c.AssetID) == 0 {
		return errf.Newf(errf.InvalidParameter, "ip and asset_id cannot be empty at the same time")
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmDeviceInfoListReq list request
type ZiyanCvmDeviceInfoListReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
	Page   *core.BasePage     `json:"page" validate:"required"`
	Fields []string           `json:"fields" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmDeviceInfoListReq) Validate() error {
	return validator.Validate.Struct(req)
}

// ZiyanCvmDeviceInfoListResult list result
type ZiyanCvmDeviceInfoListResult = core.ListResultT[*cvmapplytable.ZiyanCvmDeviceInfo]

// ZiyanCvmDeviceInfoListResp define list resp.
type ZiyanCvmDeviceInfoListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *ZiyanCvmDeviceInfoListResult `json:"data"`
}

// BatchUpdateZiyanCvmDeviceInfoReq batch update request
type BatchUpdateZiyanCvmDeviceInfoReq struct {
	Devices []ZiyanCvmDeviceInfoUpdateReq `json:"devices" validate:"required,max=100"`
}

// Validate ...
func (c *BatchUpdateZiyanCvmDeviceInfoReq) Validate() error {
	if len(c.Devices) == 0 || len(c.Devices) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"devices count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.Devices {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmDeviceInfoUpdateReq update request
type ZiyanCvmDeviceInfoUpdateReq struct {
	ID               string                  `json:"id" validate:"required"`
	OrderID          int64                   `json:"order_id" validate:"omitempty"`
	SuborderID       string                  `json:"suborder_id" validate:"omitempty,max=64"`
	GenerateID       *string                 `json:"generate_id" validate:"omitempty"`
	BkBizID          int64                   `json:"bk_biz_id" validate:"omitempty"`
	BkUsername       string                  `json:"bk_username" validate:"omitempty,max=64"`
	BkHostID         *int64                  `json:"bk_host_id" validate:"omitempty"`
	IP               string                  `json:"ip" validate:"omitempty,max=64"`
	AssetID          string                  `json:"asset_id" validate:"omitempty,max=64"`
	InstanceID       string                  `json:"instance_id" validate:"omitempty,max=64"`
	RequireType      *enumor.RequireType     `json:"require_type" validate:"omitempty"`
	ResourceType     *tasktypes.ResourceType `json:"resource_type" validate:"omitempty"`
	DeviceType       string                  `json:"device_type" validate:"omitempty,max=64"`
	Description      string                  `json:"description" validate:"omitempty"`
	Remark           string                  `json:"remark" validate:"omitempty"`
	ZoneName         string                  `json:"zone_name" validate:"omitempty,max=128"`
	ZoneID           *int64                  `json:"zone_id" validate:"omitempty"`
	CloudZone        string                  `json:"cloud_zone" validate:"omitempty"`
	CloudRegion      string                  `json:"cloud_region" validate:"omitempty"`
	ModuleName       string                  `json:"module_name" validate:"omitempty,max=128"`
	RackID           string                  `json:"rack_id" validate:"omitempty,max=64"`
	IsMatched        *bool                   `json:"is_matched" validate:"omitempty"`
	IsChecked        *bool                   `json:"is_checked" validate:"omitempty"`
	IsInited         *bool                   `json:"is_inited" validate:"omitempty"`
	IsDelivered      *bool                   `json:"is_delivered" validate:"omitempty"`
	Deliverer        string                  `json:"deliverer" validate:"omitempty,max=64"`
	GenerateTaskID   string                  `json:"generate_task_id" validate:"omitempty,max=128"`
	GenerateTaskLink string                  `json:"generate_task_link" validate:"omitempty,max=512"`
	InitTaskID       string                  `json:"init_task_id" validate:"omitempty,max=128"`
	InitTaskLink     string                  `json:"init_task_link" validate:"omitempty,max=512"`
	IsManualMatched  *bool                   `json:"is_manual_matched" validate:"omitempty"`
	OwnerIP          *string                 `json:"owner_ip" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmDeviceInfoUpdateReq) Validate() error {
	return validator.Validate.Struct(req)
}
