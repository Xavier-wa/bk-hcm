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
	_ "embed"

	coreplugin "hcm/pkg/api/core/plugin"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/json"
)

// TODO(temp_delete): 临时 SchemaGetter 自测实现，提交 MR 前删除本文件。

//go:embed tencentcloud_1.81.5_static_schema_temp_delete.json
var defaultTencentCloudStaticSchemaJSONTempDelete []byte

// StaticSchemaGetterTempDelete 基于内存 PluginSchema 或 JSON 载荷的自测用临时 SchemaGetter。
type StaticSchemaGetterTempDelete struct {
	schema          *coreplugin.PluginSchema
	rawJSON         []byte
	autoComputeHash bool
}

// NewDefaultStaticSchemaGetterTempDelete 基于内置腾讯云 1.81.5 静态 Schema 创建临时 SchemaGetter。
func NewDefaultStaticSchemaGetterTempDelete(autoComputeHash bool) *StaticSchemaGetterTempDelete {
	return &StaticSchemaGetterTempDelete{
		rawJSON:         defaultTencentCloudStaticSchemaJSONTempDelete,
		autoComputeHash: autoComputeHash,
	}
}

// NewStaticSchemaGetterTempDelete 基于内存 PluginSchema 创建 StaticSchemaGetterTempDelete。
func NewStaticSchemaGetterTempDelete(schema *coreplugin.PluginSchema,
	autoComputeHash bool) *StaticSchemaGetterTempDelete {

	return &StaticSchemaGetterTempDelete{
		schema:          schema,
		autoComputeHash: autoComputeHash,
	}
}

// NewJSONSchemaGetterTempDelete 基于 PluginSchema JSON 字节创建 StaticSchemaGetterTempDelete。
func NewJSONSchemaGetterTempDelete(rawJSON []byte, autoComputeHash bool) *StaticSchemaGetterTempDelete {
	return &StaticSchemaGetterTempDelete{
		rawJSON:         rawJSON,
		autoComputeHash: autoComputeHash,
	}
}

// GetSchema 返回 StaticSchemaGetterTempDelete 配置的 PluginSchema。
func (g *StaticSchemaGetterTempDelete) GetSchema(kt *kit.Kit) (*coreplugin.PluginSchema, error) {
	var result *coreplugin.PluginSchema
	if len(g.rawJSON) > 0 {
		parsed := new(coreplugin.PluginSchema)
		if err := json.Unmarshal(g.rawJSON, parsed); err != nil {
			logs.Errorf("unmarshal static plugin schema json failed, err: %v, rid: %s", err, kt.Rid)
			return nil, errf.NewFromErr(errf.InvalidParameter, err)
		}
		result = parsed
	} else if g.schema != nil {
		copied := *g.schema
		result = &copied
	} else {
		parsed := new(coreplugin.PluginSchema)
		if err := json.Unmarshal(defaultTencentCloudStaticSchemaJSONTempDelete, parsed); err != nil {
			logs.Errorf("unmarshal default static plugin schema json failed, err: %v, rid: %s", err, kt.Rid)
			return nil, errf.NewFromErr(errf.InvalidParameter, err)
		}
		result = parsed
	}

	if g.autoComputeHash {
		hash, err := coreplugin.ComputeRawSchemaHash(&result.RawSchema)
		if err != nil {
			logs.Errorf("compute raw schema hash in static getter failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		result.SchemaHash = hash
	}

	return result, nil
}
