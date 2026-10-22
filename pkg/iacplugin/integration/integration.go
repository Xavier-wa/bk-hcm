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

// Package integration 定义插件注册与升级流程依赖的对接接口。
//
// Schema 的落库与摘要校验，以及 tf_provider 插件进程的拉起、握手、通信、关闭，
// 均不属于注册与升级模块的开发范围，由对接方实现。本包只声明注册与升级流程消费它们所需的最小接口，
// 使注册与升级流程可以在对接方交付前独立开发与测试。
package integration

import (
	// embed static schema json for local integration testing.
	_ "embed"
	"fmt"
	"strings"

	coreplugin "hcm/pkg/api/core/plugin"
	"hcm/pkg/kit"
	"hcm/pkg/tools/json"

	tfjson "github.com/hashicorp/terraform-json"
)

//go:embed tencentcloud_1.81.5_static_schema_temp_delete.json
var staticSchemaRaw []byte

// LoadStaticSchema 加载内嵌的静态 Schema 供联调测试使用，并按传入的 pluginName、pluginVersion
// 改写插件名与资源类型名前缀。改写后按存储模型重算 schema_hash，供 Schema 同步校验。
// 内嵌文件仍是 plugin_name / plugin_version 的扁平字段，这里补进 Plugin。
func LoadStaticSchema(pluginName, pluginVersion string) (*coreplugin.PluginSchema, error) {
	var raw struct {
		coreplugin.PluginSchema
		PluginName    string `json:"plugin_name"`
		PluginVersion string `json:"plugin_version"`
	}
	if err := json.Unmarshal(staticSchemaRaw, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal static schema failed, err: %w", err)
	}

	schema := &raw.PluginSchema
	if len(schema.Plugin.Name) == 0 {
		schema.Plugin.Name = raw.PluginName
	}
	if len(schema.Plugin.Version) == 0 {
		schema.Plugin.Version = raw.PluginVersion
	}

	origName := schema.Plugin.Name
	if len(pluginName) != 0 {
		schema.Plugin.Name = pluginName
	}
	if len(pluginVersion) != 0 {
		schema.Plugin.Version = pluginVersion
	}

	origPrefix := origName + "_"
	targetPrefix := schema.Plugin.Name + "_"
	if len(origName) != 0 && targetPrefix != origPrefix {
		schema.ResourceSchemas = replaceSchemaPrefix(schema.ResourceSchemas, origPrefix, targetPrefix)
		schema.DataSourceSchemas = replaceSchemaPrefix(schema.DataSourceSchemas, origPrefix, targetPrefix)
	}

	hash, err := coreplugin.ComputeSchemaHash(schema.Plugin, &schema.SchemaDetail)
	if err != nil {
		return nil, err
	}
	schema.SchemaHash = hash

	return schema, nil
}

func replaceSchemaPrefix(in map[string]*tfjson.Schema, oldPrefix, newPrefix string) map[string]*tfjson.Schema {
	if len(in) == 0 {
		return in
	}
	out := make(map[string]*tfjson.Schema, len(in))
	for k, v := range in {
		out[newPrefix+strings.TrimPrefix(k, oldPrefix)] = v
	}
	return out
}

// SchemaGetterFunc 把方法值适配成 SchemaGetter，让调用方可以直接传取 Schema 的方法，
// 而不是把整个对象交给 SyncSchema：同步逻辑只该拿到"怎么取 Schema"，不该拿到进程或 client。
type SchemaGetterFunc func(kt *kit.Kit) (*coreplugin.PluginSchema, error)

// GetSchema 实现 SchemaGetter。
func (f SchemaGetterFunc) GetSchema(kt *kit.Kit) (*coreplugin.PluginSchema, error) { return f(kt) }

// Executor 插件进程实例的执行器（方案 §6.2）。上游拿它取 Schema，不负责进程生命周期。
type Executor interface {
	// GetProviderSchema 取装配好的统一 Schema：plugin_name、plugin_version、schema_hash、capabilities
	// 均由执行器填写，返回的不是协议层原始响应。
	GetProviderSchema(kt *kit.Kit) (*coreplugin.PluginSchema, error)
}

// ProcessManager 一个插件进程的生命周期（方案 §6.1、§6.3）：Start 拉起进程并返回执行器，Close 结束进程。
type ProcessManager interface {
	// Start 拉起进程、完成握手，返回执行器。
	Start(kt *kit.Kit) (Executor, error)
	// Close 关闭进程。必须幂等：重复调用不报错。
	// 方案要求依次等待在途调用、调用 StopProvider、结束进程；这些细节由实现方负责。
	Close() error
}

// ProcessStarter 进程管理工厂，由进程管理对接方实现。
//
// 每次 NewProcessManager 对应一个一次性进程，不进入任何进程池；握手、协议版本协商、通信等细节由实现方负责。
// 调用方用完执行器后必须调用 ProcessManager.Close。
type ProcessStarter interface {
	NewProcessManager(kt *kit.Kit, pluginName, pluginVersion, execPath string) (ProcessManager, error)
}
