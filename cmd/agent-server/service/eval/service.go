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

// Package eval provides agent-server eval query, reeval and history sync HTTP handlers.
package eval

import (
	"net/http"

	"hcm/cmd/agent-server/logics/eval"
	"hcm/cmd/agent-server/service/capability"
	"hcm/pkg/client"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/auth"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

type service struct {
	cli        *client.ClientSet
	authorizer auth.Authorizer
	dispatcher *eval.Dispatcher
	syncer     *eval.HistorySyncer
}

// InitService registers eval query, reeval and history sync routes onto /api/v1/agent.
// dispatcher / syncer 由外层 Service.initEval 组装后注入，此处不构建依赖。
func InitService(c *capability.Capability, dispatcher *eval.Dispatcher, syncer *eval.HistorySyncer) {
	svc := &service{
		cli:        c.ClientSet,
		authorizer: c.Authorizer,
		dispatcher: dispatcher,
		syncer:     syncer,
	}
	h := rest.NewHandler()
	h.Add("GetAgentEvalRun", http.MethodGet, "/eval/runs/{run_id}", svc.GetAgentEvalRun)
	h.Add("EvalReeval", http.MethodPost, "/eval/runs/{run_id}/reeval", svc.Reeval)
	h.Add("EvalSyncHistory", http.MethodPost, "/eval/runs/history/sync", svc.SyncHistory)
	h.Add("ListEvalDashboardEvalResults", http.MethodPost, "/eval/dashboard/eval_results/list",
		svc.ListDashboardEvalResults)
	h.Add("ListEvalDashboardFeedback", http.MethodPost, "/eval/dashboard/feedback/list",
		svc.ListDashboardFeedback)
	h.Load(c.WebService)
}

func (svc *service) authorizeManage(kt *kit.Kit) error {
	if err := svc.authorizer.AuthorizeWithPerm(kt, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.AgentAssistantManage, Action: meta.Find},
	}); err != nil {
		logs.Errorf("authorize agent assistant manage failed, err: %v, user: %s, rid: %s",
			err, kt.User, kt.Rid)
		return errf.New(errf.PermissionDenied, "permission denied")
	}
	return nil
}
