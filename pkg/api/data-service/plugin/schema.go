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

// Package dataplugin 定义 IaC 插件与插件 Schema 的 data-service API 请求/响应结构。
package dataplugin

import (
	"hcm/pkg/api/core"
	coreplugin "hcm/pkg/api/core/plugin"
	"hcm/pkg/criteria/validator"
	tableplugin "hcm/pkg/dal/table/plugin"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/rest"
)

// IacPluginSchemaCreateReq 创建 iac_plugin_schema 请求。
type IacPluginSchemaCreateReq struct {
	PluginName    string          `json:"plugin_name" validate:"required,lte=64"`
	PluginVersion string          `json:"plugin_version" validate:"required,lte=64"`
	SchemaHash    string          `json:"schema_hash" validate:"required,lte=80"`
	Schema        types.JsonField `json:"schema" validate:"required"`
}

// Validate 校验 IacPluginSchemaCreateReq。
func (req *IacPluginSchemaCreateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	if err := coreplugin.ValidateSchemaHashFormat(req.SchemaHash); err != nil {
		return err
	}

	return nil
}

// IacPluginSchemaListReq 列表查询 iac_plugin_schema 请求。
type IacPluginSchemaListReq = core.ListReq

// IacPluginSchemaListResult iac_plugin_schema 列表结果数据。
type IacPluginSchemaListResult = core.ListResultT[tableplugin.IacPluginSchemaTable]

// IacPluginSchemaListResp 列表查询 iac_plugin_schema 响应。
type IacPluginSchemaListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *IacPluginSchemaListResult `json:"data"`
}
