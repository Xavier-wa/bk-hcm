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

// BatchCreateZiyanCvmApplyStepReq batch create request
type BatchCreateZiyanCvmApplyStepReq struct {
	ApplySteps []ZiyanCvmApplyStepCreateReq `json:"apply_steps" validate:"required,max=100"`
}

// Validate ...
func (c *BatchCreateZiyanCvmApplyStepReq) Validate() error {
	if len(c.ApplySteps) == 0 || len(c.ApplySteps) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"apply_steps count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.ApplySteps {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmApplyStepCreateReq create request
type ZiyanCvmApplyStepCreateReq struct {
	SuborderID string                   `json:"suborder_id" validate:"required,max=64"`
	StepID     int                      `json:"step_id" validate:"required"`
	StepName   string                   `json:"step_name" validate:"required,max=64"`
	Status     tasktypes.StepStatusType `json:"status" validate:"omitempty"`
	Message    string                   `json:"message" validate:"omitempty"`
	TotalNum   uint                     `json:"total_num" validate:"omitempty"`
	SuccessNum uint                     `json:"success_num" validate:"omitempty"`
	FailedNum  uint                     `json:"failed_num" validate:"omitempty"`
	RunningNum uint                     `json:"running_num" validate:"omitempty"`
	StartAt    string                   `json:"start_at" validate:"omitempty"`
	EndAt      string                   `json:"end_at" validate:"omitempty"`
	// CreatedAt 原始创建时间（用于数据迁移，保留历史时间）
	CreatedAt types.Time `json:"created_at" validate:"omitempty"`
	UpdatedAt types.Time `json:"updated_at" validate:"omitempty"`
}

// Validate ...
func (c *ZiyanCvmApplyStepCreateReq) Validate() error {
	return validator.Validate.Struct(c)
}

// ZiyanCvmApplyStepListReq list request
type ZiyanCvmApplyStepListReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
	Page   *core.BasePage     `json:"page" validate:"required"`
	Fields []string           `json:"fields" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmApplyStepListReq) Validate() error {
	return validator.Validate.Struct(req)
}

// ZiyanCvmApplyStepListResult list result
type ZiyanCvmApplyStepListResult = core.ListResultT[*cvmapplytable.ZiyanCvmApplyStep]

// ZiyanCvmApplyStepListResp define list resp.
type ZiyanCvmApplyStepListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *ZiyanCvmApplyStepListResult `json:"data"`
}

// BatchUpdateZiyanCvmApplyStepReq batch update request
type BatchUpdateZiyanCvmApplyStepReq struct {
	ApplySteps []ZiyanCvmApplyStepUpdateReq `json:"apply_steps" validate:"required,max=100"`
}

// Validate ...
func (c *BatchUpdateZiyanCvmApplyStepReq) Validate() error {
	if len(c.ApplySteps) == 0 || len(c.ApplySteps) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"apply_steps count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.ApplySteps {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmApplyStepUpdateReq update request
type ZiyanCvmApplyStepUpdateReq struct {
	ID         string                    `json:"id" validate:"required"`
	SuborderID string                    `json:"suborder_id" validate:"omitempty,max=64"`
	StepID     *int                      `json:"step_id" validate:"omitempty"`
	StepName   string                    `json:"step_name" validate:"omitempty,max=64"`
	Status     *tasktypes.StepStatusType `json:"status" validate:"omitempty"`
	Message    string                    `json:"message" validate:"omitempty"`
	TotalNum   *uint                     `json:"total_num" validate:"omitempty"`
	SuccessNum *uint                     `json:"success_num" validate:"omitempty"`
	FailedNum  *uint                     `json:"failed_num" validate:"omitempty"`
	RunningNum *uint                     `json:"running_num" validate:"omitempty"`
	StartAt    string                    `json:"start_at" validate:"omitempty"`
	EndAt      string                    `json:"end_at" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmApplyStepUpdateReq) Validate() error {
	return validator.Validate.Struct(req)
}
