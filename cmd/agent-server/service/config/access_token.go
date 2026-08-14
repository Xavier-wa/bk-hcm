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

package config

import (
	authlogic "hcm/cmd/agent-server/logics/auth"
	proto "hcm/pkg/api/agent-server/config"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// UpsertAccessToken upserts a virtual-user access_token in global_config auth/access_token.
//
// PUT /api/v1/agent/config/access_token
func (svc *service) UpsertAccessToken(cts *rest.Contexts) (interface{}, error) {
	if svc.cli == nil {
		return nil, errf.New(errf.PermissionDenied, "data service client is not configured")
	}

	req := new(proto.UpsertAccessTokenReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := svc.authorizer.AuthorizeWithPerm(cts.Kit,
		meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.GlobalConfig, Action: meta.Update}}); err != nil {
		logs.Errorf("agent auth: permission denied, user: %s, err: %v, rid: %s", cts.Kit.User, err, cts.Kit.Rid)
		return nil, errf.New(errf.PermissionDenied, "permission denied")
	}

	if err := authlogic.UpsertAccessToken(cts.Kit, svc.cli.DataService(), req.VirtualUser,
		req.AccessToken); err != nil {
		logs.Errorf("upsert access token failed, virtual_user: %s, err: %v, rid: %s",
			req.VirtualUser, err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return nil, nil
}
