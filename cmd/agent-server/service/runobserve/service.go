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

package runobserve

import (
	"net/http"

	"hcm/cmd/agent-server/service/capability"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/cron/core"
	"hcm/pkg/iam/auth"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

type service struct {
	tasks      map[enumor.CronTask]core.Task
	authorizer auth.Authorizer
}

// InitService registers the manual sweep route onto /api/v1/agent.
func InitService(c *capability.Capability) {
	task, ok := c.Tasks[enumor.CronTaskSweepAiagentRun]
	if !ok || task == nil {
		return
	}

	svc := &service{tasks: c.Tasks, authorizer: c.Authorizer}
	h := rest.NewHandler()
	h.Add("SweepAiagentRun", http.MethodPost, task.GetURL(), svc.Sweep)
	h.Load(c.WebService)
}

// Sweep manually triggers the orphan aiagent_run sweep.
func (svc *service) Sweep(cts *rest.Contexts) (interface{}, error) {
	if err := svc.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.AgentAssistantManage, Action: meta.Find},
	}); err != nil {
		logs.Errorf("authorize agent assistant manage failed, err: %v, user: %s, rid: %s",
			err, cts.Kit.User, cts.Kit.Rid)
		return nil, errf.New(errf.PermissionDenied, "permission denied")
	}
	task, ok := svc.tasks[enumor.CronTaskSweepAiagentRun]
	if !ok || task == nil {
		logs.Errorf("sweep aiagent run failed, sweep task is not registered, rid: %s", cts.Kit.Rid)
		return nil, errf.New(errf.Aborted, "sweep task is not registered")
	}
	if err := task.Do(cts.Kit); err != nil {
		logs.Errorf("sweep aiagent run failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}
	return nil, nil
}
