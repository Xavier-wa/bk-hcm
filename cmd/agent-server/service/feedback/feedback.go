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

// Package feedback provides agent-server user feedback (like/dislike) APIs:
// user-facing commit/delete under /bizs/{bk_biz_id}. Operational listing is
// served by the eval dashboard APIs, not this package.
package feedback

import (
	"net/http"

	"hcm/cmd/agent-server/service/capability"
	"hcm/pkg/client"
	"hcm/pkg/iam/auth"
	"hcm/pkg/rest"
)

// InitService initialize the feedback service.
func InitService(cap *capability.Capability) {
	svc := &service{
		cli:        cap.ClientSet,
		authorizer: cap.Authorizer,
	}

	bizH := rest.NewHandler()
	bizH.Path("/bizs/{bk_biz_id}")
	bizH.Add("BizCommitFeedback", http.MethodPost, "/feedback/commit", svc.BizCommitFeedback)
	bizH.Add("BizDeleteFeedback", http.MethodDelete, "/feedback/{run_id}", svc.BizDeleteFeedback)
	bizH.Load(cap.WebService)
}

type service struct {
	cli        *client.ClientSet
	authorizer auth.Authorizer
}
