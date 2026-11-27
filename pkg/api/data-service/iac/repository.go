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

// Package dataiac 定义 data-service IaC 仓库、Spec、节点与 State 接口的请求和响应结构。
package dataiac

import (
	"errors"
	"fmt"

	"hcm/pkg/api/core"
	coreiac "hcm/pkg/api/core/iac"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
)

// IacRepositoryBatchCreateReq 批量创建 iac_repository 请求。
type IacRepositoryBatchCreateReq struct {
	Items []IacRepositoryCreateField `json:"items" validate:"required,min=1"`
}

// Validate 校验 IacRepositoryBatchCreateReq。
func (req *IacRepositoryBatchCreateReq) Validate() error {
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

// IacRepositoryCreateField 单条 iac_repository 创建字段。
type IacRepositoryCreateField struct {
	BkBizID         int64   `json:"bk_biz_id" validate:"required,gt=0"`
	Name            string  `json:"name" validate:"required,lte=128"`
	LatestVersion   string  `json:"latest_version" validate:"omitempty,lte=64"`
	LastExecVersion string  `json:"last_exec_version" validate:"omitempty,lte=64"`
	Memo            *string `json:"memo"`
}

// Validate 校验 IacRepositoryCreateField。
func (req *IacRepositoryCreateField) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	return validator.ValidateMemo(req.Memo, false)
}

// IacRepositoryBatchUpdateReq 批量更新 iac_repository 请求。
type IacRepositoryBatchUpdateReq struct {
	Items []IacRepositoryUpdateField `json:"items" validate:"required,min=1"`
}

// Validate 校验 IacRepositoryBatchUpdateReq。
func (req *IacRepositoryBatchUpdateReq) Validate() error {
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

// IacRepositoryUpdateField 单条 iac_repository 更新字段。
type IacRepositoryUpdateField struct {
	ID              string  `json:"id" validate:"required,lte=64"`
	Name            string  `json:"name" validate:"omitempty,lte=128"`
	LatestVersion   string  `json:"latest_version" validate:"omitempty,lte=64"`
	LastExecVersion string  `json:"last_exec_version" validate:"omitempty,lte=64"`
	Memo            *string `json:"memo"`
}

// Validate 校验 IacRepositoryUpdateField。
func (req *IacRepositoryUpdateField) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if req.Name == "" && req.LatestVersion == "" && req.LastExecVersion == "" && req.Memo == nil {
		return errors.New("at least one of name, latest_version, last_exec_version, memo must be set")
	}
	return validator.ValidateMemo(req.Memo, false)
}

// IacRepositoryBatchDeleteReq 批量删除 iac_repository 请求。
type IacRepositoryBatchDeleteReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
}

// Validate 校验 IacRepositoryBatchDeleteReq。
func (req *IacRepositoryBatchDeleteReq) Validate() error {
	return validator.Validate.Struct(req)
}

// IacRepositoryListReq 列表查询 iac_repository 请求。
type IacRepositoryListReq = core.ListReq

// IacRepositoryListResult iac_repository 列表结果。
type IacRepositoryListResult = core.ListResultT[coreiac.IacRepository]

// IacRepositoryListResp 列表查询 iac_repository 响应。
type IacRepositoryListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *IacRepositoryListResult `json:"data"`
}
