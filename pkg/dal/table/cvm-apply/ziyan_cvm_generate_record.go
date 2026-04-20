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

// ZiyanCvmGenerateRecordColumns defines ziyan_cvm_generate_record's columns.
var ZiyanCvmGenerateRecordColumns = utils.MergeColumns(nil, ZiyanCvmGenerateRecordColumnDescriptor)

// ZiyanCvmGenerateRecordColumnDescriptor is column descriptors.
var ZiyanCvmGenerateRecordColumnDescriptor = utils.ColumnDescriptors{
	{Column: "generate_id", NamedC: "generate_id", Type: enumor.String},
	{Column: "suborder_id", NamedC: "suborder_id", Type: enumor.String},
	{Column: "generate_type", NamedC: "generate_type", Type: enumor.String},
	{Column: "task_id", NamedC: "task_id", Type: enumor.String},
	{Column: "task_link", NamedC: "task_link", Type: enumor.String},
	{Column: "request_info", NamedC: "request_info", Type: enumor.String},
	{Column: "status", NamedC: "status", Type: enumor.Numeric},
	{Column: "is_matched", NamedC: "is_matched", Type: enumor.Boolean},
	{Column: "message", NamedC: "message", Type: enumor.String},
	{Column: "total_num", NamedC: "total_num", Type: enumor.Numeric},
	{Column: "success_num", NamedC: "success_num", Type: enumor.Numeric},
	{Column: "success_list", NamedC: "success_list", Type: enumor.Json},
	{Column: "start_at", NamedC: "start_at", Type: enumor.String},
	{Column: "end_at", NamedC: "end_at", Type: enumor.String},
	{Column: "is_manual_matched", NamedC: "is_manual_matched", Type: enumor.Boolean},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// ZiyanCvmGenerateRecord 自研云CVM申请单生产任务记录
type ZiyanCvmGenerateRecord struct {
	// GenerateID 生产ID(主键)
	GenerateID string `db:"generate_id" json:"generate_id"`
	// SuborderID 子单ID
	SuborderID string `db:"suborder_id" json:"suborder_id" validate:"max=64"`
	// GenerateType 生产类型(QCLOUDCVM/PM等)
	GenerateType string `db:"generate_type" json:"generate_type" validate:"max=32"`
	// TaskID 生成任务ID(如云梯任务ID)
	TaskID string `db:"task_id" json:"task_id" validate:"max=128"`
	// TaskLink 生成任务链接
	TaskLink string `db:"task_link" json:"task_link" validate:"max=512"`
	// RequestInfo 请求信息
	RequestInfo string `db:"request_info" json:"request_info"`
	// Status 状态(-1:默认 0:成功 1:进行中 2:失败)
	Status *enumor.GenerateStepStatus `db:"status" json:"status"`
	// IsMatched 是否已匹配(0:否 1:是)
	IsMatched *bool `db:"is_matched" json:"is_matched"`
	// Message 状态消息/错误信息
	Message string `db:"message" json:"message"`
	// TotalNum 总数量
	TotalNum *uint `db:"total_num" json:"total_num"`
	// SuccessNum 成功数量
	SuccessNum *uint `db:"success_num" json:"success_num"`
	// SuccessList 成功列表(IP列表等)
	SuccessList types.JsonField `db:"success_list" json:"success_list"`
	// StartAt 开始时间
	StartAt string `db:"start_at" json:"start_at"`
	// EndAt 结束时间
	EndAt string `db:"end_at" json:"end_at"`
	// IsManualMatched 是否手工匹配
	IsManualMatched bool `db:"is_manual_matched" json:"is_manual_matched"`
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
func (z *ZiyanCvmGenerateRecord) TableName() table.Name {
	return table.ZiyanCvmGenerateRecordTable
}

// InsertValidate validate insert
func (z *ZiyanCvmGenerateRecord) InsertValidate() error {
	if len(z.SuborderID) == 0 {
		return errors.New("suborder_id is required")
	}
	return validator.Validate.Struct(z)
}

// UpdateValidate validate update
func (z *ZiyanCvmGenerateRecord) UpdateValidate() error {
	return validator.Validate.Struct(z)
}
