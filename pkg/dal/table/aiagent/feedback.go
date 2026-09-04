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

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// FeedbackColumns defines all columns for the aiagent_run_feedback table.
var FeedbackColumns = utils.MergeColumns(nil, FeedbackColumnDescriptor)

// FeedbackColumnDescriptor describes each column of aiagent_run_feedback.
var FeedbackColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "run_id", NamedC: "run_id", Type: enumor.String},
	{Column: "session_id", NamedC: "session_id", Type: enumor.String},
	{Column: "user", NamedC: "user", Type: enumor.String},
	{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric},
	{Column: "scene", NamedC: "scene", Type: enumor.String},
	{Column: "query", NamedC: "query", Type: enumor.String},
	{Column: "tags", NamedC: "tags", Type: enumor.Json},
	{Column: "reaction", NamedC: "reaction", Type: enumor.String},
	{Column: "comment", NamedC: "comment", Type: enumor.String},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// FeedbackTable is used to save the user's feedback (like/dislike) on one Agent run.
// One run has at most one feedback row, enforced by the uk_run_id unique index.
type FeedbackTable struct {
	ID        string `db:"id" json:"id"`
	RunID     string `db:"run_id" validate:"max=64" json:"run_id"`
	SessionID string `db:"session_id" validate:"max=64" json:"session_id"`
	User      string `db:"user" validate:"max=64" json:"user"`
	BkBizID   int64  `db:"bk_biz_id" json:"bk_biz_id"`
	// Scene mirrors aiagent_run.scene at feedback time, immutable, kept for dashboard filter pushdown.
	Scene enumor.IntentType `db:"scene" validate:"max=32" json:"scene"`
	// Query mirrors aiagent_run.query at feedback time, immutable, kept for dashboard filter pushdown.
	Query string            `db:"query" json:"query"`
	Tags  types.StringArray `db:"tags" json:"tags"`
	// Reaction is the stored attitude, enumeration values such as: like/dislike.
	Reaction enumor.FeedbackReaction `db:"reaction" json:"reaction"`
	// Comment is the optional free-text note.
	Comment   *string    `db:"comment" validate:"omitempty,max=500" json:"comment"`
	Creator   string     `db:"creator" validate:"max=64" json:"creator"`
	Reviser   string     `db:"reviser" validate:"max=64" json:"reviser"`
	CreatedAt types.Time `db:"created_at" validate:"isdefault" json:"created_at"`
	UpdatedAt types.Time `db:"updated_at" validate:"isdefault" json:"updated_at"`
}

// TableName returns the database table name for aiagent run feedback.
func (f FeedbackTable) TableName() table.Name {
	return table.AiagentRunFeedbackTable
}

// InsertValidate validates the feedback row on insertion.
func (f FeedbackTable) InsertValidate() error {
	if err := validator.Validate.Struct(f); err != nil {
		return err
	}
	if len(f.ID) == 0 {
		return errors.New("id can not be empty")
	}
	if len(f.RunID) == 0 {
		return errors.New("run_id can not be empty")
	}
	if len(f.SessionID) == 0 {
		return errors.New("session_id can not be empty")
	}
	if len(f.User) == 0 {
		return errors.New("user can not be empty")
	}
	if f.BkBizID == 0 {
		return errors.New("bk_biz_id is required")
	}
	if err := f.Scene.ValidateRunScene(); err != nil {
		return err
	}
	if err := f.Reaction.Validate(); err != nil {
		return err
	}
	if len(f.Creator) == 0 {
		return errors.New("creator can not be empty")
	}
	return nil
}

// UpdateValidate validates the feedback row on update. run_id, session_id, user and
// creator MUST NOT be changed once created.
func (f FeedbackTable) UpdateValidate() error {
	if err := validator.Validate.Struct(f); err != nil {
		return err
	}
	if len(f.RunID) != 0 {
		return errors.New("run_id can not update")
	}
	if len(f.SessionID) != 0 {
		return errors.New("session_id can not update")
	}
	if len(f.User) != 0 {
		return errors.New("user can not update")
	}
	if f.Scene != "" {
		return errors.New("scene can not update")
	}
	if len(f.Query) != 0 {
		return errors.New("query can not update")
	}
	if len(f.Creator) != 0 {
		return errors.New("creator can not update")
	}
	if len(f.Reviser) == 0 {
		return errors.New("reviser can not be empty")
	}
	return f.Reaction.Validate()
}
