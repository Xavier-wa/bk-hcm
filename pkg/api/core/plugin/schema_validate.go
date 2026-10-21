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

package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/tools/json"

	tfjson "github.com/hashicorp/terraform-json"
)

const (
	// SchemaHashPrefix 合法 schema_hash 前缀。
	SchemaHashPrefix = "sha256:"
	// SchemaHashLength 合法 schema_hash 总长度（"sha256:" + 64 位 hex）。
	SchemaHashLength = 71
)

var schemaHashRegexp = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ComputeSchemaHash 计算包含插件标识（plugin.name、plugin.version）与 SchemaDetail 的规范 SHA-256（"sha256:" + 64 位小写 hex）。
func ComputeSchemaHash(plugin PluginInfo, detail *SchemaDetail) (string, error) {
	if detail == nil {
		return "", errf.New(errf.InvalidParameter, "schema detail is nil")
	}

	payload := struct {
		Plugin       PluginInfo `json:"plugin"`
		SchemaDetail `json:",inline"`
	}{
		Plugin:       plugin,
		SchemaDetail: *detail,
	}

	rawBytes, err := json.Marshal(payload)
	if err != nil {
		return "", errf.Newf(errf.InvalidParameter, "marshal schema detail failed, err: %v", err)
	}

	sum := sha256.Sum256(rawBytes)
	return SchemaHashPrefix + hex.EncodeToString(sum[:]), nil
}

// ValidateSchemaHashFormat 校验 schemaHash 是否符合 "sha256:" + 64 位小写 hex。
func ValidateSchemaHashFormat(schemaHash string) error {
	if len(schemaHash) != SchemaHashLength || !schemaHashRegexp.MatchString(schemaHash) {
		return errf.Newf(errf.InvalidParameter,
			"invalid schema_hash format: %q, expected sha256:<64 lowercase hex chars>", schemaHash)
	}
	return nil
}

// ValidateCapabilities 按南向协议能力位正反向约束校验 SchemaDetail.Capabilities。
func ValidateCapabilities(detail *SchemaDetail) error {
	if detail == nil {
		return errf.New(errf.InvalidParameter, "schema detail is nil")
	}

	if detail.Capabilities == (Capabilities{}) {
		return errf.New(errf.InvalidParameter, "capabilities cannot be empty")
	}

	if err := validateResourceCapabilities(detail); err != nil {
		return err
	}

	if len(detail.DataSourceSchemas) > 0 && !detail.Capabilities.ReadData {
		return errf.New(errf.InvalidParameter,
			"capabilities.read_data must be true when data_source_schemas is not empty")
	}
	if len(detail.DataSourceSchemas) == 0 && detail.Capabilities.ReadData {
		return errf.New(errf.InvalidParameter,
			"capabilities.read_data must not be true when data_source_schemas is empty")
	}

	if detail.Capabilities.Stop && !detail.Capabilities.AsyncApply {
		return errf.New(errf.InvalidParameter,
			"capabilities.async_apply must be true when capabilities.stop is true")
	}

	return nil
}

func validateResourceCapabilities(detail *SchemaDetail) error {
	if len(detail.ResourceSchemas) > 0 {
		if !detail.Capabilities.Apply {
			return errf.New(errf.InvalidParameter,
				"capabilities.apply must be true when resource_schemas is not empty")
		}
		if !detail.Capabilities.Plan {
			return errf.New(errf.InvalidParameter,
				"capabilities.plan must be true when resource_schemas is not empty")
		}
		if !detail.Capabilities.Refresh {
			return errf.New(errf.InvalidParameter,
				"capabilities.refresh must be true when resource_schemas is not empty")
		}
		return nil
	}

	if detail.Capabilities.Apply || detail.Capabilities.Plan || detail.Capabilities.Refresh ||
		detail.Capabilities.AsyncApply || detail.Capabilities.Stop ||
		detail.Capabilities.Import || detail.Capabilities.List {
		return errf.New(errf.InvalidParameter,
			"resource capabilities must not be true when resource_schemas is empty")
	}
	return nil
}

// Validate 校验南向协议返回的 PluginSchema 结构与能力位规则。
func (s *PluginSchema) Validate() error {
	if s == nil {
		return errf.New(errf.InvalidParameter, "plugin schema is nil")
	}
	if strings.TrimSpace(s.ProtocolVersion) == "" {
		return errf.New(errf.InvalidParameter, "protocol_version is required")
	}
	if strings.TrimSpace(s.Plugin.Name) == "" {
		return errf.New(errf.InvalidParameter, "plugin.name is required")
	}
	if strings.TrimSpace(s.Plugin.Version) == "" {
		return errf.New(errf.InvalidParameter, "plugin.version is required")
	}
	if strings.TrimSpace(s.SchemaHash) == "" {
		return errf.New(errf.InvalidParameter, "schema_hash is required")
	}
	if err := validateSchemaBlock(s.ConfigSchema, "provider"); err != nil {
		return err
	}
	if err := s.validateTypeSchemas(); err != nil {
		return err
	}
	return ValidateCapabilities(&s.SchemaDetail)
}

func (s *PluginSchema) validateTypeSchemas() error {
	prefix := s.Plugin.Name + "_"
	for typeName, resSchema := range s.ResourceSchemas {
		if !strings.HasPrefix(typeName, prefix) {
			return errf.Newf(errf.InvalidParameter,
				"resource type_name %q must start with %q", typeName, prefix)
		}
		if err := validateSchemaBlock(resSchema, "resource_schemas."+typeName); err != nil {
			return err
		}
	}
	for typeName, dsSchema := range s.DataSourceSchemas {
		if !strings.HasPrefix(typeName, prefix) {
			return errf.Newf(errf.InvalidParameter,
				"data_source type_name %q must start with %q", typeName, prefix)
		}
		if err := validateSchemaBlock(dsSchema, "data_source_schemas."+typeName); err != nil {
			return err
		}
	}
	return nil
}

func validateSchemaBlock(s *tfjson.Schema, path string) error {
	if s == nil || s.Block == nil {
		return errf.Newf(errf.InvalidParameter, "%s.block is required", path)
	}
	return nil
}

// ValidatePluginSchema 按期望插件身份与协议规则校验 PluginSchema 全部字段。
func ValidatePluginSchema(schema *PluginSchema, expectedPlugin PluginInfo, protocol enumor.IaCPluginProtocol) error {
	if schema == nil {
		return errf.New(errf.InvalidParameter, "plugin schema is nil")
	}

	if strings.TrimSpace(schema.Plugin.Name) == "" {
		return errf.New(errf.InvalidParameter, "plugin.name is required")
	}
	if strings.TrimSpace(schema.Plugin.Version) == "" {
		return errf.New(errf.InvalidParameter, "plugin.version is required")
	}

	if schema.Plugin.Name != expectedPlugin.Name {
		return errf.Newf(errf.InvalidParameter,
			"plugin.name mismatch: schema has %q, expected %q", schema.Plugin.Name, expectedPlugin.Name)
	}
	if schema.Plugin.Version != expectedPlugin.Version {
		return errf.Newf(errf.InvalidParameter,
			"plugin.version mismatch: schema has %q, expected %q", schema.Plugin.Version, expectedPlugin.Version)
	}

	switch protocol {
	case enumor.IaCPluginProtocolHTTP:
		if strings.TrimSpace(schema.ProtocolVersion) == "" {
			return errf.New(errf.InvalidParameter, "protocol_version is required for http protocol plugin")
		}
	case enumor.IaCPluginProtocolTFProvider:
		// protocol_version is optional for tf_provider protocol
	default:
		return errf.Newf(errf.InvalidParameter, "unsupported plugin protocol: %s", protocol)
	}

	if err := ValidateSchemaHashFormat(schema.SchemaHash); err != nil {
		return err
	}

	computedHash, err := ComputeSchemaHash(schema.Plugin, &schema.SchemaDetail)
	if err != nil {
		return err
	}
	if schema.SchemaHash != computedHash {
		return errf.Newf(errf.InvalidParameter,
			"schema_hash mismatch: declared %s, computed %s", schema.SchemaHash, computedHash)
	}

	if err := validateSchemaBlock(schema.ConfigSchema, "provider"); err != nil {
		return err
	}
	if err := schema.validateTypeSchemas(); err != nil {
		return err
	}
	return ValidateCapabilities(&schema.SchemaDetail)
}
