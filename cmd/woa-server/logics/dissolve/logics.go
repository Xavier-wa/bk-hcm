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

package dissolve

import (
	"context"
	"time"

	"hcm/cmd/woa-server/logics/config"
	dissolveconfig "hcm/cmd/woa-server/logics/dissolve/config"
	"hcm/cmd/woa-server/logics/dissolve/host"
	dissolvetable "hcm/cmd/woa-server/logics/dissolve/table"
	"hcm/pkg/api/core"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/thirdparty"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	esCli "hcm/pkg/thirdparty/es"
	"hcm/pkg/tools/utils/wait"
)

// Logics provides resource dissolve logics
type Logics interface {
	RecycledHost() host.RecycledHost
	Table() dissolvetable.Table
	Config() dissolveconfig.Config
}

type logics struct {
	recycledHost host.RecycledHost
	table        dissolvetable.Table
	config       dissolveconfig.Config
}

// New create a logics manager
func New(cmdbCli cmdb.Client, esCli *esCli.EsCli, thirdCli *thirdparty.Client,
	conf cc.WoaServerSetting, configLogics config.Logics, cliSet *client.ClientSet) Logics {

	dissolveConfig := dissolveconfig.New(cliSet)
	originDate := conf.ResDissolve.OriginDate
	recycledHost := host.New(cliSet, cmdbCli, esCli, thirdCli, dissolveConfig, conf.ResDissolve.IgnoreBiz,
		conf.ResDissolve.SvrTypeNames, originDate)

	if conf.ResDissolve.SyncDissolveHost {
		workFunc := func() error {
			kt := core.NewBackendKit()
			return recycledHost.Sync(kt)
		}
		go wait.Until(workFunc, 30*time.Minute, context.Background())
	}

	table := dissolvetable.New(recycledHost, dissolveConfig, configLogics, esCli, originDate,
		conf.ResDissolve.ListExcludedProjectIDs, cliSet)
	return &logics{
		recycledHost: recycledHost,
		table:        table,
		config:       dissolveConfig,
	}
}

// RecycledHost recycled host interface
func (l *logics) RecycledHost() host.RecycledHost {
	return l.recycledHost
}

// Table resource dissolve table interface
func (l *logics) Table() dissolvetable.Table {
	return l.table
}

// Config resource dissolve config interface
func (l *logics) Config() dissolveconfig.Config {
	return l.config
}
