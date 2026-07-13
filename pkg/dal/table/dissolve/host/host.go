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

package host

import (
	"errors"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// RecycleHostColumns defines all the recycle host table's columns.
var RecycleHostColumns = utils.MergeColumns(nil, RecycleHostColumnDescriptor)

// RecycleHostColumnDescriptor is RecycleHostInfo's column descriptors.
var RecycleHostColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "asset_id", NamedC: "asset_id", Type: enumor.String},
	{Column: "inner_ip", NamedC: "inner_ip", Type: enumor.String},
	{Column: "device_type", NamedC: "device_type", Type: enumor.String},
	{Column: "module", NamedC: "module", Type: enumor.String},
	{Column: "abolish_phase", NamedC: "abolish_phase", Type: enumor.String},
	{Column: "project_name", NamedC: "project_name", Type: enumor.String},
	{Column: "project_id", NamedC: "project_id", Type: enumor.Numeric},
	{Column: "region", NamedC: "region", Type: enumor.String},
	{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric},
	{Column: "group_id", NamedC: "group_id", Type: enumor.Numeric},
	{Column: "operators", NamedC: "operators", Type: enumor.Json},
	{Column: "cpu_core", NamedC: "cpu_core", Type: enumor.Numeric},
	{Column: "is_ignore", NamedC: "is_ignore", Type: enumor.Boolean},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// RecycleHostTable is used to save recycle host information.
type RecycleHostTable struct {
	// ID 自增ID
	ID string `db:"id" json:"id"`
	// AssetID 主机固资号
	AssetID *string `db:"asset_id" json:"asset_id"`
	// InnerIP 主机内网IP
	InnerIP *string `db:"inner_ip" json:"inner_ip"`
	// DeviceType 机型
	DeviceType *string `db:"device_type" json:"device_type"`
	// Module 主机所属的裁撤模块名称
	Module *string `db:"module" json:"module"`
	// AbolishPhase 裁撤阶段
	AbolishPhase *enumor.AbolishPhase `db:"abolish_phase" json:"abolish_phase"`
	// ProjectName 项目名称
	ProjectName *string `db:"project_name" json:"project_name"`
	// ProjectID 裁撤项目ID
	ProjectID *int `db:"project_id" json:"project_id"`
	// Region 地域ID
	Region *string `db:"region" json:"region"`
	// BkBizID 业务ID
	BkBizID *int64 `db:"bk_biz_id" json:"bk_biz_id"`
	// GroupID 组织ID
	GroupID *int64 `db:"group_id" json:"group_id"`
	// Operators 负责人列表
	Operators types.StringArray `db:"operators" json:"operators"`
	// CPUCore CPU核心数
	CPUCore *int `db:"cpu_core" json:"cpu_core"`
	// IsIgnore 是否忽略该主机
	IsIgnore *bool `db:"is_ignore" json:"is_ignore"`
	// Creator 创建者
	Creator string `db:"creator" validate:"max=64" json:"creator"`
	// Reviser 更新者
	Reviser string `db:"reviser" validate:"max=64" json:"reviser"`
	// CreatedAt 创建时间
	CreatedAt types.Time `db:"created_at" validate:"isdefault" json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt types.Time `db:"updated_at" validate:"isdefault" json:"updated_at"`
}

// TableName is the recycle host database table name.
func (r RecycleHostTable) TableName() table.Name {
	return table.RecycleHostInfo
}

// InsertValidate validate recycle host on insertion.
func (r RecycleHostTable) InsertValidate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	if len(r.ID) == 0 {
		return errors.New("id can not be empty")
	}

	if r.AssetID == nil {
		return errors.New("asset_id can not be empty")
	}

	if r.ProjectID == nil {
		return errors.New("project_id can not be empty")
	}

	if r.Module == nil {
		return errors.New("module can not be empty")
	}

	if r.AbolishPhase == nil {
		return errors.New("abolish_phase can not be empty")
	}

	if r.ProjectName == nil {
		return errors.New("project_name can not be empty")
	}

	if len(r.Creator) == 0 {
		return errors.New("creator can not be empty")
	}

	return nil
}

// UpdateValidate validate recycle host on update.
func (r RecycleHostTable) UpdateValidate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	if len(r.Creator) != 0 {
		return errors.New("creator can not update")
	}

	if len(r.Reviser) == 0 {
		return errors.New("reviser can not be empty")
	}

	return nil
}
