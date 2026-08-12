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

// Package returnplan defines return plan woa-server API types.
package returnplan

import (
	"errors"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/times"
)

const maxFilterItems = 20

// ListReturnPlanTicketReq is list return plan ticket request.
type ListReturnPlanTicketReq struct {
	BkBizIDs        []int64                         `json:"bk_biz_ids" validate:"omitempty"`
	TicketIDs       []string                        `json:"ticket_ids" validate:"omitempty"`
	Statuses        []enumor.ReturnPlanTicketStatus `json:"statuses" validate:"omitempty"`
	TicketTypes     []enumor.ReturnPlanTicketType   `json:"ticket_types" validate:"omitempty"`
	Applicants      []string                        `json:"applicants" validate:"omitempty"`
	SubmitTimeRange *times.DateRange                `json:"submit_time_range" validate:"omitempty"`
	Page            *core.BasePage                  `json:"page" validate:"required"`
}

// Validate validates ListReturnPlanTicketReq.
func (r *ListReturnPlanTicketReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	if len(r.TicketIDs) > maxFilterItems {
		return errors.New("ticket_ids length should be <= 20")
	}

	if len(r.Statuses) > maxFilterItems {
		return errors.New("statuses length should be <= 20")
	}

	if len(r.TicketTypes) > maxFilterItems {
		return errors.New("ticket_types length should be <= 20")
	}

	if len(r.Applicants) > maxFilterItems {
		return errors.New("applicants length should be <= 20")
	}

	for _, status := range r.Statuses {
		if err := status.Validate(); err != nil {
			return err
		}
	}

	for _, ticketType := range r.TicketTypes {
		if err := ticketType.Validate(); err != nil {
			return err
		}
	}

	if r.SubmitTimeRange != nil {
		if err := r.SubmitTimeRange.Validate(); err != nil {
			return err
		}
	}

	if err := r.Page.Validate(); err != nil {
		return err
	}

	return nil
}

// GenListOption generates data-service list request from ListReturnPlanTicketReq.
func (r *ListReturnPlanTicketReq) GenListOption() (*core.ListReq, error) {
	rules := make([]filter.RuleFactory, 0)

	if len(r.BkBizIDs) > 0 {
		rules = append(rules, tools.ContainersExpression("bk_biz_id", r.BkBizIDs))
	}

	if len(r.TicketIDs) > 0 {
		rules = append(rules, tools.ContainersExpression("id", r.TicketIDs))
	}

	if len(r.Statuses) > 0 {
		rules = append(rules, tools.ContainersExpression("status", r.Statuses))
	}

	if len(r.TicketTypes) > 0 {
		rules = append(rules, tools.ContainersExpression("type", r.TicketTypes))
	}

	if len(r.Applicants) > 0 {
		rules = append(rules, tools.ContainersExpression("applicant", r.Applicants))
	}

	if r.SubmitTimeRange != nil {
		drOpt, err := tools.DateRangeExpression("submitted_at", r.SubmitTimeRange)
		if err != nil {
			return nil, err
		}
		rules = append(rules, drOpt)
	}

	pageCopy := &core.BasePage{
		Count: r.Page.Count,
		Start: r.Page.Start,
		Limit: r.Page.Limit,
		Sort:  r.Page.Sort,
		Order: r.Page.Order,
	}

	if !pageCopy.Count {
		if pageCopy.Sort == "" {
			pageCopy.Sort = "submitted_at"
		}
		if pageCopy.Order == "" {
			pageCopy.Order = core.Descending
		}
	}

	return &core.ListReq{
		Filter: &filter.Expression{
			Op:    filter.And,
			Rules: rules,
		},
		Page: pageCopy,
	}, nil
}

// ListReturnPlanTicketItem is return plan ticket list item.
type ListReturnPlanTicketItem struct {
	ID              string                        `json:"id"`
	BkBizID         int64                         `json:"bk_biz_id"`
	BkBizName       string                        `json:"bk_biz_name"`
	OpProductID     int64                         `json:"op_product_id"`
	OpProductName   string                        `json:"op_product_name"`
	PlanProductID   int64                         `json:"plan_product_id"`
	PlanProductName string                        `json:"plan_product_name"`
	VirtualDeptID   int64                         `json:"virtual_dept_id"`
	VirtualDeptName string                        `json:"virtual_dept_name"`
	Status          enumor.ReturnPlanTicketStatus `json:"status"`
	StatusName      string                        `json:"status_name"`
	TicketType      enumor.ReturnPlanTicketType   `json:"ticket_type"`
	TicketTypeName  string                        `json:"ticket_type_name"`
	Applicant       string                        `json:"applicant"`
	Remark          string                        `json:"remark"`
	SubmittedAt     string                        `json:"submitted_at"`
	CompletedAt     string                        `json:"completed_at"`
	CreatedAt       string                        `json:"created_at"`
	UpdatedAt       string                        `json:"updated_at"`
}

// ListReturnPlanTicketResp is list return plan ticket response.
type ListReturnPlanTicketResp core.ListResultT[ListReturnPlanTicketItem]

// GetReturnPlanTicketResp is get return plan ticket detail response.
type GetReturnPlanTicketResp struct {
	ID         string                     `json:"id"`
	BaseInfo   *GetReturnPlanTicketBase   `json:"base_info"`
	StatusInfo *GetReturnPlanTicketStatus `json:"status_info"`
	Details    []GetReturnPlanDetailItem  `json:"details"`
}

// GetReturnPlanTicketBase is return plan ticket base info.
type GetReturnPlanTicketBase struct {
	Type            enumor.ReturnPlanTicketType `json:"type"`
	TypeName        string                      `json:"type_name"`
	Applicant       string                      `json:"applicant"`
	BkBizID         int64                       `json:"bk_biz_id"`
	BkBizName       string                      `json:"bk_biz_name"`
	OpProductID     int64                       `json:"op_product_id"`
	OpProductName   string                      `json:"op_product_name"`
	PlanProductID   int64                       `json:"plan_product_id"`
	PlanProductName string                      `json:"plan_product_name"`
	VirtualDeptID   int64                       `json:"virtual_dept_id"`
	VirtualDeptName string                      `json:"virtual_dept_name"`
	Remark          string                      `json:"remark"`
	SubmittedAt     string                      `json:"submitted_at"`
}

// GetReturnPlanTicketStatus is return plan ticket status info.
type GetReturnPlanTicketStatus struct {
	Status     enumor.ReturnPlanTicketStatus `json:"status"`
	StatusName string                        `json:"status_name"`
	Message    string                        `json:"message"`
}

// GetReturnPlanDetailItem is return plan detail item in ticket detail response.
// 按 DB 存储的 original/updated 结构返回：仅 updated 为追加，仅 original 为覆盖删除，两者兼有为调整。
type GetReturnPlanDetailItem struct {
	Type     enumor.ReturnPlanTicketType `json:"type"`
	TypeName string                      `json:"type_name"`
	Original *ReturnPlanDetailInfo       `json:"original"`
	Updated  *ReturnPlanDetailInfo       `json:"updated"`
}

// ReturnPlanDetailInfo is the return plan item info for original/updated in ticket detail response.
type ReturnPlanDetailInfo struct {
	ObsProject        enumor.ObsProject `json:"obs_project"`
	PlanTime          string            `json:"plan_time"`
	ResourcePoolName  string            `json:"resource_pool_name"`
	City              string            `json:"city"`
	Zone              string            `json:"zone"`
	InstanceModel     string            `json:"instance_model"`
	CvmAmount         int64             `json:"cvm_amount"`
	InstanceType      string            `json:"instance_type"`
	CoreTypeName      string            `json:"core_type_name"`
	CoreAmount        int64             `json:"core_amount"`
	ReturnReasonClass string            `json:"return_reason_class"`
	Desc              string            `json:"desc"`
}

// ListReturnReasonClassReq is list return reason class request.
type ListReturnReasonClassReq struct {
	ObsProject enumor.ObsProject `json:"obs_project" validate:"required"`
}

// Validate validates ListReturnReasonClassReq.
func (r *ListReturnReasonClassReq) Validate() error {
	if err := r.ObsProject.Validate(); err != nil {
		return err
	}
	return validator.Validate.Struct(r)
}

// ListReturnReasonClassResp is list return reason class response.
type ListReturnReasonClassResp struct {
	ReasonClasses []string `json:"reason_classes"`
}
