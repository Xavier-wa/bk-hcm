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

// Package cvmapply ...
package cvmapply

import (
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
)

// ---- User Recommend ----

// BatchCreateZiyanCvmApplyUserRecommendReq batch create user recommend request.
type BatchCreateZiyanCvmApplyUserRecommendReq struct {
	Items []ZiyanCvmApplyUserRecommendCreateReq `json:"items" validate:"required,min=1,max=500"`
}

// Validate ...
func (r *BatchCreateZiyanCvmApplyUserRecommendReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	for _, item := range r.Items {
		if err := item.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// ZiyanCvmApplyUserRecommendCreateReq create request item.
type ZiyanCvmApplyUserRecommendCreateReq struct {
	BkBizID     int64              `json:"bk_biz_id" validate:"required"`
	BkUsername  string             `json:"bk_username" validate:"required,max=64"`
	RequireType enumor.RequireType `json:"require_type"`
	Region      string             `json:"region" validate:"max=128"`
	DeviceType  string             `json:"device_type" validate:"max=64"`
	ImageID     string             `json:"image_id" validate:"omitempty,max=64"`
	Count       int                `json:"count" validate:"required,min=1"`
}

// Validate ...
func (r *ZiyanCvmApplyUserRecommendCreateReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}

	if err := r.RequireType.Validate(); err != nil {
		return err
	}

	return nil
}

// ZiyanCvmApplyUserRecommendListReq list request.
type ZiyanCvmApplyUserRecommendListReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
	Page   *core.BasePage     `json:"page" validate:"required"`
	Fields []string           `json:"fields" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmApplyUserRecommendListReq) Validate() error {
	return validator.Validate.Struct(req)
}

// ZiyanCvmApplyUserRecommendListResult list result.
type ZiyanCvmApplyUserRecommendListResult = core.ListResultT[*cvmapplytable.ZiyanCvmApplyUserRecommend]

// ZiyanCvmApplyUserRecommendListResp list response.
type ZiyanCvmApplyUserRecommendListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *ZiyanCvmApplyUserRecommendListResult `json:"data"`
}

// BatchUpdateZiyanCvmApplyUserRecommendReq batch update request.
type BatchUpdateZiyanCvmApplyUserRecommendReq struct {
	Items []ZiyanCvmApplyUserRecommendUpdateReq `json:"items" validate:"required,min=1,max=500"`
}

// Validate ...
func (r *BatchUpdateZiyanCvmApplyUserRecommendReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	for _, item := range r.Items {
		if err := item.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// ZiyanCvmApplyUserRecommendUpdateReq update request item.
type ZiyanCvmApplyUserRecommendUpdateReq struct {
	ID          string              `json:"id" validate:"required"`
	RequireType *enumor.RequireType `json:"require_type" validate:"omitempty"`
	Region      string              `json:"region" validate:"omitempty,max=128"`
	DeviceType  string              `json:"device_type" validate:"omitempty,max=64"`
	ImageID     string              `json:"image_id" validate:"omitempty,max=64"`
	Count       int                 `json:"count" validate:"omitempty,min=1"`
}

// Validate ...
func (req *ZiyanCvmApplyUserRecommendUpdateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	if req.RequireType != nil {
		if err := req.RequireType.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// ---- Biz Recommend ----

// BatchCreateZiyanCvmApplyBizRecommendReq batch create biz recommend request.
type BatchCreateZiyanCvmApplyBizRecommendReq struct {
	Items []ZiyanCvmApplyBizRecommendCreateReq `json:"items" validate:"required,min=1,max=500"`
}

// Validate ...
func (r *BatchCreateZiyanCvmApplyBizRecommendReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	for _, item := range r.Items {
		if err := item.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// ZiyanCvmApplyBizRecommendCreateReq create request item.
type ZiyanCvmApplyBizRecommendCreateReq struct {
	BkBizID     int64              `json:"bk_biz_id" validate:"required"`
	RequireType enumor.RequireType `json:"require_type"`
	Region      string             `json:"region" validate:"max=128"`
	DeviceType  string             `json:"device_type" validate:"max=64"`
	ImageID     string             `json:"image_id" validate:"omitempty,max=64"`
	Count       int                `json:"count" validate:"required,min=1"`
}

// Validate ...
func (req *ZiyanCvmApplyBizRecommendCreateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	if err := req.RequireType.Validate(); err != nil {
		return err
	}

	return nil
}

// ZiyanCvmApplyBizRecommendListReq list request.
type ZiyanCvmApplyBizRecommendListReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
	Page   *core.BasePage     `json:"page" validate:"required"`
	Fields []string           `json:"fields" validate:"omitempty"`
}

// Validate ...
func (req *ZiyanCvmApplyBizRecommendListReq) Validate() error {
	return validator.Validate.Struct(req)
}

// ZiyanCvmApplyBizRecommendListResult list result.
type ZiyanCvmApplyBizRecommendListResult = core.ListResultT[*cvmapplytable.ZiyanCvmApplyBizRecommend]

// ZiyanCvmApplyBizRecommendListResp list response.
type ZiyanCvmApplyBizRecommendListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *ZiyanCvmApplyBizRecommendListResult `json:"data"`
}

// BatchUpdateZiyanCvmApplyBizRecommendReq batch update request.
type BatchUpdateZiyanCvmApplyBizRecommendReq struct {
	Items []ZiyanCvmApplyBizRecommendUpdateReq `json:"items" validate:"required,min=1,max=500"`
}

// Validate ...
func (r *BatchUpdateZiyanCvmApplyBizRecommendReq) Validate() error {
	if err := validator.Validate.Struct(r); err != nil {
		return err
	}
	for _, item := range r.Items {
		if err := item.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// ZiyanCvmApplyBizRecommendUpdateReq update request item.
type ZiyanCvmApplyBizRecommendUpdateReq struct {
	ID          string              `json:"id" validate:"required"`
	RequireType *enumor.RequireType `json:"require_type" validate:"omitempty"`
	Region      string              `json:"region" validate:"omitempty,max=128"`
	DeviceType  string              `json:"device_type" validate:"omitempty,max=64"`
	ImageID     string              `json:"image_id" validate:"omitempty,max=64"`
	Count       int                 `json:"count" validate:"omitempty,min=1"`
}

// Validate ...
func (req *ZiyanCvmApplyBizRecommendUpdateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	if req.RequireType != nil {
		if err := req.RequireType.Validate(); err != nil {
			return err
		}
	}

	return nil
}
