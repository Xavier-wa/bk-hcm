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

// Package cvmapply ...
package cvmapply

import (
	tasktypes "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/criteria/validator"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
)

// BatchCreateZiyanCvmApplyOrderReq batch create request
type BatchCreateZiyanCvmApplyOrderReq struct {
	ApplyOrders []ZiyanCvmApplyOrderCreateReq `json:"apply_orders" validate:"required,max=100"`
}

// Validate ...
func (c *BatchCreateZiyanCvmApplyOrderReq) Validate() error {
	if len(c.ApplyOrders) == 0 || len(c.ApplyOrders) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"apply_orders count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.ApplyOrders {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmApplyOrderCreateReq create request
type ZiyanCvmApplyOrderCreateReq struct {
	OrderID      uint64                `json:"order_id" validate:"omitempty"`
	ProductType  enumor.ProductType    `json:"product_type" validate:"required"`
	ItsmTicketID string                `json:"itsm_ticket_id" validate:"max=64"`
	Stage        tasktypes.TicketStage `json:"stage" validate:"required,max=32"`
	BkBizID      int64                 `json:"bk_biz_id" validate:"required"`
	BkUsername   string                `json:"bk_username" validate:"required,max=64"`
	Follower     types.JsonField       `json:"follower" validate:"omitempty"`
	EnableNotice bool                  `json:"enable_notice" validate:"omitempty"`
	RequireType  enumor.RequireType    `json:"require_type" validate:"required"`
	ExpectTime   string                `json:"expect_time" validate:"omitempty"`
	Remark       string                `json:"remark" validate:"max=255"`
	Suborders    []*tasktypes.Suborder `json:"suborders" validate:"required"`
	OldSuborders []*tasktypes.Suborder `json:"old_suborders" validate:"omitempty"`
	// CreatedAt 原始创建时间（用于数据迁移，保留历史时间）
	CreatedAt types.Time `json:"created_at" validate:"omitempty"`
	UpdatedAt types.Time `json:"updated_at" validate:"omitempty"`
}

// Validate ...
func (c *ZiyanCvmApplyOrderCreateReq) Validate() error {
	if err := c.ProductType.Validate(); err != nil {
		return err
	}
	return validator.Validate.Struct(c)
}

// BatchCreateCvmApplyOrderResult batch create result
type BatchCreateCvmApplyOrderResult struct {
	IDs []uint64 `json:"ids"`
}

// BatchCreateCvmApplyOrderResp is a standard create operation http response.
type BatchCreateCvmApplyOrderResp struct {
	rest.BaseResp `json:",inline"`
	Data          *BatchCreateCvmApplyOrderResult `json:"data"`
}

// ZiyanCvmApplyOrderListReq list request
type ZiyanCvmApplyOrderListReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
	Page   *core.BasePage     `json:"page" validate:"required"`
	Fields []string           `json:"fields" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmApplyOrderListReq) Validate() error {
	return validator.Validate.Struct(req)
}

// ZiyanCvmApplyOrderListResult list result
type ZiyanCvmApplyOrderListResult = core.ListResultT[*cvmapplytable.ZiyanCvmApplyOrder]

// ZiyanCvmApplyOrderListResp define list resp.
type ZiyanCvmApplyOrderListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *ZiyanCvmApplyOrderListResult `json:"data"`
}

// BatchUpdateZiyanCvmApplyOrderReq batch update request
type BatchUpdateZiyanCvmApplyOrderReq struct {
	ApplyOrders []ZiyanCvmApplyOrderUpdateReq `json:"apply_orders" validate:"required,max=100"`
}

// Validate ...
func (c *BatchUpdateZiyanCvmApplyOrderReq) Validate() error {
	if len(c.ApplyOrders) == 0 || len(c.ApplyOrders) > constant.BatchOperationMaxLimit {
		return errf.Newf(errf.InvalidParameter,
			"apply_orders count should between 1 and %d", constant.BatchOperationMaxLimit)
	}
	for _, item := range c.ApplyOrders {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return validator.Validate.Struct(c)
}

// ZiyanCvmApplyOrderUpdateReq update request
type ZiyanCvmApplyOrderUpdateReq struct {
	OrderID      uint64                `json:"order_id" validate:"required"`
	ProductType  enumor.ProductType    `json:"product_type" validate:"omitempty"`
	ItsmTicketID string                `json:"itsm_ticket_id" validate:"omitempty,max=64"`
	Stage        tasktypes.TicketStage `json:"stage" validate:"omitempty,max=32"`
	BkBizID      int64                 `json:"bk_biz_id"`
	BkUsername   string                `json:"bk_username" validate:"omitempty,max=64"`
	Follower     types.JsonField       `json:"follower" validate:"omitempty"`
	EnableNotice *bool                 `json:"enable_notice" validate:"omitempty"`
	RequireType  enumor.RequireType    `json:"require_type" validate:"omitempty"`
	ExpectTime   *string               `json:"expect_time" validate:"omitempty"`
	Remark       string                `json:"remark" validate:"omitempty,max=255"`
	Suborders    []*tasktypes.Suborder `json:"suborders" validate:"omitempty"`
	OldSuborders []*tasktypes.Suborder `json:"old_suborders" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmApplyOrderUpdateReq) Validate() error {
	return validator.Validate.Struct(req)
}
