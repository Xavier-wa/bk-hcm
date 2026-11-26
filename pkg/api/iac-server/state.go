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

// Package iacproto 定义 iac-server 对外 REST API 的请求与响应结构体。
package iacproto

import (
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/runtime/filter"
)

// ListStateReq 查询仓库下受管资源状态摘要列表请求。
type ListStateReq struct {
	Filter *filter.Expression `json:"filter"`
	Page   *core.BasePage     `json:"page" validate:"required"`
}

// Validate 校验 ListStateReq。
func (req *ListStateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}
	return req.Page.Validate(core.NewDefaultPageOption())
}

// ListStateResp 查询仓库下受管资源状态摘要列表响应数据。
type ListStateResp struct {
	Count   uint64          `json:"count,omitempty"`
	Details []StateListItem `json:"details,omitempty"`
}

// StateListItem 受管资源状态列表摘要条目，不含敏感属性载荷。
type StateListItem struct {
	ID               string              `json:"id"`
	BlockID          string              `json:"block_id"`
	PluginInstanceID string              `json:"plugin_instance_id"`
	Kind             enumor.IacStateKind `json:"kind"`
	Plugin           string              `json:"plugin"`
	Type             string              `json:"type"`
	HasCurrent       bool                `json:"has_current"`
	DeposedCount     int                 `json:"deposed_count"`
	LastFlowID       string              `json:"last_flow_id"`
	UpdatedAt        string              `json:"updated_at"`
}

// GetStateResp 查询单条受管资源状态脱敏详情响应数据。
type GetStateResp struct {
	ID               string                        `json:"id"`
	RepositoryID     string                        `json:"repository_id"`
	BlockID          string                        `json:"block_id"`
	PluginInstanceID string                        `json:"plugin_instance_id"`
	Kind             enumor.IacStateKind           `json:"kind"`
	Current          *MaskedStateObject            `json:"current"`
	Deposed          map[string]*MaskedStateObject `json:"deposed"`
	LastFlowID       string                        `json:"last_flow_id"`
	CreatedAt        string                        `json:"created_at"`
	UpdatedAt        string                        `json:"updated_at"`
}

// MaskedStateObject 脱敏后的单个实例状态对象，不包含内部 private 字段，provider_config 原样保留。
type MaskedStateObject struct {
	Plugin         string         `json:"plugin"`
	Type           string         `json:"type"`
	SchemaVersion  int64          `json:"schema_version"`
	State          map[string]any `json:"state"`
	ProviderConfig map[string]any `json:"provider_config,omitempty"`
}
