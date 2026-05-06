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

// ZiyanCvmApplyStepColumns defines ziyan_cvm_apply_step's columns.
var ZiyanCvmApplyStepColumns = utils.MergeColumns(nil, ZiyanCvmApplyStepColumnDescriptor)

// ZiyanCvmApplyStepColumnDescriptor is column descriptors.
var ZiyanCvmApplyStepColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "suborder_id", NamedC: "suborder_id", Type: enumor.String},
	{Column: "step_id", NamedC: "step_id", Type: enumor.Numeric},
	{Column: "step_name", NamedC: "step_name", Type: enumor.String},
	{Column: "status", NamedC: "status", Type: enumor.Numeric},
	{Column: "message", NamedC: "message", Type: enumor.String},
	{Column: "total_num", NamedC: "total_num", Type: enumor.Numeric},
	{Column: "success_num", NamedC: "success_num", Type: enumor.Numeric},
	{Column: "failed_num", NamedC: "failed_num", Type: enumor.Numeric},
	{Column: "running_num", NamedC: "running_num", Type: enumor.Numeric},
	{Column: "start_at", NamedC: "start_at", Type: enumor.String},
	{Column: "end_at", NamedC: "end_at", Type: enumor.String},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// ZiyanCvmApplyStep 自研云CVM申请单步骤记录
type ZiyanCvmApplyStep struct {
	// ID 主键ID
	ID string `db:"id" json:"id"`
	// SuborderID 子单ID
	SuborderID string `db:"suborder_id" json:"suborder_id" validate:"max=64"`
	// StepID 步骤ID(1:生成 2:匹配 3:初始化等)
	StepID int `db:"step_id" json:"step_id"`
	// StepName 步骤名称(生成/匹配/初始化/交付等)
	StepName string `db:"step_name" json:"step_name" validate:"max=64"`
	// Status 状态(0:成功 1:失败 2:运行中 3:部分成功等)
	Status *enumor.StepStatusType `db:"status" json:"status"`
	// Message 状态消息/错误信息
	Message string `db:"message" json:"message"`
	// TotalNum 总数量
	TotalNum *uint `db:"total_num" json:"total_num"`
	// SuccessNum 成功数量
	SuccessNum *uint `db:"success_num" json:"success_num"`
	// FailedNum 失败数量
	FailedNum *uint `db:"failed_num" json:"failed_num"`
	// RunningNum 运行中数量
	RunningNum *uint `db:"running_num" json:"running_num"`
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
func (z *ZiyanCvmApplyStep) TableName() table.Name {
	return table.ZiyanCvmApplyStepTable
}

// InsertValidate validate insert
func (z *ZiyanCvmApplyStep) InsertValidate() error {
	if len(z.SuborderID) == 0 {
		return errors.New("suborder_id is required")
	}
	if z.StepID == 0 {
		return errors.New("step_id is required")
	}
	if len(z.StepName) == 0 {
		return errors.New("step_name is required")
	}
	return validator.Validate.Struct(z)
}

// UpdateValidate validate update
func (z *ZiyanCvmApplyStep) UpdateValidate() error {
	return validator.Validate.Struct(z)
}
