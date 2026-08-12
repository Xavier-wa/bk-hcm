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
	"encoding/json"
	"fmt"

	rptypes "hcm/cmd/woa-server/types/return-plan"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	tablerst "hcm/pkg/dal/table/return-plan/return-plan-sub-ticket"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	tabletypes "hcm/pkg/dal/table/types"
)

func isReturnPlanTicketCompleted(status enumor.ReturnPlanTicketStatus) bool {
	switch status {
	case enumor.ReturnPlanTicketStatusDone,
		enumor.ReturnPlanTicketStatusFailed,
		enumor.ReturnPlanTicketStatusPartialFailed,
		enumor.ReturnPlanTicketStatusRejected,
		enumor.ReturnPlanTicketStatusPartialRejected,
		enumor.ReturnPlanTicketStatusRevoked,
		enumor.ReturnPlanTicketStatusTerminated:
		return true
	default:
		return false
	}
}

func toListReturnPlanTicketItem(ticket tablert.ReturnPlanTicketTable) rptypes.ListReturnPlanTicketItem {
	item := rptypes.ListReturnPlanTicketItem{
		ID:              ticket.ID,
		BkBizID:         ticket.BkBizID,
		BkBizName:       ticket.BkBizName,
		OpProductID:     ticket.OpProductID,
		OpProductName:   ticket.OpProductName,
		PlanProductID:   ticket.PlanProductID,
		PlanProductName: ticket.PlanProductName,
		VirtualDeptID:   ticket.VirtualDeptID,
		VirtualDeptName: ticket.VirtualDeptName,
		Status:          ticket.Status,
		StatusName:      ticket.Status.Name(),
		TicketType:      ticket.Type,
		TicketTypeName:  ticket.Type.Name(),
		Applicant:       ticket.Applicant,
		Remark:          ticket.Remark,
		SubmittedAt:     ticket.SubmittedAt,
		CreatedAt:       ticket.CreatedAt.String(),
		UpdatedAt:       ticket.UpdatedAt.String(),
	}
	if isReturnPlanTicketCompleted(ticket.Status) {
		item.CompletedAt = item.UpdatedAt
	}
	return item
}

func toGetReturnPlanDetailItems(details tablert.ReturnPlanDetails) ([]rptypes.GetReturnPlanDetailItem, error) {
	items := make([]rptypes.GetReturnPlanDetailItem, 0, len(details))
	for i := range details {
		detailType, err := details[i].Type()
		if err != nil {
			return nil, err
		}

		items = append(items, rptypes.GetReturnPlanDetailItem{
			Type:     detailType,
			TypeName: detailType.Name(),
			Original: toReturnPlanDetailInfo(details[i].Original),
			Updated:  toReturnPlanDetailInfo(details[i].Updated),
		})
	}
	return items, nil
}

func toReturnPlanDetailInfo(item *tablert.ReturnPlanItem) *rptypes.ReturnPlanDetailInfo {
	if item == nil {
		return nil
	}
	return &rptypes.ReturnPlanDetailInfo{
		ObsProject:        item.ObsProject,
		PlanTime:          item.PlanTime,
		ResourcePoolName:  item.ResourcePoolName,
		City:              item.City,
		Zone:              item.Zone,
		InstanceModel:     item.InstanceModel,
		CvmAmount:         item.CvmAmount,
		InstanceType:      item.InstanceType,
		CoreTypeName:      item.CoreTypeName,
		CoreAmount:        item.CoreAmount,
		ReturnReasonClass: item.ReturnReasonClass,
		Desc:              item.Desc,
	}
}

func parseReturnPlanDetails(raw tabletypes.JsonField) (tablert.ReturnPlanDetails, error) {
	if len(raw) == 0 {
		return nil, errf.New(errf.Aborted, "return plan details is empty")
	}

	details := make(tablert.ReturnPlanDetails, 0)
	if err := json.Unmarshal([]byte(raw), &details); err != nil {
		return nil, errf.NewFromErr(errf.Aborted, fmt.Errorf("unmarshal return plan details failed, err: %w", err))
	}
	return details, nil
}

func toListReturnPlanSubTicketItem(sub tablerst.ReturnPlanSubTicketTable) rptypes.ListReturnPlanSubTicketItem {
	return rptypes.ListReturnPlanSubTicketItem{
		ID:                sub.ID,
		Status:            sub.Status,
		StatusName:        sub.Status.Name(),
		SubTicketType:     sub.SubType,
		SubTicketTypeName: sub.SubType.Name(),
		ObsProject:        sub.ObsProject,
		ResourcePoolName:  sub.ResPoolName,
		CrpSN:             sub.CrpSN,
		CrpURL:            sub.CrpURL,
		Message:           sub.Message,
		SubmittedAt:       sub.SubmittedAt,
		CreatedAt:         sub.CreatedAt.String(),
		UpdatedAt:         sub.UpdatedAt.String(),
	}
}

func toGetReturnPlanSubTicketDetail(sub tablerst.ReturnPlanSubTicketTable) rptypes.GetReturnPlanSubTicketDetailResp {
	return rptypes.GetReturnPlanSubTicketDetailResp{
		ID:                sub.ID,
		TicketID:          sub.TicketID,
		SubTicketType:     sub.SubType,
		SubTicketTypeName: sub.SubType.Name(),
		Status:            sub.Status,
		StatusName:        sub.Status.Name(),
		ObsProject:        sub.ObsProject,
		ResourcePoolName:  sub.ResPoolName,
		CrpSN:             sub.CrpSN,
		CrpURL:            sub.CrpURL,
		Message:           sub.Message,
		SubmittedAt:       sub.SubmittedAt,
		CreatedAt:         sub.CreatedAt.String(),
		UpdatedAt:         sub.UpdatedAt.String(),
	}
}
