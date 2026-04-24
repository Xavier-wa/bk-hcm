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

// ZiyanCvmApplyInitTaskColumns defines ziyan_cvm_apply_init_task's columns.
var ZiyanCvmApplyInitTaskColumns = utils.MergeColumns(nil, ZiyanCvmApplyInitTaskColumnDescriptor)

// ZiyanCvmApplyInitTaskColumnDescriptor is column descriptors.
var ZiyanCvmApplyInitTaskColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "suborder_id", NamedC: "suborder_id", Type: enumor.String},
	{Column: "ip", NamedC: "ip", Type: enumor.String},
	{Column: "task_id", NamedC: "task_id", Type: enumor.String},
	{Column: "task_link", NamedC: "task_link", Type: enumor.String},
	{Column: "status", NamedC: "status", Type: enumor.Numeric},
	{Column: "message", NamedC: "message", Type: enumor.String},
	{Column: "start_at", NamedC: "start_at", Type: enumor.String},
	{Column: "end_at", NamedC: "end_at", Type: enumor.String},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// ZiyanCvmApplyInitTask 自研云CVM申请单初始化任务记录
type ZiyanCvmApplyInitTask struct {
	// ID 主键ID
	ID string `db:"id" json:"id"`
	// SuborderID 子单ID
	SuborderID string `db:"suborder_id" json:"suborder_id" validate:"max=64"`
	// IP 设备IP地址
	IP string `db:"ip" json:"ip" validate:"max=64"`
	// TaskID 初始化任务ID(如SCR任务ID)
	TaskID string `db:"task_id" json:"task_id" validate:"max=128"`
	// TaskLink 初始化任务链接
	TaskLink string `db:"task_link" json:"task_link" validate:"max=512"`
	// Status 状态(-1:默认 0:成功 1:失败 2:运行中等)
	Status *enumor.InitStepStatus `db:"status" json:"status"`
	// Message 状态消息/错误信息
	Message string `db:"message" json:"message"`
	// StartAt 开始时间
	StartAt string `db:"start_at" json:"start_at"`
	// EndAt 结束时间
	EndAt string `db:"end_at" json:"end_at"`
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
func (z *ZiyanCvmApplyInitTask) TableName() table.Name {
	return table.ZiyanCvmApplyInitTaskTable
}

// InsertValidate validate insert
func (z *ZiyanCvmApplyInitTask) InsertValidate() error {
	if len(z.SuborderID) == 0 {
		return errors.New("suborder_id is required")
	}
	if len(z.IP) == 0 {
		return errors.New("ip is required")
	}
	return validator.Validate.Struct(z)
}

// UpdateValidate validate update
func (z *ZiyanCvmApplyInitTask) UpdateValidate() error {
	return validator.Validate.Struct(z)
}
