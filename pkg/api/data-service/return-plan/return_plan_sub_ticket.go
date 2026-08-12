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

package returnplan

import (
	"errors"
	"fmt"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	tablerst "hcm/pkg/dal/table/return-plan/return-plan-sub-ticket"
	"hcm/pkg/dal/table/types"
)

// ReturnPlanSubTicketBatchCreateReq is return plan sub ticket batch create request.
type ReturnPlanSubTicketBatchCreateReq struct {
	SubTickets []ReturnPlanSubTicketCreateReq `json:"sub_tickets" validate:"required,min=1"`
}

// Validate validates ReturnPlanSubTicketBatchCreateReq.
func (r *ReturnPlanSubTicketBatchCreateReq) Validate() error {
	if len(r.SubTickets) > constant.BatchOperationMaxLimit {
		return fmt.Errorf("sub_tickets count should <= %d", constant.BatchOperationMaxLimit)
	}

	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	for _, t := range r.SubTickets {
		if err := t.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// ReturnPlanSubTicketCreateReq is return plan sub ticket create request.
type ReturnPlanSubTicketCreateReq struct {
	TicketID        string                           `json:"ticket_id" validate:"required"`
	SubType         enumor.ReturnPlanTicketType      `json:"sub_type" validate:"required"`
	SubDetails      types.JsonField                  `json:"sub_details" validate:"required"`
	BkBizID         int64                            `json:"bk_biz_id" validate:"required"`
	BkBizName       string                           `json:"bk_biz_name" validate:"required"`
	OpProductID     int64                            `json:"op_product_id" validate:"required"`
	OpProductName   string                           `json:"op_product_name" validate:"required"`
	PlanProductID   int64                            `json:"plan_product_id" validate:"required"`
	PlanProductName string                           `json:"plan_product_name" validate:"required"`
	VirtualDeptID   int64                            `json:"virtual_dept_id" validate:"required"`
	VirtualDeptName string                           `json:"virtual_dept_name" validate:"required"`
	ObsProject      enumor.ObsProject                `json:"obs_project" validate:"omitempty"`
	ResPoolName     string                           `json:"res_pool_name" validate:"omitempty"`
	Status          enumor.ReturnPlanSubTicketStatus `json:"status" validate:"required"`
	CrpSN           string                           `json:"crp_sn" validate:"omitempty"`
	CrpURL          string                           `json:"crp_url" validate:"omitempty"`
	Message         string                           `json:"message" validate:"omitempty"`
	SubmittedAt     string                           `json:"submitted_at" validate:"required"`
}

// Validate validates ReturnPlanSubTicketCreateReq.
func (r *ReturnPlanSubTicketCreateReq) Validate() error {
	if err := r.SubType.Validate(); err != nil {
		return err
	}

	if err := r.Status.Validate(); err != nil {
		return err
	}

	return validator.Validate.Struct(r)
}

// ReturnPlanSubTicketBatchUpdateReq is return plan sub ticket batch update request.
type ReturnPlanSubTicketBatchUpdateReq struct {
	SubTickets []ReturnPlanSubTicketUpdateReq `json:"sub_tickets" validate:"required,min=1"`
}

// Validate validates ReturnPlanSubTicketBatchUpdateReq.
func (r *ReturnPlanSubTicketBatchUpdateReq) Validate() error {
	if len(r.SubTickets) > constant.BatchOperationMaxLimit {
		return fmt.Errorf("sub_tickets count should <= %d", constant.BatchOperationMaxLimit)
	}

	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	for _, t := range r.SubTickets {
		if err := t.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// ReturnPlanSubTicketUpdateReq is return plan sub ticket update request.
type ReturnPlanSubTicketUpdateReq struct {
	ID          string                           `json:"id" validate:"required"`
	SubType     enumor.ReturnPlanTicketType      `json:"sub_type" validate:"omitempty"`
	SubDetails  *types.JsonField                 `json:"sub_details" validate:"omitempty"`
	ObsProject  enumor.ObsProject                `json:"obs_project" validate:"omitempty"`
	ResPoolName string                           `json:"res_pool_name" validate:"omitempty"`
	Status      enumor.ReturnPlanSubTicketStatus `json:"status" validate:"omitempty"`
	CrpSN       string                           `json:"crp_sn" validate:"omitempty"`
	CrpURL      string                           `json:"crp_url" validate:"omitempty"`
	Message     *string                          `json:"message" validate:"omitempty"`
	SubmittedAt string                           `json:"submitted_at" validate:"omitempty"`
}

// Validate validates ReturnPlanSubTicketUpdateReq.
func (r *ReturnPlanSubTicketUpdateReq) Validate() error {
	if len(r.SubType) > 0 {
		if err := r.SubType.Validate(); err != nil {
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

// ReturnPlanSubTicketStatusUpdateReq is return plan sub ticket status cas update request.
type ReturnPlanSubTicketStatusUpdateReq struct {
	IDs      []string                         `json:"ids" validate:"omitempty"`
	TicketID string                           `json:"ticket_id" validate:"required"`
	Source   enumor.ReturnPlanSubTicketStatus `json:"source" validate:"required"`
	Target   enumor.ReturnPlanSubTicketStatus `json:"target" validate:"required"`
	CrpSN    *string                          `json:"crp_sn" validate:"omitempty"`
	CrpURL   *string                          `json:"crp_url" validate:"omitempty"`
	Message  *string                          `json:"message" validate:"omitempty"`
}

// Validate validates ReturnPlanSubTicketStatusUpdateReq.
func (r ReturnPlanSubTicketStatusUpdateReq) Validate() error {
	if err := r.Source.Validate(); err != nil {
		return err
	}

	if err := r.Target.Validate(); err != nil {
		return err
	}

	// 置为失败态时必须携带失败原因
	if r.Target == enumor.ReturnPlanSubTicketStatusFailed && r.Message == nil {
		return errors.New("failed status, message is required")
	}

	return validator.Validate.Struct(r)
}

// ReturnPlanSubTicketListReq is return plan sub ticket list request.
type ReturnPlanSubTicketListReq struct {
	core.ListReq `json:",inline"`
}

// Validate validates ReturnPlanSubTicketListReq.
func (r *ReturnPlanSubTicketListReq) Validate() error {
	return r.ListReq.Validate()
}

// ReturnPlanSubTicketListResult is return plan sub ticket list result.
type ReturnPlanSubTicketListResult struct {
	Count   uint64                              `json:"count"`
	Details []tablerst.ReturnPlanSubTicketTable `json:"details"`
}
