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

package plan

import (
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/dao/tools"
	dtypes "hcm/pkg/dal/dao/types"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/runtime/filter"
)

// SubTicketInfo resource plan sub ticket info.
type SubTicketInfo struct {
	ID               string                    `json:"id"`
	ParentTicketID   string                    `json:"parent_ticket_id"`
	BkBizID          int64                     `json:"bk_biz_id"`
	BkBizName        string                    `json:"bk_biz_name"`
	OpProductID      int64                     `json:"op_product_id"`
	OpProductName    string                    `json:"op_product_name"`
	PlanProductID    int64                     `json:"plan_product_id"`
	PlanProductName  string                    `json:"plan_product_name"`
	VirtualDeptID    int64                     `json:"virtual_dept_id"`
	VirtualDeptName  string                    `json:"virtual_dept_name"`
	DemandClass      enumor.DemandClass        `json:"demand_class"`
	Type             enumor.RPTicketType       `json:"type"`
	OriginalCpuCore  int64                     `json:"original_cpu_core"`
	OriginalMemory   int64                     `json:"original_memory"`
	OriginalDiskSize int64                     `json:"original_disk_size"`
	UpdatedCpuCore   int64                     `json:"updated_cpu_core"`
	UpdatedMemory    int64                     `json:"updated_memory"`
	UpdatedDiskSize  int64                     `json:"updated_disk_size"`
	Demands          rpt.ResPlanDemands        `json:"demands"`
	SubmittedAt      string                    `json:"submitted_at"`
	Status           enumor.RPSubTicketStatus  `json:"status"`
	Stage            enumor.RPSubTicketStage   `json:"stage"`
	AdminAuditStatus enumor.RPAdminAuditStatus `json:"admin_audit_status"`
	// AdminAuditOperator HCM department admin audit operator RTX, set when AdminAuditStatus is done.
	AdminAuditOperator string `json:"admin_audit_operator,omitempty"`
	CrpSN              string `json:"crp_sn"`
	CrpURL             string `json:"crp_url"`
	Applicant          string `json:"applicant"`
}

// ListResPlanSubTicketReq is list resource plan sub ticket request.
type ListResPlanSubTicketReq struct {
	BizID          int64                      `json:"-"`
	TicketID       string                     `json:"ticket_id" validate:"required"`
	Statuses       []enumor.RPSubTicketStatus `json:"statuses" validate:"omitempty"`
	SubTicketTypes []enumor.RPTicketType      `json:"sub_ticket_types" validate:"omitempty"`
	Page           *core.BasePage             `json:"page" validate:"required"`
}

// Validate whether ListResPlanSubTicketReq is valid.
func (r *ListResPlanSubTicketReq) Validate() error {
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

	if r.Page == nil {
		return errf.Newf(errf.InvalidParameter, "page can't be nil")
	}

	if err := r.Page.Validate(); err != nil {
		return err
	}

	return validator.Validate.Struct(r)
}

// GenListOption generate list option by list resource plan ticket request.
func (r *ListResPlanSubTicketReq) GenListOption() core.ListReq {
	rules := make([]filter.RuleFactory, 0)
	rules = append(rules, tools.RuleEqual("ticket_id", r.TicketID))

	if r.BizID != 0 && r.BizID != constant.AttachedAllBiz {
		rules = append(rules, tools.RuleEqual("bk_biz_id", r.BizID))
	}

	if len(r.Statuses) > 0 {
		rules = append(rules, tools.ContainersExpression("status", r.Statuses))
	}

	if len(r.SubTicketTypes) > 0 {
		rules = append(rules, tools.ContainersExpression("sub_type", r.SubTicketTypes))
	}

	// copy page for modifying.
	pageCopy := &core.BasePage{
		Count: r.Page.Count,
		Start: r.Page.Start,
		Limit: r.Page.Limit,
		Sort:  r.Page.Sort,
		Order: r.Page.Order,
	}

	// if count == false, default sort by submitted_at desc.
	if !pageCopy.Count {
		if pageCopy.Sort == "" {
			pageCopy.Sort = "submitted_at"
		}
		if pageCopy.Order == "" {
			pageCopy.Order = core.Descending
		}
	}

	opt := core.ListReq{
		Filter: &filter.Expression{
			Op:    filter.And,
			Rules: rules,
		},
		Page: pageCopy,
	}

	return opt
}

// ListResPlanSubTicketResp is list resource plan sub ticket response.
type ListResPlanSubTicketResp dtypes.ListResult[ListResPlanSubTicketItem]

// ListResPlanSubTicketItem is list resource plan sub ticket item.
type ListResPlanSubTicketItem struct {
	ID             string                   `json:"id"`
	Status         enumor.RPSubTicketStatus `json:"status"`
	StatusName     string                   `json:"status_name"`
	Message        string                   `json:"message"`
	SubDemands     types.JsonField          `json:"-"` // Demands 资源需求明细，目前用于后端拆单逻辑，不需要返回给前端
	Stage          enumor.RPSubTicketStage  `json:"stage"`
	SubTicketType  enumor.RPTicketType      `json:"sub_ticket_type"`
	TicketTypeName string                   `json:"ticket_type_name"`
	CrpSN          string                   `json:"crp_sn"`
	CrpURL         string                   `json:"crp_url"`
	OriginalInfo   RPTicketResourceInfo     `json:"original_info"`
	UpdatedInfo    RPTicketResourceInfo     `json:"updated_info"`
	SubmittedAt    string                   `json:"submitted_at"`
	CreatedAt      string                   `json:"created_at"`
	UpdatedAt      string                   `json:"updated_at"`
}

// GetSubTicketDetailResp is get resource plan sub ticket detail response.
type GetSubTicketDetailResp struct {
	Applicant  string                 `json:"-"` // Applicant 单据提单人，目前仅用于权限判断
	ID         string                 `json:"id"`
	BaseInfo   GetSubTicketBaseInfo   `json:"base_info"`
	StatusInfo GetSubTicketStatusInfo `json:"status_info"`
	Demands    []GetRPTicketDemand    `json:"demands"`
}

// GetSubTicketBaseInfo is get resource plan sub ticket base info.
type GetSubTicketBaseInfo struct {
	Type          enumor.RPTicketType `json:"type"`
	TypeName      string              `json:"type_name"`
	BkBizID       int64               `json:"bk_biz_id"`
	OpProductID   int64               `json:"op_product_id"`
	PlanProductID int64               `json:"plan_product_id"`
	VirtualDeptID int64               `json:"virtual_dept_id"`
	SubmittedAt   string              `json:"submitted_at"`
}

// GetSubTicketStatusInfo is get resource plan sub ticket status info.
type GetSubTicketStatusInfo struct {
	Status           enumor.RPSubTicketStatus  `json:"status"`
	StatusName       string                    `json:"status_name"`
	Stage            enumor.RPSubTicketStage   `json:"stage"`
	AdminAuditStatus enumor.RPAdminAuditStatus `json:"admin_audit_status"`
	CrpSN            string                    `json:"crp_sn"`
	CrpURL           string                    `json:"crp_url"`
	Message          string                    `json:"message"`
}

// GetSubTicketAuditResp is get resource plan sub ticket audit response.
type GetSubTicketAuditResp struct {
	ID         string                 `json:"id"`
	AdminAudit *GetRPTicketAdminAudit `json:"admin_audit"`
	CRPAudit   *GetRPTicketCrpAudit   `json:"crp_audit"`
}
