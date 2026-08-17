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
	"fmt"

	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// authorizeAgentAssistant checks platform agent_assistant without a biz resource.
func (svc *service) authorizeAgentAssistant(kt *kit.Kit, action meta.Action) error {
	if err := svc.authorizer.AuthorizeWithPerm(kt, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.AgentAssistant, Action: action},
	}); err != nil {
		logs.Errorf("authorize agent assistant failed, err: %v, user: %s, rid: %s", err, kt.User, kt.Rid)
		return err
	}
	return nil
}

// authorizeBizSession 校验指定业务的业务访问权限。
// 业务会话接口只能操作当前用户自己的会话，不再叠加平台智能体助手。
func (svc *service) authorizeBizSession(kt *kit.Kit, bizID int64) error {
	if err := svc.authorizer.AuthorizeWithPerm(kt, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access},
		BizID: bizID,
	}); err != nil {
		logs.Errorf("authorize biz access failed, err: %v, user: %s, bk_biz_id: %d, rid: %s",
			err, kt.User, bizID, kt.Rid)
		ef := errf.Error(err)
		if ef != nil && ef.Permissions != nil {
			return errf.NewWithPerm(errf.PermissionDenied,
				fmt.Sprintf("permission denied, bk_biz_id: %d", bizID), ef.Permissions)
		}
		return errf.Newf(errf.PermissionDenied, "permission denied, bk_biz_id: %d", bizID)
	}
	return nil
}
