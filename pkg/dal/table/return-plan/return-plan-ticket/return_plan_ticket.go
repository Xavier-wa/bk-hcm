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

// Package returnplanticket 退回计划主单表
package returnplanticket

import (
	"errors"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/dal/table/utils"
)

// ReturnPlanTicketColumns defines all the return_plan_ticket table's columns.
var ReturnPlanTicketColumns = utils.MergeColumns(nil, ReturnPlanTicketColumnDescriptor)

// ReturnPlanTicketColumnDescriptor is ReturnPlanTicketTable's column descriptors.
var ReturnPlanTicketColumnDescriptor = utils.ColumnDescriptors{
	{Column: "id", NamedC: "id", Type: enumor.String},
	{Column: "type", NamedC: "type", Type: enumor.String},
	{Column: "details", NamedC: "details", Type: enumor.Json},
	{Column: "applicant", NamedC: "applicant", Type: enumor.String},
	{Column: "bk_biz_id", NamedC: "bk_biz_id", Type: enumor.Numeric},
	{Column: "bk_biz_name", NamedC: "bk_biz_name", Type: enumor.String},
	{Column: "op_product_id", NamedC: "op_product_id", Type: enumor.Numeric},
	{Column: "op_product_name", NamedC: "op_product_name", Type: enumor.String},
	{Column: "plan_product_id", NamedC: "plan_product_id", Type: enumor.Numeric},
	{Column: "plan_product_name", NamedC: "plan_product_name", Type: enumor.String},
	{Column: "virtual_dept_id", NamedC: "virtual_dept_id", Type: enumor.Numeric},
	{Column: "virtual_dept_name", NamedC: "virtual_dept_name", Type: enumor.String},
	{Column: "status", NamedC: "status", Type: enumor.String},
	{Column: "message", NamedC: "message", Type: enumor.String},
	{Column: "remark", NamedC: "remark", Type: enumor.String},
	{Column: "submitted_at", NamedC: "submitted_at", Type: enumor.String},
	{Column: "creator", NamedC: "creator", Type: enumor.String},
	{Column: "reviser", NamedC: "reviser", Type: enumor.String},
	{Column: "created_at", NamedC: "created_at", Type: enumor.Time},
	{Column: "updated_at", NamedC: "updated_at", Type: enumor.Time},
}

// ReturnPlanTicketTable is used to save return plan ticket information.
type ReturnPlanTicketTable struct {
	// ID 主键
	ID string `db:"id" json:"id" validate:"lte=64"`
	// Type 单据类型，由 details 构成推导（add/adjust/cancel）
	Type enumor.ReturnPlanTicketType `db:"type" json:"type" validate:"lte=64"`
	// Details 退回计划条目列表（仅供详情展示与拆单，非可靠数据源）
	Details types.JsonField `db:"details" json:"details"`
	// Applicant 提单人
	Applicant string `db:"applicant" json:"applicant" validate:"lte=64"`
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
	// Status 单据状态
	Status enumor.ReturnPlanTicketStatus `db:"status" json:"status" validate:"lte=64"`
	// Message 失败原因（聚合各失败子单原因）
	Message string `db:"message" json:"message"`
	// Remark 退回计划说明
	Remark string `db:"remark" json:"remark" validate:"lte=1024"`
	// SubmittedAt 提单时间
	SubmittedAt string `db:"submitted_at" json:"submitted_at"`
	// Creator 创建人
	Creator string `db:"creator" json:"creator" validate:"max=64"`
	// Reviser 更新人
	Reviser string `db:"reviser" json:"reviser" validate:"max=64"`
	// CreatedAt 创建时间
	CreatedAt types.Time `db:"created_at" json:"created_at" validate:"isdefault"`
	// UpdatedAt 更新时间
	UpdatedAt types.Time `db:"updated_at" json:"updated_at" validate:"isdefault"`
}

// ReturnPlanDetails is Details struct of ReturnPlanTicketTable.
type ReturnPlanDetails []ReturnPlanDetail

// ReturnPlanDetail 退回计划条目，供详情展示与拆单，明细以 CRP 为准。
// 由 Original(原始退回计划)与 Updated(变更后退回计划)两部分构成，条目类型按两部分有无推导。
type ReturnPlanDetail struct {
	// Original 原始退回计划（来自 CRP queryReturnPlanItem），取消/调整时提供
	Original *ReturnPlanItem `json:"original"`
	// Updated 变更后退回计划，新增/调整时提供
	Updated *ReturnPlanItem `json:"updated"`
}

// Type 按 Original/Updated 有无推导条目类型：仅 Updated→新增(add) / 仅 Original→取消(cancel) / 兼有→调整(adjust)。
func (d ReturnPlanDetail) Type() (enumor.ReturnPlanTicketType, error) {
	switch {
	case d.Original != nil && d.Updated != nil:
		return enumor.ReturnPlanTicketTypeAdjust, nil
	case d.Updated != nil:
		return enumor.ReturnPlanTicketTypeAdd, nil
	case d.Original != nil:
		return enumor.ReturnPlanTicketTypeCancel, nil
	default:
		return "", errors.New("return plan detail must have original or updated")
	}
}

// DeriveTicketType 按明细构成推导主单类型：仅一种类型则返回该类型，多种混合→adjust。
func (details ReturnPlanDetails) DeriveTicketType() (enumor.ReturnPlanTicketType, error) {
	typeSet := make(map[enumor.ReturnPlanTicketType]struct{})
	for _, d := range details {
		dt, err := d.Type()
		if err != nil {
			return "", err
		}
		typeSet[dt] = struct{}{}
	}

	switch len(typeSet) {
	case 0:
		return "", errors.New("no valid return plan detail")
	case 1:
		for dt := range typeSet {
			return dt, nil
		}
	}

	return enumor.ReturnPlanTicketTypeAdjust, nil
}

// GroupItem 返回用于拆单分组维度(项目类型/资源池)的明细项：优先取 Original，以调整前数据分组；
// 无 Original(新增场景)时取 Updated。
func (d ReturnPlanDetail) GroupItem() *ReturnPlanItem {
	if d.Original != nil {
		return d.Original
	}
	return d.Updated
}

// ReturnPlanItem 退回计划明细项，Original 与 Updated 复用同一结构。
type ReturnPlanItem struct {
	// CrpPlanID 原始退回计划在 CRP 的 id（Original 必填，用于取消/调整定位；Updated 为空）
	CrpPlanID int64 `json:"crp_plan_id"`
	// ObsProject OBS项目类型
	ObsProject enumor.ObsProject `json:"obs_project"`
	// PlanTime 计划退回时间，格式 YYYY-MM-DD
	PlanTime string `json:"plan_time"`
	// ResourcePoolName 资源池（自研池/公有池）
	ResourcePoolName string `json:"resource_pool_name"`
	// City 城市
	City string `json:"city"`
	// Zone 可用区
	Zone string `json:"zone"`
	// InstanceModel 实例规格（与 instance_type 二选一）
	InstanceModel string `json:"instance_model"`
	// CvmAmount 退回实例数（对应 instance_model）
	CvmAmount int64 `json:"cvm_amount"`
	// InstanceType 实例类型（与 instance_model 二选一）
	InstanceType string `json:"instance_type"`
	// CoreTypeName 核心类型（对应 instance_type）
	CoreTypeName string `json:"core_type_name"`
	// CoreAmount 退回核心数（对应 instance_type）
	CoreAmount int64 `json:"core_amount"`
	// ReturnReasonClass 退回原因大类
	ReturnReasonClass string `json:"return_reason_class"`
	// Desc 备注
	Desc string `json:"desc"`
}

// TableName is the ReturnPlanTicketTable's database table name.
func (r ReturnPlanTicketTable) TableName() table.Name {
	return table.ReturnPlanTicketTable
}

// InsertValidate validate return plan ticket on insertion.
func (r ReturnPlanTicketTable) InsertValidate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	if len(r.ID) == 0 {
		return errors.New("id can not be empty")
	}

	if err := r.Type.Validate(); err != nil {
		return err
	}

	if err := r.Status.Validate(); err != nil {
		return err
	}

	if len(r.Details) == 0 {
		return errors.New("details can not be empty")
	}

	if len(r.Applicant) == 0 {
		return errors.New("applicant can not be empty")
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

	if len(r.SubmittedAt) == 0 {
		return errors.New("submitted_at can not be empty")
	}

	return nil
}

// UpdateValidate validate return plan ticket on update.
func (r ReturnPlanTicketTable) UpdateValidate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
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

	if len(r.Type) > 0 {
		if err := r.Type.Validate(); err != nil {
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
