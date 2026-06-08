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

// Package dispatcher ...
package dispatcher

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	"hcm/cmd/woa-server/logics/biz"
	demandtime "hcm/cmd/woa-server/logics/plan/demand-time"
	"hcm/cmd/woa-server/logics/plan/fetcher"
	"hcm/cmd/woa-server/storage/driver/mongodb"
	"hcm/cmd/woa-server/types/device"
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"
	"hcm/pkg/thirdparty/api-gateway/itsm"
	"hcm/pkg/thirdparty/cvmapi"
	"hcm/pkg/tools/utils/wait"
)

// Dispatcher res plan ticket and sub ticket lifecycle dispatcher.
type Dispatcher struct {
	resPlanCfg     cc.ResPlan
	sd             serviced.State
	dao            dao.Set
	client         *client.ClientSet
	bkHcmURL       string
	itsmCli        itsm.Client
	itsmFlow       cc.ItsmFlow
	crpAuditNode   cc.StateNode
	crpCli         cvmapi.CVMClientInterface
	bizLogics      biz.Logics
	deviceTypesMap *device.DeviceTypesMap
	demandTime     demandtime.DemandTime
	ctx            context.Context

	ticketQueue    *ptypes.UniQueue
	subTicketQueue *ptypes.UniQueue

	resFetcher fetcher.Fetcher

	// processingTickets 记录正在处理中的单据ID，防止同一单据被并发处理
	processingTickets sync.Map
}

// New creates a resource plan ticket Dispatcher instance.
func New(ctx context.Context, sd serviced.State, client *client.ClientSet, dao dao.Set, itsmCli itsm.Client,
	crpCli cvmapi.CVMClientInterface, bizLogic biz.Logics, deviceTypesMap *device.DeviceTypesMap,
	fetch fetcher.Fetcher) (*Dispatcher, error) {

	var itsmFlowCfg cc.ItsmFlow
	for _, itsmFlow := range cc.WoaServer().ItsmFlows {
		if itsmFlow.ServiceName == enumor.TicketSvcNameResPlan {
			itsmFlowCfg = itsmFlow
			break
		}
	}

	var crpAuditNode cc.StateNode
	for _, node := range itsmFlowCfg.StateNodes {
		if node.NodeName == enumor.TicketNodeNameCrpAudit {
			crpAuditNode = node
		}
	}

	ctrl := &Dispatcher{
		resPlanCfg:     cc.WoaServer().ResPlan,
		dao:            dao,
		sd:             sd,
		client:         client,
		bkHcmURL:       cc.WoaServer().BkHcmURL,
		itsmCli:        itsmCli,
		itsmFlow:       itsmFlowCfg,
		crpAuditNode:   crpAuditNode,
		crpCli:         crpCli,
		bizLogics:      bizLogic,
		deviceTypesMap: deviceTypesMap,
		demandTime:     demandtime.NewDemandTimeFromTable(client),
		ctx:            ctx,
		ticketQueue:    ptypes.NewUniQueue(),
		subTicketQueue: ptypes.NewUniQueue(),
		resFetcher:     fetch,
	}

	go ctrl.Run()

	return ctrl, nil
}

// recoverLog define recover log
func (d *Dispatcher) recoverLog(keywords constant.WarnSign) {
	if r := recover(); r != nil {
		logs.Errorf("%s: panic: %v\n%s", keywords, r, debug.Stack())
	}
}

// Run starts dispatcher
func (d *Dispatcher) Run() {
	// 启动后需等待一段时间，mongo等服务初始化完成后才能开始定时任务 TODO 用统一的任务调度模块来执行定时任务，确保在初始化之后
	for {
		if mongodb.Client() == nil {
			logs.Warnf("mongodb client is not ready, wait seconds to retry")
			time.Sleep(constant.IntervalWaitTaskStart)
			continue
		}
		break
	}

	// ticket watcher
	go func() {
		defer d.recoverLog(constant.ResPlanTicketWatchFailed)

		// TODO: get interval from config
		// list and watch tickets every 20 seconds
		wait.JitterUntil(d.listAndWatchTickets, 20*time.Second, 0.5, true, d.ctx)
	}()

	// ticket handler
	// TODO: get worker num from config
	for i := 0; i < 10; i++ {
		go func() {
			defer d.recoverLog(constant.ResPlanTicketWatchFailed)

			// get and handle tickets every 5 seconds
			wait.JitterUntil(d.dealTicket, 5*time.Second, 0.5, true, d.ctx)
		}()
	}

	// sub ticket watcher
	go func() {
		defer d.recoverLog(constant.ResPlanTicketWatchFailed)

		// TODO: get interval from config
		// list and watch sub tickets every 20 seconds
		wait.JitterUntil(d.listAndWatchSubTickets, 20*time.Second, 0.5, true, d.ctx)
	}()

	// sub ticket handler
	// TODO: get worker num from config
	for i := 0; i < 10; i++ {
		go func() {
			defer d.recoverLog(constant.ResPlanTicketWatchFailed)

			// get and handle sub tickets every 2 seconds
			wait.JitterUntil(d.dealSubTicket, 2*time.Second, 0.5, true, d.ctx)
		}()
	}

	select {
	case <-d.ctx.Done():
		logs.Infof("resource plan ticket dispatcher exits")
	}
}
