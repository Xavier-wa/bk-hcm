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

// Package aiagent implements data-service handlers for the aiagent session module.
package aiagent

import (
	"fmt"
	"net/http"

	"hcm/cmd/data-service/service/capability"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"

	"github.com/jmoiron/sqlx"
)

// InitService registers aiagent session routes.
func InitService(cap *capability.Capability) {
	svc := &service{
		dao: cap.Dao,
	}

	h := rest.NewHandler()
	h.Add("CreateAIAgentSession", http.MethodPost, "/aiagent/sessions/create", svc.CreateAiagentSession)
	h.Add("UpdateAIAgentSession", http.MethodPatch, "/aiagent/sessions", svc.UpdateAiagentSession)
	h.Add("ListAIAgentSessions", http.MethodPost, "/aiagent/sessions/list", svc.ListAiagentSessions)
	h.Add("BatchDeleteAIAgentSessions", http.MethodDelete, "/aiagent/sessions/batch",
		svc.BatchDeleteAiagentSessions)
	h.Add("IncrAIAgentSessionContentCount", http.MethodPatch, "/aiagent/sessions/incr_content_count",
		svc.IncrAiagentSessionContentCount)

	h.Load(cap.WebService)
}

type service struct {
	dao dao.Set
}

// CreateAiagentSession creates a new aiagent session.
func (svc *service) CreateAiagentSession(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.CreateAiagentSessionReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	var result *dsaiagent.CreateAiagentSessionResult
	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, _ *orm.TxnOption) (interface{}, error) {
		sess := &tableaiagent.SessionTable{
			AppName:     req.AppName,
			User:        req.User,
			SessionName: req.SessionName,
			IsTemporary: req.IsTemporary,
			SessionTag:  req.SessionTag,
		}

		id, sessionCode, err := svc.dao.AiagentSession().CreateWithTx(cts.Kit, txn, sess)
		if err != nil {
			return nil, fmt.Errorf("create aiagent session failed, err: %v", err)
		}

		result = &dsaiagent.CreateAiagentSessionResult{
			ID:          id,
			SessionCode: sessionCode,
			ThreadID:    id,
		}
		return result, nil
	})
	if err != nil {
		logs.Errorf("create aiagent session commit txn failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return result, nil
}

// UpdateAiagentSession updates an aiagent session's name.
func (svc *service) UpdateAiagentSession(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.UpdateAiagentSessionReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	expr, err := tools.And(filter.AtomRule{Field: "id", Op: filter.Equal.Factory(), Value: req.ID})
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	sess := &tableaiagent.SessionTable{
		SessionName: req.SessionName,
		Reviser:     req.Reviser,
		SessionTag:  req.SessionTag,
	}

	if err = svc.dao.AiagentSession().Update(cts.Kit, expr, sess); err != nil {
		logs.Errorf("update aiagent session failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

// ListAiagentSessions queries aiagent sessions with filter and pagination.
func (svc *service) ListAiagentSessions(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.ListAiagentSessionReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	opt := &types.ListOption{
		Filter: req.Filter,
		Page:   req.Page,
		Fields: req.Fields,
	}

	result, err := svc.dao.AiagentSession().List(cts.Kit, opt)
	if err != nil {
		logs.Errorf("list aiagent sessions failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &dsaiagent.ListAiagentSessionResult{
		Count:   result.Count,
		Details: result.Details,
	}, nil
}

// BatchDeleteAiagentSessions deletes aiagent sessions by filter.
func (svc *service) BatchDeleteAiagentSessions(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.DeleteAiagentSessionReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, _ *orm.TxnOption) (interface{}, error) {
		return nil, svc.dao.AiagentSession().DeleteWithTx(cts.Kit, txn, req.Filter)
	})
	if err != nil {
		logs.Errorf("batch delete aiagent sessions failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

// IncrAiagentSessionContentCount increments session_content_count.
func (svc *service) IncrAiagentSessionContentCount(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.IncrContentCountReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := svc.dao.AiagentSession().IncrContentCount(cts.Kit, req.SessionCode); err != nil {
		logs.Errorf("incr aiagent session content count failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
