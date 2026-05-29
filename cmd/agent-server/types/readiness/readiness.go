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

// Package readiness provides types for agent readiness checks.
package readiness

// AgentReadinessResp is the response body for GET /api/v1/agent/readiness.
type AgentReadinessResp struct {
	// Ready reports whether the agent is ready to handle user traffic.
	Ready bool `json:"ready"`
	// SkillReady reports whether the initial skill sync has completed.
	SkillReady bool `json:"skill_ready"`
	// PromptReady reports whether the initial prompt sync has completed.
	PromptReady bool `json:"prompt_ready"`
}
