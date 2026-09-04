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
	"hcm/pkg/api/core"
	"hcm/pkg/client"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// findFeedbackByRunID looks up the (at most one) feedback row for runID.
// Returns nil, nil when no row exists yet.
func findFeedbackByRunID(kt *kit.Kit, cli *client.ClientSet, runID string) (*tableaiagent.FeedbackTable, error) {
	listReq := &core.ListReq{
		Filter: tools.EqualExpression("run_id", runID),
		Page:   &core.BasePage{Start: 0, Limit: 1},
	}

	result, err := cli.DataService().Aiagent.Feedback.List(kt, listReq)
	if err != nil {
		return nil, err
	}
	if len(result.Details) == 0 {
		return nil, nil
	}
	return &result.Details[0], nil
}

// findOwnedSession looks up the session by id and checks it belongs to the
// caller and the given business. Returns RecordNotFound / PermissionDenied
// when the session is missing or owned by someone else.
func findOwnedSession(kt *kit.Kit, cli *client.ClientSet, sessionID string, bizID int64) (
	*tableaiagent.SessionTable, error) {

	listReq := &core.ListReq{
		Filter: tools.EqualExpression("id", sessionID),
		Page:   &core.BasePage{Start: 0, Limit: 1},
	}
	result, err := cli.DataService().Aiagent.Session.List(kt, listReq)
	if err != nil {
		logs.Errorf("find session failed, err: %v, session_id: %s, rid: %s", err, sessionID, kt.Rid)
		return nil, err
	}
	if len(result.Details) == 0 {
		return nil, errf.Newf(errf.RecordNotFound, "session not found, session_id: %s", sessionID)
	}

	sess := result.Details[0]
	if sess.User != kt.User || sess.BkBizID != bizID {
		logs.Errorf("session does not belong to caller, session_id: %s, session_user: %s, "+
			"session_biz: %d, user: %s, bk_biz_id: %d, rid: %s",
			sessionID, sess.User, sess.BkBizID, kt.User, bizID, kt.Rid)
		return nil, errf.Newf(errf.PermissionDenied, "permission denied, session_id: %s", sessionID)
	}
	return &sess, nil
}

// ensureRunMatchesSession verifies that runID exists on aiagent_run and that the
// row's session_code matches the owned session. Status is not checked. Returns the
// run row so callers can copy scene/query into the feedback row without a second query.
func ensureRunMatchesSession(kt *kit.Kit, cli *client.ClientSet, runID, sessionCode string) (
	*tableaiagent.RunTable, error) {

	listReq := &core.ListReq{
		Filter: tools.EqualExpression("run_id", runID),
		Page:   &core.BasePage{Start: 0, Limit: 1},
	}
	result, err := cli.DataService().Aiagent.Run.List(kt, listReq)
	if err != nil {
		logs.Errorf("find run failed, err: %v, run_id: %s, rid: %s", err, runID, kt.Rid)
		return nil, err
	}

	found := result != nil && len(result.Details) > 0
	if !found {
		return nil, errf.Newf(errf.RecordNotFound, "run not found, run_id: %s", runID)
	}

	run := result.Details[0]
	if sessionCode != run.SessionCode {
		return nil, errf.Newf(errf.InvalidParameter, "run_id %s does not belong to session_code %s",
			runID, run.SessionCode)
	}

	return &run, nil
}
