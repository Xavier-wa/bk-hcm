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

// Package iacplugin hc-service iac plugin api client.
package iacplugin

import (
	"hcm/pkg/api/core"
	coreplugin "hcm/pkg/api/core/plugin"
	proto "hcm/pkg/api/hc-service/iac-plugin"
	"hcm/pkg/client/common"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
)

// Client is hc-service iac plugin api client.
type Client struct {
	client rest.ClientInterface
}

// NewClient create a new hc-service iac plugin api client.
func NewClient(client rest.ClientInterface) *Client {
	return &Client{client: client}
}

// RegisterHTTP 注册 http 插件。
func (c *Client) RegisterHTTP(kt *kit.Kit, req *proto.HTTPRegisterReq) (*proto.HTTPPluginResult, error) {
	return common.Request[proto.HTTPRegisterReq, proto.HTTPPluginResult](c.client, rest.POST, kt, req,
		"/iac_plugins/http/register")
}

// UpgradeHTTP 升级 http 插件，按插件名作用于版本最高的记录。
func (c *Client) UpgradeHTTP(kt *kit.Kit, req *proto.HTTPUpgradeReq) (*proto.HTTPPluginResult, error) {
	return common.Request[proto.HTTPUpgradeReq, proto.HTTPPluginResult](c.client, rest.POST, kt, req,
		"/iac_plugins/http/upgrade")
}

// UpdateHTTP 修改 http 插件，按插件名作用于版本最高的记录。
func (c *Client) UpdateHTTP(kt *kit.Kit, req *proto.HTTPUpdateReq) (*proto.HTTPPluginResult, error) {
	return common.Request[proto.HTTPUpdateReq, proto.HTTPPluginResult](c.client, rest.POST, kt, req,
		"/iac_plugins/http/update")
}

// Health calls GET /iac_plugins/health to probe plugin liveness and return plugin_version.
func (c *Client) Health(kt *kit.Kit, pluginName string) (*proto.HealthResult, error) {
	params := map[string]string{"plugin": pluginName}
	return requestGetWithParams[proto.HealthResult](c.client, kt, "/iac_plugins/health", params)
}

// GetSchema calls GET /iac_plugins/schema to retrieve plugin schema from cache/DB.
func (c *Client) GetSchema(kt *kit.Kit, pluginName, pluginVersion string) (*coreplugin.PluginSchema, error) {
	params := map[string]string{"plugin": pluginName}
	if pluginVersion != "" {
		params["plugin_version"] = pluginVersion
	}
	return requestGetWithParams[coreplugin.PluginSchema](c.client, kt, "/iac_plugins/schema", params)
}

// Execute calls POST /iac_plugins/execute to create, update, or delete a single resource.
func (c *Client) Execute(kt *kit.Kit, req *proto.ExecuteReq) (*proto.ExecuteResult, error) {
	return common.Request[proto.ExecuteReq, proto.ExecuteResult](
		c.client, rest.POST, kt, req, "/iac_plugins/execute")
}

// GetResult calls POST /iac_plugins/result to query the current execution result of an async Execute.
func (c *Client) GetResult(kt *kit.Kit, req *proto.GetResultReq) (*proto.ExecuteResult, error) {
	return common.Request[proto.GetResultReq, proto.ExecuteResult](
		c.client, rest.POST, kt, req, "/iac_plugins/result")
}

// Stop calls POST /iac_plugins/stop to stop an in-progress async Execute.
func (c *Client) Stop(kt *kit.Kit, req *proto.StopReq) (*proto.ExecuteResult, error) {
	return common.Request[proto.StopReq, proto.ExecuteResult](
		c.client, rest.POST, kt, req, "/iac_plugins/stop")
}

// Plan calls POST /iac_plugins/plan to preview resource changes without modifying external state.
func (c *Client) Plan(kt *kit.Kit, req *proto.PlanReq) (*proto.PlanResult, error) {
	return common.Request[proto.PlanReq, proto.PlanResult](
		c.client, rest.POST, kt, req, "/iac_plugins/plan")
}

// Validate calls POST /iac_plugins/validate to validate user configuration against plugin rules.
func (c *Client) Validate(kt *kit.Kit, req *proto.ValidateReq) (*proto.ValidateResult, error) {
	return common.Request[proto.ValidateReq, proto.ValidateResult](
		c.client, rest.POST, kt, req, "/iac_plugins/validate")
}

// Import calls POST /iac_plugins/import to import an existing external resource by import_id.
func (c *Client) Import(kt *kit.Kit, req *proto.ImportReq) (*proto.StateResult, error) {
	return common.Request[proto.ImportReq, proto.StateResult](
		c.client, rest.POST, kt, req, "/iac_plugins/import")
}

// Refresh calls POST /iac_plugins/refresh to refresh external resource state using saved state.
func (c *Client) Refresh(kt *kit.Kit, req *proto.RefreshReq) (*proto.StateResult, error) {
	return common.Request[proto.RefreshReq, proto.StateResult](
		c.client, rest.POST, kt, req, "/iac_plugins/refresh")
}

// ReadData calls POST /iac_plugins/read_data to query a read-only data source.
func (c *Client) ReadData(kt *kit.Kit, req *proto.ReadDataReq) (*proto.ReadDataResult, error) {
	return common.Request[proto.ReadDataReq, proto.ReadDataResult](
		c.client, rest.POST, kt, req, "/iac_plugins/read_data")
}

// List calls POST /iac_plugins/list to paginate resource items under the plugin instance scope.
func (c *Client) List(kt *kit.Kit, req *proto.ListReq) (*proto.ListResult, error) {
	return common.Request[proto.ListReq, proto.ListResult](
		c.client, rest.POST, kt, req, "/iac_plugins/list")
}

func requestGetWithParams[R any](cli rest.ClientInterface, kt *kit.Kit, subPath string,
	params map[string]string) (*R, error) {

	resp := new(core.BaseResp[*R])
	err := cli.Verb(rest.GET).
		WithContext(kt.Ctx).
		SubResourcef(subPath).
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
