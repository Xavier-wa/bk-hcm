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

// Package state 实现 iac-server 状态查询的插件 Schema 定位与敏感属性递归脱敏逻辑。
package state

import (
	"hcm/pkg/api/core"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"

	tfjson "github.com/hashicorp/terraform-json"
)

// SchemaResolver 负责在每次状态详情查询时定位插件当前启用版本并向 hc-service 获取 SchemaBlock。
type SchemaResolver struct {
	client *client.ClientSet
}

// NewSchemaResolver 创建基于 ClientSet 的 SchemaResolver。cliSet 为空时返回错误。
func NewSchemaResolver(cliSet *client.ClientSet) (*SchemaResolver, error) {
	if cliSet == nil {
		return nil, errf.New(errf.InvalidParameter, "api client set is required")
	}
	return &SchemaResolver{client: cliSet}, nil
}

// ResolveBlock 定位指定插件的启用版本并解析对应资源或数据源类型的 SchemaBlock。
// 任意环节缺失或失败均返回错误，确保调用方按 Fail-Closed 策略阻断明文返回。
func (r *SchemaResolver) ResolveBlock(kt *kit.Kit, kind enumor.IacStateKind, pluginName, typeName string) (
	*tfjson.SchemaBlock, error) {

	if r == nil || r.client == nil || r.client.DataService() == nil || r.client.HCService() == nil {
		return nil, errf.New(errf.Aborted, "schema resolver clients are not initialized")
	}
	if len(pluginName) == 0 || len(typeName) == 0 {
		return nil, errf.New(errf.InvalidParameter, "plugin and type are required for schema lookup")
	}

	pluginRes, err := r.client.DataService().Global.IaCPlugin.List(kt, &core.ListReq{
		Filter: tools.ExpressionAnd(tools.RuleEqual("plugin_name", pluginName),
			tools.RuleEqual("state", enumor.IaCPluginStateEnabled)),
		Page: &core.BasePage{
			Start: 0,
			Limit: 1,
		},
	})
	if err != nil {
		logs.Errorf("list enabled iac plugin failed, plugin: %s, err: %v, rid: %s", pluginName, err, kt.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}
	if pluginRes == nil || len(pluginRes.Details) == 0 {
		logs.Errorf("enabled iac plugin not found, plugin: %s, rid: %s", pluginName, kt.Rid)
		return nil, errf.Newf(errf.RecordNotFound, "enabled plugin %q not found", pluginName)
	}

	pluginVersion := pluginRes.Details[0].PluginVersion
	schema, err := r.client.HCService().IaCPlugin.GetSchema(kt, pluginName, pluginVersion)
	if err != nil {
		logs.Errorf("get plugin schema from hc-service failed, plugin: %s, version: %s, err: %v, rid: %s",
			pluginName, pluginVersion, err, kt.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}
	if schema == nil {
		return nil, errf.Newf(errf.RecordNotFound, "schema for plugin %q version %q is empty",
			pluginName, pluginVersion)
	}

	var target *tfjson.Schema
	switch kind {
	case enumor.IacStateKindResource:
		target = schema.ResourceSchemas[typeName]
	case enumor.IacStateKindData:
		target = schema.DataSourceSchemas[typeName]
	default:
		return nil, errf.Newf(errf.InvalidParameter, "unsupported iac state kind: %s", kind)
	}

	if target == nil || target.Block == nil {
		logs.Errorf("schema block not found, plugin: %s, version: %s, kind: %s, type: %s, rid: %s",
			pluginName, pluginVersion, kind, typeName, kt.Rid)
		return nil, errf.Newf(errf.RecordNotFound,
			"schema block not found for plugin %q kind %q type %q", pluginName, kind, typeName)
	}

	return target.Block, nil
}
