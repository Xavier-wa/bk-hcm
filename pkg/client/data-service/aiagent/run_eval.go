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

// RunEvalClient is the data-service aiagent run eval API client.
type RunEvalClient struct {
	client rest.ClientInterface
}

// NewRunEvalClient creates a new RunEvalClient.
func NewRunEvalClient(client rest.ClientInterface) *RunEvalClient {
	return &RunEvalClient{client: client}
}

// Create creates an eval row.
func (c *RunEvalClient) Create(kt *kit.Kit, req *dsaiagent.CreateAiagentRunEvalReq) (
	*dsaiagent.CreateAiagentRunEvalResult, error) {

	return common.Request[dsaiagent.CreateAiagentRunEvalReq, dsaiagent.CreateAiagentRunEvalResult](
		c.client, rest.POST, kt, req, "/run_evals/create")
}

// Get returns one eval by run_id.
func (c *RunEvalClient) Get(kt *kit.Kit, runID string) (*dsaiagent.GetAiagentRunEvalResult, error) {
	return common.Request[common.Empty, dsaiagent.GetAiagentRunEvalResult](
		c.client, rest.GET, kt, nil, "/run_evals/%s", runID)
}

// List queries eval rows.
func (c *RunEvalClient) List(kt *kit.Kit, req *dsaiagent.ListAiagentRunEvalReq) (
	*dsaiagent.ListAiagentRunEvalResult, error) {

	return common.Request[dsaiagent.ListAiagentRunEvalReq, dsaiagent.ListAiagentRunEvalResult](
		c.client, rest.POST, kt, req, "/run_evals/list")
}

// ListGap lists terminal runs without eval.
func (c *RunEvalClient) ListGap(kt *kit.Kit, req *dsaiagent.ListAiagentRunEvalGapReq) (
	*dsaiagent.ListAiagentRunResult, error) {

	return common.Request[dsaiagent.ListAiagentRunEvalGapReq, dsaiagent.ListAiagentRunResult](
		c.client, rest.POST, kt, req, "/run_evals/gaps/list")
}

// Overwrite overwrites an existing eval row.
func (c *RunEvalClient) Overwrite(kt *kit.Kit, req *dsaiagent.OverwriteAiagentRunEvalReq) error {
	return common.RequestNoResp[dsaiagent.OverwriteAiagentRunEvalReq](
		c.client, rest.PUT, kt, req, "/run_evals/overwrite")
}
