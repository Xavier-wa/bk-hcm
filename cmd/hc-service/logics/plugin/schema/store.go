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

package schema

import (
	"fmt"

	"hcm/pkg/api/core"
	coreplugin "hcm/pkg/api/core/plugin"
	dataplugin "hcm/pkg/api/data-service/plugin"
	dataservice "hcm/pkg/client/data-service"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	tableplugin "hcm/pkg/dal/table/plugin"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/json"
)

type dataServiceSchemaStore struct {
	dataCli *dataservice.Client
}

// NewDataServiceSchemaStore 基于 data-service SDK 创建 SchemaStore。
func NewDataServiceSchemaStore(dataCli *dataservice.Client) SchemaStore {
	return &dataServiceSchemaStore{
		dataCli: dataCli,
	}
}

// GetPlugin 按 (plugin_name, plugin_version) 查询插件注册记录。
// TODO(temp_delete): 当前查询临时表 iac_plugin_temp_delete，合并注册分支后切换为正式表。
func (s *dataServiceSchemaStore) GetPlugin(kt *kit.Kit, pluginName, pluginVersion string) (
	*coreplugin.IacPluginTempDelete, bool, error) {

	if s.dataCli == nil || s.dataCli.Global == nil || s.dataCli.Global.IacPluginTempDelete == nil {
		return nil, false, errf.New(errf.InvalidParameter, "data-service iac_plugin_temp_delete client is not initialized")
	}

	req := &dataplugin.IacPluginTempDeleteListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("plugin_name", pluginName),
			tools.RuleEqual("plugin_version", pluginVersion),
		),
		Page: &core.BasePage{
			Start: 0,
			Limit: 1,
		},
	}

	res, err := s.dataCli.Global.IacPluginTempDelete.List(kt, req)
	if err != nil {
		logs.Errorf("list iac_plugin_temp_delete from data-service failed, plugin: %s@%s, err: %v, rid: %s",
			pluginName, pluginVersion, err, kt.Rid)
		return nil, false, err
	}

	if res == nil || len(res.Details) == 0 {
		return nil, false, nil
	}

	return &res.Details[0], true, nil
}

// GetSchemaHash 按 (plugin_name, plugin_version) 仅查询 schema_hash。
func (s *dataServiceSchemaStore) GetSchemaHash(kt *kit.Kit, pluginName, pluginVersion string) (string, bool, error) {
	if s.dataCli == nil || s.dataCli.Global == nil || s.dataCli.Global.IacPluginSchema == nil {
		return "", false, errf.New(errf.InvalidParameter, "data-service iac_plugin_schema client is not initialized")
	}

	req := &dataplugin.IacPluginSchemaListReq{
		Fields: []string{"id", "plugin_name", "plugin_version", "schema_hash"},
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("plugin_name", pluginName),
			tools.RuleEqual("plugin_version", pluginVersion),
		),
		Page: &core.BasePage{
			Start: 0,
			Limit: 1,
		},
	}

	res, err := s.dataCli.Global.IacPluginSchema.List(kt, req)
	if err != nil {
		logs.Errorf("list iac_plugin_schema hash from data-service failed, plugin: %s@%s, err: %v, rid: %s",
			pluginName, pluginVersion, err, kt.Rid)
		return "", false, err
	}

	if res == nil || len(res.Details) == 0 {
		return "", false, nil
	}

	return res.Details[0].SchemaHash, true, nil
}

// GetFullSchema 按 (plugin_name, plugin_version) 查询完整 Schema 记录。
func (s *dataServiceSchemaStore) GetFullSchema(kt *kit.Kit, pluginName, pluginVersion string) (
	*coreplugin.PluginSchema, bool, error) {

	if s.dataCli == nil || s.dataCli.Global == nil || s.dataCli.Global.IacPluginSchema == nil {
		return nil, false, errf.New(errf.InvalidParameter, "data-service iac_plugin_schema client is not initialized")
	}

	req := &dataplugin.IacPluginSchemaListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("plugin_name", pluginName),
			tools.RuleEqual("plugin_version", pluginVersion),
		),
		Page: &core.BasePage{
			Start: 0,
			Limit: 1,
		},
	}

	res, err := s.dataCli.Global.IacPluginSchema.List(kt, req)
	if err != nil {
		logs.Errorf("list full iac_plugin_schema from data-service failed, plugin: %s@%s, err: %v, rid: %s",
			pluginName, pluginVersion, err, kt.Rid)
		return nil, false, err
	}

	if res == nil || len(res.Details) == 0 {
		return nil, false, nil
	}

	pluginSchema, err := toPluginSchema(res.Details[0])
	if err != nil {
		logs.Errorf("convert iac_plugin_schema record to PluginSchema failed, plugin: %s@%s, err: %v, rid: %s",
			pluginName, pluginVersion, err, kt.Rid)
		return nil, false, err
	}

	return pluginSchema, true, nil
}

func toPluginSchema(record tableplugin.IacPluginSchemaTable) (*coreplugin.PluginSchema, error) {
	var raw coreplugin.RawSchema
	if len(record.Schema) > 0 {
		if err := json.Unmarshal([]byte(record.Schema), &raw); err != nil {
			return nil, fmt.Errorf("unmarshal raw schema failed, err: %w", err)
		}
	}

	return &coreplugin.PluginSchema{
		PluginName:    record.PluginName,
		PluginVersion: record.PluginVersion,
		SchemaHash:    record.SchemaHash,
		RawSchema:     raw,
	}, nil
}

// CreateSchema 通过 data-service 将 PluginSchema 写入 iac_plugin_schema。
func (s *dataServiceSchemaStore) CreateSchema(kt *kit.Kit, schema *coreplugin.PluginSchema) (string, error) {
	if s.dataCli == nil || s.dataCli.Global == nil || s.dataCli.Global.IacPluginSchema == nil {
		return "", errf.New(errf.InvalidParameter, "data-service iac_plugin_schema client is not initialized")
	}
	if schema == nil {
		return "", errf.New(errf.InvalidParameter, "plugin schema is nil")
	}

	rawBytes, err := json.Marshal(schema.RawSchema)
	if err != nil {
		logs.Errorf("marshal raw schema failed, plugin: %s@%s, err: %v, rid: %s",
			schema.PluginName, schema.PluginVersion, err, kt.Rid)
		return "", err
	}

	req := &dataplugin.IacPluginSchemaCreateReq{
		PluginName:    schema.PluginName,
		PluginVersion: schema.PluginVersion,
		SchemaHash:    schema.SchemaHash,
		Schema:        types.JsonField(rawBytes),
	}

	res, err := s.dataCli.Global.IacPluginSchema.Create(kt, req)
	if err != nil {
		logs.Errorf("create iac_plugin_schema in data-service failed, plugin: %s@%s, err: %v, rid: %s",
			schema.PluginName, schema.PluginVersion, err, kt.Rid)
		return "", err
	}

	return res.ID, nil
}
