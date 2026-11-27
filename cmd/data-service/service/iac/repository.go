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

func (svc *service) initIacRepositoryRoutes(h *rest.Handler) {
	h.Add("BatchCreateIacRepository", http.MethodPost, "/iac_repository/batch/create", svc.BatchCreateIacRepository)
	h.Add("BatchUpdateIacRepository", http.MethodPatch, "/iac_repository/batch/update", svc.BatchUpdateIacRepository)
	h.Add("BatchDeleteIacRepository", http.MethodDelete, "/iac_repository/batch", svc.BatchDeleteIacRepository)
	h.Add("ListIacRepository", http.MethodPost, "/iac_repository/list", svc.ListIacRepository)
}

// BatchCreateIacRepository 批量创建 iac_repository 记录。
func (svc *service) BatchCreateIacRepository(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacRepositoryBatchCreateReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("batch create iac repository decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("batch create iac repository validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	models := make([]tableiac.IacRepositoryTable, 0, len(req.Items))
	for _, item := range req.Items {
		models = append(models, tableiac.IacRepositoryTable{
			BkBizID:         item.BkBizID,
			Name:            item.Name,
			LatestVersion:   item.LatestVersion,
			LastExecVersion: item.LastExecVersion,
			Memo:            item.Memo,
			Creator:         cts.Kit.User,
			Reviser:         cts.Kit.User,
		})
	}

	createdIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		ids, err := svc.dao.IacRepository().BatchCreateWithTx(cts.Kit, txn, models)
		if err != nil {
			logs.Errorf("batch create iac repository in db failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		return ids, nil
	})
	if err != nil {
		logs.Errorf("batch create iac repository transaction failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	ids, ok := createdIDs.([]string)
	if !ok {
		return nil, fmt.Errorf("batch create iac repository return ids type is not []string: %v",
			reflect.TypeOf(createdIDs).String())
	}

	return &core.BatchCreateResult{IDs: ids}, nil
}

// BatchUpdateIacRepository 批量更新 iac_repository 记录。
func (svc *service) BatchUpdateIacRepository(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacRepositoryBatchUpdateReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("batch update iac repository decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("batch update iac repository validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		for _, item := range req.Items {
			model := &tableiac.IacRepositoryTable{
				Name:            item.Name,
				LatestVersion:   item.LatestVersion,
				LastExecVersion: item.LastExecVersion,
				Memo:            item.Memo,
				Reviser:         cts.Kit.User,
			}
			if err := svc.dao.IacRepository().UpdateWithTx(cts.Kit, txn, tools.EqualExpression("id", item.ID),
				model); err != nil {
				logs.Errorf("update iac repository in db failed, id: %s, err: %v, rid: %s", item.ID, err, cts.Kit.Rid)
				return nil, err
			}
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("batch update iac repository transaction failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return nil, nil
}

// BatchDeleteIacRepository 批量删除 iac_repository 记录。
func (svc *service) BatchDeleteIacRepository(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacRepositoryBatchDeleteReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("batch delete iac repository decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("batch delete iac repository validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		if err := svc.dao.IacRepository().DeleteWithTx(cts.Kit, txn, req.Filter); err != nil {
			logs.Errorf("delete iac repository in db failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("batch delete iac repository transaction failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return nil, nil
}

// ListIacRepository 列表查询 iac_repository 记录。
func (svc *service) ListIacRepository(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacRepositoryListReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("list iac repository decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("list iac repository validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	res, err := svc.dao.IacRepository().List(cts.Kit, &types.ListOption{
		Fields: req.Fields,
		Filter: req.Filter,
		Page:   req.Page,
	})
	if err != nil {
		logs.Errorf("list iac repository failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	if req.Page.Count {
		return &dataiac.IacRepositoryListResult{Count: res.Count}, nil
	}

	details := make([]coreiac.IacRepository, 0, len(res.Details))
	for i := range res.Details {
		row := &res.Details[i]
		details = append(details, coreiac.IacRepository{
			ID:              row.ID,
			BkBizID:         row.BkBizID,
			Name:            row.Name,
			LatestVersion:   row.LatestVersion,
			LastExecVersion: row.LastExecVersion,
			Memo:            row.Memo,
			TenantID:        row.TenantID,
			Revision: &core.Revision{
				Creator:   row.Creator,
				Reviser:   row.Reviser,
				CreatedAt: row.CreatedAt.String(),
				UpdatedAt: row.UpdatedAt.String(),
			},
		})
	}

	return &dataiac.IacRepositoryListResult{Details: details}, nil
}
