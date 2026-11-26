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

// Package iacplugin 定义 hc-service IaC 插件内部查询接口的请求与响应结构。
package iacplugin

import (
	coreplugin "hcm/pkg/api/core/plugin"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/rest"
)

// GetSchemaReq 查询插件 Schema 请求。
type GetSchemaReq struct {
	Plugin        string `json:"plugin" validate:"required,lte=64"`
	PluginVersion string `json:"plugin_version" validate:"required,lte=64"`
}

// Validate 校验 GetSchemaReq。
func (req *GetSchemaReq) Validate() error {
	return validator.Validate.Struct(req)
}

// GetSchemaResp 查询插件 Schema 响应。
type GetSchemaResp struct {
	rest.BaseResp `json:",inline"`
	Data          *coreplugin.PluginSchema `json:"data"`
}
