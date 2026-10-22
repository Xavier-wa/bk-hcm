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

// Package dataplugin 定义 data-service 插件相关接口的请求与响应结构。
package dataplugin

import (
	"errors"

	"hcm/pkg/api/core"
	coreplugin "hcm/pkg/api/core/plugin"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
)

// IacPluginInstanceCreateReq 创建 iac_plugin_instance 请求。
type IacPluginInstanceCreateReq struct {
	PluginName  string         `json:"plugin_name" validate:"required,lte=64"`
	Name        string         `json:"name" validate:"required,lte=128"`
	Attrs       map[string]any `json:"attrs"`
	SecretAttrs map[string]any `json:"secret_attrs"`
	Extension   map[string]any `json:"extension"`
	Memo        *string        `json:"memo"`
}

// Validate 校验 IacPluginInstanceCreateReq。
func (req *IacPluginInstanceCreateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	if err := validator.ValidateMemo(req.Memo, false); err != nil {
		return err
	}

	return nil
}

// IacPluginInstanceUpdateReq 更新 iac_plugin_instance 请求。
type IacPluginInstanceUpdateReq struct {
	ID          string         `json:"id" validate:"required,lte=64"`
	Name        string         `json:"name" validate:"omitempty,lte=128"`
	Attrs       map[string]any `json:"attrs"`
	SecretAttrs map[string]any `json:"secret_attrs"`
	Extension   map[string]any `json:"extension"`
	Memo        *string        `json:"memo"`
}

// Validate 校验 IacPluginInstanceUpdateReq。
func (req *IacPluginInstanceUpdateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	if req.Name == "" && req.Attrs == nil && req.SecretAttrs == nil && req.Extension == nil && req.Memo == nil {
		return errors.New("at least one of name, attrs, secret_attrs, extension, memo must be set")
	}

	if err := validator.ValidateMemo(req.Memo, false); err != nil {
		return err
	}

	return nil
}

// IacPluginInstanceDeleteReq 删除 iac_plugin_instance 请求。
type IacPluginInstanceDeleteReq struct {
	Filter *filter.Expression `json:"filter" validate:"required"`
}

// Validate 校验 IacPluginInstanceDeleteReq。
func (req *IacPluginInstanceDeleteReq) Validate() error {
	return validator.Validate.Struct(req)
}

// IacPluginInstanceListReq 列表查询 iac_plugin_instance 请求。
type IacPluginInstanceListReq = core.ListReq

// IacPluginInstanceListResult iac_plugin_instance 列表结果数据。
type IacPluginInstanceListResult = core.ListResultT[coreplugin.IacPluginInstance]

// IacPluginInstanceListResp 列表查询 iac_plugin_instance 响应。
type IacPluginInstanceListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *IacPluginInstanceListResult `json:"data"`
}

// IacPluginInstanceGetResp 查询单条 iac_plugin_instance 响应。
type IacPluginInstanceGetResp struct {
	rest.BaseResp `json:",inline"`
	Data          *coreplugin.IacPluginInstance `json:"data"`
}
