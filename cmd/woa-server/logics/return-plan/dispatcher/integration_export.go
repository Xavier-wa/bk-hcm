//go:build integration

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
	"context"

	"hcm/cmd/woa-server/logics/return-plan/splitter"
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/client"
	"hcm/pkg/serviced"
	"hcm/pkg/thirdparty/cvmapi"
)

// IntegrationTestDispatcher 暴露调度器 handler 供集成测试同步驱动，不启动后台 goroutine。
type IntegrationTestDispatcher struct {
	*Dispatcher
}

// NewIntegrationTestDispatcher 创建用于集成测试的调度器实例。
func NewIntegrationTestDispatcher(ctx context.Context, sd serviced.State, cli *client.ClientSet,
	crpCli cvmapi.CVMClientInterface) *IntegrationTestDispatcher {

	return &IntegrationTestDispatcher{
		Dispatcher: &Dispatcher{
			sd:             sd,
			client:         cli,
			crpCli:         crpCli,
			splitter:       splitter.New(cli),
			ctx:            ctx,
			ticketQueue:    ptypes.NewUniQueue(),
			subTicketQueue: ptypes.NewUniQueue(),
		},
	}
}

// ListAndWatchTickets 同步触发主单 watcher。
func (d *IntegrationTestDispatcher) ListAndWatchTickets() error {
	return d.listAndWatchTickets()
}

// DealTicket 同步触发主单 handler。
func (d *IntegrationTestDispatcher) DealTicket() error {
	return d.dealTicket()
}

// ListAndWatchSubTickets 同步触发子单 watcher。
func (d *IntegrationTestDispatcher) ListAndWatchSubTickets() error {
	return d.listAndWatchSubTickets()
}

// DealSubTicket 同步触发子单 handler。
func (d *IntegrationTestDispatcher) DealSubTicket() error {
	return d.dealSubTicket()
}
