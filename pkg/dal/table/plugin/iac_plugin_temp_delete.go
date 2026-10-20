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

package tableplugin

import (
	"errors"
	"fmt"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// TODO(temp_delete): 临时 iac_plugin_temp_delete 表模型，合并插件注册分支后删除本文件。
// 对齐《插件注册与升级》设计方案 §2.1

// IacPluginTempDeleteTableColumns iac_plugin_temp_delete 表全部列。
var IacPluginTempDeleteTableColumns = utils.MergeColumns(nil, IacPluginTempDeleteTableColumnDescriptors)

// IacPluginTempDeleteTableColumnDescriptors iac_plugin_temp_delete 表列描述符。
var IacPluginTempDeleteTableColumnDescriptors = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "plugin_name", NamedC: "plugin_name", Type: enumor.String},
	{Column: "plugin_version", NamedC: "plugin_version", Type: enumor.String},
	{Column: "protocol", NamedC: "protocol", Type: enumor.String},
	{Column: "state", NamedC: "state", Type: enumor.String},
	{Column: "message", NamedC: "message", Type: enumor.String},
	{Column: "extension", NamedC: "extension", Type: enumor.Json},
	{Column: "memo", NamedC: "memo", Type: enumor.String},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// IacPluginTempDeleteTable iac_plugin_temp_delete 临时表模型。
type IacPluginTempDeleteTable struct {
	ID            string          `db:"id" json:"id" validate:"lte=64"`
	PluginName    string          `db:"plugin_name" json:"plugin_name" validate:"lte=64"`
	PluginVersion string          `db:"plugin_version" json:"plugin_version" validate:"lte=64"`
	Protocol      string          `db:"protocol" json:"protocol" validate:"lte=32"`
	State         string          `db:"state" json:"state" validate:"lte=32"`
	Message       string          `db:"message" json:"message" validate:"lte=255"`
	Extension     types.JsonField `db:"extension" json:"extension"`
	Memo          *string         `db:"memo" json:"memo"`
	Creator       string          `db:"creator" json:"creator" validate:"lte=64"`
	Reviser       string          `db:"reviser" json:"reviser" validate:"lte=64"`
	CreatedAt     types.Time      `db:"created_at" json:"created_at" validate:"excluded_unless"`
	UpdatedAt     types.Time      `db:"updated_at" json:"updated_at" validate:"excluded_unless"`
}

// UniqueKey 返回 iac_plugin_temp_delete 表唯一键。
func (t IacPluginTempDeleteTable) UniqueKey() string {
	return fmt.Sprintf("(%s,%s)", t.PluginName, t.PluginVersion)
}

// Columns 返回 iac_plugin_temp_delete 表列定义。
func (t IacPluginTempDeleteTable) Columns() *utils.Columns {
	return IacPluginTempDeleteTableColumns
}

// ColumnDescriptors 返回 iac_plugin_temp_delete 表列描述符。
func (t IacPluginTempDeleteTable) ColumnDescriptors() utils.ColumnDescriptors {
	return IacPluginTempDeleteTableColumnDescriptors
}

// TableName 返回 iac_plugin_temp_delete 表名。
func (t IacPluginTempDeleteTable) TableName() table.Name {
	return table.IacPluginTempDeleteTable
}

// InsertValidate 插入前校验 IacPluginTempDeleteTable。
func (t IacPluginTempDeleteTable) InsertValidate() error {
	if err := validator.Validate.Struct(t); err != nil {
		return err
	}

	if len(t.ID) != 0 {
		return errors.New("id can not be set")
	}

	if len(t.PluginName) == 0 {
		return errors.New("plugin_name is required")
	}

	if len(t.PluginVersion) == 0 {
		return errors.New("plugin_version is required")
	}

	if len(t.Protocol) == 0 {
		return errors.New("protocol is required")
	}

	if len(t.State) == 0 {
		return errors.New("state is required")
	}

	if len(t.Extension) == 0 {
		return errors.New("extension is required")
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

// UpdateValidate 更新前校验 IacPluginTempDeleteTable。
func (t IacPluginTempDeleteTable) UpdateValidate() error {
	if err := validator.Validate.Struct(t); err != nil {
		return err
	}

	if len(t.ID) != 0 {
		return errors.New("id can not be updated")
	}

	if len(t.PluginName) != 0 {
		return errors.New("plugin_name can not be updated")
	}

	if len(t.PluginVersion) != 0 {
		return errors.New("plugin_version can not be updated")
	}

	if len(t.Creator) != 0 {
		return errors.New("creator can not be updated")
	}

	if len(t.Reviser) == 0 {
		return errors.New("reviser is required")
	}

	return nil
}
