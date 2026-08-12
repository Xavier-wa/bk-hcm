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

// Package returnplan defines return plan data-service protocol.
package returnplan

import (
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	"hcm/pkg/dal/table/types"
)

// ReturnPlanTicketCreateReq is return plan ticket create request.
type ReturnPlanTicketCreateReq struct {
	Type            enumor.ReturnPlanTicketType   `json:"type" validate:"required"`
	Details         types.JsonField               `json:"details" validate:"required"`
	Applicant       string                        `json:"applicant" validate:"required"`
	BkBizID         int64                         `json:"bk_biz_id" validate:"required"`
	BkBizName       string                        `json:"bk_biz_name" validate:"required"`
	OpProductID     int64                         `json:"op_product_id" validate:"required"`
	OpProductName   string                        `json:"op_product_name" validate:"required"`
	PlanProductID   int64                         `json:"plan_product_id" validate:"required"`
	PlanProductName string                        `json:"plan_product_name" validate:"required"`
	VirtualDeptID   int64                         `json:"virtual_dept_id" validate:"required"`
	VirtualDeptName string                        `json:"virtual_dept_name" validate:"required"`
	Status          enumor.ReturnPlanTicketStatus `json:"status" validate:"required"`
	Message         string                        `json:"message" validate:"omitempty"`
	Remark          string                        `json:"remark" validate:"omitempty"`
	SubmittedAt     string                        `json:"submitted_at" validate:"required"`
}

// Validate validates ReturnPlanTicketCreateReq.
func (r *ReturnPlanTicketCreateReq) Validate() error {
	if err := r.Type.Validate(); err != nil {
		return err
	}

	if err := r.Status.Validate(); err != nil {
		return err
	}

	return validator.Validate.Struct(r)
}

// ReturnPlanTicketUpdateReq is return plan ticket update request.
type ReturnPlanTicketUpdateReq struct {
	ID          string                        `json:"id" validate:"required"`
	Type        enumor.ReturnPlanTicketType   `json:"type" validate:"omitempty"`
	Details     *types.JsonField              `json:"details" validate:"omitempty"`
	Status      enumor.ReturnPlanTicketStatus `json:"status" validate:"omitempty"`
	Message     *string                       `json:"message" validate:"omitempty"`
	Remark      string                        `json:"remark" validate:"omitempty"`
	SubmittedAt string                        `json:"submitted_at" validate:"omitempty"`
}

// Validate validates ReturnPlanTicketUpdateReq.
func (r *ReturnPlanTicketUpdateReq) Validate() error {
	if len(r.Type) > 0 {
		if err := r.Type.Validate(); err != nil {
			return err
		}
	}

	if len(r.Status) > 0 {
		if err := r.Status.Validate(); err != nil {
			return err
		}
	}

	return validator.Validate.Struct(r)
}

// ReturnPlanTicketListReq is return plan ticket list request.
type ReturnPlanTicketListReq struct {
	core.ListReq `json:",inline"`
}

// Validate validates ReturnPlanTicketListReq.
func (r *ReturnPlanTicketListReq) Validate() error {
	return r.ListReq.Validate()
}

// ReturnPlanTicketListResult is return plan ticket list result.
type ReturnPlanTicketListResult struct {
	Count   uint64                          `json:"count"`
	Details []tablert.ReturnPlanTicketTable `json:"details"`
}
