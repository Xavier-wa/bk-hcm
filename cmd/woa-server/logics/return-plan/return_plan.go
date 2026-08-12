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

// Package returnplan 退回计划业务控制器：对内启动调度器，对外提供管理接口业务逻辑。
package returnplan

import (
	"context"

	"hcm/cmd/woa-server/logics/biz"
	"hcm/cmd/woa-server/logics/return-plan/dispatcher"
	"hcm/cmd/woa-server/logics/return-plan/fetcher"
	"hcm/pkg/client"
	"hcm/pkg/serviced"
	"hcm/pkg/thirdparty/cvmapi"
)

// Controller 退回计划控制器，内部持有并驱动调度器，并实现管理接口业务逻辑。
type Controller struct {
	client     *client.ClientSet
	crpCli     cvmapi.CVMClientInterface
	dispatcher *dispatcher.Dispatcher
	resFetcher fetcher.Fetcher
	bizLogics  biz.Logics
}

// New 创建退回计划控制器，并在内部启动调度器
func New(sd serviced.State, cli *client.ClientSet, crpCli cvmapi.CVMClientInterface, bizLogics biz.Logics) (
	*Controller, error) {

	ctx := context.Background()

	dispatch, err := dispatcher.New(ctx, sd, cli, crpCli)
	if err != nil {
		return nil, err
	}

	return &Controller{
		client:     cli,
		crpCli:     crpCli,
		dispatcher: dispatch,
		resFetcher: fetcher.New(cli),
		bizLogics:  bizLogics,
	}, nil
}

// Fetch returns return plan fetcher.
func (c *Controller) Fetch() fetcher.Fetcher {
	return c.resFetcher
}
