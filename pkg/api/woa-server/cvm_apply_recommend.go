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

package woaserver

import (
	"errors"
	"fmt"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/thirdparty/cvmapi"
)

// ApplyRecommendTopReq is the request for getting top apply recommendations.
type ApplyRecommendTopReq struct {
	BkUsername string `json:"bk_username" validate:"required,max=64"`
	Limit      int    `json:"limit" validate:"required,min=1,max=20"`
}

// Validate ...
func (r *ApplyRecommendTopReq) Validate() error {
	return validator.Validate.Struct(r)
}

// ApplyRecommendTopResp is the response for getting top apply recommendations.
type ApplyRecommendTopResp struct {
	Items []*ApplyRecommendTopElem `json:"items"`
}

// ApplyRecommendTopElem is an element in the top recommendations list.
type ApplyRecommendTopElem struct {
	RequireType enumor.RequireType `json:"require_type"`
	Region      string             `json:"region"`
	DeviceType  string             `json:"device_type"`
	ImageID     string             `json:"image_id"`
	Count       int                `json:"count"`
	// Source indicates where the recommendation comes from: "user" or "biz".
	Source enumor.ApplyRecommendSource `json:"source"`
}

// ApplyRecommendByStaticReq is the request for static-recommend-based online recommendation.
type ApplyRecommendByStaticReq struct {
	BkUsername string `json:"bk_username" validate:"required,max=64"`
	Limit      int    `json:"limit" validate:"required,min=1,max=20"`

	RequireType *enumor.RequireType `json:"require_type,omitempty"`
	Region      string              `json:"region,omitempty" validate:"omitempty,max=128"`
	DeviceType  string              `json:"device_type,omitempty" validate:"omitempty,max=64"`
	ImageID     string              `json:"image_id,omitempty" validate:"omitempty,max=64"`
	Zone        string              `json:"zone,omitempty" validate:"omitempty,max=64"`
	ResAssign   *enumor.ResAssign   `json:"res_assign,omitempty"`
	// Replicas 申请数量，未传则取 cc.ApplyRecommend.DefaultApplyNum
	Replicas *int `json:"replicas,omitempty" validate:"omitempty,min=1"`
}

// Validate ApplyRecommendByStaticReq.
func (r *ApplyRecommendByStaticReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	if r.RequireType != nil {
		if err := r.RequireType.Validate(); err != nil {
			return err
		}
	}
	if r.ResAssign != nil {
		if err := r.ResAssign.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ApplyRecommendByPlanReq is the request for forecast-remain-based online recommendation.
type ApplyRecommendByPlanReq struct {
	Limit int `json:"limit" validate:"required,min=1,max=20"`

	RequireType *enumor.RequireType `json:"require_type,omitempty"`
	Region      string              `json:"region,omitempty" validate:"omitempty,max=128"`
	DeviceType  string              `json:"device_type,omitempty" validate:"omitempty,max=64"`
	ImageID     string              `json:"image_id,omitempty" validate:"omitempty,max=64"`
	Zone        string              `json:"zone,omitempty" validate:"omitempty,max=64"`
	ResAssign   *enumor.ResAssign   `json:"res_assign,omitempty"`
	// Replicas 申请数量，未传则取 cc.ApplyRecommend.DefaultApplyNum
	Replicas *int `json:"replicas,omitempty" validate:"omitempty,min=1"`
}

// Validate ApplyRecommendByPlanReq.
func (r *ApplyRecommendByPlanReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	if r.RequireType != nil {
		if err := r.RequireType.Validate(); err != nil {
			return err
		}
	}
	if r.ResAssign != nil {
		if err := r.ResAssign.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ApplyRecommendByStaticResp is the response for static-recommend-based online recommendation.
type ApplyRecommendByStaticResp struct {
	Items []*ApplyRecommendItem `json:"items"`
}

// ApplyRecommendItem is a recommendation plan that contains exactly one suborder.
type ApplyRecommendItem struct {
	Suborder *ApplyRecommendSuborder `json:"suborder"`
	// Source indicates where the recommendation comes from: "user" or "biz".
	Source enumor.ApplyRecommendSource `json:"source,omitempty"`
}

// ApplyRecommendSplitSubOrderReq is the request for splitting a confirmed plan into a main order with suborders.
type ApplyRecommendSplitSubOrderReq struct {
	RequireType enumor.RequireType `json:"require_type" validate:"required"`
	Region      string             `json:"region" validate:"required,max=128"`
	// Zone 可用区；"all" 表示全部，或具体可用区值。
	Zone       string            `json:"zone" validate:"required,max=64"`
	DeviceType string            `json:"device_type" validate:"required,max=64"`
	ImageID    string            `json:"image_id" validate:"required,max=64"`
	ResAssign  enumor.ResAssign  `json:"res_assign" validate:"required"`
	Replicas   int               `json:"replicas" validate:"required,min=1"`
	SystemDisk enumor.DiskSpec   `json:"system_disk" validate:"required"`
	DataDisk   []enumor.DiskSpec `json:"data_disk" validate:"omitempty,dive"`
	// OccupiedSuborders 已占用子单数组（可选），用于增量拆分：扣减已占用预测余量与库存后再计算增量子单。
	OccupiedSuborders []*ApplyRecommendSuborder `json:"occupied_suborders,omitempty"`
}

// Validate ApplyRecommendSplitSubOrderReq.
func (r *ApplyRecommendSplitSubOrderReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	if err := r.RequireType.Validate(); err != nil {
		return err
	}
	if err := r.ResAssign.Validate(); err != nil {
		return err
	}
	// 同批次约束：每个占用子单的 require_type 必须等于本次请求的 require_type。
	for _, sub := range r.OccupiedSuborders {
		if sub == nil {
			return errors.New("occupied suborder should not be nil")
		}
		if sub.RequireType != r.RequireType {
			return fmt.Errorf("occupied suborder require_type %d not equal to request require_type %d",
				sub.RequireType, r.RequireType)
		}
	}
	return nil
}

// ApplyRecommendSplitSubOrderResp is the response for the split-suborder trial calculation.
type ApplyRecommendSplitSubOrderResp struct {
	Suborders []*ApplyRecommendSuborder `json:"suborders"`
}

// ApplyRecommendSuborder is the single suborder carried by a recommendation plan.
type ApplyRecommendSuborder struct {
	RequireType enumor.RequireType `json:"require_type"`
	Region      string             `json:"region"`
	// Zone 可用区；未传 zone 时为全部("all")。
	Zone       string `json:"zone"`
	DeviceType string `json:"device_type"`
	ImageID    string `json:"image_id"`
	// ResAssign 资源分配方式；指定具体 zone 时为不适用（nil，不返回）。
	ResAssign  *enumor.ResAssign `json:"res_assign,omitempty"`
	Replicas   int               `json:"replicas"`
	ChargeType cvmapi.ChargeType `json:"charge_type"`
	SystemDisk enumor.DiskSpec   `json:"system_disk"`
	DataDisk   []enumor.DiskSpec `json:"data_disk"`
}
