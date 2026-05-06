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

// Package aiagent provides data-service client for the aiagent module.
package aiagent

import (
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/client/common"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
)

// SessionClient is the data-service aiagent session API client.
type SessionClient struct {
	client rest.ClientInterface
}

// NewSessionClient creates a new SessionClient.
func NewSessionClient(client rest.ClientInterface) *SessionClient {
	return &SessionClient{client: client}
}

// Create creates an aiagent session.
func (c *SessionClient) Create(kt *kit.Kit,
	req *dsaiagent.CreateAiagentSessionReq) (*dsaiagent.CreateAiagentSessionResult, error) {

	return common.Request[dsaiagent.CreateAiagentSessionReq, dsaiagent.CreateAiagentSessionResult](
		c.client, rest.POST, kt, req, "/sessions/create")
}

// Update updates an aiagent session.
func (c *SessionClient) Update(kt *kit.Kit, req *dsaiagent.UpdateAiagentSessionReq) error {
	return common.RequestNoResp[dsaiagent.UpdateAiagentSessionReq](
		c.client, rest.PATCH, kt, req, "/sessions")
}

// List queries aiagent sessions.
func (c *SessionClient) List(kt *kit.Kit,
	req *dsaiagent.ListAiagentSessionReq) (*dsaiagent.ListAiagentSessionResult, error) {

	return common.Request[dsaiagent.ListAiagentSessionReq, dsaiagent.ListAiagentSessionResult](
		c.client, rest.POST, kt, req, "/sessions/list")
}

// Delete batch-deletes aiagent sessions.
func (c *SessionClient) Delete(kt *kit.Kit, req *dsaiagent.DeleteAiagentSessionReq) error {
	return common.RequestNoResp[dsaiagent.DeleteAiagentSessionReq](
		c.client, rest.DELETE, kt, req, "/sessions/batch")
}

// IncrContentCount increments session content count.
func (c *SessionClient) IncrContentCount(kt *kit.Kit, req *dsaiagent.IncrContentCountReq) error {
	return common.RequestNoResp[dsaiagent.IncrContentCountReq](
		c.client, rest.PATCH, kt, req, "/sessions/incr_content_count")
}
