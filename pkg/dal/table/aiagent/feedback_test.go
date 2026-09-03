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
	"strings"
	"testing"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
	"hcm/pkg/tools/converter"
)

func validFeedbackTable() FeedbackTable {
	return FeedbackTable{
		ID:        "00000001",
		RunID:     "run1",
		SessionID: "session1",
		User:      "zhangsan",
		BkBizID:   123,
		Reaction:  enumor.FeedbackReactionLike,
		Creator:   "zhangsan",
	}
}

func TestFeedbackTable_InsertValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(f *FeedbackTable)
		wantErr bool
	}{
		{name: "valid row passes", mutate: func(f *FeedbackTable) {}, wantErr: false},
		{name: "missing id fails", mutate: func(f *FeedbackTable) { f.ID = "" }, wantErr: true},
		{name: "missing run_id fails", mutate: func(f *FeedbackTable) { f.RunID = "" }, wantErr: true},
		{name: "missing session_id fails", mutate: func(f *FeedbackTable) { f.SessionID = "" }, wantErr: true},
		{name: "missing user fails", mutate: func(f *FeedbackTable) { f.User = "" }, wantErr: true},
		{name: "zero bk_biz_id fails", mutate: func(f *FeedbackTable) { f.BkBizID = 0 }, wantErr: true},
		{name: "missing creator fails", mutate: func(f *FeedbackTable) { f.Creator = "" }, wantErr: true},
		{name: "missing reaction fails", mutate: func(f *FeedbackTable) { f.Reaction = "" }, wantErr: true},
		{name: "invalid reaction fails", mutate: func(f *FeedbackTable) {
			f.Reaction = enumor.FeedbackReaction("neutral")
		}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := validFeedbackTable()
			tc.mutate(&f)
			err := f.InsertValidate()
			if (err != nil) != tc.wantErr {
				t.Errorf("InsertValidate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestFeedbackTable_UpdateValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(f *FeedbackTable)
		wantErr bool
	}{
		{
			name: "valid update passes",
			mutate: func(f *FeedbackTable) {
				f.RunID, f.SessionID, f.User, f.Creator = "", "", "", ""
				f.Reviser = "lisi"
			},
			wantErr: false,
		},
		{
			name: "invalid reaction is rejected",
			mutate: func(f *FeedbackTable) {
				f.RunID, f.SessionID, f.User, f.Creator = "", "", "", ""
				f.Reviser = "lisi"
				f.Reaction = enumor.FeedbackReaction("neutral")
			},
			wantErr: true,
		},
		{
			name: "changing run_id is rejected",
			mutate: func(f *FeedbackTable) {
				f.SessionID, f.User, f.Creator = "", "", ""
				f.Reviser = "lisi"
			},
			wantErr: true,
		},
		{
			name: "changing session_id is rejected",
			mutate: func(f *FeedbackTable) {
				f.RunID, f.User, f.Creator = "", "", ""
				f.Reviser = "lisi"
			},
			wantErr: true,
		},
		{
			name: "changing user is rejected",
			mutate: func(f *FeedbackTable) {
				f.RunID, f.SessionID, f.Creator = "", "", ""
				f.Reviser = "lisi"
			},
			wantErr: true,
		},
		{
			name: "changing creator is rejected",
			mutate: func(f *FeedbackTable) {
				f.RunID, f.SessionID, f.User = "", "", ""
				f.Reviser = "lisi"
			},
			wantErr: true,
		},
		{
			name: "missing reviser is rejected",
			mutate: func(f *FeedbackTable) {
				f.RunID, f.SessionID, f.User, f.Creator = "", "", "", ""
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := validFeedbackTable()
			tc.mutate(&f)
			err := f.UpdateValidate()
			if (err != nil) != tc.wantErr {
				t.Errorf("UpdateValidate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestFeedbackTable_TableName(t *testing.T) {
	f := validFeedbackTable()
	if got := f.TableName(); got != "aiagent_run_feedback" {
		t.Errorf("TableName() = %q, want %q", got, "aiagent_run_feedback")
	}
}

func TestFeedbackTable_RearrangeBlankedFields(t *testing.T) {
	tests := []struct {
		name           string
		tags           types.StringArray
		comment        *string
		wantTagsSet    bool
		wantCommentSet bool
	}{
		{
			name:           "empty tags and comment are written",
			tags:           types.StringArray{},
			comment:        converter.ValToPtr(""),
			wantTagsSet:    true,
			wantCommentSet: true,
		},
		{
			name:           "nil tags and comment are skipped",
			tags:           nil,
			comment:        nil,
			wantTagsSet:    false,
			wantCommentSet: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := &FeedbackTable{
				Reaction: enumor.FeedbackReactionLike,
				Tags:     tc.tags,
				Comment:  tc.comment,
				Reviser:  "lisi",
			}
			opts := utils.NewFieldOptions().AddBlankedFields("tags", "comment").
				AddIgnoredFields("id", "creator", "created_at")
			expr, toUpdate, err := utils.RearrangeSQLDataWithOption(model, opts)
			if err != nil {
				t.Fatalf("RearrangeSQLDataWithOption() err = %v", err)
			}

			if got := strings.Contains(expr, ":tags"); got != tc.wantTagsSet {
				t.Errorf("set expr tags present = %v, want %v, expr = %q", got, tc.wantTagsSet, expr)
			}
			if got := strings.Contains(expr, ":comment"); got != tc.wantCommentSet {
				t.Errorf("set expr comment present = %v, want %v, expr = %q", got, tc.wantCommentSet, expr)
			}
			if _, ok := toUpdate["tags"]; ok != tc.wantTagsSet {
				t.Errorf("toUpdate tags present = %v, want %v", ok, tc.wantTagsSet)
			}
			if _, ok := toUpdate["comment"]; ok != tc.wantCommentSet {
				t.Errorf("toUpdate comment present = %v, want %v", ok, tc.wantCommentSet)
			}
		})
	}
}
