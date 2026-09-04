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

package eval

import (
	"encoding/json"

	evalogic "hcm/cmd/agent-server/logics/eval"
	proto "hcm/pkg/api/agent-server/eval"
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// GetAgentEvalRun returns ledger + eval detail. Track events are never queried.
func (svc *service) GetAgentEvalRun(cts *rest.Contexts) (interface{}, error) {
	if err := svc.authorizeManage(cts.Kit); err != nil {
		return nil, err
	}
	runID := cts.PathParameter("run_id").String()
	if runID == "" {
		logs.Errorf("get eval run validate request failed, err: run_id is required, rid: %s",
			cts.Kit.Rid)
		return nil, errf.New(errf.InvalidParameter, "run_id is required")
	}
	includeTrace := cts.Request.QueryParameter("include_trace") == "true"

	runResult, err := svc.cli.DataService().Aiagent.Run.List(cts.Kit, &core.ListReq{
		Filter: tools.EqualExpression("run_id", runID),
		Page:   &core.BasePage{Start: 0, Limit: 1},
	})
	if err != nil {
		logs.Errorf("get eval run list failed, err: %v, run_id: %s, rid: %s", err, runID, cts.Kit.Rid)
		return nil, err
	}
	if runResult == nil || len(runResult.Details) == 0 {
		logs.Errorf("get eval run not found, run_id: %s, rid: %s", runID, cts.Kit.Rid)
		return nil, errf.New(errf.RecordNotFound, "run not found")
	}
	run := runResult.Details[0]
	resp := &proto.RunDetailResp{
		RunID:       run.RunID,
		SessionCode: run.SessionCode,
		User:        run.User,
		Scene:       string(run.Scene),
		Status:      string(run.Status),
		Query:       run.Query,
		Window:      []proto.WindowDetail{},
	}

	ev, err := svc.cli.DataService().Aiagent.RunEval.Get(cts.Kit, runID)
	if err != nil && errf.Error(err).Code != errf.RecordNotFound {
		logs.Errorf("get eval run eval failed, err: %v, run_id: %s, rid: %s", err, runID, cts.Kit.Rid)
		return nil, err
	}
	if ev == nil || ev.ID == "" {
		resp.Window = []proto.WindowDetail{{
			RunID: run.RunID, Transcript: jsonItems(run.Transcript), Query: run.Query,
		}}
		return resp, nil
	}

	var evalResult evalogic.EvalResult
	if err := json.Unmarshal([]byte(ev.EvalResult), &evalResult); err != nil {
		logs.Warnf("unmarshal eval result failed, err: %v, run_id: %s, rid: %s",
			err, runID, cts.Kit.Rid)
	}
	var snapshot evalogic.ContextSnapshot
	if err := json.Unmarshal([]byte(ev.ContextSnapshot), &snapshot); err != nil {
		logs.Warnf("unmarshal eval context snapshot failed, err: %v, run_id: %s, rid: %s",
			err, runID, cts.Kit.Rid)
	}
	resp.Summary = evalResult.Summary
	detail := &proto.EvalDetail{
		ProcessScore: ev.ProcessScore,
		OutcomeScore: ev.OutcomeScore,
		QualityScore: ev.QualityScore,
		Passed:       isPassed(ev.QualityScore, ev.Redlines),
		Redlines:     ev.Redlines,
		ReasonCode:   string(ev.ReasonCode),
		Dims:         evalResult.Dims,
		Summary:      evalResult.Summary,
	}
	if includeTrace {
		detail.Trace = json.RawMessage(ev.EvalTrace)
	}
	resp.Eval = detail
	resp.Window = svc.buildWindow(cts.Kit, snapshot)
	return resp, nil
}
