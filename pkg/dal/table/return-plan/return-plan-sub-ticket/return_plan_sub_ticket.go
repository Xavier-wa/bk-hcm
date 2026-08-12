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

// Package returnplansubticket 退回计划子单表
package returnplansubticket

import (
	"errors"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// ReturnPlanSubTicketColumns defines all the return_plan_sub_ticket table's columns.
var ReturnPlanSubTicketColumns = utils.MergeColumns(nil, ReturnPlanSubTicketColumnDescriptor)

// ReturnPlanSubTicketColumnDescriptor is ReturnPlanSubTicketTable's column descriptors.
var ReturnPlanSubTicketColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "ticket_id", NamedC: "ticket_id", Type: enumor.String},
	{Column: "sub_type", NamedC: "sub_type", Type: enumor.String},
	{Column: "sub_details", NamedC: "sub_details", Type: enumor.Json},
	{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric},
	{Column: "bk_biz_name", NamedC: "bk_biz_name", Type: enumor.String},
	{Column: "op_product_id", NamedC: "op_product_id", Type: enumor.Numeric},
	{Column: "op_product_name", NamedC: "op_product_name", Type: enumor.String},
	{Column: "plan_product_id", NamedC: "plan_product_id", Type: enumor.Numeric},
	{Column: "plan_product_name", NamedC: "plan_product_name", Type: enumor.String},
	{Column: "virtual_dept_id", NamedC: "virtual_dept_id", Type: enumor.Numeric},
	{Column: "virtual_dept_name", NamedC: "virtual_dept_name", Type: enumor.String},
	{Column: "obs_project", NamedC: "obs_project", Type: enumor.String},
	{Column: "res_pool_name", NamedC: "res_pool_name", Type: enumor.String},
	{Column: "status", NamedC: "status", Type: enumor.String},
	{Column: "crp_sn", NamedC: "crp_sn", Type: enumor.String},
	{Column: "crp_url", NamedC: "crp_url", Type: enumor.String},
	{Column: "message", NamedC: "message", Type: enumor.String},
	{Column: "submitted_at", NamedC: "submitted_at", Type: enumor.String},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// ReturnPlanSubTicketTable is used to save return plan sub_ticket information.
// 子单与 CRP 退回计划单一一对应。
type ReturnPlanSubTicketTable struct {
	// ID 主键
	ID string `db:"id" json:"id" validate:"lte=64"`
	// TicketID 父单据ID
	TicketID string `db:"ticket_id" json:"ticket_id" validate:"lte=64"`
	// SubType 子单类型（复用主单类型枚举）
	SubType enumor.ReturnPlanTicketType `db:"sub_type" json:"sub_type" validate:"lte=64"`
	// SubDetails 该子单对应的退回计划条目
	SubDetails types.JsonField `db:"sub_details" json:"sub_details"`
	// BkBizID 业务ID
	BkBizID int64 `db:"bk_biz_id" json:"bk_biz_id"`
	// BkBizName 业务名称
	BkBizName string `db:"bk_biz_name" json:"bk_biz_name" validate:"lte=64"`
	// OpProductID 运营产品ID
	OpProductID int64 `db:"op_product_id" json:"op_product_id"`
	// OpProductName 运营产品名称
	OpProductName string `db:"op_product_name" json:"op_product_name" validate:"lte=64"`
	// PlanProductID 规划产品ID
	PlanProductID int64 `db:"plan_product_id" json:"plan_product_id"`
	// PlanProductName 规划产品名称
	PlanProductName string `db:"plan_product_name" json:"plan_product_name" validate:"lte=64"`
	// VirtualDeptID 虚拟部门ID
	VirtualDeptID int64 `db:"virtual_dept_id" json:"virtual_dept_id"`
	// VirtualDeptName 虚拟部门名称
	VirtualDeptName string `db:"virtual_dept_name" json:"virtual_dept_name" validate:"lte=64"`
	// ObsProject 项目类型（拆单维度）
	ObsProject enumor.ObsProject `db:"obs_project" json:"obs_project" validate:"lte=64"`
	// ResPoolName 资源池（拆单维度）
	ResPoolName string `db:"res_pool_name" json:"res_pool_name" validate:"lte=64"`
	// Status 子单状态
	Status enumor.ReturnPlanSubTicketStatus `db:"status" json:"status" validate:"lte=64"`
	// CrpSN CRP单号
	CrpSN string `db:"crp_sn" json:"crp_sn" validate:"lte=64"`
	// CrpURL CRP单据链接
	CrpURL string `db:"crp_url" json:"crp_url" validate:"lte=255"`
	// Message 失败原因
	Message string `db:"message" json:"message"`
	// SubmittedAt 提单时间
	SubmittedAt string `db:"submitted_at" json:"submitted_at"`
	// Creator 创建人
	Creator string `db:"creator" json:"creator" validate:"lte=64"`
	// Reviser 更新人
	Reviser string `db:"reviser" json:"reviser" validate:"lte=64"`
	// CreatedAt 创建时间
	CreatedAt types.Time `db:"created_at" json:"created_at" validate:"isdefault"`
	// UpdatedAt 更新时间
	UpdatedAt types.Time `db:"updated_at" json:"updated_at" validate:"isdefault"`
}

// TableName is the ReturnPlanSubTicketTable's database table name.
func (r ReturnPlanSubTicketTable) TableName() table.Name {
	return table.ReturnPlanSubTicketTable
}

// InsertValidate validate return plan sub_ticket on insertion.
func (r ReturnPlanSubTicketTable) InsertValidate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	if len(r.ID) == 0 || len(r.TicketID) == 0 {
		return errors.New("id and ticket_id can not be empty")
	}

	if err := r.SubType.Validate(); err != nil {
		return err
	}

	if err := r.Status.Validate(); err != nil {
		return err
	}

	if len(r.SubDetails) == 0 {
		return errors.New("sub_details can not be empty")
	}

	if r.BkBizID <= 0 {
		return errors.New("bk biz id should be > 0")
	}

	if len(r.BkBizName) == 0 {
		return errors.New("bk biz name can not be empty")
	}

	if r.OpProductID <= 0 {
		return errors.New("op product id should be > 0")
	}

	if len(r.OpProductName) == 0 {
		return errors.New("op product name can not be empty")
	}

	if r.PlanProductID <= 0 {
		return errors.New("plan product id should be > 0")
	}

	if len(r.PlanProductName) == 0 {
		return errors.New("plan product name can not be empty")
	}

	if r.VirtualDeptID <= 0 {
		return errors.New("virtual dept id should be > 0")
	}

	if len(r.VirtualDeptName) == 0 {
		return errors.New("virtual dept name can not be empty")
	}

	if len(r.Creator) == 0 {
		return errors.New("creator can not be empty")
	}

	return nil
}

// UpdateValidate validate return plan sub_ticket on update.
func (r ReturnPlanSubTicketTable) UpdateValidate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	// 父单据不可变更
	if len(r.TicketID) != 0 {
		return errors.New("ticket_id can not update")
	}

	if r.BkBizID < 0 {
		return errors.New("bk biz id should be >= 0")
	}

	if r.OpProductID < 0 {
		return errors.New("op product id should be >= 0")
	}

	if r.PlanProductID < 0 {
		return errors.New("plan product id should be >= 0")
	}

	if r.VirtualDeptID < 0 {
		return errors.New("virtual dept id should be >= 0")
	}

	if len(r.SubType) > 0 {
		if err := r.SubType.Validate(); err != nil {
			return err
		}
	}

	if len(r.Status) > 0 {
		if err := r.Status.Validate(); err != nil {
			return err
		}
	}

	if len(r.Creator) != 0 {
		return errors.New("creator can not update")
	}

	if len(r.Reviser) == 0 {
		return errors.New("reviser can not be empty")
	}

	return nil
}
