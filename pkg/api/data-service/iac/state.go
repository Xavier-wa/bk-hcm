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
	"hcm/pkg/dal/table/types"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
)

// IacStateBatchCreateReq 批量创建 iac_state 请求。
type IacStateBatchCreateReq struct {
	Items []IacStateCreateField `json:"items" validate:"required,min=1"`
}

// Validate 校验 IacStateBatchCreateReq。
func (req *IacStateBatchCreateReq) Validate() error {
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

// IacStateCreateField 单条 iac_state 创建字段。
type IacStateCreateField struct {
	RepositoryID     string              `json:"repository_id" validate:"required,lte=64"`
	BlockID          string              `json:"block_id" validate:"required,lte=512"`
	Kind             enumor.IacStateKind `json:"kind" validate:"required,lte=32"`
	Current          types.JsonField     `json:"current,omitempty"`
	Deposed          types.JsonField     `json:"deposed,omitempty"`
	LastFlowID       string              `json:"last_flow_id" validate:"omitempty,lte=64"`
	ImportFromSpecID string              `json:"import_from_spec_id" validate:"omitempty,lte=64"`
}

// Validate 校验 IacStateCreateField。
func (req *IacStateCreateField) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	return req.Kind.Validate()
}

// IacStateBatchUpdateReq 批量更新 iac_state 请求。
type IacStateBatchUpdateReq struct {
	Items []IacStateUpdateField `json:"items" validate:"required,min=1"`
}

// Validate 校验 IacStateBatchUpdateReq。
func (req *IacStateBatchUpdateReq) Validate() error {
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

// IacStateUpdateField 单条 iac_state 更新字段。
// Current 为空字符串表示跳过更新，为 "null" 表示清空为 SQL NULL，为 JSON 对象表示覆盖。
type IacStateUpdateField struct {
	ID               string              `json:"id" validate:"required,lte=64"`
	BlockID          string              `json:"block_id" validate:"omitempty,lte=512"`
	Kind             enumor.IacStateKind `json:"kind" validate:"omitempty,lte=32"`
	Current          types.JsonField     `json:"current,omitempty"`
	Deposed          types.JsonField     `json:"deposed,omitempty"`
	LastFlowID       string              `json:"last_flow_id" validate:"omitempty,lte=64"`
	ImportFromSpecID *string             `json:"import_from_spec_id"`
}

// Validate 校验 IacStateUpdateField。
func (req *IacStateUpdateField) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if req.BlockID == "" && req.Kind == "" && req.Current == "" &&
		req.Deposed == "" && req.LastFlowID == "" && req.ImportFromSpecID == nil {
		return errors.New("at least one updatable field must be set")
	}
	if req.Kind != "" {
		if err := req.Kind.Validate(); err != nil {
			return err
		}
	}
	if req.ImportFromSpecID != nil && len(*req.ImportFromSpecID) > 64 {
		return errors.New("import_from_spec_id length must be <= 64")
	}
	return nil
}

// IacStateBatchDeleteReq 批量删除 iac_state 请求。
type IacStateBatchDeleteReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
}

// Validate 校验 IacStateBatchDeleteReq。
func (req *IacStateBatchDeleteReq) Validate() error {
	return validator.Validate.Struct(req)
}

// IacStateListReq 列表查询 iac_state 请求。
type IacStateListReq = core.ListReq

// IacStateListResult iac_state 列表结果。
type IacStateListResult = core.ListResultT[coreiac.IacState]

// IacStateListResp 列表查询 iac_state 响应。
type IacStateListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *IacStateListResult `json:"data"`
}
