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
)

// CreateAiagentRunReq 创建 aiagent_run 的请求。
// 实时 Ledger 只填身份和 query；status/transcript/时间字段给 history sync 用，
// RUN_STARTED 路径必须留空。
type CreateAiagentRunReq struct {
	RunID       string            `json:"run_id" validate:"required,max=64"`
	SessionCode string            `json:"session_code" validate:"required,max=128"`
	User        string            `json:"user" validate:"required,max=64"`
	BkBizID     int64             `json:"bk_biz_id"`
	Scene       enumor.IntentType `json:"scene" validate:"max=32"`
	// Query 为本轮用户首句，创建时允许为空。
	Query string `json:"query"`
	// Status 为空时 DAO 默认 running（实时 RUN_STARTED）。history sync 写入终态。
	Status enumor.AiagentRunStatus `json:"status" validate:"max=32"`
	// Reason 为可选终态说明，例如 history_sync。
	Reason string `json:"reason" validate:"max=256"`
	// Transcript 实时路径创建时不写，由终态 CAS 补齐；history sync 创建时直接带上。
	Transcript types.JsonField `json:"transcript"`
	// OccurredAt 非空时回写 created_at。实时路径不传，保留库默认 now()。
	// 格式：2006-01-02 15:04:05。
	OccurredAt string `json:"occurred_at"`
	// EndedAt 非空时回写 updated_at。为空则与 OccurredAt 相同。
	EndedAt string `json:"ended_at"`
}

// Validate validates the create run request.
func (req *CreateAiagentRunReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if req.RunID == "" {
		return errors.New("run_id is required")
	}
	if req.SessionCode == "" {
		return errors.New("session_code is required")
	}
	if req.User == "" {
		return errors.New("user is required")
	}
	if req.Scene == "" {
		req.Scene = enumor.IntentTypeUnsupported
	}
	if err := req.Scene.ValidateRunScene(); err != nil {
		return err
	}
	if req.Status != "" {
		if err := req.Status.Validate(); err != nil {
			return err
		}
	}
	return validateOccurredAt(req.OccurredAt, req.EndedAt)
}

func validateOccurredAt(occurredAt, endedAt string) error {
	if occurredAt == "" && endedAt == "" {
		return nil
	}
	if occurredAt == "" {
		return errors.New("occurred_at is required when ended_at is set")
	}
	if _, err := time.Parse(constant.DateTimeLayout, occurredAt); err != nil {
		return errors.New("occurred_at must be 2006-01-02 15:04:05")
	}
	if endedAt == "" {
		return nil
	}
	if _, err := time.Parse(constant.DateTimeLayout, endedAt); err != nil {
		return errors.New("ended_at must be 2006-01-02 15:04:05")
	}
	return nil
}

// CreateAiagentRunResult defines the response for creating an aiagent run.
type CreateAiagentRunResult struct {
	ID string `json:"id"`
}

// UpdateAiagentRunStatusReq defines the CAS terminal-status update request.
type UpdateAiagentRunStatusReq struct {
	RunID      string                  `json:"run_id" validate:"required,max=64"`
	Status     enumor.AiagentRunStatus `json:"status" validate:"required,max=32"`
	Reason     string                  `json:"reason" validate:"max=256"`
	Query      string                  `json:"query"`
	Transcript types.JsonField         `json:"transcript"`
}

// Validate validates the CAS update request.
func (req *UpdateAiagentRunStatusReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if req.RunID == "" {
		return errors.New("run_id is required")
	}
	if err := req.Status.Validate(); err != nil {
		return err
	}
	if !req.Status.IsTerminal() {
		return errors.New("status must be a terminal state")
	}
	return nil
}

// PatchAiagentRunTranscriptReq defines the request to backfill query/transcript.
type PatchAiagentRunTranscriptReq struct {
	RunID      string          `json:"run_id" validate:"required,max=64"`
	Query      string          `json:"query"`
	Transcript types.JsonField `json:"transcript"`
}

// Validate validates the patch transcript request.
func (req *PatchAiagentRunTranscriptReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if req.RunID == "" {
		return errors.New("run_id is required")
	}
	return nil
}

// ListAiagentRunReq is an alias for core.ListReq for listing aiagent runs.
type ListAiagentRunReq = core.ListReq

// ListAiagentRunResult defines the response for listing aiagent runs.
type ListAiagentRunResult struct {
	Count   uint64                  `json:"count"`
	Details []tableaiagent.RunTable `json:"details"`
}
