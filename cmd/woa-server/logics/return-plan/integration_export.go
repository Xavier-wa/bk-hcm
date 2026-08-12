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

package returnplan

import (
	"hcm/cmd/woa-server/logics/biz"
	"hcm/cmd/woa-server/logics/return-plan/fetcher"
	"hcm/pkg/client"
	"hcm/pkg/thirdparty/cvmapi"
)

// NewIntegrationTestController 创建用于集成测试的控制器实例，仅提供管理接口业务逻辑，不启动后台调度器。
// bizLogics 用于覆盖追加链路的业务→组织维度转换（集成测试中可传入 mock）。
func NewIntegrationTestController(cli *client.ClientSet, crpCli cvmapi.CVMClientInterface,
	bizLogics biz.Logics) *Controller {

	return &Controller{
		client:     cli,
		crpCli:     crpCli,
		resFetcher: fetcher.New(cli),
		bizLogics:  bizLogics,
	}
}
