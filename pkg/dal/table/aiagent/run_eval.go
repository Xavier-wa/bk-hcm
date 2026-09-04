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
	"fmt"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// RunEvalColumns defines all columns for the aiagent_run_eval table.
var RunEvalColumns = utils.MergeColumns(nil, RunEvalColumnDescriptor)

// RunEvalColumnDescriptor describes each column of aiagent_run_eval.
var RunEvalColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "run_id", NamedC: "run_id", Type: enumor.String},
	{Column: "session_id", NamedC: "session_id", Type: enumor.String},
	{Column: "user", NamedC: "user", Type: enumor.String},
	{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric},
	{Column: "scene", NamedC: "scene", Type: enumor.String},
	{Column: "query", NamedC: "query", Type: enumor.String},
	{Column: "start_run_id", NamedC: "start_run_id", Type: enumor.String},
	{Column: "process_score", NamedC: "process_score", Type: enumor.Numeric},
	{Column: "outcome_score", NamedC: "outcome_score", Type: enumor.Numeric},
	{Column: "quality_score", NamedC: "quality_score", Type: enumor.Numeric},
	{Column: "redlines", NamedC: "redlines", Type: enumor.Json},
	{Column: "reason_code", NamedC: "reason_code", Type: enumor.String},
	{Column: "rubric_version", NamedC: "rubric_version", Type: enumor.String},
	{Column: "eval_result", NamedC: "eval_result", Type: enumor.Json},
	{Column: "context_snapshot", NamedC: "context_snapshot", Type: enumor.Json},
	{Column: "eval_trace", NamedC: "eval_trace", Type: enumor.Json},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// RunEvalTable is used to save one model evaluation result.
type RunEvalTable struct {
	ID        string `db:"id" json:"id"`
	RunID     string `db:"run_id" validate:"max=64" json:"run_id"`
	SessionID string `db:"session_id" validate:"max=128" json:"session_id"`
	User      string `db:"user" validate:"max=64" json:"user"`
	BkBizID   int64  `db:"bk_biz_id" json:"bk_biz_id"`
	// Scene mirrors aiagent_run.scene at eval time, kept for dashboard filter/sort pushdown.
	Scene enumor.IntentType `db:"scene" validate:"max=32" json:"scene"`
	// Query mirrors aiagent_run.query at eval time, kept for dashboard filter/sort pushdown.
	Query           string                       `db:"query" json:"query"`
	StartRunID      string                       `db:"start_run_id" validate:"max=64" json:"start_run_id"`
	ProcessScore    int                          `db:"process_score" json:"process_score"`
	OutcomeScore    int                          `db:"outcome_score" json:"outcome_score"`
	QualityScore    int                          `db:"quality_score" json:"quality_score"`
	Redlines        types.StringArray            `db:"redlines" json:"redlines"`
	ReasonCode      enumor.AiagentEvalReasonCode `db:"reason_code" validate:"max=64" json:"reason_code"`
	RubricVersion   string                       `db:"rubric_version" validate:"max=32" json:"rubric_version"`
	EvalResult      types.JsonField              `db:"eval_result" json:"eval_result"`
	ContextSnapshot types.JsonField              `db:"context_snapshot" json:"context_snapshot"`
	EvalTrace       types.JsonField              `db:"eval_trace" json:"eval_trace"`
	Creator         string                       `db:"creator" validate:"max=64" json:"creator"`
	Reviser         string                       `db:"reviser" validate:"max=64" json:"reviser"`
	CreatedAt       types.Time                   `db:"created_at" validate:"isdefault" json:"created_at"`
	UpdatedAt       types.Time                   `db:"updated_at" validate:"isdefault" json:"updated_at"`
}

// TableName returns the database table name for aiagent run evals.
func (r RunEvalTable) TableName() table.Name {
	return table.AiagentRunEvalTable
}

func validateEvalScore(name string, score int) error {
	if score < constant.MinEvalScore || score > constant.MaxEvalScore {
		return fmt.Errorf("%s must be in [%d, %d]", name, constant.MinEvalScore, constant.MaxEvalScore)
	}
	return nil
}

// InsertValidate validates the eval row on insertion.
func (r RunEvalTable) InsertValidate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	if len(r.ID) == 0 {
		return errors.New("id can not be empty")
	}
	if len(r.RunID) == 0 {
		return errors.New("run_id can not be empty")
	}
	if len(r.SessionID) == 0 {
		return errors.New("session_id can not be empty")
	}
	if len(r.User) == 0 {
		return errors.New("user can not be empty")
	}
	if err := r.Scene.ValidateRunScene(); err != nil {
		return err
	}
	if len(r.StartRunID) == 0 {
		return errors.New("start_run_id can not be empty")
	}
	if len(r.Creator) == 0 {
		return errors.New("creator can not be empty")
	}
	if err := validateEvalScore("process_score", r.ProcessScore); err != nil {
		return err
	}
	if err := validateEvalScore("outcome_score", r.OutcomeScore); err != nil {
		return err
	}
	if err := validateEvalScore("quality_score", r.QualityScore); err != nil {
		return err
	}
	if err := r.ReasonCode.Validate(); err != nil {
		return err
	}
	if err := enumor.ValidateAiagentEvalRedlines(r.Redlines); err != nil {
		return err
	}
	if len(r.RubricVersion) == 0 {
		return errors.New("rubric_version can not be empty")
	}
	if len(r.EvalResult) == 0 {
		return errors.New("eval_result can not be empty")
	}
	if len(r.ContextSnapshot) == 0 {
		return errors.New("context_snapshot can not be empty")
	}
	return nil
}

// UpdateValidate validates the eval row on update.
func (r RunEvalTable) UpdateValidate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	if len(r.Creator) != 0 {
		return errors.New("creator can not update")
	}
	if len(r.Reviser) == 0 {
		return errors.New("reviser can not be empty")
	}
	if r.Scene != "" {
		if err := r.Scene.ValidateRunScene(); err != nil {
			return err
		}
	}
	if r.ReasonCode != "" {
		if err := r.ReasonCode.Validate(); err != nil {
			return err
		}
	}
	if r.Redlines != nil {
		if err := enumor.ValidateAiagentEvalRedlines(r.Redlines); err != nil {
			return err
		}
	}
	if r.ProcessScore != 0 {
		if err := validateEvalScore("process_score", r.ProcessScore); err != nil {
			return err
		}
	}
	if r.OutcomeScore != 0 {
		if err := validateEvalScore("outcome_score", r.OutcomeScore); err != nil {
			return err
		}
	}
	if r.QualityScore != 0 {
		if err := validateEvalScore("quality_score", r.QualityScore); err != nil {
			return err
		}
	}
	return nil
}
