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

// Package session ...
package session

// ContextStatsResponse is the JSON body returned by the context-stats endpoint.
type ContextStatsResponse struct {
	TotalEvents        int                `json:"total_events"`
	UnsummarizedEvents int                `json:"unsummarized_events"`
	EstimatedTokens    int                `json:"estimated_tokens"`
	HasSummary         bool               `json:"has_summary"`
	LastSummaryAt      string             `json:"last_summary_at,omitempty"`
	LastEventAt        string             `json:"last_event_at,omitempty"`
	Thresholds         *SummaryThresholds `json:"thresholds"`
}

// SummaryThresholds mirrors the operator-configured summary trigger thresholds
// so the frontend can compute proximity to the next summarization.
type SummaryThresholds struct {
	Enabled        bool   `json:"enabled"`
	Policy         string `json:"policy"`
	EventThreshold int    `json:"event_threshold"`
	TokenThreshold int    `json:"token_threshold"`
	IdleThreshold  string `json:"idle_threshold,omitempty"`
}
