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

package feedback

import (
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// BizDeleteFeedback revokes the current user's feedback on one Agent run. Only the
// feedback row belonging to the caller, the path business, and the given session
// may be deleted.
//
// DELETE /api/v1/agent/bizs/{bk_biz_id}/feedback/{run_id}?session_id={session_id}
func (svc *service) BizDeleteFeedback(cts *rest.Contexts) (interface{}, error) {
	bizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	if bizID <= 0 {
		return nil, errf.New(errf.InvalidParameter, "bk_biz_id must be greater than 0")
	}

	if err := svc.authorizeBizSession(cts.Kit, bizID); err != nil {
		return nil, err
	}

	runID := cts.PathParameter("run_id").String()
	if runID == "" {
		return nil, errf.New(errf.InvalidParameter, `missing path parameter "run_id"`)
	}

	sessionID := cts.Request.QueryParameter("session_id")
	if sessionID == "" {
		return nil, errf.New(errf.InvalidParameter, "session_id is required")
	}

	if _, err := findOwnedSession(cts.Kit, svc.cli, sessionID, bizID); err != nil {
		return nil, err
	}

	row, err := findFeedbackByRunID(cts.Kit, svc.cli, runID)
	if err != nil {
		logs.Errorf("find feedback failed, err: %v, run_id: %s, rid: %s", err, runID, cts.Kit.Rid)
		return nil, err
	}
	if row == nil {
		return nil, errf.Newf(errf.RecordNotFound, "feedback not found, run_id: %s", runID)
	}

	if row.User != cts.Kit.User || row.BkBizID != bizID || row.SessionID != sessionID {
		logs.Errorf("delete feedback denied, feedback does not belong to caller, run_id: %s, "+
			"row_user: %s, row_biz: %d, row_session: %s, user: %s, bk_biz_id: %d, session_id: %s, rid: %s",
			runID, row.User, row.BkBizID, row.SessionID, cts.Kit.User, bizID, sessionID, cts.Kit.Rid)
		return nil, errf.Newf(errf.PermissionDenied, "permission denied, run_id: %s", runID)
	}

	deleteReq := &dsaiagent.DeleteAgentRunFeedbackReq{Filter: tools.EqualExpression("id", row.ID)}
	if err := svc.cli.DataService().Aiagent.Feedback.Delete(cts.Kit, deleteReq); err != nil {
		logs.Errorf("delete feedback failed, err: %v, run_id: %s, rid: %s", err, runID, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
