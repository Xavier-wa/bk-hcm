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

// Package iacplugin 提供 hc-service IaC 插件内部接口客户端。
package iacplugin

import (
	"hcm/pkg/api/core"
	coreplugin "hcm/pkg/api/core/plugin"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
)

// Client hc-service IaC 插件接口客户端。
type Client struct {
	client rest.ClientInterface
}

// NewClient 创建 hc-service IaC 插件客户端。
func NewClient(client rest.ClientInterface) *Client {
	return &Client{client: client}
}

// GetSchema 调用 GET /iac_plugins/schema 查询指定插件及版本的完整 Schema。
func (c *Client) GetSchema(kt *kit.Kit, pluginName, pluginVersion string) (*coreplugin.PluginSchema, error) {
	params := map[string]string{"plugin": pluginName}
	if pluginVersion != "" {
		params["plugin_version"] = pluginVersion
	}
	resp := new(core.BaseResp[*coreplugin.PluginSchema])
	err := c.client.Verb(rest.GET).
		WithContext(kt.Ctx).
		SubResourcef("/iac_plugins/schema").
		WithParams(params).
		WithHeaders(kt.Header()).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}
	return resp.Data, nil
}
