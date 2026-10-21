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

package plugin

import (
	"fmt"
	"reflect"

	"hcm/pkg/api/core"
	dataplugin "hcm/pkg/api/data-service/plugin"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/types"
	tableplugin "hcm/pkg/dal/table/plugin"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"github.com/jmoiron/sqlx"
)

// CreateIacPluginSchema 创建 iac_plugin_schema 记录。
func (svc *service) CreateIacPluginSchema(cts *rest.Contexts) (interface{}, error) {
	req := new(dataplugin.IacPluginSchemaCreateReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("create iac plugin schema decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("create iac plugin schema validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	createdID, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		model := &tableplugin.IacPluginSchemaTable{
			PluginName:    req.PluginName,
			PluginVersion: req.PluginVersion,
			SchemaHash:    req.SchemaHash,
			SchemaDetail:  req.SchemaDetail,
			Creator:       cts.Kit.User,
			Reviser:       cts.Kit.User,
		}

		id, err := svc.dao.IacPluginSchema().CreateWithTx(cts.Kit, txn, model)
		if err != nil {
			logs.Errorf("create iac plugin schema in db failed, plugin: %s@%s, err: %v, rid: %s",
				req.PluginName, req.PluginVersion, err, cts.Kit.Rid)
			return nil, err
		}

		return id, nil
	})
	if err != nil {
		logs.Errorf("create iac plugin schema transaction failed, plugin: %s@%s, err: %v, rid: %s",
			req.PluginName, req.PluginVersion, err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	id, ok := createdID.(string)
	if !ok {
		return nil, fmt.Errorf("create iac plugin schema return id type is not string: %v",
			reflect.TypeOf(createdID).String())
	}

	return &core.CreateResult{ID: id}, nil
}

// ListIacPluginSchemas 列表查询 iac_plugin_schema，支持 fields 投影。
func (svc *service) ListIacPluginSchemas(cts *rest.Contexts) (interface{}, error) {
	req := new(dataplugin.IacPluginSchemaListReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("list iac plugin schema decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("list iac plugin schema validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	opt := &types.ListOption{
		Fields: req.Fields,
		Filter: req.Filter,
		Page:   req.Page,
	}
	res, err := svc.dao.IacPluginSchema().List(cts.Kit, opt)
	if err != nil {
		logs.Errorf("list iac plugin schema failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	if req.Page.Count {
		return &dataplugin.IacPluginSchemaListResult{Count: res.Count}, nil
	}

	return &dataplugin.IacPluginSchemaListResult{Details: res.Details}, nil
}
