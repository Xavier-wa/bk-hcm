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

// Package dispatcher 退回计划单据调度器：主单拆单、子单提单/轮询/超时、主单状态与失败原因聚合。
// 相比资源预测调度器，退回计划无 ITSM 阶段、无本地明细/状态表，读写单据统一经 data-service client。
package dispatcher

import (
	"context"
	"runtime/debug"
	"sync"

	"hcm/cmd/woa-server/logics/return-plan/splitter"
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"
	"hcm/pkg/thirdparty/cvmapi"
	"hcm/pkg/tools/utils/wait"
)

// Dispatcher 退回计划主单/子单生命周期调度器。
type Dispatcher struct {
	sd       serviced.State
	client   *client.ClientSet
	crpCli   cvmapi.CVMClientInterface
	splitter splitter.Splitter
	ctx      context.Context

	ticketQueue    *ptypes.UniQueue
	subTicketQueue *ptypes.UniQueue

	// processingTickets 记录正在处理中的主单ID，防止同一主单被并发处理
	processingTickets sync.Map
	// processingSubTickets 记录正在处理中的子单ID，防止同一子单被并发处理
	processingSubTickets sync.Map
}

// New 创建退回计划调度器并异步启动。
func New(ctx context.Context, sd serviced.State, cli *client.ClientSet, crpCli cvmapi.CVMClientInterface) (
	*Dispatcher, error) {

	d := &Dispatcher{
		sd:             sd,
		client:         cli,
		crpCli:         crpCli,
		splitter:       splitter.New(cli),
		ctx:            ctx,
		ticketQueue:    ptypes.NewUniQueue(),
		subTicketQueue: ptypes.NewUniQueue(),
	}

	go d.Run()

	return d, nil
}

// recoverLog 兜底 recover 并记录日志。
func (d *Dispatcher) recoverLog(keywords constant.WarnSign) {
	if r := recover(); r != nil {
		logs.Errorf("%s: panic: %v\n%s", keywords, r, debug.Stack())
	}
}

// Run 启动调度器：主单/子单各一 watcher + 多 handler，仅 master 节点实际处理。
// watcher 拉取间隔、handler 并发数与处理间隔均由 woa-server 配置 returnPlan.dispatcher 驱动。
func (d *Dispatcher) Run() {
	cfg := cc.WoaServer().ReturnPlan.Dispatcher

	// 主单 watcher
	go func() {
		defer d.recoverLog(constant.ReturnPlanTicketWatchFailed)

		wait.JitterUntil(d.listAndWatchTickets, cfg.TicketWatchInterval, 0.5, true, d.ctx)
	}()

	// 主单 handler
	for i := 0; i < cfg.TicketWorkerNum; i++ {
		go func() {
			defer d.recoverLog(constant.ReturnPlanTicketWatchFailed)

			wait.JitterUntil(d.dealTicket, cfg.TicketDealInterval, 0.5, true, d.ctx)
		}()
	}

	// 子单 watcher
	go func() {
		defer d.recoverLog(constant.ReturnPlanTicketWatchFailed)

		wait.JitterUntil(d.listAndWatchSubTickets, cfg.SubTicketWatchInterval, 0.5, true, d.ctx)
	}()

	// 子单 handler
	for i := 0; i < cfg.SubTicketWorkerNum; i++ {
		go func() {
			defer d.recoverLog(constant.ReturnPlanTicketWatchFailed)

			wait.JitterUntil(d.dealSubTicket, cfg.SubTicketDealInterval, 0.5, true, d.ctx)
		}()
	}

	<-d.ctx.Done()
	logs.Infof("return plan ticket dispatcher exits")
}
