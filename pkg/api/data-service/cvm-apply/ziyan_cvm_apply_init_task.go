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

// BatchCreateZiyanCvmApplyInitTaskReq batch create request
type BatchCreateZiyanCvmApplyInitTaskReq struct {
	InitTasks []ZiyanCvmApplyInitTaskCreateReq `json:"init_tasks" validate:"required,max=100"`
}

// Validate ...
func (c *BatchCreateZiyanCvmApplyInitTaskReq) Validate() error {
	if len(c.InitTasks) == 0 || len(c.InitTasks) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"init_tasks count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.InitTasks {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmApplyInitTaskCreateReq create request
type ZiyanCvmApplyInitTaskCreateReq struct {
	SuborderID string                   `json:"suborder_id" validate:"required,max=64"`
	IP         string                   `json:"ip" validate:"required,max=64"`
	TaskID     string                   `json:"task_id" validate:"max=128"`
	TaskLink   string                   `json:"task_link" validate:"max=512"`
	Status     tasktypes.InitStepStatus `json:"status" validate:"omitempty"`
	Message    string                   `json:"message" validate:"omitempty"`
	StartAt    string                   `json:"start_at" validate:"omitempty"`
	EndAt      string                   `json:"end_at" validate:"omitempty"`
	// CreatedAt 原始创建时间（用于数据迁移，保留历史时间）
	CreatedAt types.Time `json:"created_at" validate:"omitempty"`
	UpdatedAt types.Time `json:"updated_at" validate:"omitempty"`
}

// Validate ...
func (c *ZiyanCvmApplyInitTaskCreateReq) Validate() error {
	return validator.Validate.Struct(c)
}

// ZiyanCvmApplyInitTaskListReq list request
type ZiyanCvmApplyInitTaskListReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
	Page   *core.BasePage     `json:"page" validate:"required"`
	Fields []string           `json:"fields" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmApplyInitTaskListReq) Validate() error {
	return validator.Validate.Struct(req)
}

// ZiyanCvmApplyInitTaskListResult list result
type ZiyanCvmApplyInitTaskListResult = core.ListResultT[*cvmapplytable.ZiyanCvmApplyInitTask]

// ZiyanCvmApplyInitTaskListResp define list resp.
type ZiyanCvmApplyInitTaskListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *ZiyanCvmApplyInitTaskListResult `json:"data"`
}

// BatchUpdateZiyanCvmApplyInitTaskReq batch update request
type BatchUpdateZiyanCvmApplyInitTaskReq struct {
	InitTasks []ZiyanCvmApplyInitTaskUpdateReq `json:"init_tasks" validate:"required,max=100"`
}

// Validate ...
func (c *BatchUpdateZiyanCvmApplyInitTaskReq) Validate() error {
	if len(c.InitTasks) == 0 || len(c.InitTasks) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"init_tasks count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.InitTasks {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmApplyInitTaskUpdateReq update request
type ZiyanCvmApplyInitTaskUpdateReq struct {
	ID         string                    `json:"id" validate:"required"`
	SuborderID string                    `json:"suborder_id" validate:"omitempty,max=64"`
	IP         string                    `json:"ip" validate:"omitempty,max=64"`
	TaskID     string                    `json:"task_id" validate:"omitempty,max=128"`
	TaskLink   string                    `json:"task_link" validate:"omitempty,max=512"`
	Status     *tasktypes.InitStepStatus `json:"status" validate:"omitempty"`
	Message    string                    `json:"message" validate:"omitempty"`
	StartAt    string                    `json:"start_at" validate:"omitempty"`
	EndAt      string                    `json:"end_at" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmApplyInitTaskUpdateReq) Validate() error {
	return validator.Validate.Struct(req)
}
