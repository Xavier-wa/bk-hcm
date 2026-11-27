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

package dataiac

import (
	"errors"
	"fmt"

	"hcm/pkg/api/core"
	coreiac "hcm/pkg/api/core/iac"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
)

// IacSpecBatchCreateReq 批量创建 iac_spec 请求。
type IacSpecBatchCreateReq struct {
	Items []IacSpecCreateField `json:"items" validate:"required,min=1"`
}

// Validate 校验 IacSpecBatchCreateReq。
func (req *IacSpecBatchCreateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if len(req.Items) > constant.BatchOperationMaxLimit {
		return fmt.Errorf("items count exceeds max limit %d", constant.BatchOperationMaxLimit)
	}
	for i := range req.Items {
		if err := req.Items[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// IacSpecCreateField 单条 iac_spec 创建字段。
type IacSpecCreateField struct {
	BkBizID      int64                `json:"bk_biz_id" validate:"required,gt=0"`
	RepositoryID string               `json:"repository_id" validate:"required,lte=64"`
	Version      string               `json:"version" validate:"required,lte=64"`
	Status       enumor.IacSpecStatus `json:"status" validate:"required,lte=32"`
	LastRunID    string               `json:"last_run_id" validate:"omitempty,lte=64"`
	Memo         *string              `json:"memo"`
}

// Validate 校验 IacSpecCreateField。
func (req *IacSpecCreateField) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if err := req.Status.Validate(); err != nil {
		return err
	}
	return validator.ValidateMemo(req.Memo, false)
}

// IacSpecBatchUpdateReq 批量更新 iac_spec 请求。
type IacSpecBatchUpdateReq struct {
	Items []IacSpecUpdateField `json:"items" validate:"required,min=1"`
}

// Validate 校验 IacSpecBatchUpdateReq。
func (req *IacSpecBatchUpdateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if len(req.Items) > constant.BatchOperationMaxLimit {
		return fmt.Errorf("items count exceeds max limit %d", constant.BatchOperationMaxLimit)
	}
	for i := range req.Items {
		if err := req.Items[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// IacSpecUpdateField 单条 iac_spec 更新字段。
type IacSpecUpdateField struct {
	ID        string               `json:"id" validate:"required,lte=64"`
	Status    enumor.IacSpecStatus `json:"status" validate:"omitempty,lte=32"`
	LastRunID string               `json:"last_run_id" validate:"omitempty,lte=64"`
	Memo      *string              `json:"memo"`
}

// Validate 校验 IacSpecUpdateField。
func (req *IacSpecUpdateField) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if req.Status == "" && req.LastRunID == "" && req.Memo == nil {
		return errors.New("at least one of status, last_run_id, memo must be set")
	}
	if req.Status != "" {
		if err := req.Status.Validate(); err != nil {
			return err
		}
	}
	return validator.ValidateMemo(req.Memo, false)
}

// IacSpecBatchDeleteReq 批量删除 iac_spec 请求。
type IacSpecBatchDeleteReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
}

// Validate 校验 IacSpecBatchDeleteReq。
func (req *IacSpecBatchDeleteReq) Validate() error {
	return validator.Validate.Struct(req)
}

// IacSpecListReq 列表查询 iac_spec 请求。
type IacSpecListReq = core.ListReq

// IacSpecListResult iac_spec 列表结果。
type IacSpecListResult = core.ListResultT[coreiac.IacSpec]

// IacSpecListResp 列表查询 iac_spec 响应。
type IacSpecListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *IacSpecListResult `json:"data"`
}
