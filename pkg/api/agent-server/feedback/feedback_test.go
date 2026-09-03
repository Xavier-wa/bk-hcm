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

package feedback

import (
	"strings"
	"testing"

	"hcm/pkg/criteria/enumor"
)

func TestCommitFeedbackReq_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     CommitFeedbackReq
		wantErr bool
	}{
		{
			name:    "valid like without tags/comment",
			req:     CommitFeedbackReq{SessionID: "sess1", RunID: "run1", Reaction: enumor.FeedbackReactionLike},
			wantErr: false,
		},
		{
			name: "valid dislike with tags and comment",
			req: CommitFeedbackReq{
				SessionID: "sess1",
				RunID:     "run1",
				Reaction:  enumor.FeedbackReactionDislike,
				Tags:      []string{"factual_error"},
				Comment:   "回答有误",
			},
			wantErr: false,
		},
		{
			name:    "missing session_id fails",
			req:     CommitFeedbackReq{RunID: "run1", Reaction: enumor.FeedbackReactionLike},
			wantErr: true,
		},
		{
			name:    "missing run_id fails",
			req:     CommitFeedbackReq{SessionID: "sess1", Reaction: enumor.FeedbackReactionLike},
			wantErr: true,
		},
		{
			name:    "invalid reaction fails",
			req:     CommitFeedbackReq{SessionID: "sess1", RunID: "run1", Reaction: enumor.FeedbackReaction("neutral")},
			wantErr: true,
		},
		{
			name: "comment exceeding max runes fails",
			req: CommitFeedbackReq{
				SessionID: "sess1",
				RunID:     "run1",
				Reaction:  enumor.FeedbackReactionLike,
				Comment:   strings.Repeat("字", MaxCommentRunes+1),
			},
			wantErr: true,
		},
		{
			name: "comment exactly at max runes passes",
			req: CommitFeedbackReq{
				SessionID: "sess1",
				RunID:     "run1",
				Reaction:  enumor.FeedbackReactionLike,
				Comment:   strings.Repeat("字", MaxCommentRunes),
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
