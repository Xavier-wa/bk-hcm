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

// Package iacstate 提供无状态的 IaC 受管资源状态读写封装，供编排与执行引擎复用。
package iacstate

import (
	"errors"
	"strings"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
)

// StateRecord 对应 iac_state 单行受管实例状态记录。
type StateRecord struct {
	ID               string                  `json:"id"`
	RepositoryID     string                  `json:"repository_id"`
	BlockID          string                  `json:"block_id"`
	PluginInstanceID string                  `json:"plugin_instance_id"`
	Kind             enumor.IacStateKind     `json:"kind"`
	Current          *StateObject            `json:"current"`
	Deposed          map[string]*StateObject `json:"deposed"`
	LastFlowID       string                  `json:"last_flow_id"`
	ImportFromSpecID string                  `json:"import_from_spec_id"`
}

// CurrentOrNil 返回当前活跃状态对象；当记录为空或 current 为空时返回 nil。
func (r *StateRecord) CurrentOrNil() *StateObject {
	if r == nil {
		return nil
	}
	return r.Current
}

// StateObject 对应单个实例状态对象（current 字段或 deposed 中的单个元素）。
type StateObject struct {
	Plugin         string         `json:"plugin"`
	Type           string         `json:"type"`
	SchemaVersion  int64          `json:"schema_version"`
	State          map[string]any `json:"state"`
	Private        string         `json:"private,omitempty"`
	ProviderConfig map[string]any `json:"provider_config,omitempty"`
}

// UpsertAppliedReq 执行阶段写回资源或数据源实例状态的请求参数。
type UpsertAppliedReq struct {
	RepositoryID     string              `json:"repository_id" validate:"required"`
	BlockID          string              `json:"block_id" validate:"required"`
	PluginInstanceID string              `json:"plugin_instance_id"`
	Kind             enumor.IacStateKind `json:"kind"`
	Object           *StateObject        `json:"object" validate:"required"`
	DeposedKey       string              `json:"deposed_key,omitempty"`
	FlowID           string              `json:"flow_id"`
}

// Validate 校验 UpsertAppliedReq 参数。
func (req *UpsertAppliedReq) Validate() error {
	if req == nil {
		return errors.New("upsert applied state request is nil")
	}
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	if strings.HasPrefix(req.BlockID, constant.IacStateImportBlockPrefix) {
		if len(strings.TrimPrefix(req.BlockID, constant.IacStateImportBlockPrefix)) == 0 {
			return errors.New("import block_id missing target resource address")
		}
		return nil
	}
	return req.Kind.Validate()
}

// RemoveDestroyedReq 执行阶段销毁资源实例状态的请求参数。
type RemoveDestroyedReq struct {
	RepositoryID string `json:"repository_id" validate:"required"`
	BlockID      string `json:"block_id" validate:"required"`
	DeposedKey   string `json:"deposed_key,omitempty"`
	FlowID       string `json:"flow_id"`
}

// Validate 校验 RemoveDestroyedReq 参数。
func (req *RemoveDestroyedReq) Validate() error {
	if req == nil {
		return errors.New("remove destroyed state request is nil")
	}
	return validator.Validate.Struct(req)
}
