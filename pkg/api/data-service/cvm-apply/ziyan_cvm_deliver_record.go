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

// BatchCreateZiyanCvmDeliverRecordReq batch create request
type BatchCreateZiyanCvmDeliverRecordReq struct {
	Records []ZiyanCvmDeliverRecordCreateReq `json:"records" validate:"required,max=100"`
}

// Validate ...
func (c *BatchCreateZiyanCvmDeliverRecordReq) Validate() error {
	if len(c.Records) == 0 || len(c.Records) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"records count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.Records {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmDeliverRecordCreateReq create request
type ZiyanCvmDeliverRecordCreateReq struct {
	SuborderID       string                      `json:"suborder_id" validate:"required,max=64"`
	IP               string                      `json:"ip" validate:"required,max=45"`
	AssetID          string                      `json:"asset_id" validate:"omitempty,max=64"`
	Status           tasktypes.DeliverStepStatus `json:"status" validate:"omitempty"`
	Message          string                      `json:"message" validate:"max=512"`
	Deliverer        string                      `json:"deliverer" validate:"max=32"`
	GenerateTaskID   string                      `json:"generate_task_id" validate:"max=64"`
	GenerateTaskLink string                      `json:"generate_task_link" validate:"max=512"`
	InitTaskID       string                      `json:"init_task_id" validate:"max=64"`
	InitTaskLink     string                      `json:"init_task_link" validate:"max=512"`
	IsManualMatched  bool                        `json:"is_manual_matched" validate:"omitempty"`
	StartAt          string                      `json:"start_at" validate:"omitempty"`
	EndAt            string                      `json:"end_at" validate:"omitempty"`
	// CreatedAt 原始创建时间（用于数据迁移，保留历史时间）
	CreatedAt types.Time `json:"created_at" validate:"omitempty"`
	UpdatedAt types.Time `json:"updated_at" validate:"omitempty"`
}

// Validate ...
func (c *ZiyanCvmDeliverRecordCreateReq) Validate() error {
	return validator.Validate.Struct(c)
}

// ZiyanCvmDeliverRecordListReq list request
type ZiyanCvmDeliverRecordListReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
	Page   *core.BasePage     `json:"page" validate:"required"`
	Fields []string           `json:"fields" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmDeliverRecordListReq) Validate() error {
	return validator.Validate.Struct(req)
}

// ZiyanCvmDeliverRecordListResult list result
type ZiyanCvmDeliverRecordListResult = core.ListResultT[*cvmapplytable.ZiyanCvmDeliverRecord]

// ZiyanCvmDeliverRecordListResp define list resp.
type ZiyanCvmDeliverRecordListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *ZiyanCvmDeliverRecordListResult `json:"data"`
}

// BatchUpdateZiyanCvmDeliverRecordReq batch update request
type BatchUpdateZiyanCvmDeliverRecordReq struct {
	Records []ZiyanCvmDeliverRecordUpdateReq `json:"records" validate:"required,max=100"`
}

// Validate ...
func (c *BatchUpdateZiyanCvmDeliverRecordReq) Validate() error {
	if len(c.Records) == 0 || len(c.Records) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"records count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.Records {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmDeliverRecordUpdateReq update request
type ZiyanCvmDeliverRecordUpdateReq struct {
	ID               string                       `json:"id" validate:"required"`
	SuborderID       string                       `json:"suborder_id" validate:"omitempty,max=64"`
	IP               string                       `json:"ip" validate:"omitempty,max=45"`
	AssetID          string                       `json:"asset_id" validate:"omitempty,max=64"`
	Status           *tasktypes.DeliverStepStatus `json:"status" validate:"omitempty"`
	Message          string                       `json:"message" validate:"omitempty,max=512"`
	Deliverer        string                       `json:"deliverer" validate:"omitempty,max=32"`
	GenerateTaskID   string                       `json:"generate_task_id" validate:"omitempty,max=64"`
	GenerateTaskLink string                       `json:"generate_task_link" validate:"omitempty,max=512"`
	InitTaskID       string                       `json:"init_task_id" validate:"omitempty,max=64"`
	InitTaskLink     string                       `json:"init_task_link" validate:"omitempty,max=512"`
	IsManualMatched  *bool                        `json:"is_manual_matched" validate:"omitempty"`
	StartAt          string                       `json:"start_at" validate:"omitempty"`
	EndAt            string                       `json:"end_at" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmDeliverRecordUpdateReq) Validate() error {
	return validator.Validate.Struct(req)
}
