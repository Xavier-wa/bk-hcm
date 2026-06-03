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

// Package prompt provides HTTP handlers for prompt management operations.
package prompt

import (
	"net/http"

	"hcm/cmd/agent-server/service/capability"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/cron/core"
	"hcm/pkg/rest"
)

type service struct {
	tasks map[enumor.CronTask]core.Task
}

// InitService registers prompt management routes onto the WebService.
// It is a no-op when prompt sync cron task is not registered.
func InitService(c *capability.Capability) {
	if _, ok := c.Tasks[enumor.CronTaskSyncAgentPrompts]; !ok {
		return
	}

	s := &service{tasks: c.Tasks}
	h := rest.NewHandler()
	s.initService(h)
	h.Load(c.WebService)
}

func (s *service) initService(h *rest.Handler) {
	h.Add("SyncPrompts", http.MethodPost, s.tasks[enumor.CronTaskSyncAgentPrompts].GetURL(), s.SyncPrompts)
}
