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

// IacSpecNodeBatchCreateReq 批量创建 iac_spec_node 请求。
type IacSpecNodeBatchCreateReq struct {
	Items []IacSpecNodeCreateField `json:"items" validate:"required,min=1"`
}

// Validate 校验 IacSpecNodeBatchCreateReq。
func (req *IacSpecNodeBatchCreateReq) Validate() error {
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

// IacSpecNodeCreateField 单条 iac_spec_node 创建字段。
type IacSpecNodeCreateField struct {
	RepositoryID    string                 `json:"repository_id" validate:"required,lte=64"`
	SpecID          string                 `json:"spec_id" validate:"required,lte=64"`
	ParentID        string                 `json:"parent_id" validate:"omitempty,lte=64"`
	Level           int                    `json:"level" validate:"gte=0"`
	NodeType        enumor.IacSpecNodeType `json:"node_type" validate:"required,lte=16"`
	NodeName        string                 `json:"node_name" validate:"required,lte=256"`
	ChildrenNodeIDs *types.JsonField       `json:"children_node_ids"`
	FileSize        int64                  `json:"file_size" validate:"gte=0"`
	Content         *string                `json:"content"`
}

// Validate 校验 IacSpecNodeCreateField。
func (req *IacSpecNodeCreateField) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if err := req.NodeType.Validate(); err != nil {
		return err
	}
	if req.NodeType == enumor.IacSpecNodeTypeDir && req.Content != nil {
		return errors.New("dir node content must be nil")
	}
	if req.NodeType == enumor.IacSpecNodeTypeFile && req.ChildrenNodeIDs != nil {
		return errors.New("file node children_node_ids must be nil")
	}
	return nil
}

// IacSpecNodeBatchUpdateReq 批量更新 iac_spec_node 请求。
type IacSpecNodeBatchUpdateReq struct {
	Items []IacSpecNodeUpdateField `json:"items" validate:"required,min=1"`
}

// Validate 校验 IacSpecNodeBatchUpdateReq。
func (req *IacSpecNodeBatchUpdateReq) Validate() error {
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

// IacSpecNodeUpdateField 单条 iac_spec_node 更新字段。
type IacSpecNodeUpdateField struct {
	ID              string           `json:"id" validate:"required,lte=64"`
	ParentID        string           `json:"parent_id" validate:"omitempty,lte=64"`
	NodeName        string           `json:"node_name" validate:"omitempty,lte=256"`
	ChildrenNodeIDs *types.JsonField `json:"children_node_ids"`
	FileSize        int64            `json:"file_size" validate:"gte=0"`
	Content         *string          `json:"content"`
}

// Validate 校验 IacSpecNodeUpdateField。
func (req *IacSpecNodeUpdateField) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if req.ParentID == "" && req.NodeName == "" && req.ChildrenNodeIDs == nil && req.FileSize == 0 &&
		req.Content == nil {
		return errors.New("at least one updatable field must be set")
	}
	return nil
}

// IacSpecNodeBatchDeleteReq 批量删除 iac_spec_node 请求。
type IacSpecNodeBatchDeleteReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
}

// Validate 校验 IacSpecNodeBatchDeleteReq。
func (req *IacSpecNodeBatchDeleteReq) Validate() error {
	return validator.Validate.Struct(req)
}

// IacSpecNodeListReq 列表查询 iac_spec_node 请求。
type IacSpecNodeListReq = core.ListReq

// IacSpecNodeListResult iac_spec_node 列表结果。
type IacSpecNodeListResult = core.ListResultT[coreiac.IacSpecNode]

// IacSpecNodeListResp 列表查询 iac_spec_node 响应。
type IacSpecNodeListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *IacSpecNodeListResult `json:"data"`
}
