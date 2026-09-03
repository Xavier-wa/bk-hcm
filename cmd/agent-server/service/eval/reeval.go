/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may obtain a copy of the License at http://opensource.org/licenses/MIT
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
	proto "hcm/pkg/api/agent-server/eval"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/converter"
)

// Reeval enqueues a manual evaluation. overwrite is required to replace a row.
func (svc *service) Reeval(cts *rest.Contexts) (interface{}, error) {
	runID := cts.PathParameter("run_id").String()
	if runID == "" {
		logs.Errorf("reeval validate request failed, err: run_id is required, rid: %s", cts.Kit.Rid)
		return nil, errf.New(errf.InvalidParameter, "run_id is required")
	}
	req := new(proto.ReevalReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("reeval decode request failed, err: %v, run_id: %s, rid: %s",
			err, runID, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("reeval validate request failed, err: %v, run_id: %s, rid: %s",
			err, runID, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := svc.authorizeManage(cts.Kit); err != nil {
		return nil, err
	}
	if !cc.AgentServer().Eval.IsEnabled() {
		logs.Errorf("reeval skipped, eval is disabled, rid: %s", cts.Kit.Rid)
		return nil, errf.New(errf.Aborted, "eval is disabled")
	}

	existingEval, err := svc.cli.DataService().Aiagent.RunEval.Get(cts.Kit, runID)
	if err != nil && errf.Error(err).Code != errf.RecordNotFound {
		logs.Errorf("get eval for reeval failed, err: %v, run_id: %s, rid: %s", err, runID, cts.Kit.Rid)
		return nil, err
	}
	if err := requireOverwrite(existingEval, converter.PtrToVal(req.Overwrite)); err != nil {
		logs.Errorf("reeval overwrite required, run_id: %s, rid: %s", runID, cts.Kit.Rid)
		return nil, err
	}
	if svc.dispatcher == nil {
		logs.Errorf("reeval failed, dispatcher is not initialized, run_id: %s, rid: %s",
			runID, cts.Kit.Rid)
		return nil, errf.New(errf.Aborted, "evaluator is not initialized")
	}
	if ok := svc.dispatcher.SubmitOverwrite(cts.Kit.NewSubKit(), runID, *req.Overwrite); !ok {
		logs.Warnf("reeval submit wait timeout, run_id: %s, rid: %s", runID, cts.Kit.Rid)
		return nil, errf.New(errf.TooManyRequest, "eval queue is full")
	}
	return nil, nil
}

func requireOverwrite(existingEval *dsaiagent.GetAiagentRunEvalResult, overwrite bool) error {
	if existingEval != nil && existingEval.ID != "" && !overwrite {
		return errf.New(errf.InvalidParameter, "overwrite=true is required to replace an existing eval")
	}
	return nil
}
