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
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// ListReturnPlanSubTicket list return plan sub tickets.
func (f *ReturnPlanFetcher) ListReturnPlanSubTicket(kt *kit.Kit, req *rptypes.ListReturnPlanSubTicketReq) (
	*rptypes.ListReturnPlanSubTicketResp, error) {

	listReq := &rpproto.ReturnPlanSubTicketListReq{ListReq: req.GenListOption()}
	rst, err := f.client.DataService().Global.ReturnPlan.ListReturnPlanSubTicket(kt, listReq)
	if err != nil {
		logs.Errorf("list return plan sub ticket failed, err: %v, ticket_id: %s, rid: %s",
			err, req.TicketID, kt.Rid)
		return nil, err
	}

	if len(rst.Details) == 0 {
		return &rptypes.ListReturnPlanSubTicketResp{Count: rst.Count}, nil
	}

	details := make([]rptypes.ListReturnPlanSubTicketItem, 0, len(rst.Details))
	for i := range rst.Details {
		details = append(details, toListReturnPlanSubTicketItem(rst.Details[i]))
	}

	return &rptypes.ListReturnPlanSubTicketResp{
		Count:   rst.Count,
		Details: details,
	}, nil
}

// GetReturnPlanSubTicketDetail get return plan sub ticket detail.
func (f *ReturnPlanFetcher) GetReturnPlanSubTicketDetail(kt *kit.Kit, bkBizID int64, subTicketID string) (
	*rptypes.GetReturnPlanSubTicketDetailResp, error) {

	req := &rpproto.ReturnPlanSubTicketListReq{
		ListReq: core.ListReq{
			Filter: tools.ExpressionAnd(tools.RuleEqual("id", subTicketID)),
			Page:   core.NewDefaultBasePage(),
		},
	}
	rst, err := f.client.DataService().Global.ReturnPlan.ListReturnPlanSubTicket(kt, req)
	if err != nil {
		logs.Errorf("get return plan sub ticket failed, err: %v, id: %s, rid: %s", err, subTicketID, kt.Rid)
		return nil, err
	}
	if len(rst.Details) != 1 {
		return nil, errf.New(errf.RecordNotFound, "return plan sub ticket not found")
	}

	sub := rst.Details[0]
	if bkBizID > 0 && sub.BkBizID != bkBizID {
		return nil, errf.New(errf.PermissionDenied, "no permission to access this sub ticket")
	}

	detail := toGetReturnPlanSubTicketDetail(sub)
	return &detail, nil
}
