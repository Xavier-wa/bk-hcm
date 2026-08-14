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
	proto "hcm/pkg/api/agent-server/session"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// CreateSession creates a new agent session.
//
// POST /api/v1/agent/sessions/create
func (svc *service) CreateSession(cts *rest.Contexts) (interface{}, error) {
	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AgentAssistant, Action: meta.Create}}
	return svc.createSession(cts, constant.UnassignedBiz, authRes)
}

// BizCreateSession creates a new session under a business.
//
// POST /api/v1/agent/bizs/{bk_biz_id}/sessions/create
func (svc *service) BizCreateSession(cts *rest.Contexts) (interface{}, error) {
	bizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	if bizID <= 0 {
		return nil, errf.Newf(errf.InvalidParameter, "bk_biz_id must be greater than 0")
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AgentAssistant, Action: meta.Create}, BizID: bizID}
	return svc.createSession(cts, bizID, authRes)
}

func (svc *service) createSession(cts *rest.Contexts, bizID int64, authRes meta.ResourceAttribute) (
	*proto.CreateSessionResp, error) {

	if svc.cli == nil {
		return nil, errf.New(errf.UnHealthy, "data service client is not configured")
	}

	req := new(proto.CreateSessionReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := svc.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		logs.Errorf("create session: permission denied, user: %s, bk_biz_id: %d, err: %v, rid: %s",
			cts.Kit.User, bizID, err, cts.Kit.Rid)
		return nil, errf.New(errf.PermissionDenied, "permission denied")
	}

	createReq := &dsaiagent.CreateAiagentSessionReq{
		AppName:     svc.appName,
		User:        cts.Kit.User,
		BkBizID:     bizID,
		SessionName: req.SessionName,
		SessionTag:  req.SessionTag,
	}

	result, err := svc.cli.DataService().Aiagent.Session.Create(cts.Kit, createReq)
	if err != nil {
		logs.Errorf("create session failed, user: %s, bk_biz_id: %d, err: %v, rid: %s", cts.Kit.User,
			bizID, err, cts.Kit.Rid)
		return nil, err
	}

	return &proto.CreateSessionResp{
		ID:          result.ID,
		SessionCode: result.SessionCode,
		ThreadID:    result.ThreadID,
		SessionName: req.SessionName,
		SessionTag:  req.SessionTag,
	}, nil
}
