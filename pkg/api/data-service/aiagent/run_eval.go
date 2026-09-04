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
	"errors"
	"time"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/tools/times"
)

// CreateAiagentRunEvalReq defines the request for creating an eval row.
type CreateAiagentRunEvalReq struct {
	RunID     string `json:"run_id" validate:"required,max=64"`
	SessionID string `json:"session_id" validate:"required,max=128"`
	User      string `json:"user" validate:"required,max=64"`
	BkBizID   int64  `json:"bk_biz_id"`
	// Scene mirrors aiagent_run.scene at eval time, kept for dashboard filter/sort pushdown.
	Scene enumor.IntentType `json:"scene" validate:"required,max=32"`
	// Query mirrors aiagent_run.query at eval time, kept for dashboard filter/sort pushdown.
	Query           string                       `json:"query"`
	StartRunID      string                       `json:"start_run_id" validate:"required,max=64"`
	ProcessScore    int                          `json:"process_score"`
	OutcomeScore    int                          `json:"outcome_score"`
	QualityScore    int                          `json:"quality_score"`
	Redlines        types.StringArray            `json:"redlines"`
	ReasonCode      enumor.AiagentEvalReasonCode `json:"reason_code" validate:"required,max=64"`
	RubricVersion   string                       `json:"rubric_version" validate:"required,max=32"`
	EvalResult      types.JsonField              `json:"eval_result" validate:"required"`
	ContextSnapshot types.JsonField              `json:"context_snapshot" validate:"required"`
	EvalTrace       types.JsonField              `json:"eval_trace"`
}

// Validate validates the create eval request.
func (req *CreateAiagentRunEvalReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if req.RunID == "" {
		return errors.New("run_id is required")
	}
	if req.SessionID == "" {
		return errors.New("session_id is required")
	}
	if req.User == "" {
		return errors.New("user is required")
	}
	if err := req.Scene.ValidateRunScene(); err != nil {
		return err
	}
	if err := req.ReasonCode.Validate(); err != nil {
		return err
	}
	if req.Redlines == nil {
		req.Redlines = []string{}
	}
	return enumor.ValidateAiagentEvalRedlines(req.Redlines)
}

// CreateAiagentRunEvalResult defines the response for creating an eval row.
type CreateAiagentRunEvalResult struct {
	ID string `json:"id"`
}

// GetAiagentRunEvalReq is unused; get uses path run_id.
type GetAiagentRunEvalResult = tableaiagent.RunEvalTable

// ListAiagentRunEvalReq is an alias for core.ListReq.
type ListAiagentRunEvalReq = core.ListReq

// ListAiagentRunEvalResult defines the list eval response.
type ListAiagentRunEvalResult struct {
	Count   uint64                      `json:"count"`
	Details []tableaiagent.RunEvalTable `json:"details"`
}

// ListAiagentRunEvalGapReq defines the coverage-gap list request.
type ListAiagentRunEvalGapReq struct {
	LookbackSec int64 `json:"lookback_sec"`
	// From is the inclusive range start in constant.TimeStdFormat.
	From string `json:"from"`
	// To is the inclusive range end in constant.TimeStdFormat.
	To    string `json:"to"`
	Start uint32 `json:"start"`
	// Limit is the page size. Zero is normalized to constant.DefaultAiagentRunEvalGapLimit.
	Limit uint `json:"limit"`
	Count bool `json:"count"`
}

// Validate validates the gap list request.
func (req *ListAiagentRunEvalGapReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if req.From != "" || req.To != "" {
		if req.From == "" || req.To == "" {
			return errors.New("from and to must be set together")
		}
		if _, err := req.FromTime(); err != nil {
			return err
		}
		if _, err := req.ToTime(); err != nil {
			return err
		}
	} else if req.LookbackSec <= 0 {
		return errors.New("lookback_sec must be positive")
	}
	req.Limit = constant.AiagentRunEvalGapLimit(req.Limit)
	return nil
}

// Lookback returns the lookback duration.
func (req *ListAiagentRunEvalGapReq) Lookback() time.Duration {
	return time.Duration(req.LookbackSec) * time.Second
}

// FromTime parses From as constant.TimeStdFormat. Empty From returns zero time.
func (req *ListAiagentRunEvalGapReq) FromTime() (time.Time, error) {
	if req.From == "" {
		return time.Time{}, nil
	}
	return times.ParseDateTime(constant.TimeStdFormat, req.From)
}

// ToTime parses To as constant.TimeStdFormat. Empty To returns zero time.
func (req *ListAiagentRunEvalGapReq) ToTime() (time.Time, error) {
	if req.To == "" {
		return time.Time{}, nil
	}
	return times.ParseDateTime(constant.TimeStdFormat, req.To)
}

// OverwriteAiagentRunEvalReq defines the explicit overwrite request.
type OverwriteAiagentRunEvalReq struct {
	CreateAiagentRunEvalReq
}

// Validate validates the overwrite request.
func (req *OverwriteAiagentRunEvalReq) Validate() error {
	return req.CreateAiagentRunEvalReq.Validate()
}
