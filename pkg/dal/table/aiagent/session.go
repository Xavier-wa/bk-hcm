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

// Package aiagent defines table structures for the aiagent module.
package aiagent

import (
	"errors"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// SessionColumns defines all columns for the aiagent_session table.
var SessionColumns = utils.MergeColumns(nil, SessionColumnDescriptor)

// SessionColumnDescriptor describes each column of aiagent_session.
var SessionColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "session_code", NamedC: "session_code", Type: enumor.String},
	{Column: "session_name", NamedC: "session_name", Type: enumor.String},
	{Column: "app_name", NamedC: "app_name", Type: enumor.String},
	{Column: "user", NamedC: "user", Type: enumor.String},
	{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric},
	{Column: "thread_id", NamedC: "thread_id", Type: enumor.String},
	{Column: "is_temporary", NamedC: "is_temporary", Type: enumor.Boolean},
	{Column: "session_content_count", NamedC: "session_content_count", Type: enumor.Numeric},
	{Column: "session_tag", NamedC: "session_tag", Type: enumor.String},
	{Column: "extension", NamedC: "extension", Type: enumor.Json},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// SessionTable is used to save aiagent session information.
type SessionTable struct {
	ID                  string            `db:"id" json:"id"`
	SessionCode         string            `db:"session_code" validate:"max=128" json:"session_code"`
	SessionName         string            `db:"session_name" validate:"max=255" json:"session_name"`
	AppName             string            `db:"app_name" validate:"max=64" json:"app_name"`
	User                string            `db:"user" validate:"max=64" json:"user"`
	BkBizID             int64             `db:"bk_biz_id" json:"bk_biz_id"`
	ThreadID            string            `db:"thread_id" validate:"max=64" json:"thread_id"`
	IsTemporary         bool              `db:"is_temporary" json:"is_temporary"`
	SessionContentCount uint32            `db:"session_content_count" json:"session_content_count"`
	SessionTag          enumor.IntentType `db:"session_tag" validate:"max=64" json:"session_tag"`
	Extension           types.JsonField   `db:"extension" json:"extension"`
	Creator             string            `db:"creator" validate:"max=64" json:"creator"`
	Reviser             string            `db:"reviser" validate:"max=64" json:"reviser"`
	CreatedAt           types.Time        `db:"created_at" validate:"isdefault" json:"created_at"`
	UpdatedAt           types.Time        `db:"updated_at" validate:"isdefault" json:"updated_at"`
}

// TableName returns the database table name for aiagent sessions.
func (s SessionTable) TableName() table.Name {
	return table.AiagentSessionTable
}

// InsertValidate validates the session on insertion.
func (s SessionTable) InsertValidate() error {
	if err := validator.Validate.Struct(s); err != nil {
		return err
	}
	if len(s.ID) == 0 {
		return errors.New("id can not be empty")
	}
	if len(s.AppName) == 0 {
		return errors.New("app_name can not be empty")
	}
	if len(s.User) == 0 {
		return errors.New("user can not be empty")
	}
	if s.BkBizID == 0 {
		return errors.New("bk_biz_id is required")
	}
	if len(s.Creator) == 0 {
		return errors.New("creator can not be empty")
	}
	return nil
}

// UpdateValidate validates the session on update.
func (s SessionTable) UpdateValidate() error {
	if err := validator.Validate.Struct(s); err != nil {
		return err
	}
	if len(s.Creator) != 0 {
		return errors.New("creator can not update")
	}
	if len(s.Reviser) == 0 {
		return errors.New("reviser can not be empty")
	}
	return nil
}
