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

package enumor

import "fmt"

// FeedbackReaction is the user's attitude toward one Agent run. It is stored in
// aiagent_run_feedback.reaction as the same string values used on the API.
type FeedbackReaction string

const (
	// FeedbackReactionLike 点赞
	FeedbackReactionLike FeedbackReaction = "like"
	// FeedbackReactionDislike 点踩
	FeedbackReactionDislike FeedbackReaction = "dislike"
)

// Validate checks whether the reaction is one of the declared values.
func (r FeedbackReaction) Validate() error {
	switch r {
	case FeedbackReactionLike, FeedbackReactionDislike:
		return nil
	default:
		return fmt.Errorf("unsupported feedback reaction: %s", r)
	}
}
