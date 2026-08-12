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

package dispatcher

import (
	"time"

	"hcm/pkg/api/core"
	rpproto "hcm/pkg/api/data-service/return-plan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	tablerst "hcm/pkg/dal/table/return-plan/return-plan-sub-ticket"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/times"
)

// maxTicketMessageLen 主单 message 列长度上限（见建表脚本 varchar(2048)），聚合超长时截断。
const maxTicketMessageLen = 2048

// listAndWatchTickets 主单 watcher：仅 master 拉取待处理主单入队，非 master 清空队列与处理中记录。
func (d *Dispatcher) listAndWatchTickets() error {
	if !d.sd.IsMaster() {
		d.ticketQueue.Clear()
		d.processingTickets.Range(func(key, value any) bool {
			d.processingTickets.Delete(key)
			return true
		})
		return nil
	}

	kt := core.NewBackendKit()
	tkIDs, err := d.listAllPendingTickets(kt)
	if err != nil {
		logs.Errorf("failed to list pending return plan tickets, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	for _, id := range tkIDs {
		d.ticketQueue.Enqueue(id)
	}
	return nil
}

// listAllPendingTickets 拉取最近 PendingTicketTraceDay 天内、状态未终结(init/auditing)的主单 ID。
func (d *Dispatcher) listAllPendingTickets(kt *kit.Kit) ([]string, error) {
	dr := &times.DateRange{
		Start: time.Now().AddDate(0, 0, -enumor.PendingTicketTraceDay).Format(constant.DateLayout),
		End:   time.Now().Format(constant.DateLayout),
	}
	drExpr, err := tools.DateRangeExpression("submitted_at", dr)
	if err != nil {
		return nil, err
	}

	statusRule := tools.RuleIn("status", []enumor.ReturnPlanTicketStatus{
		enumor.ReturnPlanTicketStatusInit,
		enumor.ReturnPlanTicketStatusAuditing,
	})
	filterExpr, err := tools.And(drExpr, statusRule)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0)
	start := uint32(0)
	for {
		req := &rpproto.ReturnPlanTicketListReq{
			ListReq: core.ListReq{
				Filter: filterExpr,
				Page:   &core.BasePage{Start: start, Limit: core.DefaultMaxPageLimit},
				Fields: []string{"id"},
			},
		}
		rst, err := d.client.DataService().Global.ReturnPlan.ListReturnPlanTicket(kt, req)
		if err != nil {
			return nil, err
		}
		for i := range rst.Details {
			ids = append(ids, rst.Details[i].ID)
		}
		if len(rst.Details) < int(core.DefaultMaxPageLimit) {
			break
		}
		start += uint32(core.DefaultMaxPageLimit)
	}
	return ids, nil
}

// dealTicket 主单 handler：init 主单触发拆单并置 auditing；auditing 主单聚合子单状态。
func (d *Dispatcher) dealTicket() error {
	if !d.sd.IsMaster() {
		return nil
	}

	tkID, ok := d.ticketQueue.Pop()
	if !ok {
		return nil
	}

	kt := core.NewBackendKit()
	if _, loaded := d.processingTickets.LoadOrStore(tkID, struct{}{}); loaded {
		logs.Warnf("return plan ticket %s is already being processed, skip, rid: %s", tkID, kt.Rid)
		return nil
	}
	defer d.processingTickets.Delete(tkID)

	ticket, err := d.getTicket(kt, tkID)
	if err != nil {
		logs.Errorf("failed to get return plan ticket, err: %v, id: %s, rid: %s", err, tkID, kt.Rid)
		return err
	}
	if ticket == nil {
		logs.Warnf("return plan ticket not found, id: %s, rid: %s", tkID, kt.Rid)
		return nil
	}
	if !ticket.Status.IsUnfinished() {
		logs.Warnf("return plan ticket status %s is finished, skip, id: %s, rid: %s",
			ticket.Status, tkID, kt.Rid)
		return nil
	}

	if ticket.Status == enumor.ReturnPlanTicketStatusInit {
		return d.splitTicket(kt, ticket)
	}
	return d.aggregateTicket(kt, ticket)
}

// splitTicket 拆单主流程：无子单时拆单，随后将主单由 init 推进为 auditing。
func (d *Dispatcher) splitTicket(kt *kit.Kit, ticket *tablert.ReturnPlanTicketTable) error {
	subTickets, err := d.listSubTicketsByTicket(kt, ticket.ID)
	if err != nil {
		logs.Errorf("failed to list return plan sub tickets before split, err: %v, ticket_id: %s, rid: %s",
			err, ticket.ID, kt.Rid)
		return err
	}

	// 拆单幂等：已有子单则跳过拆单，仅推进主单状态
	if len(subTickets) == 0 {
		if err := d.splitter.Split(kt, ticket); err != nil {
			logs.Errorf("failed to split return plan ticket, err: %v, ticket_id: %s, rid: %s",
				err, ticket.ID, kt.Rid)
			return err
		}
	}

	return d.updateTicketStatus(kt, ticket, enumor.ReturnPlanTicketStatusAuditing, "")
}

// aggregateTicket 聚合子单状态推导主单状态与失败原因；仍有未终结子单则等待下轮。
func (d *Dispatcher) aggregateTicket(kt *kit.Kit, ticket *tablert.ReturnPlanTicketTable) error {
	subTickets, err := d.listSubTicketsByTicket(kt, ticket.ID)
	if err != nil {
		logs.Errorf("failed to list return plan sub tickets, err: %v, ticket_id: %s, rid: %s",
			err, ticket.ID, kt.Rid)
		return err
	}
	if len(subTickets) == 0 {
		logs.Warnf("auditing return plan ticket has no sub ticket, ticket_id: %s, rid: %s", ticket.ID, kt.Rid)
		return nil
	}

	for i := range subTickets {
		if subTickets[i].Status.IsUnfinished() {
			logs.Infof("return plan ticket has unfinished sub ticket, wait, ticket_id: %s, rid: %s",
				ticket.ID, kt.Rid)
			return nil
		}
	}

	status := determineTicketStatus(subTickets)
	message := ""
	if status != enumor.ReturnPlanTicketStatusDone {
		message = aggregateFailMessage(subTickets)
	}
	return d.updateTicketStatus(kt, ticket, status, message)
}

// getTicket 按 ID 查询主单，未找到返回 nil。
func (d *Dispatcher) getTicket(kt *kit.Kit, id string) (*tablert.ReturnPlanTicketTable, error) {
	req := &rpproto.ReturnPlanTicketListReq{
		ListReq: core.ListReq{
			Filter: tools.EqualExpression("id", id),
			Page:   core.NewDefaultBasePage(),
		},
	}
	rst, err := d.client.DataService().Global.ReturnPlan.ListReturnPlanTicket(kt, req)
	if err != nil {
		return nil, err
	}
	if len(rst.Details) == 0 {
		return nil, nil
	}
	return &rst.Details[0], nil
}

// listSubTicketsByTicket 分页查询某主单下的全部子单。
func (d *Dispatcher) listSubTicketsByTicket(kt *kit.Kit, ticketID string) (
	[]tablerst.ReturnPlanSubTicketTable, error) {

	subTickets := make([]tablerst.ReturnPlanSubTicketTable, 0)
	start := uint32(0)
	for {
		req := &rpproto.ReturnPlanSubTicketListReq{
			ListReq: core.ListReq{
				Filter: tools.EqualExpression("ticket_id", ticketID),
				Page:   &core.BasePage{Start: start, Limit: core.DefaultMaxPageLimit},
			},
		}
		rst, err := d.client.DataService().Global.ReturnPlan.ListReturnPlanSubTicket(kt, req)
		if err != nil {
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

// updateTicketStatus 更新主单状态与 message（以提单人身份），message 为空则不写。
func (d *Dispatcher) updateTicketStatus(kt *kit.Kit, ticket *tablert.ReturnPlanTicketTable,
	status enumor.ReturnPlanTicketStatus, message string) error {

	kt.User = ticket.Applicant
	req := &rpproto.ReturnPlanTicketUpdateReq{
		ID:     ticket.ID,
		Status: status,
	}
	if message != "" {
		req.Message = &message
	}
	if err := d.client.DataService().Global.ReturnPlan.UpdateReturnPlanTicket(kt, req); err != nil {
		logs.Errorf("failed to update return plan ticket status, err: %v, ticket_id: %s, status: %s, rid: %s",
			err, ticket.ID, status, kt.Rid)
		return err
	}

	logs.Infof("update return plan ticket status success, ticket_id: %s, status: %s, rid: %s",
		ticket.ID, status, kt.Rid)
	return nil
}
