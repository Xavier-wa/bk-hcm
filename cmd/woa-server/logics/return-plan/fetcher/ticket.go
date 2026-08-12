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

package fetcher

import (
	rptypes "hcm/cmd/woa-server/types/return-plan"
	"hcm/pkg/api/core"
	rpproto "hcm/pkg/api/data-service/return-plan"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// ListReturnPlanTicket list return plan tickets.
func (f *ReturnPlanFetcher) ListReturnPlanTicket(kt *kit.Kit, opt *core.ListReq) (*rptypes.ListReturnPlanTicketResp,
	error) {

	req := &rpproto.ReturnPlanTicketListReq{ListReq: *opt}
	rst, err := f.client.DataService().Global.ReturnPlan.ListReturnPlanTicket(kt, req)
	if err != nil {
		logs.Errorf("list return plan ticket failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if len(rst.Details) == 0 {
		return &rptypes.ListReturnPlanTicketResp{Count: rst.Count}, nil
	}

	details := make([]rptypes.ListReturnPlanTicketItem, 0, len(rst.Details))
	for i := range rst.Details {
		details = append(details, toListReturnPlanTicketItem(rst.Details[i]))
	}

	return &rptypes.ListReturnPlanTicketResp{
		Count:   rst.Count,
		Details: details,
	}, nil
}

// GetReturnPlanTicket get return plan ticket detail.
func (f *ReturnPlanFetcher) GetReturnPlanTicket(kt *kit.Kit, bkBizID int64, ticketID string) (
	*rptypes.GetReturnPlanTicketResp, error) {

	ticket, err := f.GetReturnPlanTicketByID(kt, ticketID)
	if err != nil {
		return nil, err
	}

	if bkBizID > 0 && ticket.BkBizID != bkBizID {
		return nil, errf.New(errf.PermissionDenied, "no permission to access this ticket")
	}

	details, err := parseReturnPlanDetails(ticket.Details)
	if err != nil {
		logs.Errorf("parse return plan details failed, err: %v, ticket_id: %s, rid: %s", err, ticketID, kt.Rid)
		return nil, err
	}

	detailItems, err := toGetReturnPlanDetailItems(details)
	if err != nil {
		logs.Errorf("convert return plan details failed, err: %v, ticket_id: %s, rid: %s", err, ticketID, kt.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return &rptypes.GetReturnPlanTicketResp{
		ID: ticket.ID,
		BaseInfo: &rptypes.GetReturnPlanTicketBase{
			Type:            ticket.Type,
			TypeName:        ticket.Type.Name(),
			Applicant:       ticket.Applicant,
			BkBizID:         ticket.BkBizID,
			BkBizName:       ticket.BkBizName,
			OpProductID:     ticket.OpProductID,
			OpProductName:   ticket.OpProductName,
			PlanProductID:   ticket.PlanProductID,
			PlanProductName: ticket.PlanProductName,
			VirtualDeptID:   ticket.VirtualDeptID,
			VirtualDeptName: ticket.VirtualDeptName,
			Remark:          ticket.Remark,
			SubmittedAt:     ticket.SubmittedAt,
		},
		StatusInfo: &rptypes.GetReturnPlanTicketStatus{
			Status:     ticket.Status,
			StatusName: ticket.Status.Name(),
			Message:    ticket.Message,
		},
		Details: detailItems,
	}, nil
}

// GetReturnPlanTicketByID get return plan ticket by id.
func (f *ReturnPlanFetcher) GetReturnPlanTicketByID(kt *kit.Kit, ticketID string) (*tablert.ReturnPlanTicketTable,
	error) {

	req := &rpproto.ReturnPlanTicketListReq{
		ListReq: core.ListReq{
			Filter: tools.ExpressionAnd(tools.RuleEqual("id", ticketID)),
			Page:   core.NewDefaultBasePage(),
		},
	}
	rst, err := f.client.DataService().Global.ReturnPlan.ListReturnPlanTicket(kt, req)
	if err != nil {
		logs.Errorf("get return plan ticket failed, err: %v, ticket_id: %s, rid: %s", err, ticketID, kt.Rid)
		return nil, err
	}
	if len(rst.Details) != 1 {
		return nil, errf.New(errf.RecordNotFound, "return plan ticket not found")
	}
	return &rst.Details[0], nil
}
