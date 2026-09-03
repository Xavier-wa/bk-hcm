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

// Package feedback defines agent-server user feedback (like/dislike) API types.
package feedback

import (
	"errors"
	"unicode/utf8"

	"hcm/pkg/criteria/enumor"
)

// MaxCommentRunes is the maximum length of a feedback comment, counted by
// Unicode character (rune), not byte.
const MaxCommentRunes = 500

// CommitFeedbackReq is the request body for committing (upserting) a user's
// feedback on one Agent run. bk_biz_id is taken from the path; session_id is
// required to verify the run belongs to that session.
type CommitFeedbackReq struct {
	SessionID string                  `json:"session_id"`
	RunID     string                  `json:"run_id"`
	Reaction  enumor.FeedbackReaction `json:"reaction"`
	// Tags 归因标签，可选。取值必须是当前 Reaction 对应 global_config 标签 map 的 key。
	Tags []string `json:"tags,omitempty"`
	// Comment 用户手输文本，可选，上限 MaxCommentRunes 个字符。
	Comment string `json:"comment,omitempty"`
}

// Validate validates the request body.
func (r *CommitFeedbackReq) Validate() error {
	if r.SessionID == "" {
		return errors.New("session_id is required")
	}
	if r.RunID == "" {
		return errors.New("run_id is required")
	}
	if err := r.Reaction.Validate(); err != nil {
		return err
	}
	if utf8.RuneCountInString(r.Comment) > MaxCommentRunes {
		return errors.New("comment exceeds the maximum length")
	}
	return nil
}

// CommitFeedbackResp is the response body for committing feedback.
type CommitFeedbackResp struct {
	// ID 反馈表主键。新建时为新 ID，覆盖更新时为该 run 已有行的 ID。
	ID string `json:"id"`
}
