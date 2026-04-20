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
	"hcm/pkg/criteria/errf"
	"hcm/pkg/criteria/validator"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
)

// BatchCreateZiyanCvmGenerateRecordReq batch create request
type BatchCreateZiyanCvmGenerateRecordReq struct {
	GenerateRecords []ZiyanCvmGenerateRecordCreateReq `json:"generate_records" validate:"required,max=100"`
}

// Validate ...
func (c *BatchCreateZiyanCvmGenerateRecordReq) Validate() error {
	if len(c.GenerateRecords) == 0 || len(c.GenerateRecords) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"generate_records count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.GenerateRecords {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmGenerateRecordCreateReq create request
type ZiyanCvmGenerateRecordCreateReq struct {
	GenerateID      string                       `json:"generate_id" validate:"omitempty"`
	SuborderID      string                       `json:"suborder_id" validate:"required,max=64"`
	GenerateType    string                       `json:"generate_type" validate:"max=32"`
	TaskID          string                       `json:"task_id" validate:"max=128"`
	TaskLink        string                       `json:"task_link" validate:"max=512"`
	RequestInfo     string                       `json:"request_info" validate:"omitempty"`
	Status          tasktypes.GenerateStepStatus `json:"status" validate:"omitempty"`
	IsMatched       bool                         `json:"is_matched" validate:"omitempty"`
	Message         string                       `json:"message" validate:"omitempty"`
	TotalNum        uint                         `json:"total_num" validate:"omitempty"`
	SuccessNum      uint                         `json:"success_num" validate:"omitempty"`
	SuccessList     types.JsonField              `json:"success_list" validate:"omitempty"`
	StartAt         string                       `json:"start_at" validate:"omitempty"`
	EndAt           string                       `json:"end_at" validate:"omitempty"`
	IsManualMatched bool                         `json:"is_manual_matched" validate:"omitempty"`
	// CreatedAt 原始创建时间（用于数据迁移，保留历史时间）
	CreatedAt types.Time `json:"created_at" validate:"omitempty"`
	UpdatedAt types.Time `json:"updated_at" validate:"omitempty"`
}

// Validate ...
func (c *ZiyanCvmGenerateRecordCreateReq) Validate() error {
	return validator.Validate.Struct(c)
}

// ZiyanCvmGenerateRecordListReq list request
type ZiyanCvmGenerateRecordListReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
	Page   *core.BasePage     `json:"page" validate:"required"`
	Fields []string           `json:"fields" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmGenerateRecordListReq) Validate() error {
	return validator.Validate.Struct(req)
}

// ZiyanCvmGenerateRecordListResult list result
type ZiyanCvmGenerateRecordListResult = core.ListResultT[*cvmapplytable.ZiyanCvmGenerateRecord]

// ZiyanCvmGenerateRecordListResp define list resp.
type ZiyanCvmGenerateRecordListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *ZiyanCvmGenerateRecordListResult `json:"data"`
}

// BatchUpdateZiyanCvmGenerateRecordReq batch update request
type BatchUpdateZiyanCvmGenerateRecordReq struct {
	GenerateRecords []ZiyanCvmGenerateRecordUpdateReq `json:"generate_records" validate:"required,max=100"`
}

// Validate ...
func (c *BatchUpdateZiyanCvmGenerateRecordReq) Validate() error {
	if len(c.GenerateRecords) == 0 || len(c.GenerateRecords) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"generate_records count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.GenerateRecords {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmGenerateRecordUpdateReq update request
type ZiyanCvmGenerateRecordUpdateReq struct {
	GenerateID      string                        `json:"generate_id" validate:"required"`
	SuborderID      string                        `json:"suborder_id" validate:"omitempty,max=64"`
	GenerateType    *string                       `json:"generate_type" validate:"omitempty,max=32"`
	TaskID          *string                       `json:"task_id" validate:"omitempty,max=128"`
	TaskLink        *string                       `json:"task_link" validate:"omitempty,max=512"`
	RequestInfo     string                        `json:"request_info" validate:"omitempty"`
	Status          *tasktypes.GenerateStepStatus `json:"status" validate:"omitempty"`
	IsMatched       *bool                         `json:"is_matched" validate:"omitempty"`
	Message         *string                       `json:"message" validate:"omitempty"`
	TotalNum        *uint                         `json:"total_num" validate:"omitempty"`
	SuccessNum      *uint                         `json:"success_num" validate:"omitempty"`
	SuccessList     *types.JsonField              `json:"success_list" validate:"omitempty"`
	StartAt         *string                       `json:"start_at" validate:"omitempty"`
	EndAt           *string                       `json:"end_at" validate:"omitempty"`
	IsManualMatched *bool                         `json:"is_manual_matched" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmGenerateRecordUpdateReq) Validate() error {
	return validator.Validate.Struct(req)
}
