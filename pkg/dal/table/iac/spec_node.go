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

// IacSpecNodeTableColumns iac_spec_node 表全部列。
var IacSpecNodeTableColumns = utils.MergeColumns(nil, IacSpecNodeTableColumnDescriptors)

// IacSpecNodeTableColumnDescriptors iac_spec_node 表列描述符。
var IacSpecNodeTableColumnDescriptors = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "repository_id", NamedC: "repository_id", Type: enumor.String},
	{Column: "spec_id", NamedC: "spec_id", Type: enumor.String},
	{Column: "parent_id", NamedC: "parent_id", Type: enumor.String},
	{Column: "level", NamedC: "level", Type: enumor.Numeric},
	{Column: "node_type", NamedC: "node_type", Type: enumor.String},
	{Column: "node_name", NamedC: "node_name", Type: enumor.String},
	{Column: "children_node_ids", NamedC: "children_node_ids", Type: enumor.Json},
	{Column: "file_size", NamedC: "file_size", Type: enumor.Numeric},
	{Column: "content", NamedC: "content", Type: enumor.String},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// IacSpecNodeTable iac_spec_node 表模型。
type IacSpecNodeTable struct {
	ID              string                 `db:"id" json:"id" validate:"lte=64"`
	RepositoryID    string                 `db:"repository_id" json:"repository_id" validate:"lte=64"`
	SpecID          string                 `db:"spec_id" json:"spec_id" validate:"lte=64"`
	ParentID        string                 `db:"parent_id" json:"parent_id" validate:"lte=64"`
	Level           int                    `db:"level" json:"level"`
	NodeType        enumor.IacSpecNodeType `db:"node_type" json:"node_type" validate:"lte=16"`
	NodeName        string                 `db:"node_name" json:"node_name" validate:"lte=256"`
	ChildrenNodeIDs *types.JsonField       `db:"children_node_ids" json:"children_node_ids"`
	FileSize        int64                  `db:"file_size" json:"file_size"`
	Content         *string                `db:"content" json:"content"`
	TenantID        string                 `db:"tenant_id" json:"tenant_id" validate:"lte=64"`
	Creator         string                 `db:"creator" json:"creator" validate:"lte=64"`
	Reviser         string                 `db:"reviser" json:"reviser" validate:"lte=64"`
	CreatedAt       types.Time             `db:"created_at" json:"created_at" validate:"excluded_unless"`
	UpdatedAt       types.Time             `db:"updated_at" json:"updated_at" validate:"excluded_unless"`
}

// Columns 返回 iac_spec_node 表列定义。
func (t IacSpecNodeTable) Columns() *utils.Columns {
	return IacSpecNodeTableColumns
}

// ColumnDescriptors 返回 iac_spec_node 表列描述符。
func (t IacSpecNodeTable) ColumnDescriptors() utils.ColumnDescriptors {
	return IacSpecNodeTableColumnDescriptors
}

// TableName 返回 iac_spec_node 表名。
func (t IacSpecNodeTable) TableName() table.Name {
	return table.IacSpecNodeTable
}

// InsertValidate 插入前校验 IacSpecNodeTable。
func (t IacSpecNodeTable) InsertValidate() error {
	if err := validator.Validate.Struct(t); err != nil {
		return err
	}

	if len(t.ID) != 0 {
		return errors.New("id can not be set")
	}

	if len(t.RepositoryID) == 0 {
		return errors.New("repository_id is required")
	}

	if len(t.SpecID) == 0 {
		return errors.New("spec_id is required")
	}

	if t.Level < 0 {
		return errors.New("level must be >= 0")
	}

	if err := t.NodeType.Validate(); err != nil {
		return err
	}

	if len(t.NodeName) == 0 {
		return errors.New("node_name is required")
	}

	if t.FileSize < 0 {
		return errors.New("file_size must be >= 0")
	}

	if t.NodeType == enumor.IacSpecNodeTypeDir && t.Content != nil {
		return errors.New("dir node content must be nil")
	}

	if t.NodeType == enumor.IacSpecNodeTypeFile && t.ChildrenNodeIDs != nil {
		return errors.New("file node children_node_ids must be nil")
	}

	if len(t.Creator) == 0 {
		return errors.New("creator is required")
	}

	if len(t.Reviser) == 0 {
		return errors.New("reviser is required")
	}

	return nil
}

// UpdateValidate 更新前校验 IacSpecNodeTable。
func (t IacSpecNodeTable) UpdateValidate() error {
	if err := validator.Validate.Struct(t); err != nil {
		return err
	}

	if len(t.ID) != 0 {
		return errors.New("id can not update")
	}

	if len(t.Creator) != 0 {
		return errors.New("creator can not update")
	}

	if t.Level < 0 {
		return errors.New("level must be >= 0")
	}

	if len(t.NodeType) != 0 {
		if err := t.NodeType.Validate(); err != nil {
			return err
		}
	}

	if t.FileSize < 0 {
		return errors.New("file_size must be >= 0")
	}

	if len(t.Reviser) == 0 {
		return errors.New("reviser is required")
	}

	return nil
}
