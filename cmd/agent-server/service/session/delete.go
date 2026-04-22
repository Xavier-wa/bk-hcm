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
	if svc.cli == nil {
		return nil, errf.New(errf.PermissionDenied, "data service client is not configured")
	}

	sessionCode := cts.PathParameter("session_code").String()
	if sessionCode == "" {
		return nil, errf.New(errf.InvalidParameter, `missing path parameter "session_code"`)
	}

	if err := svc.authorizer.AuthorizeWithPerm(cts.Kit,
		meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AgentAssistant, Action: meta.Delete}}); err != nil {
		logs.Errorf("agent auth: permission denied, user: %s, err: %v, rid: %s", cts.Kit.User, err, cts.Kit.Rid)
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

	deleteReq := &dsaiagent.DeleteAiagentSessionReq{
		Filter: tools.EqualExpression("id", sess.ID),
	}

	if err = svc.cli.DataService().Aiagent.Session.Delete(cts.Kit, deleteReq); err != nil {
		logs.Errorf("delete session failed, session_code: %s, err: %v, rid: %s", sessionCode, err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
