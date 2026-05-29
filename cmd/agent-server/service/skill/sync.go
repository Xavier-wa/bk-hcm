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

package skill

import (
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// SyncSkills manually triggers a full incremental skill sync from BKAIDev.
func (s *service) SyncSkills(cts *rest.Contexts) (interface{}, error) {
	if err := s.authorizer.AuthorizeWithPerm(cts.Kit,
		meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AgentAssistant, Action: meta.Create}}); err != nil {
		logs.Errorf("agent auth: permission denied, user: %s, err: %v, rid: %s", cts.Kit.User, err, cts.Kit.Rid)
		return nil, errf.New(errf.PermissionDenied, "permission denied")
	}

	if err := s.tasks[enumor.CronTaskSyncAgentSkills].Do(cts.Kit); err != nil {
		logs.Errorf("manual skill sync failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	logs.Infof("manual skill sync triggered successfully, rid: %s", cts.Kit.Rid)
	return nil, nil
}
