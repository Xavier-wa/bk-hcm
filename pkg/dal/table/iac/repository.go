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

// Package tableiac 定义 IaC 仓库、Spec 版本、目录文件节点与受管实例状态的数据库表模型。
package tableiac

import (
	"errors"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// IacRepositoryTableColumns iac_repository 表全部列。
var IacRepositoryTableColumns = utils.MergeColumns(nil, IacRepositoryTableColumnDescriptors)

// IacRepositoryTableColumnDescriptors iac_repository 表列描述符。
var IacRepositoryTableColumnDescriptors = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric},
	{Column: "name", NamedC: "name", Type: enumor.String},
	{Column: "latest_version", NamedC: "latest_version", Type: enumor.String},
	{Column: "last_exec_version", NamedC: "last_exec_version", Type: enumor.String},
	{Column: "memo", NamedC: "memo", Type: enumor.String},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// IacRepositoryTable iac_repository 表模型。
type IacRepositoryTable struct {
	ID              string     `db:"id" json:"id" validate:"lte=64"`
	BkBizID         int64      `db:"bk_biz_id" json:"bk_biz_id"`
	Name            string     `db:"name" json:"name" validate:"lte=128"`
	LatestVersion   string     `db:"latest_version" json:"latest_version" validate:"lte=64"`
	LastExecVersion string     `db:"last_exec_version" json:"last_exec_version" validate:"lte=64"`
	Memo            *string    `db:"memo" json:"memo"`
	TenantID        string     `db:"tenant_id" json:"tenant_id" validate:"lte=64"`
	Creator         string     `db:"creator" json:"creator" validate:"lte=64"`
	Reviser         string     `db:"reviser" json:"reviser" validate:"lte=64"`
	CreatedAt       types.Time `db:"created_at" json:"created_at" validate:"excluded_unless"`
	UpdatedAt       types.Time `db:"updated_at" json:"updated_at" validate:"excluded_unless"`
}

// Columns 返回 iac_repository 表列定义。
func (t IacRepositoryTable) Columns() *utils.Columns {
	return IacRepositoryTableColumns
}

// ColumnDescriptors 返回 iac_repository 表列描述符。
func (t IacRepositoryTable) ColumnDescriptors() utils.ColumnDescriptors {
	return IacRepositoryTableColumnDescriptors
}

// TableName 返回 iac_repository 表名。
func (t IacRepositoryTable) TableName() table.Name {
	return table.IacRepositoryTable
}

// InsertValidate 插入前校验 IacRepositoryTable。
func (t IacRepositoryTable) InsertValidate() error {
	if err := validator.Validate.Struct(t); err != nil {
		return err
	}

	if len(t.ID) != 0 {
		return errors.New("id can not be set")
	}

	if t.BkBizID <= 0 {
		return errors.New("bk_biz_id must be greater than 0")
	}

	if len(t.Name) == 0 {
		return errors.New("name is required")
	}

	if err := validator.ValidateMemo(t.Memo, false); err != nil {
		return err
	}

	if len(t.Creator) == 0 {
		return errors.New("creator is required")
	}

	if len(t.Reviser) == 0 {
		return errors.New("reviser is required")
	}

	return nil
}

// UpdateValidate 更新前校验 IacRepositoryTable。
func (t IacRepositoryTable) UpdateValidate() error {
	if err := validator.Validate.Struct(t); err != nil {
		return err
	}

	if len(t.ID) != 0 {
		return errors.New("id can not update")
	}

	if len(t.Creator) != 0 {
		return errors.New("creator can not update")
	}

	if err := validator.ValidateMemo(t.Memo, false); err != nil {
		return err
	}

	if len(t.Reviser) == 0 {
		return errors.New("reviser is required")
	}

	return nil
}
