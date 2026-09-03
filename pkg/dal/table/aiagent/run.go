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

// RunColumns defines all columns for the aiagent_run table.
var RunColumns = utils.MergeColumns(nil, RunColumnDescriptor)

// RunColumnDescriptor describes each column of aiagent_run.
var RunColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "run_id", NamedC: "run_id", Type: enumor.String},
	{Column: "session_code", NamedC: "session_code", Type: enumor.String},
	{Column: "user", NamedC: "user", Type: enumor.String},
	{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric},
	{Column: "scene", NamedC: "scene", Type: enumor.String},
	{Column: "status", NamedC: "status", Type: enumor.String},
	{Column: "reason", NamedC: "reason", Type: enumor.String},
	{Column: "query", NamedC: "query", Type: enumor.String},
	{Column: "transcript", NamedC: "transcript", Type: enumor.Json},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// RunTable is used to save one aiagent conversation run.
type RunTable struct {
	ID          string                  `db:"id" json:"id"`
	RunID       string                  `db:"run_id" validate:"max=64" json:"run_id"`
	SessionCode string                  `db:"session_code" validate:"max=128" json:"session_code"`
	User        string                  `db:"user" validate:"max=64" json:"user"`
	BkBizID     int64                   `db:"bk_biz_id" json:"bk_biz_id"`
	Scene       enumor.IntentType       `db:"scene" validate:"max=32" json:"scene"`
	Status      enumor.AiagentRunStatus `db:"status" validate:"max=32" json:"status"`
	Reason      string                  `db:"reason" validate:"max=256" json:"reason"`
	Query       string                  `db:"query" json:"query"`
	Transcript  types.JsonField         `db:"transcript" json:"transcript"`
	Creator     string                  `db:"creator" validate:"max=64" json:"creator"`
	Reviser     string                  `db:"reviser" validate:"max=64" json:"reviser"`
	CreatedAt   types.Time              `db:"created_at" validate:"isdefault" json:"created_at"`
	UpdatedAt   types.Time              `db:"updated_at" validate:"isdefault" json:"updated_at"`
}

// TableName returns the database table name for aiagent runs.
func (r RunTable) TableName() table.Name {
	return table.AiagentRunTable
}

// InsertValidate validates the run on insertion.
func (r RunTable) InsertValidate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	if len(r.ID) == 0 {
		return errors.New("id can not be empty")
	}
	if len(r.RunID) == 0 {
		return errors.New("run_id can not be empty")
	}
	if len(r.SessionCode) == 0 {
		return errors.New("session_code can not be empty")
	}
	if len(r.User) == 0 {
		return errors.New("user can not be empty")
	}
	if len(r.Creator) == 0 {
		return errors.New("creator can not be empty")
	}
	status := r.Status
	if status == "" {
		status = enumor.AiagentRunStatusRunning
	}
	if err := status.Validate(); err != nil {
		return err
	}
	scene := r.Scene
	if scene == "" {
		scene = enumor.IntentTypeUnsupported
	}
	if err := scene.ValidateRunScene(); err != nil {
		return err
	}
	return nil
}

// UpdateValidate validates the run on update.
func (r RunTable) UpdateValidate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	if len(r.Creator) != 0 {
		return errors.New("creator can not update")
	}
	if len(r.Reviser) == 0 {
		return errors.New("reviser can not be empty")
	}
	if r.Status != "" {
		if err := r.Status.Validate(); err != nil {
			return err
		}
	}
	if r.Scene != "" {
		if err := r.Scene.ValidateRunScene(); err != nil {
			return err
		}
	}
	return nil
}
