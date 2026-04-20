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
	"hcm/pkg/thirdparty/cvmapi"
)

// ZiyanCvmApplySuborderColumns defines ziyan_cvm_apply_suborder's columns.
var ZiyanCvmApplySuborderColumns = utils.MergeColumns(nil, ZiyanCvmApplySuborderColumnDescriptor)

// ZiyanCvmApplySuborderColumnDescriptor is column descriptors.
var ZiyanCvmApplySuborderColumnDescriptor = utils.ColumnDescriptors{
	{Column: "suborder_id", NamedC: "suborder_id", Type: enumor.String},
	{Column: "order_id", NamedC: "order_id", Type: enumor.Numeric},
	{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric},
	{Column: "bk_username", NamedC: "bk_username", Type: enumor.String},
	{Column: "follower", NamedC: "follower", Type: enumor.Json},
	{Column: "auditor", NamedC: "auditor", Type: enumor.String},
	{Column: "source", NamedC: "source", Type: enumor.String},
	{Column: "product_type", NamedC: "product_type", Type: enumor.String},
	{Column: "require_type", NamedC: "require_type", Type: enumor.Numeric},
	{Column: "expect_time", NamedC: "expect_time", Type: enumor.Time},
	{Column: "resource_type", NamedC: "resource_type", Type: enumor.String},
	{Column: "anti_affinity_level", NamedC: "anti_affinity_level", Type: enumor.String},
	{Column: "enable_disk_check", NamedC: "enable_disk_check", Type: enumor.Boolean},
	{Column: "obs_project", NamedC: "obs_project", Type: enumor.String},
	{Column: "description", NamedC: "description", Type: enumor.String},
	{Column: "remark", NamedC: "remark", Type: enumor.String},
	{Column: "region", NamedC: "region", Type: enumor.String},
	{Column: "zone", NamedC: "zone", Type: enumor.String},
	{Column: "device_group", NamedC: "device_group", Type: enumor.String},
	{Column: "device_size", NamedC: "device_size", Type: enumor.String},
	{Column: "device_type", NamedC: "device_type", Type: enumor.String},
	{Column: "image_id", NamedC: "image_id", Type: enumor.String},
	{Column: "image", NamedC: "image", Type: enumor.String},
	{Column: "disk_size", NamedC: "disk_size", Type: enumor.Numeric},
	{Column: "disk_type", NamedC: "disk_type", Type: enumor.String},
	{Column: "network_type", NamedC: "network_type", Type: enumor.String},
	{Column: "vpc", NamedC: "vpc", Type: enumor.String},
	{Column: "subnet", NamedC: "subnet", Type: enumor.String},
	{Column: "os_type", NamedC: "os_type", Type: enumor.String},
	{Column: "raid_type", NamedC: "raid_type", Type: enumor.String},
	{Column: "isp", NamedC: "isp", Type: enumor.String},
	{Column: "failed_zone_ids", NamedC: "failed_zone_ids", Type: enumor.Json},
	{Column: "charge_type", NamedC: "charge_type", Type: enumor.String},
	{Column: "charge_months", NamedC: "charge_months", Type: enumor.Numeric},
	{Column: "inherit_instance_id", NamedC: "inherit_instance_id", Type: enumor.String},
	{Column: "bk_asset_id", NamedC: "bk_asset_id", Type: enumor.String},
	{Column: "res_assign", NamedC: "res_assign", Type: enumor.Numeric},
	{Column: "cpu_thread_switch", NamedC: "cpu_thread_switch", Type: enumor.Numeric},
	{Column: "system_disk", NamedC: "system_disk", Type: enumor.Json},
	{Column: "data_disk", NamedC: "data_disk", Type: enumor.Json},
	{Column: "zones", NamedC: "zones", Type: enumor.Json},
	{Column: "upgrade_cvm_list", NamedC: "upgrade_cvm_list", Type: enumor.Json},
	{Column: "stage", NamedC: "stage", Type: enumor.String},
	{Column: "status", NamedC: "status", Type: enumor.String},
	{Column: "retry_time", NamedC: "retry_time", Type: enumor.Numeric},
	{Column: "modify_time", NamedC: "modify_time", Type: enumor.Numeric},
	{Column: "applied_core", NamedC: "applied_core", Type: enumor.Numeric},
	{Column: "delivered_core", NamedC: "delivered_core", Type: enumor.Numeric},
	{Column: "plan_expend_group", NamedC: "plan_expend_group", Type: enumor.Json},
	{Column: "origin_num", NamedC: "origin_num", Type: enumor.Numeric},
	{Column: "total_num", NamedC: "total_num", Type: enumor.Numeric},
	{Column: "success_num", NamedC: "success_num", Type: enumor.Numeric},
	{Column: "pending_num", NamedC: "pending_num", Type: enumor.Numeric},
	{Column: "failed_num", NamedC: "failed_num", Type: enumor.Numeric},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// ZiyanCvmApplySuborder 自研云CVM申请子单
type ZiyanCvmApplySuborder struct {
	// SuborderID 主键ID&子单ID
	SuborderID string `db:"suborder_id" json:"suborder_id" validate:"max=64"`
	// OrderID 主单ID
	OrderID uint64 `db:"order_id" json:"order_id"`
	// BkBizID 业务ID
	BkBizID int64 `db:"bk_biz_id" json:"bk_biz_id"`
	// BkUsername 申请人
	BkUsername string `db:"bk_username" json:"bk_username" validate:"max=64"`
	// Follower 关注人列表(JSON数组)
	Follower types.JsonField `db:"follower" json:"follower"`
	// Auditor 审批人
	Auditor string `db:"auditor" json:"auditor" validate:"max=64"`
	// Source 来源(business:业务 purchase_to_resource_pool:资源池)
	Source enumor.ApplyTicketSource `db:"source" json:"source" validate:"max=64"`
	// ProductType 产品类型
	ProductType enumor.ProductType `db:"product_type" json:"product_type" validate:"max=64"`
	// RequireType 需求类型
	RequireType enumor.RequireType `db:"require_type" json:"require_type"`
	// ExpectTime 期望交付时间
	ExpectTime string `db:"expect_time" json:"expect_time"`
	// ResourceType 资源类型(QCLOUDCVM/IDCDVM/PM等)
	ResourceType enumor.ResourceType `db:"resource_type" json:"resource_type" validate:"max=32"`
	// AntiAffinityLevel 反亲和级别
	AntiAffinityLevel string `db:"anti_affinity_level" json:"anti_affinity_level" validate:"max=64"`
	// EnableDiskCheck 是否检查磁盘
	EnableDiskCheck *bool `db:"enable_disk_check" json:"enable_disk_check"`
	// ObsProject OBS项目名称
	ObsProject enumor.ObsProject `db:"obs_project" json:"obs_project" validate:"max=64"`
	// Description 主单备注
	Description string `db:"description" json:"description" validate:"max=255"`
	// Remark 子单备注
	Remark string `db:"remark" json:"remark" validate:"max=255"`
	// Region 地域
	Region string `db:"region" json:"region" validate:"max=64"`
	// Zone 可用区
	Zone string `db:"zone" json:"zone" validate:"max=64"`
	// DeviceGroup 机型族
	DeviceGroup string `db:"device_group" json:"device_group" validate:"max=128"`
	// DeviceSize 机型核心类型(小核心、中核心、大核心)
	DeviceSize enumor.CoreType `db:"device_size" json:"device_size" validate:"max=64"`
	// DeviceType 机型
	DeviceType string `db:"device_type" json:"device_type" validate:"max=64"`
	// ImageID 镜像ID
	ImageID string `db:"image_id" json:"image_id" validate:"max=64"`
	// Image 镜像名称
	Image string `db:"image" json:"image" validate:"max=128"`
	// DiskSize 磁盘大小(GB)
	DiskSize int64 `db:"disk_size" json:"disk_size"`
	// DiskType 磁盘类型
	DiskType enumor.DiskType `db:"disk_type" json:"disk_type" validate:"max=64"`
	// NetworkType 网络类型
	NetworkType string `db:"network_type" json:"network_type" validate:"max=64"`
	// Vpc VPC ID
	Vpc string `db:"vpc" json:"vpc" validate:"max=64"`
	// Subnet 子网ID
	Subnet string `db:"subnet" json:"subnet" validate:"max=64"`
	// OsType 操作系统类型
	OsType string `db:"os_type" json:"os_type" validate:"max=64"`
	// RaidType RAID类型
	RaidType string `db:"raid_type" json:"raid_type" validate:"max=64"`
	// Isp 运营商
	Isp string `db:"isp" json:"isp" validate:"max=64"`
	// FailedZoneIds 记录报错的可用区
	FailedZoneIds types.JsonField `db:"failed_zone_ids" json:"failed_zone_ids"`
	// ChargeType 计费模式
	ChargeType cvmapi.ChargeType `db:"charge_type" json:"charge_type" validate:"max=64"`
	// ChargeMonths 计费时长，单位：月
	ChargeMonths uint `db:"charge_months" json:"charge_months"`
	// InheritInstanceID 被继承云主机实例ID
	InheritInstanceID string `db:"inherit_instance_id" json:"inherit_instance_id" validate:"max=64"`
	// BkAssetID 被继承固资编号
	BkAssetID string `db:"bk_asset_id" json:"bk_asset_id" validate:"max=64"`
	// ResAssign 资源分配方式
	ResAssign enumor.ResAssign `db:"res_assign" json:"res_assign"`
	// CPUThreadSwitch CPU线程开关
	CPUThreadSwitch enumor.CPUThreadSwitch `db:"cpu_thread_switch" json:"cpu_thread_switch"`
	// SystemDisk 系统盘
	SystemDisk types.JsonField `db:"system_disk" json:"system_disk"`
	// DataDisk 数据盘
	DataDisk types.JsonField `db:"data_disk" json:"data_disk"`
	// Zones 多可用区
	Zones types.JsonField `db:"zones" json:"zones"`
	// UpgradeCvmList cvm升降配列表
	UpgradeCvmList types.JsonField `db:"upgrade_cvm_list" json:"upgrade_cvm_list"`
	// Stage 阶段
	Stage enumor.TicketStage `db:"stage" json:"stage" validate:"max=32"`
	// Status 状态
	Status enumor.ApplyStatus `db:"status" json:"status" validate:"max=32"`
	// RetryTime 重试次数
	RetryTime *uint `db:"retry_time" json:"retry_time"`
	// ModifyTime 修改次数
	ModifyTime *uint `db:"modify_time" json:"modify_time"`
	// AppliedCore 需求核心数
	AppliedCore *uint `db:"applied_core" json:"applied_core"`
	// DeliveredCore 交付核心数
	DeliveredCore *uint `db:"delivered_core" json:"delivered_core"`
	// PlanExpendGroup 预测使用记录
	PlanExpendGroup types.JsonField `db:"plan_expend_group" json:"plan_expend_group"`
	// OriginNum 原始总数量
	OriginNum *uint `db:"origin_num" json:"origin_num"`
	// TotalNum 需求总数量
	TotalNum *uint `db:"total_num" json:"total_num"`
	// SuccessNum 成功数量
	SuccessNum *uint `db:"success_num" json:"success_num"`
	// PendingNum 待处理数量
	PendingNum *uint `db:"pending_num" json:"pending_num"`
	// FailedNum 失败数量
	FailedNum *uint `db:"failed_num" json:"failed_num"`
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
func (z *ZiyanCvmApplySuborder) TableName() table.Name {
	return table.ZiyanCvmApplySuborderTable
}

// InsertValidate validate insert
func (z *ZiyanCvmApplySuborder) InsertValidate() error {
	if len(z.SuborderID) == 0 {
		return errors.New("suborder_id is required")
	}
	if z.OrderID == 0 {
		return errors.New("order_id is required")
	}
	if z.BkBizID == 0 {
		return errors.New("bk_biz_id is required")
	}
	if len(z.Source) == 0 {
		return errors.New("source is required")
	}
	if err := z.ProductType.Validate(); err != nil {
		return err
	}
	if z.RequireType <= 0 {
		return errors.New("require_type is required")
	}
	if len(z.ResourceType) == 0 {
		return errors.New("resource_type is required")
	}
	if len(z.Stage) == 0 {
		return errors.New("stage is required")
	}
	if len(z.Status) == 0 {
		return errors.New("status is required")
	}
	return validator.Validate.Struct(z)
}

// UpdateValidate validate update
func (z *ZiyanCvmApplySuborder) UpdateValidate() error {
	if len(z.SuborderID) == 0 {
		return errors.New("suborder_id is required")
	}
	return validator.Validate.Struct(z)
}
