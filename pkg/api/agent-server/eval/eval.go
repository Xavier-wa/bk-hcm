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

// Package eval defines agent-server eval query, reeval and history sync APIs.
package eval

import (
	"fmt"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/tools/slice"
)

// ReevalReq is the request for POST /eval/runs/{run_id}/reeval.
type ReevalReq struct {
	// Overwrite is required. false refuses to replace an existing eval row.
	Overwrite *bool `json:"overwrite" validate:"required"`
}

// Validate validates the reeval request.
func (req *ReevalReq) Validate() error {
	return validator.Validate.Struct(req)
}

// SyncHistoryReq is the request for POST /eval/runs/history/sync.
type SyncHistoryReq struct {
	// SessionCodes is the session_code list to sync. Required, at most 20 unique values.
	SessionCodes []string `json:"session_codes" validate:"required,min=1,dive,required,max=128"`
}

// Validate validates the history sync request.
func (req *SyncHistoryReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if len(req.SessionCodes) > constant.MaxHistorySyncSessionLimit {
		return fmt.Errorf("session_codes must be <= %d", constant.MaxHistorySyncSessionLimit)
	}
	req.SessionCodes = slice.Unique(req.SessionCodes)
	return nil
}

// SyncHistoryResp is the result of a history sync call.
type SyncHistoryResp struct {
	SessionScanned uint64 `json:"session_scanned"`
	SessionFailed  uint64 `json:"session_failed"`
	RunCreated     uint64 `json:"run_created"`
	RunSkipped     uint64 `json:"run_skipped"`
	RunFailed      uint64 `json:"run_failed"`
}

// RunDetailResp is GET /eval/runs/{run_id}.
type RunDetailResp struct {
	RunID       string         `json:"run_id"`
	SessionCode string         `json:"session_code"`
	User        string         `json:"user"`
	Scene       string         `json:"scene"`
	Status      string         `json:"status"`
	Query       string         `json:"query"`
	Summary     string         `json:"summary"`
	Eval        *EvalDetail    `json:"eval"`
	Window      []WindowDetail `json:"window"`
}

// EvalDetail is the eval block on the detail page.
type EvalDetail struct {
	ProcessScore int            `json:"process_score"`
	OutcomeScore int            `json:"outcome_score"`
	QualityScore int            `json:"quality_score"`
	Passed       bool           `json:"passed"`
	Redlines     []string       `json:"redlines"`
	ReasonCode   string         `json:"reason_code"`
	Dims         map[string]int `json:"dims"`
	Summary      string         `json:"summary"`
	Trace        any            `json:"eval_trace,omitempty"`
}

// WindowDetail is one drill-down row.
type WindowDetail struct {
	RunID      string `json:"run_id"`
	Role       string `json:"role"`
	Brief      string `json:"brief"`
	Query      string `json:"query"`
	Transcript any    `json:"transcript"`
}
