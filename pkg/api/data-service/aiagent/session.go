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

// Package aiagent defines data-service API request/response types for the aiagent module.
package aiagent

import (
	"errors"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/runtime/filter"
)

// -------------------------- Create --------------------------

// CreateAiagentSessionReq defines the request for creating an aiagent session.
type CreateAiagentSessionReq struct {
	AppName     string            `json:"app_name" validate:"required,max=64"`
	User        string            `json:"user" validate:"required,max=64"`
	BkBizID     int64             `json:"bk_biz_id" validate:"required"`
	SessionName string            `json:"session_name" validate:"max=255"`
	IsTemporary bool              `json:"is_temporary"`
	SessionTag  enumor.IntentType `json:"session_tag" validate:"max=64"`
	Extension   types.JsonField   `json:"extension,omitempty"`
}

// Validate validates the create request.
func (req *CreateAiagentSessionReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if len(req.AppName) == 0 {
		return errors.New("app_name is required")
	}
	if len(req.User) == 0 {
		return errors.New("user is required")
	}
	if req.SessionTag != "" {
		if err := req.SessionTag.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// CreateAiagentSessionResult defines the response for creating an aiagent session.
type CreateAiagentSessionResult struct {
	ID          string `json:"id"`
	SessionCode string `json:"session_code"`
	ThreadID    string `json:"thread_id"`
}

// -------------------------- Update --------------------------

// UpdateAiagentSessionReq defines the request for updating an aiagent session.
type UpdateAiagentSessionReq struct {
	ID          string            `json:"id" validate:"required,max=64"`
	SessionName string            `json:"session_name" validate:"max=255"`
	Reviser     string            `json:"reviser" validate:"required,max=64"`
	SessionTag  enumor.IntentType `json:"session_tag" validate:"max=64"`
}

// Validate validates the update request.
func (req *UpdateAiagentSessionReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if req.SessionTag != "" {
		if err := req.SessionTag.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// -------------------------- List --------------------------

// ListAiagentSessionReq is an alias for core.ListReq for listing aiagent sessions.
type ListAiagentSessionReq = core.ListReq

// ListAiagentSessionResult defines the response for listing aiagent sessions.
type ListAiagentSessionResult struct {
	Count   uint64                      `json:"count"`
	Details []tableaiagent.SessionTable `json:"details"`
}

// -------------------------- Delete --------------------------

// DeleteAiagentSessionReq defines the request for batch-deleting aiagent sessions.
type DeleteAiagentSessionReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
}

// Validate validates the delete request.
func (req *DeleteAiagentSessionReq) Validate() error {
	if req.Filter == nil {
		return errors.New("filter is required")
	}
	return validator.Validate.Struct(req)
}

// -------------------------- IncrContentCount --------------------------

// IncrContentCountReq defines the request for incrementing session content count.
type IncrContentCountReq struct {
	SessionCode string `json:"session_code" validate:"required,max=128"`
}

// Validate validates the incr content count request.
func (req *IncrContentCountReq) Validate() error {
	return validator.Validate.Struct(req)
}
