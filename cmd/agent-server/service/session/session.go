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

// Package session ...
package session

import (
	"net/http"

	"hcm/cmd/agent-server/service/capability"
	"hcm/pkg/api/core"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/client"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/table/aiagent"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/session"
)

// InitService initialize the session service.
func InitService(cap *capability.Capability, sessionSvc session.Service, resolver *Resolver, appName string) {
	svc := &service{
		cli:        cap.ClientSet,
		resolver:   resolver,
		sessionSvc: sessionSvc,
		appName:    appName,
	}

	h := rest.NewHandler()
	// Session management APIs.
	h.Add("CreateSession", http.MethodPost, "/sessions/create", svc.CreateSession)
	h.Add("ListSessions", http.MethodPost, "/sessions/list", svc.ListSessions)
	h.Add("UpdateSession", http.MethodPatch, "/sessions/{session_code}", svc.UpdateSession)
	h.Add("DeleteSession", http.MethodDelete, "/sessions/{session_code}", svc.DeleteSession)
	// Context stats (uses session_code now).
	h.Add("GetContextStats", http.MethodGet, "/sessions/{thread_id}/context_stats", svc.GetContextStats)

	h.Load(cap.WebService)
}

type service struct {
	cli *client.ClientSet

	resolver   *Resolver
	sessionSvc session.Service
	appName    string
}

// getSessionByCode queries a session by session_code and returns it.
func (svc *service) getSessionByCode(cts *rest.Contexts, sessionCode string) (*aiagent.SessionTable, error) {
	listReq := &dsaiagent.ListAiagentSessionReq{
		Filter: tools.EqualExpression("session_code", sessionCode),
		Page:   &core.BasePage{Start: 0, Limit: 1},
	}

	result, err := svc.cli.DataService().Aiagent.Session.List(cts.Kit, listReq)
	if err != nil {
		return nil, err
	}

	if len(result.Details) == 0 {
		return nil, errf.New(errf.RecordNotFound, "session not found")
	}

	return &result.Details[0], nil
}
