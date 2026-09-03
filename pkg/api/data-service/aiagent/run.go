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

import "hcm/pkg/api/core"

// ListAiagentRunReq is an alias for core.ListReq for listing aiagent runs.
type ListAiagentRunReq = core.ListReq

// ListAiagentRunResult defines the response for listing aiagent runs.
type ListAiagentRunResult struct {
	Count   uint64    `json:"count"`
	Details []RunElem `json:"details"`
}

// RunElem is the subset of an aiagent_run row used to match a run to a session.
// Extra JSON fields from data-service are ignored.
type RunElem struct {
	RunID       string `json:"run_id"`
	SessionCode string `json:"session_code"`
}
