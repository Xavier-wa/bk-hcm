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

package cvmapply

import (
	"errors"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// ZiyanCvmApplyOrderColumns defines ziyan_cvm_apply_order's columns.
var ZiyanCvmApplyOrderColumns = utils.MergeColumns(nil, ZiyanCvmApplyOrderColumnDescriptor)

// ZiyanCvmApplyOrderColumnDescriptor is column descriptors.
var ZiyanCvmApplyOrderColumnDescriptor = utils.ColumnDescriptors{
	{Column: "order_id", NamedC: "order_id", Type: enumor.Numeric},
	{Column: "product_type", NamedC: "product_type", Type: enumor.String},
	{Column: "itsm_ticket_id", NamedC: "itsm_ticket_id", Type: enumor.String},
	{Column: "stage", NamedC: "stage", Type: enumor.String},
	{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric},
	{Column: "bk_username", NamedC: "bk_username", Type: enumor.String},
	{Column: "follower", NamedC: "follower", Type: enumor.Json},
	{Column: "enable_notice", NamedC: "enable_notice", Type: enumor.Boolean},
	{Column: "require_type", NamedC: "require_type", Type: enumor.Numeric},
	{Column: "expect_time", NamedC: "expect_time", Type: enumor.Time},
	{Column: "remark", NamedC: "remark", Type: enumor.String},
	{Column: "suborders", NamedC: "suborders", Type: enumor.Json},
	{Column: "old_suborders", NamedC: "old_suborders", Type: enumor.Json},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// ZiyanCvmApplyOrder 自研云CVM申请单主单
type ZiyanCvmApplyOrder struct {
	// OrderID 申请单ID
	OrderID uint64 `db:"order_id" json:"order_id"`
	// ProductType 产品类型
	ProductType enumor.ProductType `db:"product_type" json:"product_type" validate:"max=64"`
	// ItsmTicketID ITSM工单ID
	ItsmTicketID string `db:"itsm_ticket_id" json:"itsm_ticket_id" validate:"max=64"`
	// Stage 阶段(AUDIT:审核中 RUNNING:运行中 DONE:已完成 SUSPEND:已暂停)
	Stage enumor.TicketStage `db:"stage" json:"stage" validate:"max=32"`
	// BkBizID 业务ID
	BkBizID int64 `db:"bk_biz_id" json:"bk_biz_id"`
	// BkUsername 申请人
	BkUsername string `db:"bk_username" json:"bk_username" validate:"max=64"`
	// Follower 关注人列表(JSON数组)
	Follower types.JsonField `db:"follower" json:"follower"`
	// EnableNotice 是否启用通知(0:否 1:是)
	EnableNotice bool `db:"enable_notice" json:"enable_notice"`
	// RequireType 需求类型(1:常规 2:春保等)
	RequireType enumor.RequireType `db:"require_type" json:"require_type"`
	// ExpectTime 期望交付时间
	ExpectTime string `db:"expect_time" json:"expect_time"`
	// Remark 备注
	Remark string `db:"remark" json:"remark" validate:"max=255"`
	// Suborders 子单列表(JSON数组)
	Suborders types.JsonField `db:"suborders" json:"suborders"`
	// OldSuborders 旧子单列表(JSON数组)
	OldSuborders types.JsonField `db:"old_suborders" json:"old_suborders"`
	// Creator 创建人
	Creator string `db:"creator" json:"creator" validate:"max=64"`
	// Reviser 修改人
	Reviser string `db:"reviser" json:"reviser" validate:"max=64"`
	// CreatedAt 创建时间
	CreatedAt types.Time `db:"created_at" json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt types.Time `db:"updated_at" validate:"excluded_unless" json:"updated_at"`
}

// TableName 表名
func (z *ZiyanCvmApplyOrder) TableName() table.Name {
	return table.ZiyanCvmApplyOrderTable
}

// InsertValidate validate insert
func (z *ZiyanCvmApplyOrder) InsertValidate() error {
	if err := z.ProductType.Validate(); err != nil {
		return err
	}
	if len(z.Stage) == 0 {
		return errors.New("stage is required")
	}
	if z.BkBizID == 0 {
		return errors.New("bk_biz_id is required")
	}
	if z.RequireType <= 0 {
		return errors.New("require_type is required")
	}
	if len(z.Suborders) == 0 {
		return errors.New("suborders is required")
	}
	return validator.Validate.Struct(z)
}

// UpdateValidate validate update
func (z *ZiyanCvmApplyOrder) UpdateValidate() error {
	if z.OrderID == 0 {
		return errors.New("order_id is required")
	}
	return validator.Validate.Struct(z)
}
