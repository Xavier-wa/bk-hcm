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
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/session"
)

// InitService initialize the session service.
func InitService(cap *capability.Capability, sessionSvc session.Service, appName string) {
	svc := &service{
		sessionSvc: sessionSvc,
		appName:    appName,
	}

	h := rest.NewHandler()
	h.Add("GetContextStats", http.MethodGet, "/sessions/{thread_id}/context-stats", svc.GetContextStats)

	h.Load(cap.WebService)
}

type service struct {
	sessionSvc session.Service
	appName    string
}
