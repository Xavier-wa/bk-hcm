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

// BatchCreateZiyanCvmModifyRecordReq batch create request
type BatchCreateZiyanCvmModifyRecordReq struct {
	Records []ZiyanCvmModifyRecordCreateReq `json:"records" validate:"required,max=100"`
}

// Validate ...
func (c *BatchCreateZiyanCvmModifyRecordReq) Validate() error {
	if len(c.Records) == 0 || len(c.Records) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"records count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.Records {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmModifyRecordCreateReq create request
type ZiyanCvmModifyRecordCreateReq struct {
	ID                   string                       `json:"id" validate:"omitempty"`
	SuborderID           string                       `json:"suborder_id" validate:"required,max=64"`
	BkUsername           string                       `json:"bk_username" validate:"required,max=64"`
	PreTotalNum          *uint                        `json:"pre_total_num" validate:"omitempty"`
	PreReplicas          *uint                        `json:"pre_replicas" validate:"omitempty"`
	PreRegion            string                       `json:"pre_region" validate:"max=64"`
	PreZone              string                       `json:"pre_zone" validate:"max=64"`
	PreDeviceType        string                       `json:"pre_device_type" validate:"max=64"`
	PreImageID           string                       `json:"pre_image_id" validate:"max=64"`
	PreDiskSize          *int                         `json:"pre_disk_size" validate:"omitempty"`
	PreDiskType          enumor.DiskType              `json:"pre_disk_type" validate:"max=32"`
	PreNetworkType       string                       `json:"pre_network_type" validate:"max=32"`
	PreVpc               string                       `json:"pre_vpc" validate:"max=64"`
	PreSubnet            string                       `json:"pre_subnet" validate:"max=64"`
	PreSystemDiskType    enumor.DiskType              `json:"pre_system_disk_type" validate:"max=32"`
	PreSystemDiskSize    *int                         `json:"pre_system_disk_size" validate:"omitempty"`
	PreSystemDiskNum     *int                         `json:"pre_system_disk_num" validate:"omitempty"`
	PreDataDisk          types.JsonField              `json:"pre_data_disk" validate:"omitempty"`
	PreZones             types.JsonField              `json:"pre_zones" validate:"omitempty"`
	PreResAssign         enumor.ResAssign             `json:"pre_res_assign" validate:"omitempty"`
	PreBkAssetID         string                       `json:"pre_bk_asset_id" validate:"max=64"`
	PreInheritInstanceID string                       `json:"pre_inherit_instance_id" validate:"max=64"`
	CurTotalNum          *uint                        `json:"cur_total_num" validate:"omitempty"`
	CurReplicas          *uint                        `json:"cur_replicas" validate:"omitempty"`
	CurRegion            string                       `json:"cur_region" validate:"max=64"`
	CurZone              string                       `json:"cur_zone" validate:"max=64"`
	CurDeviceType        string                       `json:"cur_device_type" validate:"max=64"`
	CurImageID           string                       `json:"cur_image_id" validate:"max=64"`
	CurDiskSize          *int                         `json:"cur_disk_size" validate:"omitempty"`
	CurDiskType          enumor.DiskType              `json:"cur_disk_type" validate:"max=32"`
	CurNetworkType       string                       `json:"cur_network_type" validate:"max=32"`
	CurVpc               string                       `json:"cur_vpc" validate:"max=64"`
	CurSubnet            string                       `json:"cur_subnet" validate:"max=64"`
	CurSystemDiskType    enumor.DiskType              `json:"cur_system_disk_type" validate:"max=32"`
	CurSystemDiskSize    *int                         `json:"cur_system_disk_size" validate:"omitempty"`
	CurSystemDiskNum     *int                         `json:"cur_system_disk_num" validate:"omitempty"`
	CurDataDisk          types.JsonField              `json:"cur_data_disk" validate:"omitempty"`
	CurZones             types.JsonField              `json:"cur_zones" validate:"omitempty"`
	CurResAssign         enumor.ResAssign             `json:"cur_res_assign" validate:"omitempty"`
	CurBkAssetID         string                       `json:"cur_bk_asset_id" validate:"max=64"`
	CurInheritInstanceID string                       `json:"cur_inherit_instance_id" validate:"max=64"`
	Status               enumor.CvmModifyRecordStatus `json:"status" validate:"omitempty"`
	Approver             string                       `json:"approver" validate:"max=64"`
	// CreatedAt 原始创建时间（用于数据迁移，保留历史时间）
	CreatedAt types.Time `json:"created_at" validate:"omitempty"`
	UpdatedAt types.Time `json:"updated_at" validate:"omitempty"`
}

// Validate ...
func (c *ZiyanCvmModifyRecordCreateReq) Validate() error {
	return validator.Validate.Struct(c)
}

// ZiyanCvmModifyRecordListReq list request
type ZiyanCvmModifyRecordListReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
	Page   *core.BasePage     `json:"page" validate:"required"`
	Fields []string           `json:"fields" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmModifyRecordListReq) Validate() error {
	return validator.Validate.Struct(req)
}

// ZiyanCvmModifyRecordListResult list result
type ZiyanCvmModifyRecordListResult = core.ListResultT[*cvmapplytable.ZiyanCvmModifyRecord]

// ZiyanCvmModifyRecordListResp define list resp.
type ZiyanCvmModifyRecordListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *ZiyanCvmModifyRecordListResult `json:"data"`
}

// BatchUpdateZiyanCvmModifyRecordReq batch update request
type BatchUpdateZiyanCvmModifyRecordReq struct {
	Records []ZiyanCvmModifyRecordUpdateReq `json:"records" validate:"required,max=100"`
}

// Validate ...
func (c *BatchUpdateZiyanCvmModifyRecordReq) Validate() error {
	if len(c.Records) == 0 || len(c.Records) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"records count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.Records {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmModifyRecordUpdateReq update request
type ZiyanCvmModifyRecordUpdateReq struct {
	ID                   string                        `json:"id" validate:"required"`
	SuborderID           string                        `json:"suborder_id" validate:"omitempty,max=64"`
	BkUsername           string                        `json:"bk_username" validate:"omitempty,max=64"`
	PreTotalNum          *uint                         `json:"pre_total_num" validate:"omitempty"`
	PreReplicas          *uint                         `json:"pre_replicas" validate:"omitempty"`
	PreRegion            string                        `json:"pre_region" validate:"omitempty,max=64"`
	PreZone              string                        `json:"pre_zone" validate:"omitempty,max=64"`
	PreDeviceType        string                        `json:"pre_device_type" validate:"omitempty,max=64"`
	PreImageID           string                        `json:"pre_image_id" validate:"omitempty,max=64"`
	PreDiskSize          *int                          `json:"pre_disk_size" validate:"omitempty"`
	PreDiskType          enumor.DiskType               `json:"pre_disk_type" validate:"omitempty,max=32"`
	PreNetworkType       string                        `json:"pre_network_type" validate:"omitempty,max=32"`
	PreVpc               string                        `json:"pre_vpc" validate:"omitempty,max=64"`
	PreSubnet            string                        `json:"pre_subnet" validate:"omitempty,max=64"`
	PreSystemDiskType    enumor.DiskType               `json:"pre_system_disk_type" validate:"omitempty,max=32"`
	PreSystemDiskSize    *int                          `json:"pre_system_disk_size" validate:"omitempty"`
	PreSystemDiskNum     *int                          `json:"pre_system_disk_num" validate:"omitempty"`
	PreDataDisk          types.JsonField               `json:"pre_data_disk" validate:"omitempty"`
	PreZones             types.JsonField               `json:"pre_zones" validate:"omitempty"`
	PreResAssign         *enumor.ResAssign             `json:"pre_res_assign" validate:"omitempty"`
	PreBkAssetID         string                        `json:"pre_bk_asset_id" validate:"omitempty,max=64"`
	PreInheritInstanceID string                        `json:"pre_inherit_instance_id" validate:"omitempty,max=64"`
	CurTotalNum          *uint                         `json:"cur_total_num" validate:"omitempty"`
	CurReplicas          *uint                         `json:"cur_replicas" validate:"omitempty"`
	CurRegion            string                        `json:"cur_region" validate:"omitempty,max=64"`
	CurZone              string                        `json:"cur_zone" validate:"omitempty,max=64"`
	CurDeviceType        string                        `json:"cur_device_type" validate:"omitempty,max=64"`
	CurImageID           string                        `json:"cur_image_id" validate:"omitempty,max=64"`
	CurDiskSize          *int                          `json:"cur_disk_size" validate:"omitempty"`
	CurDiskType          enumor.DiskType               `json:"cur_disk_type" validate:"omitempty,max=32"`
	CurNetworkType       string                        `json:"cur_network_type" validate:"omitempty,max=32"`
	CurVpc               string                        `json:"cur_vpc" validate:"omitempty,max=64"`
	CurSubnet            string                        `json:"cur_subnet" validate:"omitempty,max=64"`
	CurSystemDiskType    enumor.DiskType               `json:"cur_system_disk_type" validate:"omitempty,max=32"`
	CurSystemDiskSize    *int                          `json:"cur_system_disk_size" validate:"omitempty"`
	CurSystemDiskNum     *int                          `json:"cur_system_disk_num" validate:"omitempty"`
	CurDataDisk          types.JsonField               `json:"cur_data_disk" validate:"omitempty"`
	CurZones             types.JsonField               `json:"cur_zones" validate:"omitempty"`
	CurResAssign         *enumor.ResAssign             `json:"cur_res_assign" validate:"omitempty"`
	CurBkAssetID         string                        `json:"cur_bk_asset_id" validate:"omitempty,max=64"`
	CurInheritInstanceID string                        `json:"cur_inherit_instance_id" validate:"omitempty,max=64"`
	Status               *enumor.CvmModifyRecordStatus `json:"status" validate:"omitempty"`
	Approver             string                        `json:"approver" validate:"omitempty,max=64"`
}

// Validate ...
func (req *ZiyanCvmModifyRecordUpdateReq) Validate() error {
	return validator.Validate.Struct(req)
}
