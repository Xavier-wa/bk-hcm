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

package session

import (
	"strings"

	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// DeleteSession deletes a session.
//
// DELETE /api/v1/agent/sessions/{session_code}
func (svc *service) DeleteSession(cts *rest.Contexts) (interface{}, error) {
	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AgentAssistant, Action: meta.Delete}}
	return svc.deleteSession(cts, constant.UnassignedBiz, authRes)
}

// BizDeleteSession deletes a session under a business.
//
// DELETE /api/v1/agent/bizs/{bk_biz_id}/sessions/{session_code}
func (svc *service) BizDeleteSession(cts *rest.Contexts) (interface{}, error) {
	bizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	if bizID <= 0 {
		return nil, errf.Newf(errf.InvalidParameter, "bk_biz_id must be greater than 0")
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AgentAssistant, Action: meta.Delete}, BizID: bizID}
	return svc.deleteSession(cts, bizID, authRes)
}

func (svc *service) deleteSession(cts *rest.Contexts, bkBizID int64, authRes meta.ResourceAttribute) (
	interface{}, error) {

	if svc.cli == nil {
		return nil, errf.New(errf.UnHealthy, "data service client is not configured")
	}

	sessionCode := cts.PathParameter("session_code").String()
	if sessionCode == "" {
		return nil, errf.New(errf.InvalidParameter, `missing path parameter "session_code"`)
	}

	if err := svc.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		logs.Errorf("delete session: permission denied, user: %s, bk_biz_id: %d, err: %v, rid: %s",
			cts.Kit.User, bkBizID, err, cts.Kit.Rid)
		return nil, errf.New(errf.PermissionDenied, "permission denied")
	}

	// Verify session belongs to current user.
	sess, err := svc.getSessionByCode(cts, sessionCode)
	if err != nil {
		return nil, err
	}

	if !strings.EqualFold(sess.User, cts.Kit.User) {
		return nil, errf.New(errf.PermissionDenied, "session does not belong to current user")
	}

	if bkBizID != constant.UnassignedBiz && sess.BkBizID != bkBizID {
		return nil, errf.Newf(errf.PermissionDenied, "session does not belong to bk_biz_id: %d", bkBizID)
	}

	deleteReq := &dsaiagent.DeleteAiagentSessionReq{
		Filter: tools.EqualExpression("id", sess.ID),
	}

	if err = svc.cli.DataService().Aiagent.Session.Delete(cts.Kit, deleteReq); err != nil {
		logs.Errorf("delete session failed, session_code: %s, bk_biz_id: %d, err: %v, rid: %s", sessionCode,
			bkBizID, err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
