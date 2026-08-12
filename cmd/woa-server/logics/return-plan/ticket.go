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
	"time"

	rptypes "hcm/cmd/woa-server/types/return-plan"
	"hcm/pkg/api/core"
	rpproto "hcm/pkg/api/data-service/return-plan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	tablerst "hcm/pkg/dal/table/return-plan/return-plan-sub-ticket"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/cvmapi"
	"hcm/pkg/tools/slice"
)

// invalidatedSubTicketGroup 记录一批被置为已失效的子单及其原状态，用于重试失败时回滚。
type invalidatedSubTicketGroup struct {
	ids    []string
	origin enumor.ReturnPlanSubTicketStatus
}

// RetryReturnPlanFailedSubTickets 重试主单下失败/驳回的子单。
// 子单不直接改回 init，而是先将其置为已失效(保留 CRP 单号与失败原因，保证可追溯)，
// 再按原内容重建为待处理子单，交由调度器重新提单。重建内容与失效前一致，无需重新拆单。
func (c *Controller) RetryReturnPlanFailedSubTickets(kt *kit.Kit, ticketID string) error {
	ticket, err := c.resFetcher.GetReturnPlanTicketByID(kt, ticketID)
	if err != nil {
		return err
	}

	if ticket.Status == enumor.ReturnPlanTicketStatusTerminated {
		logs.Errorf("return plan ticket already terminated, ticket_id: %s, rid: %s", ticketID, kt.Rid)
		return errf.New(errf.InvalidParameter, "ticket already terminated")
	}

	retried, err := c.retryReturnPlanSubTickets(kt, ticket)
	if err != nil {
		logs.Errorf("retry return plan sub tickets failed, err: %v, ticket_id: %s, rid: %s", err, ticketID, kt.Rid)
		return err
	}
	if !retried {
		logs.Warnf("no failed/rejected return plan sub tickets to retry, ticket_id: %s, rid: %s", ticketID, kt.Rid)
		return nil
	}

	// 防止重试期间新建子单先行全部到达终态而触发主单结单，主单重新置为处理中。
	updateReq := &rpproto.ReturnPlanTicketUpdateReq{
		ID:     ticketID,
		Status: enumor.ReturnPlanTicketStatusAuditing,
	}
	if err = c.client.DataService().Global.ReturnPlan.UpdateReturnPlanTicket(kt, updateReq); err != nil {
		logs.Errorf("update return plan ticket to auditing failed, err: %v, ticket_id: %s, rid: %s",
			err, ticketID, kt.Rid)
		return err
	}

	return nil
}

// retryReturnPlanSubTickets 将失败/驳回子单置为已失效(可追溯)，再按原内容重建为待处理子单。
// 返回是否存在可重试子单；重建失败时回滚已失效子单至原状态，避免子单被永久失效。
func (c *Controller) retryReturnPlanSubTickets(kt *kit.Kit, ticket *tablert.ReturnPlanTicketTable) (
	retried bool, err error) {

	retrySources := []enumor.ReturnPlanSubTicketStatus{
		enumor.ReturnPlanSubTicketStatusFailed,
		enumor.ReturnPlanSubTicketStatusRejected,
	}

	invalidated := make([]invalidatedSubTicketGroup, 0, len(retrySources))
	rebuild := make([]tablerst.ReturnPlanSubTicketTable, 0)

	defer func() {
		if err == nil {
			return
		}
		for _, g := range invalidated {
			if rbErr := c.casReturnPlanSubTicketsStatus(kt, ticket.ID, g.ids,
				enumor.ReturnPlanSubTicketStatusInvalid, g.origin); rbErr != nil {
				logs.Errorf("rollback return plan sub tickets to %s failed, err: %v, ticket_id: %s, rid: %s",
					g.origin, rbErr, ticket.ID, kt.Rid)
			}
		}
	}()

	for _, source := range retrySources {
		var items []tablerst.ReturnPlanSubTicketTable
		items, err = c.listReturnPlanSubTicketsByStatus(kt, ticket.ID, source)
		if err != nil {
			return false, err
		}
		if len(items) == 0 {
			continue
		}

		ids := make([]string, 0, len(items))
		for i := range items {
			ids = append(ids, items[i].ID)
		}
		// 置为已失效(保留原 crp_sn/crp_url/message 以可追溯)
		if err = c.casReturnPlanSubTicketsStatus(kt, ticket.ID, ids, source,
			enumor.ReturnPlanSubTicketStatusInvalid); err != nil {
			return false, err
		}
		invalidated = append(invalidated, invalidatedSubTicketGroup{ids: ids, origin: source})
		rebuild = append(rebuild, items...)
	}

	if len(rebuild) == 0 {
		return false, nil
	}

	if err = c.recreateReturnPlanSubTickets(kt, ticket, rebuild); err != nil {
		return false, err
	}

	return true, nil
}

// recreateReturnPlanSubTickets 以提单人身份按原内容重建子单(状态置 init，不携带 CRP 单号/失败原因)。
// 与拆单保持一致以提单人身份创建，保证后续向 CRP 提单的 userName 正确。
func (c *Controller) recreateReturnPlanSubTickets(kt *kit.Kit, ticket *tablert.ReturnPlanTicketTable,
	subTickets []tablerst.ReturnPlanSubTicketTable) error {

	createReqs := make([]rpproto.ReturnPlanSubTicketCreateReq, 0, len(subTickets))
	for i := range subTickets {
		sub := subTickets[i]
		createReqs = append(createReqs, rpproto.ReturnPlanSubTicketCreateReq{
			TicketID:        sub.TicketID,
			SubType:         sub.SubType,
			SubDetails:      sub.SubDetails,
			BkBizID:         sub.BkBizID,
			BkBizName:       sub.BkBizName,
			OpProductID:     sub.OpProductID,
			OpProductName:   sub.OpProductName,
			PlanProductID:   sub.PlanProductID,
			PlanProductName: sub.PlanProductName,
			VirtualDeptID:   sub.VirtualDeptID,
			VirtualDeptName: sub.VirtualDeptName,
			ObsProject:      sub.ObsProject,
			ResPoolName:     sub.ResPoolName,
			Status:          enumor.ReturnPlanSubTicketStatusInit,
			SubmittedAt:     time.Now().Format(constant.DateTimeLayout),
		})
	}

	kt.User = ticket.Applicant
	for _, batch := range slice.Split(createReqs, constant.BatchOperationMaxLimit) {
		createReq := &rpproto.ReturnPlanSubTicketBatchCreateReq{SubTickets: batch}
		if _, err := c.client.DataService().Global.ReturnPlan.BatchCreateReturnPlanSubTicket(kt,
			createReq); err != nil {
			logs.Errorf("batch recreate return plan sub tickets failed, err: %v, ticket_id: %s, "+
				"batch_size: %d, rid: %s", err, ticket.ID, len(batch), kt.Rid)
			return err
		}
	}

	logs.Infof("recreate return plan sub tickets success, ticket_id: %s, count: %d, rid: %s",
		ticket.ID, len(createReqs), kt.Rid)
	return nil
}

// casReturnPlanSubTicketsStatus 批量 CAS 更新子单状态，不覆盖 crp_sn/crp_url 与原失败原因。
func (c *Controller) casReturnPlanSubTicketsStatus(kt *kit.Kit, ticketID string, ids []string,
	source, target enumor.ReturnPlanSubTicketStatus) error {

	req := &rpproto.ReturnPlanSubTicketStatusUpdateReq{
		IDs:      ids,
		TicketID: ticketID,
		Source:   source,
		Target:   target,
	}
	if err := c.client.DataService().Global.ReturnPlan.UpdateReturnPlanSubTicketStatusCAS(kt, req); err != nil {
		logs.Errorf("cas return plan sub tickets status failed, err: %v, source: %s, target: %s, "+
			"ticket_id: %s, rid: %s", err, source, target, ticketID, kt.Rid)
		return err
	}
	return nil
}

// TerminateReturnPlanFailedTicket terminates a non-final return plan ticket locally.
func (c *Controller) TerminateReturnPlanFailedTicket(kt *kit.Kit, ticketID string) error {
	ticket, err := c.resFetcher.GetReturnPlanTicketByID(kt, ticketID)
	if err != nil {
		return err
	}

	if !ticket.Status.CanTerminate() {
		logs.Errorf("return plan ticket status %s can't terminate, ticket_id: %s, rid: %s",
			ticket.Status, ticketID, kt.Rid)
		return errf.Newf(errf.InvalidParameter, "ticket status is %s, can't terminate", ticket.Status)
	}

	updateReq := &rpproto.ReturnPlanTicketUpdateReq{
		ID:     ticketID,
		Status: enumor.ReturnPlanTicketStatusTerminated,
	}
	if err = c.client.DataService().Global.ReturnPlan.UpdateReturnPlanTicket(kt, updateReq); err != nil {
		logs.Errorf("terminate return plan ticket failed, err: %v, ticket_id: %s, rid: %s", err, ticketID, kt.Rid)
		return err
	}
	return nil
}

// ListReturnReasonClass proxies CRP getReasonClassByObsProject.
func (c *Controller) ListReturnReasonClass(kt *kit.Kit, req *rptypes.ListReturnReasonClassReq) (
	*rptypes.ListReturnReasonClassResp, error) {

	crpReq := cvmapi.NewGetReasonClassByObsProjectReq(&cvmapi.GetReasonClassByObsProjectParam{
		ObsProject: req.ObsProject,
	})
	resp, err := c.crpCli.GetReasonClassByObsProject(kt.Ctx, kt.Header(), crpReq)
	if err != nil {
		logs.Errorf("get return reason class from crp failed, err: %v, obs_project: %s, rid: %s",
			err, req.ObsProject, kt.Rid)
		return nil, err
	}

	reasonClasses := make([]string, 0)
	if resp.Result != nil {
		for _, item := range resp.Result.Data {
			if item == nil || item.EnableFlag != 1 {
				continue
			}
			if len(item.ReturnReasonClass) == 0 {
				continue
			}
			reasonClasses = append(reasonClasses, item.ReturnReasonClass)
		}
	}

	return &rptypes.ListReturnReasonClassResp{ReasonClasses: reasonClasses}, nil
}

// listReturnPlanSubTicketsByStatus 分页拉取主单下指定状态的全部子单(完整记录，供重建复制内容)。
func (c *Controller) listReturnPlanSubTicketsByStatus(kt *kit.Kit, ticketID string,
	status enumor.ReturnPlanSubTicketStatus) ([]tablerst.ReturnPlanSubTicketTable, error) {

	subTickets := make([]tablerst.ReturnPlanSubTicketTable, 0)
	start := uint32(0)
	for {
		req := &rpproto.ReturnPlanSubTicketListReq{
			ListReq: core.ListReq{
				Filter: tools.ExpressionAnd(
					tools.RuleEqual("ticket_id", ticketID),
					tools.RuleEqual("status", status),
				),
				Page: &core.BasePage{Start: start, Limit: core.DefaultMaxPageLimit},
			},
		}
		rst, err := c.client.DataService().Global.ReturnPlan.ListReturnPlanSubTicket(kt, req)
		if err != nil {
			logs.Errorf("list return plan sub ticket failed, err: %v, ticket_id: %s, status: %s, rid: %s",
				err, ticketID, status, kt.Rid)
			return nil, err
		}
		subTickets = append(subTickets, rst.Details...)
		if len(rst.Details) < int(core.DefaultMaxPageLimit) {
			break
		}
		start += uint32(core.DefaultMaxPageLimit)
	}
	return subTickets, nil
}
