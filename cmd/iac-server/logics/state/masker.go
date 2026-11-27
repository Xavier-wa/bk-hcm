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

package state

import (
	"slices"

	iacproto "hcm/pkg/api/iac-server"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iac/iacstate"

	tfjson "github.com/hashicorp/terraform-json"
)

// MaskStateObject 基于 tfjson.SchemaBlock 对单个实例状态执行递归脱敏，剔除 private 字段，
// 原样保留 depends_on、identity 与 provider_config。
func MaskStateObject(obj *iacstate.StateObject, block *tfjson.SchemaBlock) (*iacproto.MaskedStateObject, error) {
	if obj == nil {
		return nil, nil
	}
	if block == nil {
		return nil, errf.New(errf.Aborted, "schema block is nil, refuse to return unmasked state")
	}

	maskedState := maskBlockMap(obj.State, block)
	return &iacproto.MaskedStateObject{
		Plugin:           obj.Plugin,
		PluginInstanceID: obj.PluginInstanceID,
		Type:             obj.Type,
		State:            maskedState,
		DependsOn:        slices.Clone(obj.DependsOn),
		Identity:         cloneAnyMap(obj.Identity),
		ProviderConfig:   cloneAnyMap(obj.ProviderConfig),
	}, nil
}

func maskBlockMap(src map[string]any, block *tfjson.SchemaBlock) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		if attr, ok := block.Attributes[k]; ok && attr != nil {
			out[k] = maskAttributeValue(v, attr)
			continue
		}
		if nested, ok := block.NestedBlocks[k]; ok && nested != nil && nested.Block != nil {
			out[k] = maskNestedBlockValue(v, nested)
			continue
		}
		out[k] = cloneAny(v)
	}
	return out
}

func maskAttributeValue(val any, attr *tfjson.SchemaAttribute) any {
	if val == nil {
		return nil
	}
	if attr.Sensitive {
		return constant.IacStateSensitiveValueMask
	}
	if attr.AttributeNestedType != nil {
		return maskNestedTypeValue(val, attr.AttributeNestedType)
	}
	return cloneAny(val)
}

func maskNestedTypeValue(val any, nt *tfjson.SchemaNestedAttributeType) any {
	if val == nil || nt == nil {
		return nil
	}
	switch nt.NestingMode {
	case tfjson.SchemaNestingModeSingle:
		if m, ok := val.(map[string]any); ok {
			return maskAttributeMap(m, nt.Attributes)
		}
	case tfjson.SchemaNestingModeList, tfjson.SchemaNestingModeSet:
		if arr, ok := val.([]any); ok {
			out := make([]any, len(arr))
			for i, elem := range arr {
				if m, ok := elem.(map[string]any); ok {
					out[i] = maskAttributeMap(m, nt.Attributes)
				} else {
					out[i] = cloneAny(elem)
				}
			}
			return out
		}
	case tfjson.SchemaNestingModeMap:
		if m, ok := val.(map[string]any); ok {
			out := make(map[string]any, len(m))
			for k, elem := range m {
				if sub, ok := elem.(map[string]any); ok {
					out[k] = maskAttributeMap(sub, nt.Attributes)
				} else {
					out[k] = cloneAny(elem)
				}
			}
			return out
		}
	default:
		if m, ok := val.(map[string]any); ok {
			return maskAttributeMap(m, nt.Attributes)
		}
	}
	return cloneAny(val)
}

func maskAttributeMap(src map[string]any, attrs map[string]*tfjson.SchemaAttribute) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		if attr, ok := attrs[k]; ok && attr != nil {
			out[k] = maskAttributeValue(v, attr)
			continue
		}
		out[k] = cloneAny(v)
	}
	return out
}

func maskNestedBlockValue(val any, nested *tfjson.SchemaBlockType) any {
	if val == nil || nested == nil || nested.Block == nil {
		return nil
	}
	switch nested.NestingMode {
	case tfjson.SchemaNestingModeSingle, tfjson.SchemaNestingModeGroup:
		if m, ok := val.(map[string]any); ok {
			return maskBlockMap(m, nested.Block)
		}
		if arr, ok := val.([]any); ok {
			return maskBlockSlice(arr, nested.Block)
		}
	case tfjson.SchemaNestingModeList, tfjson.SchemaNestingModeSet:
		if arr, ok := val.([]any); ok {
			return maskBlockSlice(arr, nested.Block)
		}
		if m, ok := val.(map[string]any); ok {
			return maskBlockMap(m, nested.Block)
		}
	case tfjson.SchemaNestingModeMap:
		if m, ok := val.(map[string]any); ok {
			out := make(map[string]any, len(m))
			for k, elem := range m {
				if sub, ok := elem.(map[string]any); ok {
					out[k] = maskBlockMap(sub, nested.Block)
				} else {
					out[k] = cloneAny(elem)
				}
			}
			return out
		}
	default:
		if m, ok := val.(map[string]any); ok {
			return maskBlockMap(m, nested.Block)
		}
		if arr, ok := val.([]any); ok {
			return maskBlockSlice(arr, nested.Block)
		}
	}
	return cloneAny(val)
}

func maskBlockSlice(arr []any, block *tfjson.SchemaBlock) []any {
	out := make([]any, len(arr))
	for i, elem := range arr {
		if m, ok := elem.(map[string]any); ok {
			out[i] = maskBlockMap(m, block)
		} else {
			out[i] = cloneAny(elem)
		}
	}
	return out
}

func cloneAny(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, elem := range typed {
			out[i] = cloneAny(elem)
		}
		return out
	default:
		return v
	}
}

func cloneAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = cloneAny(v)
	}
	return out
}
