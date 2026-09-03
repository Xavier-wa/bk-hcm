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

package aiagent

import (
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/types"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"github.com/jmoiron/sqlx"
)

// CreateAiagentRun 创建一条 aiagent_run。
//
// 两条写入路径共用本接口，实时记账行为不变：
//   - Ledger RUN_STARTED：只传 run_id/session/user/biz/scene/query；
//     status 为空则 DAO 默认 running，不改 created_at/updated_at。
//   - History sync：可带终态 status、transcript、occurred_at，在同一事务里回写时间戳。
func (svc *service) CreateAiagentRun(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.CreateAiagentRunReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("create aiagent run decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("create aiagent run validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	var result *dsaiagent.CreateAiagentRunResult
	// 事务原本就包住 CreateWithTx；history sync 的时间戳回写也必须同事务，避免只插入、未改时间。
	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, _ *orm.TxnOption) (interface{}, error) {
		id, err := svc.insertAiagentRun(cts.Kit, txn, req)
		if err != nil {
			logs.Errorf("create aiagent run failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		result = &dsaiagent.CreateAiagentRunResult{ID: id}
		return result, nil
	})
	if err != nil {
		logs.Errorf("create aiagent run commit txn failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// insertAiagentRun 插入一条 run，必要时回写 created_at / updated_at。
// occurred_at 回写仅用于 history sync；实时 Ledger 创建时该字段为空，
// CreateWithTx 之后直接返回，时间戳保持库默认 now()。
func (svc *service) insertAiagentRun(kt *kit.Kit, txn *sqlx.Tx, req *dsaiagent.CreateAiagentRunReq) (
	string, error) {

	run := &tableaiagent.RunTable{
		RunID:       req.RunID,
		SessionCode: req.SessionCode,
		User:        req.User,
		BkBizID:     req.BkBizID,
		Scene:       req.Scene,
		// Status / Reason / Transcript：实时路径为空；空 status 在 CreateWithTx 里落成 running。
		Status:     req.Status,
		Reason:     req.Reason,
		Query:      req.Query,
		Transcript: req.Transcript,
	}
	id, err := svc.dao.AiagentRun().CreateWithTx(kt, txn, run)
	if err != nil {
		return "", err
	}
	// 实时记账不传 occurred_at，跳过时间戳回写。
	if req.OccurredAt == "" {
		return id, nil
	}
	endedAt := req.EndedAt
	if endedAt == "" {
		endedAt = req.OccurredAt
	}
	if err := svc.dao.AiagentRun().PatchOccurredAtWithTx(kt, txn, req.RunID, req.OccurredAt, endedAt,
		kt.User); err != nil {
		return "", err
	}
	return id, nil
}

// UpdateAiagentRunStatus CAS-updates a running row to a terminal status.
func (svc *service) UpdateAiagentRunStatus(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.UpdateAiagentRunStatusReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("cas update aiagent run decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("cas update aiagent run validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := svc.dao.AiagentRun().UpdateStatusCAS(cts.Kit, req.RunID, req.Status, req.Reason,
		req.Query, req.Transcript, cts.Kit.User); err != nil {
		logs.Errorf("cas update aiagent run failed, err: %v, run_id: %s, rid: %s",
			err, req.RunID, cts.Kit.Rid)
		return nil, err
	}
	return nil, nil
}

// PatchAiagentRunTranscript backfills query/transcript without refreshing updated_at.
func (svc *service) PatchAiagentRunTranscript(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.PatchAiagentRunTranscriptReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("patch aiagent run transcript decode request failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("patch aiagent run transcript validate request failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := svc.dao.AiagentRun().PatchTranscript(cts.Kit, req.RunID, req.Query, req.Transcript,
		cts.Kit.User); err != nil {
		logs.Errorf("patch aiagent run transcript failed, err: %v, run_id: %s, rid: %s",
			err, req.RunID, cts.Kit.Rid)
		return nil, err
	}
	return nil, nil
}

// ListAiagentRuns lists aiagent_run rows.
func (svc *service) ListAiagentRuns(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.ListAiagentRunReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("list aiagent runs decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("list aiagent runs validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	opt := &types.ListOption{Filter: req.Filter, Page: req.Page, Fields: req.Fields}
	result, err := svc.dao.AiagentRun().List(cts.Kit, opt)
	if err != nil {
		logs.Errorf("list aiagent runs failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	return &dsaiagent.ListAiagentRunResult{Count: result.Count, Details: result.Details}, nil
}
