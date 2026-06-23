/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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

package cvmapply

import (
	"errors"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// ZiyanCvmApplyUserRecommendColumns defines ziyan_cvm_apply_user_recommend's columns.
var ZiyanCvmApplyUserRecommendColumns = utils.MergeColumns(nil, ZiyanCvmApplyUserRecommendColumnDescriptor)

// ZiyanCvmApplyUserRecommendColumnDescriptor is column descriptors.
var ZiyanCvmApplyUserRecommendColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric},
	{Column: "bk_username", NamedC: "bk_username", Type: enumor.String},
	{Column: "require_type", NamedC: "require_type", Type: enumor.Numeric},
	{Column: "region", NamedC: "region", Type: enumor.String},
	{Column: "device_type", NamedC: "device_type", Type: enumor.String},
	{Column: "image_id", NamedC: "image_id", Type: enumor.String},
	{Column: "count", NamedC: "count", Type: enumor.Numeric},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// ZiyanCvmApplyUserRecommend 用户维度申领机型推荐记录
type ZiyanCvmApplyUserRecommend struct {
	// ID 主键ID
	ID string `db:"id" json:"id"`
	// BkBizID 业务ID
	BkBizID int64 `db:"bk_biz_id" json:"bk_biz_id"`
	// BkUsername 申请人
	BkUsername string `db:"bk_username" json:"bk_username" validate:"max=64"`
	// RequireType 需求类型
	RequireType enumor.RequireType `db:"require_type" json:"require_type"`
	// Region 地域
	Region string `db:"region" json:"region" validate:"max=128"`
	// DeviceType 机型
	DeviceType string `db:"device_type" json:"device_type" validate:"max=64"`
	// ImageID 镜像ID
	ImageID string `db:"image_id" json:"image_id" validate:"max=64"`
	// Count 历史申领次数
	Count int `db:"count" json:"count"`
	// Creator 创建人
	Creator string `db:"creator" json:"creator" validate:"max=64"`
	// Reviser 更新人
	Reviser string `db:"reviser" json:"reviser" validate:"max=64"`
	// CreatedAt 创建时间
	CreatedAt types.Time `db:"created_at" json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt types.Time `db:"updated_at" validate:"excluded_unless" json:"updated_at"`
}

// TableName 表名
func (z *ZiyanCvmApplyUserRecommend) TableName() table.Name {
	return table.ZiyanCvmApplyUserRecommendTable
}

// InsertValidate validate insert
func (z *ZiyanCvmApplyUserRecommend) InsertValidate() error {
	if z.BkBizID == 0 {
		return errors.New("bk_biz_id is required")
	}
	if len(z.BkUsername) == 0 {
		return errors.New("bk_username is required")
	}
	if z.Count <= 0 {
		return errors.New("count must be greater than 0")
	}
	if len(z.Region) == 0 {
		return errors.New("region is required")
	}
	if len(z.DeviceType) == 0 {
		return errors.New("device_type is required")
	}
	if z.ImageID == "" {
		return errors.New("image_id is required")
	}
	return validator.Validate.Struct(z)
}

// UpdateValidate validate update
func (z *ZiyanCvmApplyUserRecommend) UpdateValidate() error {
	if len(z.Creator) != 0 {
		return errors.New("creator can not update")
	}
	if len(z.Reviser) == 0 {
		return errors.New("reviser can not be empty")
	}
	return validator.Validate.Struct(z)
}
