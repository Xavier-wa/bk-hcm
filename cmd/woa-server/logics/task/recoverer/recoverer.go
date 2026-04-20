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

// Package recoverer provides ...
package recoverer

import (
	"hcm/cmd/woa-server/logics/task/recoverer/apply"
	"hcm/cmd/woa-server/logics/task/recoverer/recycle"
	"hcm/cmd/woa-server/logics/task/recycler"
	"hcm/cmd/woa-server/logics/task/scheduler"
	"hcm/pkg/cc"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/thirdparty/api-gateway/itsm"
	"hcm/pkg/thirdparty/api-gateway/sopsapi"
)

// New create a recoverer
func New(kt *kit.Kit, cfg *cc.Recover, itsmCli itsm.Client, recycler recycler.Interface, scheduler scheduler.Interface,
	cmdbCli cmdb.Client, sopsCli sopsapi.SopsClientInterface, sd serviced.State) error {

	// 查看配置是否开启
	if cfg.EnableApplyRecover {
		logs.Infof("start apply recover service, rid: %s", kt.Rid)
		if err := apply.StartRecover(kt, itsmCli, scheduler, cmdbCli, sopsCli, sd); err != nil {
			logs.Errorf("failed to start apply recoverer, err: %v, rid: %s", err, kt.Rid)
			return err
		}
	}

	if cfg.EnableRecycleRecover {
		logs.Infof("start recycle recover service, rid: %s", kt.Rid)
		if err := recycle.StartRecover(kt, itsmCli, recycler, cmdbCli, sd); err != nil {
			logs.Errorf("failed to start recycle recoverer, err: %v, rid: %s", err, kt.Rid)
			return err
		}
	}

	return nil
}
