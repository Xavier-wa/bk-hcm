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

func (svc *service) initIacSpecNodeRoutes(h *rest.Handler) {
	h.Add("BatchCreateIacSpecNode", http.MethodPost, "/iac_spec_node/batch/create", svc.BatchCreateIacSpecNode)
	h.Add("BatchUpdateIacSpecNode", http.MethodPatch, "/iac_spec_node/batch/update", svc.BatchUpdateIacSpecNode)
	h.Add("BatchDeleteIacSpecNode", http.MethodDelete, "/iac_spec_node/batch", svc.BatchDeleteIacSpecNode)
	h.Add("ListIacSpecNode", http.MethodPost, "/iac_spec_node/list", svc.ListIacSpecNode)
}

// BatchCreateIacSpecNode 批量创建 iac_spec_node 记录。
func (svc *service) BatchCreateIacSpecNode(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacSpecNodeBatchCreateReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("batch create iac spec node decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("batch create iac spec node validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	models := make([]tableiac.IacSpecNodeTable, 0, len(req.Items))
	for _, item := range req.Items {
		models = append(models, tableiac.IacSpecNodeTable{
			RepositoryID:    item.RepositoryID,
			SpecID:          item.SpecID,
			ParentID:        item.ParentID,
			Level:           item.Level,
			NodeType:        item.NodeType,
			NodeName:        item.NodeName,
			ChildrenNodeIDs: item.ChildrenNodeIDs,
			FileSize:        item.FileSize,
			Content:         item.Content,
			Creator:         cts.Kit.User,
			Reviser:         cts.Kit.User,
		})
	}

	createdIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		ids, err := svc.dao.IacSpecNode().BatchCreateWithTx(cts.Kit, txn, models)
		if err != nil {
			logs.Errorf("batch create iac spec node in db failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		return ids, nil
	})
	if err != nil {
		logs.Errorf("batch create iac spec node transaction failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	ids, ok := createdIDs.([]string)
	if !ok {
		return nil, fmt.Errorf("batch create iac spec node return ids type is not []string: %v",
			reflect.TypeOf(createdIDs).String())
	}

	return &core.BatchCreateResult{IDs: ids}, nil
}

// BatchUpdateIacSpecNode 批量更新 iac_spec_node 记录。
func (svc *service) BatchUpdateIacSpecNode(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacSpecNodeBatchUpdateReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("batch update iac spec node decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("batch update iac spec node validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		for _, item := range req.Items {
			model := &tableiac.IacSpecNodeTable{
				ParentID:        item.ParentID,
				NodeName:        item.NodeName,
				ChildrenNodeIDs: item.ChildrenNodeIDs,
				FileSize:        item.FileSize,
				Content:         item.Content,
				Reviser:         cts.Kit.User,
			}
			if err := svc.dao.IacSpecNode().UpdateWithTx(cts.Kit, txn, tools.EqualExpression("id", item.ID),
				model); err != nil {
				logs.Errorf("update iac spec node in db failed, id: %s, err: %v, rid: %s", item.ID, err, cts.Kit.Rid)
				return nil, err
			}
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("batch update iac spec node transaction failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return nil, nil
}

// BatchDeleteIacSpecNode 批量删除 iac_spec_node 记录。
func (svc *service) BatchDeleteIacSpecNode(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacSpecNodeBatchDeleteReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("batch delete iac spec node decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("batch delete iac spec node validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (any, error) {
		if err := svc.dao.IacSpecNode().DeleteWithTx(cts.Kit, txn, req.Filter); err != nil {
			logs.Errorf("delete iac spec node in db failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("batch delete iac spec node transaction failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return nil, nil
}

// ListIacSpecNode 列表查询 iac_spec_node 记录。
func (svc *service) ListIacSpecNode(cts *rest.Contexts) (any, error) {
	req := new(dataiac.IacSpecNodeListReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("list iac spec node decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("list iac spec node validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	res, err := svc.dao.IacSpecNode().List(cts.Kit, &types.ListOption{
		Fields: req.Fields,
		Filter: req.Filter,
		Page:   req.Page,
	})
	if err != nil {
		logs.Errorf("list iac spec node failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	if req.Page.Count {
		return &dataiac.IacSpecNodeListResult{Count: res.Count}, nil
	}

	details := make([]coreiac.IacSpecNode, 0, len(res.Details))
	for i := range res.Details {
		row := &res.Details[i]
		details = append(details, coreiac.IacSpecNode{
			ID:              row.ID,
			RepositoryID:    row.RepositoryID,
			SpecID:          row.SpecID,
			ParentID:        row.ParentID,
			Level:           row.Level,
			NodeType:        row.NodeType,
			NodeName:        row.NodeName,
			ChildrenNodeIDs: row.ChildrenNodeIDs,
			FileSize:        row.FileSize,
			Content:         row.Content,
			TenantID:        row.TenantID,
			Revision: &core.Revision{
				Creator:   row.Creator,
				Reviser:   row.Reviser,
				CreatedAt: row.CreatedAt.String(),
				UpdatedAt: row.UpdatedAt.String(),
			},
		})
	}

	return &dataiac.IacSpecNodeListResult{Details: details}, nil
}
