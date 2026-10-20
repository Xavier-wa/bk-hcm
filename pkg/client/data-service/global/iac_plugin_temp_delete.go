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

package global

import (
	"hcm/pkg/api/core"
	dataplugin "hcm/pkg/api/data-service/plugin"
	"hcm/pkg/client/common"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
)

// TODO(temp_delete): 临时 iac_plugin_temp_delete Global SDK Client，合并插件注册分支后删除本文件。

// IacPluginTempDeleteClient data-service iac_plugin_temp_delete API 客户端。
type IacPluginTempDeleteClient struct {
	client rest.ClientInterface
}

// NewIacPluginTempDeleteClient 创建 IacPluginTempDeleteClient。
func NewIacPluginTempDeleteClient(client rest.ClientInterface) *IacPluginTempDeleteClient {
	return &IacPluginTempDeleteClient{
		client: client,
	}
}

// Create 创建 iac_plugin_temp_delete 记录。
func (c *IacPluginTempDeleteClient) Create(kt *kit.Kit, req *dataplugin.IacPluginTempDeleteCreateReq) (
	*core.CreateResult, error) {

	return common.Request[dataplugin.IacPluginTempDeleteCreateReq, core.CreateResult](
		c.client, rest.POST, kt, req, "/iac_plugins_temp_delete/create")
}

// Update 更新 iac_plugin_temp_delete 记录。
func (c *IacPluginTempDeleteClient) Update(kt *kit.Kit, req *dataplugin.IacPluginTempDeleteUpdateReq) error {
	return common.RequestNoResp[dataplugin.IacPluginTempDeleteUpdateReq](
		c.client, rest.PATCH, kt, req, "/iac_plugins_temp_delete/update")
}

// List 列表查询 iac_plugin_temp_delete 记录。
func (c *IacPluginTempDeleteClient) List(kt *kit.Kit, req *dataplugin.IacPluginTempDeleteListReq) (
	*dataplugin.IacPluginTempDeleteListResult, error) {

	return common.Request[dataplugin.IacPluginTempDeleteListReq, dataplugin.IacPluginTempDeleteListResult](
		c.client, rest.POST, kt, req, "/iac_plugins_temp_delete/list")
}
