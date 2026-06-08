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

package cvmapply

import (
	"errors"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
	cvt "hcm/pkg/tools/converter"
)

// ZiyanCvmModifyRecordColumns defines ziyan_cvm_modify_record's columns.
var ZiyanCvmModifyRecordColumns = utils.MergeColumns(nil, ZiyanCvmModifyRecordColumnDescriptor)

// ZiyanCvmModifyRecordColumnDescriptor is column descriptors.
var ZiyanCvmModifyRecordColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "suborder_id", NamedC: "suborder_id", Type: enumor.String},
	{Column: "bk_username", NamedC: "bk_username", Type: enumor.String},
	{Column: "pre_total_num", NamedC: "pre_total_num", Type: enumor.Numeric},
	{Column: "pre_replicas", NamedC: "pre_replicas", Type: enumor.Numeric},
	{Column: "pre_region", NamedC: "pre_region", Type: enumor.String},
	{Column: "pre_zone", NamedC: "pre_zone", Type: enumor.String},
	{Column: "pre_device_type", NamedC: "pre_device_type", Type: enumor.String},
	{Column: "pre_image_id", NamedC: "pre_image_id", Type: enumor.String},
	{Column: "pre_disk_size", NamedC: "pre_disk_size", Type: enumor.Numeric},
	{Column: "pre_disk_type", NamedC: "pre_disk_type", Type: enumor.String},
	{Column: "pre_network_type", NamedC: "pre_network_type", Type: enumor.String},
	{Column: "pre_vpc", NamedC: "pre_vpc", Type: enumor.String},
	{Column: "pre_subnet", NamedC: "pre_subnet", Type: enumor.String},
	{Column: "pre_system_disk_type", NamedC: "pre_system_disk_type", Type: enumor.String},
	{Column: "pre_system_disk_size", NamedC: "pre_system_disk_size", Type: enumor.Numeric},
	{Column: "pre_system_disk_num", NamedC: "pre_system_disk_num", Type: enumor.Numeric},
	{Column: "pre_data_disk", NamedC: "pre_data_disk", Type: enumor.Json},
	{Column: "pre_zones", NamedC: "pre_zones", Type: enumor.Json},
	{Column: "pre_res_assign", NamedC: "pre_res_assign", Type: enumor.Numeric},
	{Column: "pre_bk_asset_id", NamedC: "pre_bk_asset_id", Type: enumor.String},
	{Column: "pre_inherit_instance_id", NamedC: "pre_inherit_instance_id", Type: enumor.String},
	{Column: "cur_total_num", NamedC: "cur_total_num", Type: enumor.Numeric},
	{Column: "cur_replicas", NamedC: "cur_replicas", Type: enumor.Numeric},
	{Column: "cur_region", NamedC: "cur_region", Type: enumor.String},
	{Column: "cur_zone", NamedC: "cur_zone", Type: enumor.String},
	{Column: "cur_device_type", NamedC: "cur_device_type", Type: enumor.String},
	{Column: "cur_image_id", NamedC: "cur_image_id", Type: enumor.String},
	{Column: "cur_disk_size", NamedC: "cur_disk_size", Type: enumor.Numeric},
	{Column: "cur_disk_type", NamedC: "cur_disk_type", Type: enumor.String},
	{Column: "cur_network_type", NamedC: "cur_network_type", Type: enumor.String},
	{Column: "cur_vpc", NamedC: "cur_vpc", Type: enumor.String},
	{Column: "cur_subnet", NamedC: "cur_subnet", Type: enumor.String},
	{Column: "cur_system_disk_type", NamedC: "cur_system_disk_type", Type: enumor.String},
	{Column: "cur_system_disk_size", NamedC: "cur_system_disk_size", Type: enumor.Numeric},
	{Column: "cur_system_disk_num", NamedC: "cur_system_disk_num", Type: enumor.Numeric},
	{Column: "cur_data_disk", NamedC: "cur_data_disk", Type: enumor.Json},
	{Column: "cur_zones", NamedC: "cur_zones", Type: enumor.Json},
	{Column: "cur_res_assign", NamedC: "cur_res_assign", Type: enumor.Numeric},
	{Column: "cur_bk_asset_id", NamedC: "cur_bk_asset_id", Type: enumor.String},
	{Column: "cur_inherit_instance_id", NamedC: "cur_inherit_instance_id", Type: enumor.String},
	{Column: "status", NamedC: "status", Type: enumor.Numeric},
	{Column: "approver", NamedC: "approver", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// ZiyanCvmModifyRecord 自研云CVM变更记录
type ZiyanCvmModifyRecord struct {
	// ID 主键ID
	ID string `db:"id" json:"id"`
	// SuborderID 子订单ID
	SuborderID *string `db:"suborder_id" json:"suborder_id" validate:"omitempty,max=64"`
	// BkUsername 蓝鲸用户名
	BkUsername *string `db:"bk_username" json:"bk_username" validate:"omitempty,max=64"`
	// PreTotalNum 修改前-总数量
	PreTotalNum *uint `db:"pre_total_num" json:"pre_total_num"`
	// PreReplicas 修改前-副本数
	PreReplicas *uint `db:"pre_replicas" json:"pre_replicas"`
	// PreRegion 修改前-地域
	PreRegion *string `db:"pre_region" json:"pre_region" validate:"omitempty,max=64"`
	// PreZone 修改前-可用区
	PreZone *string `db:"pre_zone" json:"pre_zone" validate:"omitempty,max=64"`
	// PreDeviceType 修改前-机型
	PreDeviceType *string `db:"pre_device_type" json:"pre_device_type" validate:"omitempty,max=64"`
	// PreImageID 修改前-镜像ID
	PreImageID *string `db:"pre_image_id" json:"pre_image_id" validate:"omitempty,max=64"`
	// PreDiskSize 修改前-磁盘大小
	PreDiskSize *int `db:"pre_disk_size" json:"pre_disk_size"`
	// PreDiskType 修改前-磁盘类型
	PreDiskType *enumor.DiskType `db:"pre_disk_type" json:"pre_disk_type" validate:"omitempty,max=32"`
	// PreNetworkType 修改前-网络类型
	PreNetworkType *string `db:"pre_network_type" json:"pre_network_type" validate:"omitempty,max=32"`
	// PreVpc 修改前-VPC
	PreVpc *string `db:"pre_vpc" json:"pre_vpc" validate:"omitempty,max=64"`
	// PreSubnet 修改前-子网
	PreSubnet *string `db:"pre_subnet" json:"pre_subnet" validate:"omitempty,max=64"`
	// PreSystemDiskType 修改前-系统盘类型
	PreSystemDiskType *enumor.DiskType `db:"pre_system_disk_type" json:"pre_system_disk_type" validate:"omitempty,max=32"`
	// PreSystemDiskSize 修改前-系统盘大小
	PreSystemDiskSize *int `db:"pre_system_disk_size" json:"pre_system_disk_size"`
	// PreSystemDiskNum 修改前-系统盘数量
	PreSystemDiskNum *int `db:"pre_system_disk_num" json:"pre_system_disk_num"`
	// PreDataDisk 修改前-数据盘配置（JSON数组）
	PreDataDisk *types.JsonField `db:"pre_data_disk" json:"pre_data_disk"`
	// PreZones 修改前-可用区列表（JSON数组）
	PreZones *types.JsonField `db:"pre_zones" json:"pre_zones"`
	// PreResAssign 修改前-资源分配方式
	PreResAssign *enumor.ResAssign `db:"pre_res_assign" json:"pre_res_assign"`
	// PreBkAssetID 修改前-继承主机的固资号
	PreBkAssetID *string `db:"pre_bk_asset_id" json:"pre_bk_asset_id" validate:"omitempty,max=64"`
	// PreInheritInstanceID 修改前-被继承云主机实例ID
	PreInheritInstanceID *string `db:"pre_inherit_instance_id" json:"pre_inherit_instance_id" validate:"omitempty,max=64"`
	// CurTotalNum 修改后-总数量
	CurTotalNum *uint `db:"cur_total_num" json:"cur_total_num"`
	// CurReplicas 修改后-副本数
	CurReplicas *uint `db:"cur_replicas" json:"cur_replicas"`
	// CurRegion 修改后-地域
	CurRegion *string `db:"cur_region" json:"cur_region" validate:"omitempty,max=64"`
	// CurZone 修改后-可用区
	CurZone *string `db:"cur_zone" json:"cur_zone" validate:"omitempty,max=64"`
	// CurDeviceType 修改后-机型
	CurDeviceType *string `db:"cur_device_type" json:"cur_device_type" validate:"omitempty,max=64"`
	// CurImageID 修改后-镜像ID
	CurImageID *string `db:"cur_image_id" json:"cur_image_id" validate:"omitempty,max=64"`
	// CurDiskSize 修改后-磁盘大小
	CurDiskSize *int `db:"cur_disk_size" json:"cur_disk_size"`
	// CurDiskType 修改后-磁盘类型
	CurDiskType *enumor.DiskType `db:"cur_disk_type" json:"cur_disk_type" validate:"omitempty,max=32"`
	// CurNetworkType 修改后-网络类型
	CurNetworkType *string `db:"cur_network_type" json:"cur_network_type" validate:"omitempty,max=32"`
	// CurVpc 修改后-VPC
	CurVpc *string `db:"cur_vpc" json:"cur_vpc" validate:"omitempty,max=64"`
	// CurSubnet 修改后-子网
	CurSubnet *string `db:"cur_subnet" json:"cur_subnet" validate:"omitempty,max=64"`
	// CurSystemDiskType 修改后-系统盘类型
	CurSystemDiskType *enumor.DiskType `db:"cur_system_disk_type" json:"cur_system_disk_type" validate:"omitempty,max=32"`
	// CurSystemDiskSize 修改后-系统盘大小
	CurSystemDiskSize *int `db:"cur_system_disk_size" json:"cur_system_disk_size"`
	// CurSystemDiskNum 修改后-系统盘数量
	CurSystemDiskNum *int `db:"cur_system_disk_num" json:"cur_system_disk_num"`
	// CurDataDisk 修改后-数据盘配置（JSON数组）
	CurDataDisk *types.JsonField `db:"cur_data_disk" json:"cur_data_disk"`
	// CurZones 修改后-可用区列表（JSON数组）
	CurZones *types.JsonField `db:"cur_zones" json:"cur_zones"`
	// CurResAssign 修改后-资源分配方式
	CurResAssign *enumor.ResAssign `db:"cur_res_assign" json:"cur_res_assign"`
	// CurBkAssetID 修改后-继承主机的固资号
	CurBkAssetID *string `db:"cur_bk_asset_id" json:"cur_bk_asset_id" validate:"omitempty,max=64"`
	// CurInheritInstanceID 修改后-被继承云主机实例ID
	CurInheritInstanceID *string `db:"cur_inherit_instance_id" json:"cur_inherit_instance_id" validate:"omitempty,max=64"`
	// Status 状态：0-待审批, 1-已审批, 2-审批失败, 3-已拒绝, 4-审批超时/作废
	Status enumor.CvmModifyRecordStatus `db:"status" json:"status"`
	// Approver 审批人
	Approver *string `db:"approver" json:"approver" validate:"omitempty,max=64"`
	// CreatedAt 创建时间（毫秒精度）
	CreatedAt types.Time `db:"created_at" json:"created_at"`
	// UpdatedAt 更新时间（毫秒精度）
	UpdatedAt types.Time `db:"updated_at" validate:"excluded_unless" json:"updated_at"`
}

// TableName 表名
func (z *ZiyanCvmModifyRecord) TableName() table.Name {
	return table.ZiyanCvmModifyRecordTable
}

// InsertValidate validate insert
func (z *ZiyanCvmModifyRecord) InsertValidate() error {
	if z.SuborderID == nil || len(cvt.PtrToVal(z.SuborderID)) == 0 {
		return errors.New("suborder_id is required")
	}
	return validator.Validate.Struct(z)
}

// UpdateValidate validate update
func (z *ZiyanCvmModifyRecord) UpdateValidate() error {
	return validator.Validate.Struct(z)
}
