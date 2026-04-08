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

// ZiyanCvmDeliverRecordColumns defines ziyan_cvm_deliver_record's columns.
var ZiyanCvmDeliverRecordColumns = utils.MergeColumns(nil, ZiyanCvmDeliverRecordColumnDescriptor)

// ZiyanCvmDeliverRecordColumnDescriptor is column descriptors.
var ZiyanCvmDeliverRecordColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "suborder_id", NamedC: "suborder_id", Type: enumor.String},
	{Column: "ip", NamedC: "ip", Type: enumor.String},
	{Column: "asset_id", NamedC: "asset_id", Type: enumor.String},
	{Column: "status", NamedC: "status", Type: enumor.Numeric},
	{Column: "message", NamedC: "message", Type: enumor.String},
	{Column: "deliverer", NamedC: "deliverer", Type: enumor.String},
	{Column: "generate_task_id", NamedC: "generate_task_id", Type: enumor.String},
	{Column: "generate_task_link", NamedC: "generate_task_link", Type: enumor.String},
	{Column: "init_task_id", NamedC: "init_task_id", Type: enumor.String},
	{Column: "init_task_link", NamedC: "init_task_link", Type: enumor.String},
	{Column: "is_manual_matched", NamedC: "is_manual_matched", Type: enumor.Boolean},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
	{Column: "start_at", NamedC: "start_at", Type: enumor.Time},
	{Column: "end_at", NamedC: "end_at", Type: enumor.Time},
}

// ZiyanCvmDeliverRecord 自研云CVM设备交付记录
type ZiyanCvmDeliverRecord struct {
	// ID 主键ID
	ID string `db:"id" json:"id"`
	// SuborderID 子订单ID
	SuborderID string `db:"suborder_id" json:"suborder_id" validate:"max=64"`
	// IP IP地址
	IP string `db:"ip" json:"ip" validate:"max=45"`
	// AssetID 固资号
	AssetID string `db:"asset_id" json:"asset_id" validate:"max=64"`
	// Status 状态（-1:默认 0:成功 1:失败 2:处理中）
	Status *enumor.DeliverStepStatus `db:"status" json:"status"`
	// Message 状态消息
	Message string `db:"message" json:"message" validate:"max=512"`
	// Deliverer 交付方式
	Deliverer string `db:"deliverer" json:"deliverer" validate:"max=32"`
	// GenerateTaskID 生产任务ID
	GenerateTaskID string `db:"generate_task_id" json:"generate_task_id" validate:"max=64"`
	// GenerateTaskLink 生产任务链接
	GenerateTaskLink string `db:"generate_task_link" json:"generate_task_link" validate:"max=512"`
	// InitTaskID 初始化任务ID
	InitTaskID string `db:"init_task_id" json:"init_task_id" validate:"max=64"`
	// InitTaskLink 初始化任务链接
	InitTaskLink string `db:"init_task_link" json:"init_task_link" validate:"max=512"`
	// IsManualMatched 是否手动匹配（0-否, 1-是）
	IsManualMatched *bool `db:"is_manual_matched" json:"is_manual_matched"`
	// CreatedAt 创建时间（毫秒精度）
	CreatedAt types.Time `db:"created_at" json:"created_at"`
	// UpdatedAt 更新时间（毫秒精度）
	UpdatedAt types.Time `db:"updated_at" validate:"excluded_unless" json:"updated_at"`
	// StartAt 开始时间
	StartAt string `db:"start_at" json:"start_at"`
	// EndAt 结束时间
	EndAt string `db:"end_at" json:"end_at"`
}

// TableName 表名
func (z *ZiyanCvmDeliverRecord) TableName() table.Name {
	return table.ZiyanCvmDeliverRecordTable
}

// InsertValidate validate insert
func (z *ZiyanCvmDeliverRecord) InsertValidate() error {
	if len(z.SuborderID) == 0 {
		return errors.New("suborder_id is required")
	}
	if len(z.IP) == 0 {
		return errors.New("ip is required")
	}
	return validator.Validate.Struct(z)
}

// UpdateValidate validate update
func (z *ZiyanCvmDeliverRecord) UpdateValidate() error {
	return validator.Validate.Struct(z)
}
