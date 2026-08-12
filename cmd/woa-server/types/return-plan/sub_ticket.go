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

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/runtime/filter"
)

// ListReturnPlanSubTicketReq is list return plan sub ticket request.
type ListReturnPlanSubTicketReq struct {
	BkBizID        int64                              `json:"-"`
	TicketID       string                             `json:"ticket_id" validate:"required"`
	Statuses       []enumor.ReturnPlanSubTicketStatus `json:"statuses" validate:"omitempty"`
	SubTicketTypes []enumor.ReturnPlanTicketType      `json:"sub_ticket_types" validate:"omitempty"`
	Page           *core.BasePage                     `json:"page" validate:"required"`
}

// Validate validates ListReturnPlanSubTicketReq.
func (r *ListReturnPlanSubTicketReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	if len(r.Statuses) > maxFilterItems {
		return errors.New("statuses length should be <= 20")
	}

	if len(r.SubTicketTypes) > maxFilterItems {
		return errors.New("sub_ticket_types length should be <= 20")
	}

	for _, status := range r.Statuses {
		if err := status.Validate(); err != nil {
			return err
		}
	}

	for _, ticketType := range r.SubTicketTypes {
		if err := ticketType.Validate(); err != nil {
			return err
		}
	}

	return r.Page.Validate()
}

// GenListOption generates data-service list request from ListReturnPlanSubTicketReq.
func (r *ListReturnPlanSubTicketReq) GenListOption() core.ListReq {
	rules := make([]filter.RuleFactory, 0)
	rules = append(rules, tools.RuleEqual("ticket_id", r.TicketID))

	if r.BkBizID > 0 {
		rules = append(rules, tools.RuleEqual("bk_biz_id", r.BkBizID))
	}

	if len(r.Statuses) > 0 {
		rules = append(rules, tools.ContainersExpression("status", r.Statuses))
	}

	if len(r.SubTicketTypes) > 0 {
		rules = append(rules, tools.ContainersExpression("sub_type", r.SubTicketTypes))
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

	return core.ListReq{
		Filter: &filter.Expression{
			Op:    filter.And,
			Rules: rules,
		},
		Page: pageCopy,
	}
}

// ListReturnPlanSubTicketItem is list return plan sub ticket item.
type ListReturnPlanSubTicketItem struct {
	ID                string                           `json:"id"`
	Status            enumor.ReturnPlanSubTicketStatus `json:"status"`
	StatusName        string                           `json:"status_name"`
	SubTicketType     enumor.ReturnPlanTicketType      `json:"sub_ticket_type"`
	SubTicketTypeName string                           `json:"sub_ticket_type_name"`
	ObsProject        enumor.ObsProject                `json:"obs_project"`
	ResourcePoolName  string                           `json:"resource_pool_name"`
	CrpSN             string                           `json:"crp_sn"`
	CrpURL            string                           `json:"crp_url"`
	Message           string                           `json:"message"`
	SubmittedAt       string                           `json:"submitted_at"`
	CreatedAt         string                           `json:"created_at"`
	UpdatedAt         string                           `json:"updated_at"`
}

// ListReturnPlanSubTicketResp is list return plan sub ticket response.
type ListReturnPlanSubTicketResp core.ListResultT[ListReturnPlanSubTicketItem]

// GetReturnPlanSubTicketDetailResp is get return plan sub ticket detail response.
type GetReturnPlanSubTicketDetailResp struct {
	ID                string                           `json:"id"`
	TicketID          string                           `json:"ticket_id"`
	SubTicketType     enumor.ReturnPlanTicketType      `json:"sub_ticket_type"`
	SubTicketTypeName string                           `json:"sub_ticket_type_name"`
	Status            enumor.ReturnPlanSubTicketStatus `json:"status"`
	StatusName        string                           `json:"status_name"`
	ObsProject        enumor.ObsProject                `json:"obs_project"`
	ResourcePoolName  string                           `json:"resource_pool_name"`
	CrpSN             string                           `json:"crp_sn"`
	CrpURL            string                           `json:"crp_url"`
	Message           string                           `json:"message"`
	SubmittedAt       string                           `json:"submitted_at"`
	CreatedAt         string                           `json:"created_at"`
	UpdatedAt         string                           `json:"updated_at"`
}
