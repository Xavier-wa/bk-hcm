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

package iac

import (
	"fmt"
	"net/http"
	"reflect"

	"hcm/pkg/api/core"
	coreiac "hcm/pkg/api/core/iac"
	dataiac "hcm/pkg/api/data-service/iac"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	tableiac "hcm/pkg/dal/table/iac"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"github.com/jmoiron/sqlx"
)

func (svc *service) initIacStateRoutes(h *rest.Handler) {
	h.Add("BatchCreateIacState", http.MethodPost, "/iac_state/batch/create", svc.BatchCreateIacState)
	h.Add("BatchUpdateIacState", http.MethodPatch, "/iac_state/batch/update", svc.BatchUpdateIacState)
	h.Add("BatchDeleteIacState", http.MethodDelete, "/iac_state/batch", svc.BatchDeleteIacState)
	h.Add("ListIacState", http.MethodPost, "/iac_state/list", svc.ListIacState)
}

// BatchCreateIacState 批量创建 iac_state 记录。
func (svc *service) BatchCreateIacState(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacStateBatchCreateReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("batch create iac state decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("batch create iac state validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	models := make([]tableiac.IacStateTable, 0, len(req.Items))
	for _, item := range req.Items {
		deposed := item.Deposed
		if len(deposed) == 0 {
			deposed = tabletypes.JsonField(constant.IacStateEmptyDeposedJSON)
		}
		current := item.Current
		if len(current) == 0 {
			current = tabletypes.JsonField(constant.IacStateNullJSON)
		}
		importFromSpecID := item.ImportFromSpecID
		models = append(models, tableiac.IacStateTable{
			RepositoryID:     item.RepositoryID,
			BlockID:          item.BlockID,
			Kind:             item.Kind,
			Current:          current,
			Deposed:          deposed,
			LastFlowID:       item.LastFlowID,
			ImportFromSpecID: &importFromSpecID,
			Creator:          cts.Kit.User,
			Reviser:          cts.Kit.User,
		})
	}

	createdIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		ids, err := svc.dao.IacState().BatchCreateWithTx(cts.Kit, txn, models)
		if err != nil {
			logs.Errorf("batch create iac state in db failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		return ids, nil
	})
	if err != nil {
		logs.Errorf("batch create iac state transaction failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	ids, ok := createdIDs.([]string)
	if !ok {
		return nil, fmt.Errorf("batch create iac state return ids type is not []string: %v",
			reflect.TypeOf(createdIDs).String())
	}

	return &core.BatchCreateResult{IDs: ids}, nil
}

// BatchUpdateIacState 批量更新 iac_state 记录。
func (svc *service) BatchUpdateIacState(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacStateBatchUpdateReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("batch update iac state decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("batch update iac state validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		for _, item := range req.Items {
			model := &tableiac.IacStateTable{
				BlockID:          item.BlockID,
				Kind:             item.Kind,
				Current:          item.Current,
				Deposed:          item.Deposed,
				LastFlowID:       item.LastFlowID,
				ImportFromSpecID: item.ImportFromSpecID,
				Reviser:          cts.Kit.User,
			}
			if err := svc.dao.IacState().UpdateWithTx(cts.Kit, txn, tools.EqualExpression("id", item.ID),
				model); err != nil {
				logs.Errorf("update iac state in db failed, id: %s, err: %v, rid: %s", item.ID, err, cts.Kit.Rid)
				return nil, err
			}
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("batch update iac state transaction failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return nil, nil
}

// BatchDeleteIacState 批量删除 iac_state 记录。
func (svc *service) BatchDeleteIacState(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacStateBatchDeleteReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("batch delete iac state decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("batch delete iac state validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		if err := svc.dao.IacState().DeleteWithTx(cts.Kit, txn, req.Filter); err != nil {
			logs.Errorf("delete iac state in db failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("batch delete iac state transaction failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return nil, nil
}

// ListIacState 列表查询 iac_state 记录。
func (svc *service) ListIacState(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacStateListReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("list iac state decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("list iac state validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	res, err := svc.dao.IacState().List(cts.Kit, &types.ListOption{
		Fields: req.Fields,
		Filter: req.Filter,
		Page:   req.Page,
	})
	if err != nil {
		logs.Errorf("list iac state failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	if req.Page.Count {
		return &dataiac.IacStateListResult{Count: res.Count}, nil
	}

	details := make([]coreiac.IacState, 0, len(res.Details))
	for i := range res.Details {
		details = append(details, convertTableToIacState(&res.Details[i]))
	}

	return &dataiac.IacStateListResult{Details: details}, nil
}

func convertTableToIacState(row *tableiac.IacStateTable) coreiac.IacState {
	var currentPtr *tabletypes.JsonField
	if len(row.Current) > 0 && !row.Current.IsNull() {
		cur := row.Current
		currentPtr = &cur
	}

	deposed := row.Deposed
	if len(deposed) == 0 {
		deposed = tabletypes.JsonField(constant.IacStateEmptyDeposedJSON)
	}

	var importFromSpecID string
	if row.ImportFromSpecID != nil {
		importFromSpecID = *row.ImportFromSpecID
	}

	return coreiac.IacState{
		ID:               row.ID,
		RepositoryID:     row.RepositoryID,
		BlockID:          row.BlockID,
		Kind:             row.Kind,
		Current:          currentPtr,
		Deposed:          deposed,
		LastFlowID:       row.LastFlowID,
		ImportFromSpecID: importFromSpecID,
		TenantID:         row.TenantID,
		Revision: &core.Revision{
			Creator:   row.Creator,
			Reviser:   row.Reviser,
			CreatedAt: row.CreatedAt.String(),
			UpdatedAt: row.UpdatedAt.String(),
		},
	}
}
