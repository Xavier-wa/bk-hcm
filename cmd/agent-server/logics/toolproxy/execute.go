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

package toolproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/assert"

	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// ExecuteToolParams is the JSON envelope for the execute_tool meta-tool:
// {tool_name, parameters, schema_token}.
type ExecuteToolParams struct {
	ToolName    string          `json:"tool_name"`
	Parameters  json.RawMessage `json:"parameters"`
	SchemaToken string          `json:"schema_token"`
}

// ParametersMap unmarshals Parameters into a map for schema validation and execution.
func (p ExecuteToolParams) ParametersMap() (map[string]interface{}, error) {
	if len(p.Parameters) == 0 {
		return nil, nil
	}
	var parameters map[string]interface{}
	if err := json.Unmarshal(p.Parameters, &parameters); err != nil {
		return nil, err
	}
	return parameters, nil
}

// ExecuteToolTool implements the execute_tool meta-tool.
type ExecuteToolTool struct {
	proxy *ToolProxy
}

// NewExecuteToolTool creates an ExecuteToolTool bound to the given proxy.
func NewExecuteToolTool(proxy *ToolProxy) *ExecuteToolTool {
	return &ExecuteToolTool{proxy: proxy}
}

// Declaration returns the tool declaration for execute_tool.
func (t *ExecuteToolTool) Declaration() *trpctool.Declaration {
	return &trpctool.Declaration{
		Name: constant.ExecuteToolToolName,
		Description: "【执行实际 MCP 工具】根据 tool_name、parameters 和 schema_token 调用 MCP 工具。" +
			"必须先通过 search_tools 或 get_tool_schema 获取 schema 和 schema_token 后再调用，禁止猜测参数。",
		InputSchema: &trpctool.Schema{
			Type: "object",
			Properties: map[string]*trpctool.Schema{
				"tool_name": {
					Type:        "string",
					Description: "要执行的 MCP 工具名称",
				},
				"parameters": {
					Type:        "object",
					Description: "工具参数，须严格符合 search_tools 或 get_tool_schema 返回的 schema，不得传入 schema 未定义的字段",
				},
				"schema_token": {
					Type: "string",
					Description: "必填。由 search_tools 或 get_tool_schema 返回的 token，" +
						"证明已读取当前工具的参数规范。缺少或无效时调用会被系统拒绝。",
				},
			},
			Required: []string{"tool_name", "parameters", "schema_token"},
		},
	}
}

// Call validates parameters and calls the underlying MCP tool.
func (t *ExecuteToolTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	rid := rest.RidFromContext(ctx)
	var params ExecuteToolParams
	if err := json.Unmarshal(jsonArgs, &params); err != nil {
		logs.Errorf("execute_tool failed, err: %v, args=%s, rid: %s", err, string(jsonArgs), rid)
		return buildErrorResult("invalid_parameters", "parameter parse failed: "+err.Error(), nil), nil
	}
	params.ToolName = strings.TrimSpace(params.ToolName)
	if params.ToolName == "" {
		logs.Errorf("execute_tool failed, err: tool_name required, rid: %s", rid)
		return buildErrorResult("invalid_parameters", "parameter tool_name is required", nil), nil
	}
	parameters, err := params.ParametersMap()
	if err != nil {
		logs.Errorf("execute_tool failed, err: %v, tool_name=%s, rid: %s", err, params.ToolName, rid)
		return buildErrorResult("invalid_parameters", "parameter parameters parse failed: "+err.Error(), nil), nil
	}
	if parameters == nil {
		logs.Errorf("execute_tool failed, err: parameters required, tool_name=%s, rid: %s", params.ToolName, rid)
		return buildErrorResult("invalid_parameters", "parameter parameters is required", nil), nil
	}
	logs.Infof("execute_tool called, tool_name=%s, rid: %s", params.ToolName, rid)

	meta, err := t.proxy.RegistrySnapshot().GetTool(params.ToolName)
	if err != nil {
		logs.Errorf("execute_tool failed, err: %v, tool_name=%s, rid: %s", err, params.ToolName, rid)
		return buildErrorResult("tool_not_found", "tool "+params.ToolName+" not found", nil), nil
	}

	if !t.proxy.VerifySchemaToken(meta.Name, meta.Schema, params.SchemaToken) {
		logs.Errorf("execute_tool failed, err: invalid schema_token, tool_name=%s, rid: %s", meta.Name, rid)
		return buildErrorResult("schema_token_invalid",
			"schema_token 无效，请先调用 search_tools 或 get_tool_schema 获取最新 schema 和 token", nil), nil
	}

	if err = validateParameters(meta.Schema, parameters); err != nil {
		logs.Errorf("execute_tool failed, err: %v, tool_name=%s, canonical=%s, rid: %s",
			err, params.ToolName, meta.Name, rid)
		return buildErrorResultWithSchema("invalid_parameters", err.Error(), meta.Schema), nil
	}

	argsBytes, err := json.Marshal(parameters)
	if err != nil {
		logs.Errorf("execute_tool failed, err: %v, tool_name=%s, rid: %s", err, meta.Name, rid)
		return buildErrorResult("invalid_parameters", "参数序列化失败: "+err.Error(), nil), nil
	}

	logs.Infof("execute_tool called, tool_name=%s, args=%s, rid: %s", meta.Name, string(argsBytes), rid)
	result, err := t.proxy.CallTool(ctx, meta.Name, argsBytes)
	if err != nil {
		if isPermissionDenied(err) {
			logs.Errorf("execute_tool failed, err: %v, tool_name=%s, rid: %s", err, meta.Name, rid)
			return buildErrorResult("permission_denied", constant.PermissionDeniedMsg, nil), nil
		}
		logs.Errorf("execute_tool failed, err: %v, tool_name=%s, rid: %s", err, meta.Name, rid)
		return buildErrorResult("execution_error", err.Error(), nil), nil
	}

	logs.Infof("execute_tool success, tool_name=%s, rid: %s", meta.Name, rid)
	return buildSuccessResult(result), nil
}

func validateParameters(schema map[string]interface{}, params map[string]interface{}) error {
	if schema == nil {
		return nil
	}
	return validateValue(schema, params, "parameters")
}

func validateValue(schema map[string]interface{}, value interface{}, path string) error {
	typ, _ := schema["type"].(string)
	switch typ {
	case "", "any":
		return validateEnum(schema, value, path)
	case "object":
		obj, ok := value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("参数 %s 应为 object", path)
		}
		return validateObject(schema, obj, path)
	case "array":
		items, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("参数 %s 应为 array", path)
		}
		itemSchema, _ := schema["items"].(map[string]interface{})
		if itemSchema != nil {
			for i, item := range items {
				if err := validateValue(itemSchema, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("参数 %s 应为 string", path)
		}
	case "integer":
		if !assert.IsInteger(value) {
			return fmt.Errorf("参数 %s 应为 integer", path)
		}
	case "number":
		if !assert.IsNumeric(value) {
			return fmt.Errorf("参数 %s 应为 number", path)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("参数 %s 应为 boolean", path)
		}
	default:
		return nil
	}
	return validateEnum(schema, value, path)
}

func validateObject(schema map[string]interface{}, params map[string]interface{}, path string) error {
	for _, name := range schemaRequiredFields(schema) {
		if _, exists := params[name]; !exists {
			return fmt.Errorf("参数 %q 是必需的", name)
		}
	}

	props, _ := schema["properties"].(map[string]interface{})

	// Reject unknown parameter keys when schema defines properties, unless additionalProperties is
	// explicitly true. Unknown keys are treated as LLM hallucinations that must not be executed.
	allowAdditional, _ := schema["additionalProperties"].(bool)
	if len(props) > 0 && !allowAdditional {
		for name := range params {
			if _, exists := props[name]; !exists {
				return fmt.Errorf("参数 %q 不在工具 schema 定义中，请重新阅读 schema", name)
			}
		}
	}

	for name, raw := range props {
		if val, exists := params[name]; exists {
			propSchema, _ := raw.(map[string]interface{})
			if propSchema == nil {
				continue
			}
			if err := validateValue(propSchema, val, path+"."+name); err != nil {
				return err
			}
		}
	}
	return validateEnum(schema, params, path)
}

func schemaRequiredFields(schema map[string]interface{}) []string {
	raw := schema["required"]
	switch v := raw.(type) {
	case []string:
		return v
	case []interface{}:
		names := make([]string, 0, len(v))
		for _, item := range v {
			if name, ok := item.(string); ok {
				names = append(names, name)
			}
		}
		return names
	default:
		return nil
	}
}

func validateEnum(schema map[string]interface{}, value interface{}, path string) error {
	rawEnum, _ := schema["enum"].([]interface{})
	if len(rawEnum) == 0 {
		return nil
	}
	for _, allowed := range rawEnum {
		if assert.InterfaceValuesEqual(allowed, value) {
			return nil
		}
	}
	return fmt.Errorf("参数 %s 不在枚举值范围内", path)
}
