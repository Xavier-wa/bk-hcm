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
	"net/http"
	"reflect"

	"hcm/pkg/api/core"
	coreplugin "hcm/pkg/api/core/plugin"
	dataplugin "hcm/pkg/api/data-service/plugin"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	tableplugin "hcm/pkg/dal/table/plugin"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"github.com/jmoiron/sqlx"
)

// TODO(temp_delete): 临时 iac_plugin_temp_delete data-service handler，合并插件注册分支后删除本文件。

func (svc *service) initIacPluginTempDeleteRoutes(h *rest.Handler) {
	h.Add("CreateIacPluginTempDelete", http.MethodPost, "/iac_plugins_temp_delete/create", svc.CreateIacPluginTempDelete)
	h.Add("UpdateIacPluginTempDelete", http.MethodPatch, "/iac_plugins_temp_delete/update", svc.UpdateIacPluginTempDelete)
	h.Add("ListIacPluginsTempDelete", http.MethodPost, "/iac_plugins_temp_delete/list", svc.ListIacPluginsTempDelete)
}

// CreateIacPluginTempDelete 创建 iac_plugin_temp_delete 记录。
func (svc *service) CreateIacPluginTempDelete(cts *rest.Contexts) (interface{}, error) {
	req := new(dataplugin.IacPluginTempDeleteCreateReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("create iac plugin temp decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("create iac plugin temp validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	createdID, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		model := &tableplugin.IacPluginTempDeleteTable{
			PluginName:    req.PluginName,
			PluginVersion: req.PluginVersion,
			Protocol:      req.Protocol,
			State:         req.State,
			Message:       req.Message,
			Extension:     req.Extension,
			Memo:          req.Memo,
			Creator:       cts.Kit.User,
			Reviser:       cts.Kit.User,
		}

		id, err := svc.dao.IacPluginTempDelete().CreateWithTx(cts.Kit, txn, model)
		if err != nil {
			logs.Errorf("create iac plugin temp in db failed, plugin: %s@%s, err: %v, rid: %s",
				req.PluginName, req.PluginVersion, err, cts.Kit.Rid)
			return nil, err
		}

		return id, nil
	})
	if err != nil {
		logs.Errorf("create iac plugin temp transaction failed, plugin: %s@%s, err: %v, rid: %s",
			req.PluginName, req.PluginVersion, err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	id, ok := createdID.(string)
	if !ok {
		return nil, fmt.Errorf("create iac plugin temp return id type is not string: %v",
			reflect.TypeOf(createdID).String())
	}

	return &core.CreateResult{ID: id}, nil
}

// UpdateIacPluginTempDelete 更新 iac_plugin_temp_delete 记录。
func (svc *service) UpdateIacPluginTempDelete(cts *rest.Contexts) (interface{}, error) {
	req := new(dataplugin.IacPluginTempDeleteUpdateReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("update iac plugin temp decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("update iac plugin temp validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		model := &tableplugin.IacPluginTempDeleteTable{
			State:   req.State,
			Memo:    req.Memo,
			Reviser: cts.Kit.User,
		}
		if req.Message != nil {
			model.Message = *req.Message
		}

		if err := svc.dao.IacPluginTempDelete().UpdateWithTx(cts.Kit, txn,
			tools.EqualExpression("id", req.ID), model); err != nil {
			logs.Errorf("update iac plugin temp in db failed, id: %s, err: %v, rid: %s", req.ID, err, cts.Kit.Rid)
			return nil, err
		}

		return nil, nil
	})
	if err != nil {
		logs.Errorf("update iac plugin temp transaction failed, id: %s, err: %v, rid: %s", req.ID, err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return nil, nil
}

// ListIacPluginsTempDelete 列表查询 iac_plugin_temp_delete 记录。
func (svc *service) ListIacPluginsTempDelete(cts *rest.Contexts) (interface{}, error) {
	req := new(dataplugin.IacPluginTempDeleteListReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("list iac plugin temp decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("list iac plugin temp validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	opt := &types.ListOption{
		Fields: req.Fields,
		Filter: req.Filter,
		Page:   req.Page,
	}
	res, err := svc.dao.IacPluginTempDelete().List(cts.Kit, opt)
	if err != nil {
		logs.Errorf("list iac plugin temp failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	if req.Page.Count {
		return &dataplugin.IacPluginTempDeleteListResult{Count: res.Count}, nil
	}

	details := make([]coreplugin.IacPluginTempDelete, 0, len(res.Details))
	for _, item := range res.Details {
		details = append(details, coreplugin.IacPluginTempDelete{
			ID:            item.ID,
			PluginName:    item.PluginName,
			PluginVersion: item.PluginVersion,
			Protocol:      item.Protocol,
			State:         item.State,
			Message:       item.Message,
			Extension:     item.Extension,
			Memo:          item.Memo,
			Revision: &core.Revision{
				Creator:   item.Creator,
				Reviser:   item.Reviser,
				CreatedAt: item.CreatedAt.String(),
				UpdatedAt: item.UpdatedAt.String(),
			},
		})
	}

	return &dataplugin.IacPluginTempDeleteListResult{Details: details}, nil
}
