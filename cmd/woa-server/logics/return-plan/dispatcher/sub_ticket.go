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
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/times"
)

// maxSubTicketMessageLen 子单 message 列长度上限（见建表脚本 varchar(512)）。
const maxSubTicketMessageLen = 512

// listAndWatchSubTickets 子单 watcher：仅 master 拉取待处理子单入队，非 master 清空队列与处理中记录。
func (d *Dispatcher) listAndWatchSubTickets() error {
	if !d.sd.IsMaster() {
		d.subTicketQueue.Clear()
		d.processingSubTickets.Range(func(key, value any) bool {
			d.processingSubTickets.Delete(key)
			return true
		})
		return nil
	}

	kt := core.NewBackendKit()
	ids, err := d.listAllPendingSubTickets(kt)
	if err != nil {
		logs.Errorf("failed to list pending return plan sub tickets, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	for _, id := range ids {
		d.subTicketQueue.Enqueue(id)
	}
	return nil
}

// listAllPendingSubTickets 拉取最近 PendingTicketTraceDay 天内、状态未终结(init/auditing)的子单 ID。
func (d *Dispatcher) listAllPendingSubTickets(kt *kit.Kit) ([]string, error) {
	dr := &times.DateRange{
		Start: time.Now().AddDate(0, 0, -enumor.PendingTicketTraceDay).Format(constant.DateLayout),
		End:   time.Now().Format(constant.DateLayout),
	}
	drExpr, err := tools.DateRangeExpression("submitted_at", dr)
	if err != nil {
		return nil, err
	}

	statusRule := tools.RuleIn("status", []enumor.ReturnPlanSubTicketStatus{
		enumor.ReturnPlanSubTicketStatusInit,
		enumor.ReturnPlanSubTicketStatusAuditing,
	})
	filterExpr, err := tools.And(drExpr, statusRule)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0)
	start := uint32(0)
	for {
		req := &rpproto.ReturnPlanSubTicketListReq{
			ListReq: core.ListReq{
				Filter: filterExpr,
				Page:   &core.BasePage{Start: start, Limit: core.DefaultMaxPageLimit},
				Fields: []string{"id"},
			},
		}
		rst, err := d.client.DataService().Global.ReturnPlan.ListReturnPlanSubTicket(kt, req)
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

// dealSubTicket 子单 handler：init 子单向 CRP 提单；auditing 子单轮询 CRP 状态并处理超时。
func (d *Dispatcher) dealSubTicket() error {
	if !d.sd.IsMaster() {
		return nil
	}

	subID, ok := d.subTicketQueue.Pop()
	if !ok {
		return nil
	}

	kt := core.NewBackendKit()
	if _, loaded := d.processingSubTickets.LoadOrStore(subID, struct{}{}); loaded {
		logs.Warnf("return plan sub ticket %s is already being processed, skip, rid: %s", subID, kt.Rid)
		return nil
	}
	defer d.processingSubTickets.Delete(subID)

	logs.Infof("ready to handle return plan sub ticket %s, rid: %s", subID, kt.Rid)
	subTicket, err := d.getSubTicket(kt, subID)
	if err != nil {
		logs.Errorf("failed to get return plan sub ticket, err: %v, id: %s, rid: %s", err, subID, kt.Rid)
		return err
	}
	if subTicket == nil {
		logs.Warnf("return plan sub ticket not found, id: %s, rid: %s", subID, kt.Rid)
		return nil
	}
	if !subTicket.Status.IsUnfinished() {
		logs.Warnf("return plan sub ticket status %s is finished, skip, id: %s, rid: %s",
			subTicket.Status, subID, kt.Rid)
		return nil
	}

	switch subTicket.Status {
	case enumor.ReturnPlanSubTicketStatusInit:
		return d.submitSubTicket(kt, subTicket)
	case enumor.ReturnPlanSubTicketStatusAuditing:
		return d.pollSubTicket(kt, subTicket)
	default:
		return nil
	}
}

// submitSubTicket 向 CRP 提单：成功则 init→auditing 并写 crp_sn/crp_url，失败则 init→failed 并写原因。
func (d *Dispatcher) submitSubTicket(kt *kit.Kit, subTicket *tablerst.ReturnPlanSubTicketTable) error {
	crpSN, crpURL, err := d.submitCrpOrder(kt, subTicket)
	if err != nil {
		return d.casSubTicketStatus(kt, subTicket, enumor.ReturnPlanSubTicketStatusInit,
			enumor.ReturnPlanSubTicketStatusFailed, nil, nil, truncateMessage(err.Error(), maxSubTicketMessageLen))
	}

	return d.casSubTicketStatus(kt, subTicket, enumor.ReturnPlanSubTicketStatusInit,
		enumor.ReturnPlanSubTicketStatusAuditing, &crpSN, &crpURL, "")
}

// pollSubTicket 轮询 CRP 单据状态推进子单终态；未达终态则检查审批超时。
func (d *Dispatcher) pollSubTicket(kt *kit.Kit, subTicket *tablerst.ReturnPlanSubTicketTable) error {
	status, message, err := d.pollCrpOrder(kt, subTicket)
	if err != nil {
		return err
	}
	if status == "" {
		return d.checkSubTicketTimeout(kt, subTicket)
	}

	msg := ""
	if status != enumor.ReturnPlanSubTicketStatusDone {
		msg = truncateMessage(message, maxSubTicketMessageLen)
	}
	return d.casSubTicketStatus(kt, subTicket, enumor.ReturnPlanSubTicketStatusAuditing, status, nil, nil, msg)
}

// checkSubTicketTimeout 子单自 submitted_at 起超过 AuditFlowTimeoutDay 仍在 auditing → 置 failed。
func (d *Dispatcher) checkSubTicketTimeout(kt *kit.Kit, subTicket *tablerst.ReturnPlanSubTicketTable) error {
	submitTime, err := time.Parse(constant.TimeStdFormat, subTicket.SubmittedAt)
	if err != nil {
		logs.Errorf("failed to parse return plan sub ticket submitted_at %s, err: %v, sub_ticket_id: %s, rid: %s",
			subTicket.SubmittedAt, err, subTicket.ID, kt.Rid)
		return err
	}

	if time.Now().Before(submitTime.AddDate(0, 0, enumor.AuditFlowTimeoutDay)) {
		return nil
	}

	logs.Warnf("return plan sub ticket audit flow timeout, sub_ticket_id: %s, rid: %s", subTicket.ID, kt.Rid)
	return d.casSubTicketStatus(kt, subTicket, enumor.ReturnPlanSubTicketStatusAuditing,
		enumor.ReturnPlanSubTicketStatusFailed, nil, nil, "audit flow timeout")
}

// getSubTicket 按 ID 查询子单，未找到返回 nil。
func (d *Dispatcher) getSubTicket(kt *kit.Kit, id string) (*tablerst.ReturnPlanSubTicketTable, error) {
	req := &rpproto.ReturnPlanSubTicketListReq{
		ListReq: core.ListReq{
			Filter: tools.EqualExpression("id", id),
			Page:   core.NewDefaultBasePage(),
		},
	}
	rst, err := d.client.DataService().Global.ReturnPlan.ListReturnPlanSubTicket(kt, req)
	if err != nil {
		return nil, err
	}
	if len(rst.Details) == 0 {
		return nil, nil
	}
	return &rst.Details[0], nil
}

// casSubTicketStatus 以提单人身份 CAS 更新子单状态，可选写入 crp_sn/crp_url/message。
func (d *Dispatcher) casSubTicketStatus(kt *kit.Kit, subTicket *tablerst.ReturnPlanSubTicketTable,
	source, target enumor.ReturnPlanSubTicketStatus, crpSN, crpURL *string, message string) error {

	// 置失败态必须携带失败原因（data-service 侧强校验）
	if target == enumor.ReturnPlanSubTicketStatusFailed && message == "" {
		message = "return plan sub ticket failed"
	}

	kt.User = subTicket.Creator
	req := &rpproto.ReturnPlanSubTicketStatusUpdateReq{
		IDs:      []string{subTicket.ID},
		TicketID: subTicket.TicketID,
		Source:   source,
		Target:   target,
		CrpSN:    crpSN,
		CrpURL:   crpURL,
	}
	if message != "" {
		req.Message = &message
	}
	if err := d.client.DataService().Global.ReturnPlan.UpdateReturnPlanSubTicketStatusCAS(kt, req); err != nil {
		logs.Errorf("failed to cas return plan sub ticket status, err: %v, sub_ticket_id: %s, "+
			"source: %s, target: %s, rid: %s", err, subTicket.ID, source, target, kt.Rid)
		return err
	}

	logs.Infof("cas return plan sub ticket status success, sub_ticket_id: %s, source: %s, target: %s, rid: %s",
		subTicket.ID, source, target, kt.Rid)
	return nil
}
