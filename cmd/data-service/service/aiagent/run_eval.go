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
	"fmt"

	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/criteria/errf"
	daoaiagent "hcm/pkg/dal/dao/aiagent"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/types"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"github.com/jmoiron/sqlx"
)

// CreateAiagentRunEval creates an eval row.
func (svc *service) CreateAiagentRunEval(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.CreateAiagentRunEvalReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("create aiagent run eval decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("create aiagent run eval validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	var result *dsaiagent.CreateAiagentRunEvalResult
	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, _ *orm.TxnOption) (interface{}, error) {
		model := evalReqToTable(req)
		id, err := svc.dao.AiagentRunEval().CreateWithTx(cts.Kit, txn, model)
		if err != nil {
			return nil, fmt.Errorf("create aiagent run eval failed, err: %v", err)
		}
		result = &dsaiagent.CreateAiagentRunEvalResult{ID: id}
		return result, nil
	})
	if err != nil {
		logs.Errorf("create aiagent run eval commit txn failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// GetAiagentRunEval returns one eval by run_id.
func (svc *service) GetAiagentRunEval(cts *rest.Contexts) (interface{}, error) {
	runID := cts.PathParameter("run_id").String()
	if runID == "" {
		logs.Errorf("get aiagent run eval validate request failed, err: run_id is required, rid: %s",
			cts.Kit.Rid)
		return nil, errf.New(errf.InvalidParameter, "run_id is required")
	}
	detail, err := svc.dao.AiagentRunEval().GetByRunID(cts.Kit, runID)
	if err != nil {
		// 首次评估没有 eval 行是正常探测，不打 error。
		if errf.Error(err).Code != errf.RecordNotFound {
			logs.Errorf("get aiagent run eval failed, err: %v, run_id: %s, rid: %s",
				err, runID, cts.Kit.Rid)
		}
		return nil, err
	}
	return detail, nil
}

// ListAiagentRunEvals lists eval rows.
func (svc *service) ListAiagentRunEvals(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.ListAiagentRunEvalReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("list aiagent run evals decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("list aiagent run evals validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	opt := &types.ListOption{Filter: req.Filter, Page: req.Page, Fields: req.Fields}
	result, err := svc.dao.AiagentRunEval().List(cts.Kit, opt)
	if err != nil {
		logs.Errorf("list aiagent run evals failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	return &dsaiagent.ListAiagentRunEvalResult{Count: result.Count, Details: result.Details}, nil
}

// ListAiagentRunEvalGaps lists terminal runs without eval.
func (svc *service) ListAiagentRunEvalGaps(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.ListAiagentRunEvalGapReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("list aiagent run eval gaps decode request failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("list aiagent run eval gaps validate request failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	from, err := req.FromTime()
	if err != nil {
		logs.Errorf("list aiagent run eval gaps parse from failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	to, err := req.ToTime()
	if err != nil {
		logs.Errorf("list aiagent run eval gaps parse to failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := svc.dao.AiagentRunEval().ListGap(cts.Kit, &daoaiagent.AiagentRunEvalGapOption{
		Lookback: req.Lookback(),
		From:     from,
		To:       to,
		Limit:    req.Limit,
		Count:    req.Count,
		Start:    req.Start,
	})
	if err != nil {
		logs.Errorf("list aiagent run eval gaps failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	return &dsaiagent.ListAiagentRunResult{Count: result.Count, Details: result.Details}, nil
}

// OverwriteAiagentRunEval overwrites an existing eval row.
func (svc *service) OverwriteAiagentRunEval(cts *rest.Contexts) (interface{}, error) {
	req := new(dsaiagent.OverwriteAiagentRunEvalReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("overwrite aiagent run eval decode request failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("overwrite aiagent run eval validate request failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, _ *orm.TxnOption) (interface{}, error) {
		model := evalReqToTable(&req.CreateAiagentRunEvalReq)
		if err := svc.dao.AiagentRunEval().UpdateByRunIDWithTx(cts.Kit, txn, model); err != nil {
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("overwrite aiagent run eval commit txn failed, err: %v, run_id: %s, rid: %s",
			err, req.RunID, cts.Kit.Rid)
		return nil, err
	}
	return nil, nil
}

func evalReqToTable(req *dsaiagent.CreateAiagentRunEvalReq) *tableaiagent.RunEvalTable {
	return &tableaiagent.RunEvalTable{
		RunID:           req.RunID,
		SessionID:       req.SessionID,
		User:            req.User,
		BkBizID:         req.BkBizID,
		StartRunID:      req.StartRunID,
		ProcessScore:    req.ProcessScore,
		OutcomeScore:    req.OutcomeScore,
		QualityScore:    req.QualityScore,
		Redlines:        req.Redlines,
		ReasonCode:      req.ReasonCode,
		RubricVersion:   req.RubricVersion,
		EvalResult:      req.EvalResult,
		ContextSnapshot: req.ContextSnapshot,
		EvalTrace:       req.EvalTrace,
	}
}
