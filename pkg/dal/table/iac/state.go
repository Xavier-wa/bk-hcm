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

package tableiac

import (
	"errors"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// IacStateTableColumns iac_state 表全部列。
var IacStateTableColumns = utils.MergeColumns(nil, IacStateTableColumnDescriptors)

// IacStateTableColumnDescriptors iac_state 表列描述符。
var IacStateTableColumnDescriptors = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "repository_id", NamedC: "repository_id", Type: enumor.String},
	{Column: "block_id", NamedC: "block_id", Type: enumor.String},
	{Column: "kind", NamedC: "kind", Type: enumor.String},
	{Column: "current", NamedC: "current", Type: enumor.Json},
	{Column: "deposed", NamedC: "deposed", Type: enumor.Json},
	{Column: "last_flow_id", NamedC: "last_flow_id", Type: enumor.String},
	{Column: "import_from_spec_id", NamedC: "import_from_spec_id", Type: enumor.String},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// IacStateTable iac_state 表模型。
type IacStateTable struct {
	ID               string              `db:"id" json:"id" validate:"lte=64"`
	RepositoryID     string              `db:"repository_id" json:"repository_id" validate:"lte=64"`
	BlockID          string              `db:"block_id" json:"block_id" validate:"lte=512"`
	Kind             enumor.IacStateKind `db:"kind" json:"kind" validate:"lte=32"`
	Current          types.JsonField     `db:"current" json:"current"`
	Deposed          types.JsonField     `db:"deposed" json:"deposed"`
	LastFlowID       string              `db:"last_flow_id" json:"last_flow_id" validate:"lte=64"`
	ImportFromSpecID *string             `db:"import_from_spec_id" json:"import_from_spec_id"`
	TenantID         string              `db:"tenant_id" json:"tenant_id" validate:"lte=64"`
	Creator          string              `db:"creator" json:"creator" validate:"lte=64"`
	Reviser          string              `db:"reviser" json:"reviser" validate:"lte=64"`
	CreatedAt        types.Time          `db:"created_at" json:"created_at" validate:"excluded_unless"`
	UpdatedAt        types.Time          `db:"updated_at" json:"updated_at" validate:"excluded_unless"`
}

// Columns 返回 iac_state 表列定义。
func (t IacStateTable) Columns() *utils.Columns {
	return IacStateTableColumns
}

// ColumnDescriptors 返回 iac_state 表列描述符。
func (t IacStateTable) ColumnDescriptors() utils.ColumnDescriptors {
	return IacStateTableColumnDescriptors
}

// TableName 返回 iac_state 表名。
func (t IacStateTable) TableName() table.Name {
	return table.IacStateTable
}

// InsertValidate 插入前校验 IacStateTable。
func (t IacStateTable) InsertValidate() error {
	if err := validator.Validate.Struct(t); err != nil {
		return err
	}

	if len(t.ID) != 0 {
		return errors.New("id can not be set")
	}

	if len(t.RepositoryID) == 0 {
		return errors.New("repository_id is required")
	}

	if len(t.BlockID) == 0 {
		return errors.New("block_id is required")
	}

	if err := t.Kind.Validate(); err != nil {
		return err
	}

	if len(t.Deposed) == 0 {
		return errors.New("deposed is required")
	}

	if t.ImportFromSpecID != nil && len(*t.ImportFromSpecID) > 64 {
		return errors.New("import_from_spec_id length must be <= 64")
	}

	if len(t.Creator) == 0 {
		return errors.New("creator is required")
	}

	if len(t.Reviser) == 0 {
		return errors.New("reviser is required")
	}

	return nil
}

// UpdateValidate 更新前校验 IacStateTable。
func (t IacStateTable) UpdateValidate() error {
	if err := validator.Validate.Struct(t); err != nil {
		return err
	}

	if len(t.ID) != 0 {
		return errors.New("id can not update")
	}

	if len(t.Creator) != 0 {
		return errors.New("creator can not update")
	}

	if len(t.Kind) != 0 {
		if err := t.Kind.Validate(); err != nil {
			return err
		}
	}

	if t.ImportFromSpecID != nil && len(*t.ImportFromSpecID) > 64 {
		return errors.New("import_from_spec_id length must be <= 64")
	}

	if len(t.Reviser) == 0 {
		return errors.New("reviser is required")
	}

	return nil
}
