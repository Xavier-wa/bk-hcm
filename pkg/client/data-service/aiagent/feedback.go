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

package aiagent

import (
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/client/common"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
)

// FeedbackClient is the data-service aiagent run feedback API client.
type FeedbackClient struct {
	client rest.ClientInterface
}

// NewFeedbackClient creates a new FeedbackClient.
func NewFeedbackClient(client rest.ClientInterface) *FeedbackClient {
	return &FeedbackClient{client: client}
}

// Create creates an aiagent run feedback row.
func (c *FeedbackClient) Create(kt *kit.Kit,
	req *dsaiagent.CreateAgentRunFeedbackReq) (*dsaiagent.CreateAgentRunFeedbackResult, error) {

	return common.Request[dsaiagent.CreateAgentRunFeedbackReq, dsaiagent.CreateAgentRunFeedbackResult](
		c.client, rest.POST, kt, req, "/feedbacks/create")
}

// Update overwrites tags/reaction/comment of an existing feedback row.
func (c *FeedbackClient) Update(kt *kit.Kit, req *dsaiagent.UpdateAgentRunFeedbackReq) error {
	return common.RequestNoResp[dsaiagent.UpdateAgentRunFeedbackReq](
		c.client, rest.PATCH, kt, req, "/feedbacks")
}

// List queries aiagent run feedback rows.
func (c *FeedbackClient) List(kt *kit.Kit,
	req *dsaiagent.ListAgentRunFeedbackReq) (*dsaiagent.ListAgentRunFeedbackResult, error) {

	return common.Request[dsaiagent.ListAgentRunFeedbackReq, dsaiagent.ListAgentRunFeedbackResult](
		c.client, rest.POST, kt, req, "/feedbacks/list")
}

// Delete batch-deletes aiagent run feedback rows.
func (c *FeedbackClient) Delete(kt *kit.Kit, req *dsaiagent.DeleteAgentRunFeedbackReq) error {
	return common.RequestNoResp[dsaiagent.DeleteAgentRunFeedbackReq](
		c.client, rest.DELETE, kt, req, "/feedbacks/batch")
}
