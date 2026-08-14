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
	"strings"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// GetToolSchemaTool implements the get_tool_schema meta-tool.
type GetToolSchemaTool struct {
	proxy *ToolProxy
}

// NewGetToolSchemaTool creates a GetToolSchemaTool bound to the given proxy.
func NewGetToolSchemaTool(proxy *ToolProxy) *GetToolSchemaTool {
	return &GetToolSchemaTool{proxy: proxy}
}

// Declaration returns the tool declaration for get_tool_schema.
func (t *GetToolSchemaTool) Declaration() *trpctool.Declaration {
	return &trpctool.Declaration{
		Name: constant.GetToolSchemaToolName,
		Description: "【已知工具名时使用】根据工具名称返回完整 JSON Schema。" +
			"当用户已明确工具名、或 search_tools 结果中已选定工具、需确认参数格式时调用。",
		InputSchema: &trpctool.Schema{
			Type: "object",
			Properties: map[string]*trpctool.Schema{
				"tool_name": {
					Type: "string",
					Description: "目标 MCP 工具名称，支持带 toolset 前缀的规范名（如 my-mcp_search_code）" +
						"或无前缀的原始名（如 mcp_search_code，全局唯一时可用）",
				},
			},
			Required: []string{"tool_name"},
		},
	}
}

// Call returns the schema for the requested tool.
func (t *GetToolSchemaTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	rid := rest.RidFromContext(ctx)
	var params struct {
		ToolName string `json:"tool_name"`
	}
	if err := json.Unmarshal(jsonArgs, &params); err != nil {
		logs.Errorf("get_tool_schema failed, err: %v, args=%s, rid: %s", err, string(jsonArgs), rid)
		return buildErrorResult("invalid_parameters", "parameter parse failed: "+err.Error(), nil), nil
	}
	params.ToolName = strings.TrimSpace(params.ToolName)
	if params.ToolName == "" {
		logs.Errorf("get_tool_schema failed, err: tool_name required, rid: %s", rid)
		return buildErrorResult("invalid_parameters", "parameter tool_name is required", nil), nil
	}

	logs.Infof("get_tool_schema called, tool_name=%s, rid: %s", params.ToolName, rid)
	meta, err := t.proxy.RegistrySnapshot().GetTool(params.ToolName)
	if err != nil {
		logs.Errorf("get_tool_schema failed, err: %v, tool_name=%s, rid: %s", err, params.ToolName, rid)
		return buildErrorResult("tool_not_found", "tool "+params.ToolName+" not found", nil), nil
	}

	logs.Infof("get_tool_schema success, tool_name=%s, canonical=%s, desc: %s, rid: %s",
		params.ToolName, meta.Name, meta.Description, rid)
	return map[string]interface{}{
		"success":      true,
		"tool_name":    meta.Name,
		"description":  meta.Description,
		"tags":         meta.Tags,
		"schema":       meta.Schema,
		"examples":     meta.Examples,
		"schema_token": t.proxy.GenerateSchemaToken(meta.Name, meta.Schema),
	}, nil
}
