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
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	tableiac "hcm/pkg/dal/table/iac"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"github.com/jmoiron/sqlx"
)

func (svc *service) initIacSpecRoutes(h *rest.Handler) {
	h.Add("BatchCreateIacSpec", http.MethodPost, "/iac_spec/batch/create", svc.BatchCreateIacSpec)
	h.Add("BatchUpdateIacSpec", http.MethodPatch, "/iac_spec/batch/update", svc.BatchUpdateIacSpec)
	h.Add("BatchDeleteIacSpec", http.MethodDelete, "/iac_spec/batch", svc.BatchDeleteIacSpec)
	h.Add("ListIacSpec", http.MethodPost, "/iac_spec/list", svc.ListIacSpec)
}

// BatchCreateIacSpec 批量创建 iac_spec 记录。
func (svc *service) BatchCreateIacSpec(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacSpecBatchCreateReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("batch create iac spec decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("batch create iac spec validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	models := make([]tableiac.IacSpecTable, 0, len(req.Items))
	for _, item := range req.Items {
		models = append(models, tableiac.IacSpecTable{
			BkBizID:      item.BkBizID,
			RepositoryID: item.RepositoryID,
			Version:      item.Version,
			Status:       item.Status,
			LastRunID:    item.LastRunID,
			Memo:         item.Memo,
			Creator:      cts.Kit.User,
			Reviser:      cts.Kit.User,
		})
	}

	createdIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		ids, err := svc.dao.IacSpec().BatchCreateWithTx(cts.Kit, txn, models)
		if err != nil {
			logs.Errorf("batch create iac spec in db failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		return ids, nil
	})
	if err != nil {
		logs.Errorf("batch create iac spec transaction failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	ids, ok := createdIDs.([]string)
	if !ok {
		return nil, fmt.Errorf("batch create iac spec return ids type is not []string: %v",
			reflect.TypeOf(createdIDs).String())
	}

	return &core.BatchCreateResult{IDs: ids}, nil
}

// BatchUpdateIacSpec 批量更新 iac_spec 记录。
func (svc *service) BatchUpdateIacSpec(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacSpecBatchUpdateReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("batch update iac spec decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("batch update iac spec validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		for _, item := range req.Items {
			model := &tableiac.IacSpecTable{
				Status:    item.Status,
				LastRunID: item.LastRunID,
				Memo:      item.Memo,
				Reviser:   cts.Kit.User,
			}
			if err := svc.dao.IacSpec().UpdateWithTx(cts.Kit, txn, tools.EqualExpression("id", item.ID),
				model); err != nil {
				logs.Errorf("update iac spec in db failed, id: %s, err: %v, rid: %s", item.ID, err, cts.Kit.Rid)
				return nil, err
			}
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("batch update iac spec transaction failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return nil, nil
}

// BatchDeleteIacSpec 批量删除 iac_spec 记录。
func (svc *service) BatchDeleteIacSpec(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacSpecBatchDeleteReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("batch delete iac spec decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("batch delete iac spec validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		if err := svc.dao.IacSpec().DeleteWithTx(cts.Kit, txn, req.Filter); err != nil {
			logs.Errorf("delete iac spec in db failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("batch delete iac spec transaction failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return nil, nil
}

// ListIacSpec 列表查询 iac_spec 记录。
func (svc *service) ListIacSpec(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacSpecListReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("list iac spec decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("list iac spec validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	res, err := svc.dao.IacSpec().List(cts.Kit, &types.ListOption{
		Fields: req.Fields,
		Filter: req.Filter,
		Page:   req.Page,
	})
	if err != nil {
		logs.Errorf("list iac spec failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	if req.Page.Count {
		return &dataiac.IacSpecListResult{Count: res.Count}, nil
	}

	details := make([]coreiac.IacSpec, 0, len(res.Details))
	for i := range res.Details {
		row := &res.Details[i]
		details = append(details, coreiac.IacSpec{
			ID:           row.ID,
			BkBizID:      row.BkBizID,
			RepositoryID: row.RepositoryID,
			Version:      row.Version,
			Status:       row.Status,
			LastRunID:    row.LastRunID,
			Memo:         row.Memo,
			TenantID:     row.TenantID,
			Revision: &core.Revision{
				Creator:   row.Creator,
				Reviser:   row.Reviser,
				CreatedAt: row.CreatedAt.String(),
				UpdatedAt: row.UpdatedAt.String(),
			},
		})
	}

	return &dataiac.IacSpecListResult{Details: details}, nil
}
