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

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/runtime/filter"
)

// -------------------------- Create --------------------------

// CreateAgentRunFeedbackReq defines the request for creating an agent run feedback row.
type CreateAgentRunFeedbackReq struct {
	RunID     string `json:"run_id" validate:"required,max=64"`
	SessionID string `json:"session_id" validate:"required,max=64"`
	User      string `json:"user" validate:"required,max=64"`
	BkBizID   int64  `json:"bk_biz_id" validate:"required"`
	// Scene mirrors aiagent_run.scene at feedback time, kept for dashboard filter pushdown.
	Scene enumor.IntentType `json:"scene" validate:"required,max=32"`
	// Query mirrors aiagent_run.query at feedback time, kept for dashboard filter pushdown.
	Query string            `json:"query"`
	Tags  types.StringArray `json:"tags,omitempty"`
	// Reaction is the stored attitude, enumeration values such as: like/dislike.
	Reaction enumor.FeedbackReaction `json:"reaction"`
	Comment  string                  `json:"comment,omitempty" validate:"max=500"`
}

// Validate validates the create request.
func (req *CreateAgentRunFeedbackReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if err := req.Scene.ValidateRunScene(); err != nil {
		return err
	}
	return req.Reaction.Validate()
}

// CreateAgentRunFeedbackResult defines the response for creating an agent run feedback row.
type CreateAgentRunFeedbackResult struct {
	ID string `json:"id"`
}

// -------------------------- Update --------------------------

// UpdateAgentRunFeedbackReq defines the request for overwriting an existing feedback
// row's tags/reaction/comment, matched by id.
type UpdateAgentRunFeedbackReq struct {
	ID   string            `json:"id" validate:"required,max=64"`
	Tags types.StringArray `json:"tags,omitempty"`
	// Reaction is the stored attitude, enumeration values such as: like/dislike.
	Reaction enumor.FeedbackReaction `json:"reaction"`
	Comment  string                  `json:"comment,omitempty" validate:"max=500"`
}

// Validate validates the update request.
func (req *UpdateAgentRunFeedbackReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	return req.Reaction.Validate()
}

// -------------------------- List --------------------------

// ListAgentRunFeedbackReq is an alias for core.ListReq for listing agent run feedback.
type ListAgentRunFeedbackReq = core.ListReq

// ListAgentRunFeedbackResult defines the response for listing agent run feedback.
type ListAgentRunFeedbackResult struct {
	Count   uint64                       `json:"count"`
	Details []tableaiagent.FeedbackTable `json:"details"`
}

// -------------------------- Delete --------------------------

// DeleteAgentRunFeedbackReq defines the request for batch-deleting agent run feedback.
type DeleteAgentRunFeedbackReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
}

// Validate validates the delete request.
func (req *DeleteAgentRunFeedbackReq) Validate() error {
	if req.Filter == nil {
		return errors.New("filter is required")
	}
	return validator.Validate.Struct(req)
}
