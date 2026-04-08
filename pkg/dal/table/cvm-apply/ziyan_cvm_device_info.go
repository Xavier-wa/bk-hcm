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
)

// ZiyanCvmDeviceInfoColumns defines ziyan_cvm_device_info's columns.
var ZiyanCvmDeviceInfoColumns = utils.MergeColumns(nil, ZiyanCvmDeviceInfoColumnDescriptor)

// ZiyanCvmDeviceInfoColumnDescriptor is column descriptors.
var ZiyanCvmDeviceInfoColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "order_id", NamedC: "order_id", Type: enumor.Numeric},
	{Column: "suborder_id", NamedC: "suborder_id", Type: enumor.String},
	{Column: "generate_id", NamedC: "generate_id", Type: enumor.String},
	{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric},
	{Column: "bk_username", NamedC: "bk_username", Type: enumor.String},
	{Column: "bk_host_id", NamedC: "bk_host_id", Type: enumor.Numeric},
	{Column: "ip", NamedC: "ip", Type: enumor.String},
	{Column: "asset_id", NamedC: "asset_id", Type: enumor.String},
	{Column: "instance_id", NamedC: "instance_id", Type: enumor.String},
	{Column: "require_type", NamedC: "require_type", Type: enumor.Numeric},
	{Column: "resource_type", NamedC: "resource_type", Type: enumor.String},
	{Column: "device_type", NamedC: "device_type", Type: enumor.String},
	{Column: "description", NamedC: "description", Type: enumor.String},
	{Column: "remark", NamedC: "remark", Type: enumor.String},
	{Column: "zone_name", NamedC: "zone_name", Type: enumor.String},
	{Column: "zone_id", NamedC: "zone_id", Type: enumor.Numeric},
	{Column: "cloud_zone", NamedC: "cloud_zone", Type: enumor.String},
	{Column: "cloud_region", NamedC: "cloud_region", Type: enumor.String},
	{Column: "module_name", NamedC: "module_name", Type: enumor.String},
	{Column: "rack_id", NamedC: "rack_id", Type: enumor.String},
	{Column: "is_matched", NamedC: "is_matched", Type: enumor.Boolean},
	{Column: "is_checked", NamedC: "is_checked", Type: enumor.Boolean},
	{Column: "is_inited", NamedC: "is_inited", Type: enumor.Boolean},
	{Column: "is_delivered", NamedC: "is_delivered", Type: enumor.Boolean},
	{Column: "deliverer", NamedC: "deliverer", Type: enumor.String},
	{Column: "generate_task_id", NamedC: "generate_task_id", Type: enumor.String},
	{Column: "generate_task_link", NamedC: "generate_task_link", Type: enumor.String},
	{Column: "init_task_id", NamedC: "init_task_id", Type: enumor.String},
	{Column: "init_task_link", NamedC: "init_task_link", Type: enumor.String},
	{Column: "is_manual_matched", NamedC: "is_manual_matched", Type: enumor.Boolean},
	{Column: "owner_ip", NamedC: "owner_ip", Type: enumor.String},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// ZiyanCvmDeviceInfo 自研云CVM设备交付记录
type ZiyanCvmDeviceInfo struct {
	// ID 主键ID
	ID string `db:"id" json:"id"`
	// OrderID 主单ID
	OrderID int64 `db:"order_id" json:"order_id"`
	// SuborderID 子单ID
	SuborderID string `db:"suborder_id" json:"suborder_id" validate:"max=64"`
	// GenerateID 生产ID
	GenerateID string `db:"generate_id" json:"generate_id" validate:"max=64"`
	// BkBizID 业务ID
	BkBizID int64 `db:"bk_biz_id" json:"bk_biz_id"`
	// BkUsername 申请人
	BkUsername string `db:"bk_username" json:"bk_username" validate:"max=64"`
	// BkHostID 主机ID
	BkHostID int64 `db:"bk_host_id" json:"bk_host_id"`
	// IP IP地址
	IP string `db:"ip" json:"ip" validate:"max=64"`
	// AssetID 资产ID
	AssetID string `db:"asset_id" json:"asset_id" validate:"max=64"`
	// InstanceID 实例ID
	InstanceID string `db:"instance_id" json:"instance_id" validate:"max=64"`
	// RequireType 需求类型
	RequireType enumor.RequireType `db:"require_type" json:"require_type"`
	// ResourceType 资源类型(QCLOUDCVM/PM等)
	ResourceType enumor.ResourceType `db:"resource_type" json:"resource_type" validate:"max=32"`
	// DeviceType 设备类型/机型
	DeviceType  string `db:"device_type" json:"device_type" validate:"max=64"`
	Description string `db:"description" json:"description"`
	Remark      string `db:"remark" json:"remark"`
	// ZoneName 可用区名称(从bkcc获取并写入)
	ZoneName string `db:"zone_name" json:"zone_name" validate:"max=128"`
	// ZoneID 可用区ID(从bkcc获取并写入)
	ZoneID int64 `db:"zone_id" json:"zone_id"`
	// CloudZone 云可用区
	CloudZone string `db:"cloud_zone" json:"cloud_zone"`
	// CloudRegion 云地域
	CloudRegion string `db:"cloud_region" json:"cloud_region"`
	// ModuleName 模块名称(从bkcc获取并写入)
	ModuleName string `db:"module_name" json:"module_name" validate:"max=128"`
	// RackID 机架ID(从bkcc获取并写入)
	RackID string `db:"rack_id" json:"rack_id" validate:"max=64"`
	// IsMatched 是否已匹配(0:否 1:是)
	IsMatched *bool `db:"is_matched" json:"is_matched"`
	// IsChecked 是否已核验(0:否 1:是)
	IsChecked *bool `db:"is_checked" json:"is_checked"`
	// IsInited 是否已初始化(0:否 1:是)
	IsInited *bool `db:"is_inited" json:"is_inited"`
	// IsDelivered 是否已交付(0:否 1:是)
	IsDelivered *bool `db:"is_delivered" json:"is_delivered"`
	// Deliverer 交付人/交付系统
	Deliverer string `db:"deliverer" json:"deliverer" validate:"max=64"`
	// GenerateTaskID 生成任务ID
	GenerateTaskID string `db:"generate_task_id" json:"generate_task_id" validate:"max=128"`
	// GenerateTaskLink 生成任务链接
	GenerateTaskLink string `db:"generate_task_link" json:"generate_task_link" validate:"max=512"`
	// InitTaskID 初始化任务ID
	InitTaskID string `db:"init_task_id" json:"init_task_id" validate:"max=128"`
	// InitTaskLink 初始化任务链接
	InitTaskLink string `db:"init_task_link" json:"init_task_link" validate:"max=512"`
	// IsManualMatched 是否手工匹配
	IsManualMatched bool `db:"is_manual_matched" json:"is_manual_matched"`
	// OwnerIP 所属的母机IP
	OwnerIP string `db:"owner_ip" json:"owner_ip"`
	// Creator 创建人
	Creator string `db:"creator" json:"creator" validate:"max=64"`
	// Reviser 修改人
	Reviser string `db:"reviser" json:"reviser" validate:"max=64"`
	// CreatedAt 创建时间
	CreatedAt types.Time `db:"created_at" json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt types.Time `db:"updated_at" validate:"excluded_unless" json:"updated_at"`
}

// TableName 表名
func (z *ZiyanCvmDeviceInfo) TableName() table.Name {
	return table.ZiyanCvmDeviceInfoTable
}

// InsertValidate validate insert
func (z *ZiyanCvmDeviceInfo) InsertValidate() error {
	if z.OrderID == 0 {
		return errors.New("order_id is required")
	}
	if len(z.SuborderID) == 0 {
		return errors.New("suborder_id is required")
	}
	if z.BkBizID == 0 {
		return errors.New("bk_biz_id is required")
	}
	if len(z.IP) == 0 && len(z.AssetID) == 0 {
		return errors.New("ip or asset_id is required")
	}
	return validator.Validate.Struct(z)
}

// UpdateValidate validate update
func (z *ZiyanCvmDeviceInfo) UpdateValidate() error {
	return validator.Validate.Struct(z)
}
