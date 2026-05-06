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

// Package apply provides apply recover service
package apply

import (
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"hcm/cmd/woa-server/logics/task/scheduler"
	"hcm/cmd/woa-server/storage/driver/mongodb"
	recovertask "hcm/cmd/woa-server/types/task"
	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/thirdparty/api-gateway/itsm"
	"hcm/pkg/thirdparty/api-gateway/sopsapi"
)

// Interface apply recover interface
type Interface interface {
	recoverApplyTickets(kt *kit.Kit) error
	recoverAuditTicket(kt *kit.Kit, auditTickets []*types.ApplyTicket) error
	recoverRunningTickets(kt *kit.Kit, tickets []*types.ApplyTicket) error
}

// StartRecover 创建applyRecoverer
func StartRecover(kt *kit.Kit, itsmCli itsm.Client, scheduler scheduler.Interface, cmdbCli cmdb.Client,
	sopsCli sopsapi.SopsClientInterface, sd serviced.State) error {

	subKit := kt.NewSubKit()
	applyRecoverer := &applyRecoverer{
		schedulerIf: scheduler,
		itsmCli:     itsmCli,
		cmdbCli:     cmdbCli,
		sopsCli:     sopsCli,
	}

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logs.Errorf("[hcm server panic], err: %v, rid: %s, debug strace: %s", err, kt.Rid, debug.Stack())
			}
		}()

		for !sd.IsMaster() {
			logs.Warnf("current server is not master, sleep one minute, rid: %s", kt.Rid)
			time.Sleep(time.Minute)
		}
		logs.Infof("start apply recover logic, rid: %s", kt.Rid)

		if err := applyRecoverer.recoverApplyTickets(subKit); err != nil {
			logs.Errorf("failed to start apply recover service, err: %v, rid: %s", err, subKit.Rid)
		}
	}()

	return nil
}

// recoverApplyTickets 后台执行，恢复未结束订单继续流转
func (r *applyRecoverer) recoverApplyTickets(kt *kit.Kit) error {
	// get restart time and expire time
	restartTime := time.Now()
	expireTime := restartTime.AddDate(0, 0, recovertask.ExpireDays)
	auditTickets, err := r.getAuditTickets(kt, restartTime, expireTime, types.TicketStageAudit)
	if err != nil {
		logs.Errorf("failed to get apply orders with AUDIT stage, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	runningTickets, err := r.getRunningTickets(kt, restartTime, expireTime, types.TicketStageRunning)
	if err != nil {
		logs.Errorf("failed to get apply ticket with RUNNING stage, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	if err := r.recoverAuditTicket(kt, auditTickets); err != nil {
		logs.Errorf("failed to recover apply ticket with AUDIT stage, err: %v, rid: %s", err, kt.Rid)
	}

	if err := r.recoverRunningTickets(kt, runningTickets); err != nil {
		logs.Errorf("failed to recover apply ticket with RUNNING stage, err: %v, rid: %s", err, kt.Rid)
	}

	return nil
}

// recoverAuditTicket 恢复创建成功，itsm审批结果未获得订单
func (r *applyRecoverer) recoverAuditTicket(kt *kit.Kit, auditTickets []*types.ApplyTicket) error {
	logs.Infof("apply recover: start recover AUDIT stage tickets, ticketNum: %d, rid: %s", len(auditTickets), kt.Rid)
	sns := make([]string, 0, len(auditTickets))
	for _, ticket := range auditTickets {
		sns = append(sns, ticket.ItsmTicketId)
	}

	// 限制itsm单次查询数量
	itsmResults := make([]itsm.TicketResult, 0, len(sns))
	for snIndex := 0; snIndex < len(sns); snIndex += 100 {
		end := snIndex + 100
		if end > len(sns) {
			end = len(sns)
		}
		subSns := sns[snIndex:end]
		subItsmResults, err := r.itsmCli.GetTicketResults(kt, subSns)
		if err != nil {
			logs.Errorf("failed to get itsm tickets results, err: %v, rid: %s", err, kt.Rid)
			return err
		}
		itsmResults = append(itsmResults, subItsmResults...)
	}

	if len(itsmResults) != len(auditTickets) {
		logs.Errorf("itsm tickets results count not equal audit tickets count, itsmTicketNum: %d, ticketCount: %d, "+
			"rid: %s", len(itsmResults), len(auditTickets), kt.Rid)
		return fmt.Errorf("itsm tickets results count not equal audit tickets count")
	}

	failedOrders := make([]uint64, 0)
	for index, itsmResult := range itsmResults {
		ticket := auditTickets[index]
		if itsmResult.CurrentStatus != string(itsm.StatusFinished) {
			logs.Infof("recover audit: itsm ticket is running, skip it, itsm status: %s, orderId: %d, itsmURL: %s, "+
				"rid: %s", itsmResult.CurrentStatus, ticket.OrderId, itsmResult.TicketURL, kt.Rid)
			continue
		}
		// 审批完毕
		approveReq := &types.ApproveApplyReq{
			Approval: itsmResult.ApproveResult,
			OrderId:  ticket.OrderId,
		}
		if err := r.schedulerIf.ApproveTicket(kt, approveReq); err != nil {
			if mongodb.Client().IsDuplicatedInsertError(err) {
				logs.Errorf("duplicated insert error, skip it, err: %v, orderId: %d, rid: %s", err, ticket.OrderId,
					kt.Rid)
				continue
			}
			// 若某个订单恢复失败，忽略继续恢复其他订单
			failedOrders = append(failedOrders, ticket.OrderId)
			logs.Errorf("failed to approve ticket, err: %v, isApproved: %t, orderId: %d, rid: %s", err,
				itsmResult.ApproveResult, ticket.OrderId, kt.Rid)
		}
	}
	logs.Infof("apply recover: finish start recover AUDIT stage tickets, failed orderIds: %v, rid: %s",
		failedOrders, kt.Rid)

	return nil
}

// recoverRunningTickets 获得状态为TicketStageRunning状态订单
func (r *applyRecoverer) recoverRunningTickets(kt *kit.Kit, tickets []*types.ApplyTicket) error {
	logs.Infof("start recover tickets, ticketNum: %d, rid: %s", len(tickets), kt.Rid)
	orders := make([]*types.ApplyOrder, 0)
	for _, ticket := range tickets {
		subOrders, err := r.getSuborders(kt, ticket.OrderId)
		if err != nil {
			logs.Errorf("failed to get suborders by orderId, will be skipped, err: %v, orderId: %d, rid: %s", err,
				ticket.OrderId, kt.Rid)
			continue
		}
		orders = append(orders, subOrders...)
	}

	success, failed := int64(0), int64(0)
	dealChan := make(chan struct{}, recovertask.ApplyGoroutinesNum)
	defer close(dealChan)
	wg := sync.WaitGroup{}
	for _, order := range orders {
		wg.Add(1)
		dealChan <- struct{}{}
		go func(kt *kit.Kit, order *types.ApplyOrder) {
			defer func() {
				wg.Done()
				<-dealChan
			}()

			if err := r.recoverOrders(kt, order); err != nil {
				atomic.AddInt64(&failed, 1)
				logs.Errorf("failed to recover order, err: %v, subOrderId: %s, rid: %s", err, order.SubOrderId, kt.Rid)
				return
			}
			atomic.AddInt64(&success, 1)
		}(kt, order)
	}
	wg.Wait()

	logs.Infof("end recover RUNNING tickets, totalOrderNum: %d, success: %d, failed: %d, rid: %s", len(orders), success,
		failed, kt.Rid)
	return nil
}

// recoverOrders 恢复状态为ApplyStatusMatching 和 ApplyStatusGracefulTerminate 状态订单
func (r *applyRecoverer) recoverOrders(kt *kit.Kit, order *types.ApplyOrder) error {
	// execute only one step
	if order.Status != types.ApplyStatusMatching && order.Status != types.ApplyStatusGracefulTerminate {
		logs.Infof("order status is not matching or graceful terminate, ignore it, suborderId: %s, status: %s,rid: %s",
			order.SubOrderId, order.Status, kt.Rid)
		return nil
	}

	logs.Infof("start recover order with status %s, subOrderId: %s, rid: %s", order.Status, order.SubOrderId, kt.Rid)
	// 恢复generate step
	generateStep, err := r.getOrderStep(kt, types.StepNameGenerate, order.SubOrderId)
	if err != nil {
		logs.Errorf("failed to get generate step, err: %v, suborderId: %s, rid: %s", err, order.SubOrderId, kt.Rid)
		return err
	}
	if generateStep.Status == types.StepStatusInit || generateStep.Status == types.StepStatusHandling {
		if err = r.recoverGenerateStep(kt, order, generateStep); err != nil {
			logs.Errorf("failed to recover apply generate step, err: %v, suborderId: %s, rid: %s", err,
				order.SubOrderId, kt.Rid)
			return err
		}
		logs.Infof("success recover order with status %s in generate step, subOrderId: %s, rid: %s",
			order.Status, order.SubOrderId, kt.Rid)
		return nil
	}
	// 恢复init step
	initStep, err := r.getOrderStep(kt, types.StepNameInit, order.SubOrderId)
	if err != nil {
		logs.Errorf("failed to get init step, err: %v, suborderId: %s, rid: %s", err, order.SubOrderId, kt.Rid)
		return err
	}
	if initStep.Status == types.StepStatusInit || initStep.Status == types.StepStatusHandling {
		if err = r.recoverInitStep(kt, order); err != nil {
			logs.Errorf("failed to recover apply init step, err: %v, suborderId: %s, rid: %s", err,
				order.SubOrderId, kt.Rid)
			return err
		}
		logs.Infof("success recover order with status %s in init step, subOrderId: %s, rid: %s",
			order.Status, order.SubOrderId, kt.Rid)
		return nil
	}
	// 恢复deliver step
	deliverStep, err := r.getOrderStep(kt, types.StepNameDeliver, order.SubOrderId)
	if err != nil {
		logs.Errorf("failed to get deliver step, suborderId: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return err
	}
	if deliverStep.Status == types.StepStatusInit || deliverStep.Status == types.StepStatusHandling {
		if err = r.recoverDeliverStep(kt, order); err != nil {
			logs.Errorf("failed to recover apply deliver step, err: %v, suborderId: %s, rid: %s", err,
				order.SubOrderId, kt.Rid)
			return err
		}
		logs.Infof("success recover order with status %s in deliver step, subOrderId: %s, rid: %s",
			order.Status, order.SubOrderId, kt.Rid)
		return nil
	}

	// 特殊处理 GracefulTerminate 状态
	// 由于 GracefulTerminate 可能出现前序状态均已完毕，无法触发单据更新逻辑，于此处进行单据状态更新
	if order.Status == types.ApplyStatusGracefulTerminate && order.Stage == types.TicketStageRunning {
		if err = r.schedulerIf.GetMatcher().UpdateApplyOrderStatus(kt, order); err != nil {
			logs.Errorf("failed to update apply order status, subOrderId: %s, err: %v, rid: %s",
				order.SubOrderId, err, kt.Rid)
			return err
		}

	}

	logs.Infof("end recover order with status %s, subOrderId: %s, rid: %s", order.Status, order.SubOrderId, kt.Rid)
	return nil
}

// applyRecoverer provides apply resource recycle service
type applyRecoverer struct {
	schedulerIf scheduler.Interface
	itsmCli     itsm.Client
	cmdbCli     cmdb.Client
	sopsCli     sopsapi.SopsClientInterface
}
