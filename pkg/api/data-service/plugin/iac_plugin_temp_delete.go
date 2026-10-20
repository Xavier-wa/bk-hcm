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

package dataplugin

import (
	"hcm/pkg/api/core"
	coreplugin "hcm/pkg/api/core/plugin"
	"hcm/pkg/criteria/validator"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/rest"
)

// TODO(temp_delete): 临时 iac_plugin_temp_delete API 结构，合并注册分支后删除。

// IacPluginTempDeleteCreateReq 创建 iac_plugin_temp_delete 请求。
type IacPluginTempDeleteCreateReq struct {
	PluginName    string          `json:"plugin_name" validate:"required,lte=64"`
	PluginVersion string          `json:"plugin_version" validate:"required,lte=64"`
	Protocol      string          `json:"protocol" validate:"required,lte=32"`
	State         string          `json:"state" validate:"required,lte=32"`
	Message       string          `json:"message" validate:"lte=255"`
	Extension     types.JsonField `json:"extension" validate:"required"`
	Memo          *string         `json:"memo"`
}

// Validate 校验 IacPluginTempDeleteCreateReq。
func (req *IacPluginTempDeleteCreateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	if err := validator.ValidateMemo(req.Memo, false); err != nil {
		return err
	}

	return nil
}

// IacPluginTempDeleteUpdateReq 更新 iac_plugin_temp_delete 请求。
type IacPluginTempDeleteUpdateReq struct {
	ID      string  `json:"id" validate:"required,lte=64"`
	State   string  `json:"state" validate:"omitempty,lte=32"`
	Message *string `json:"message" validate:"omitempty,lte=255"`
	Memo    *string `json:"memo"`
}

// Validate 校验 IacPluginTempDeleteUpdateReq。
func (req *IacPluginTempDeleteUpdateReq) Validate() error {
	if err := validator.Validate.Struct(req); err != nil {
		return err
	}

	if err := validator.ValidateMemo(req.Memo, false); err != nil {
		return err
	}

	return nil
}

// IacPluginTempDeleteListReq 列表查询 iac_plugin_temp_delete 请求。
type IacPluginTempDeleteListReq = core.ListReq

// IacPluginTempDeleteListResult iac_plugin_temp_delete 列表结果数据。
type IacPluginTempDeleteListResult = core.ListResultT[coreplugin.IacPluginTempDelete]

// IacPluginTempDeleteListResp 列表查询 iac_plugin_temp_delete 响应。
type IacPluginTempDeleteListResp struct {
	rest.BaseResp `json:",inline"`
	Data          *IacPluginTempDeleteListResult `json:"data"`
}
