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

package tool

import (
	"context"
	"encoding/json"

	"hcm/pkg/logs"

	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// MakeParamFixCallbacks creates tool callbacks that fix common parameter issues
// before they are sent to MCP tools. Currently handles:
// 1. Converting string-ified JSON objects back to actual JSON objects for path_param and body_param
func MakeParamFixCallbacks() tool.BeforeToolCallbackStructured {
	return func(ctx context.Context, args *tool.BeforeToolArgs) (*tool.BeforeToolResult, error) {
		if args == nil || len(args.Arguments) == 0 {
			return nil, nil
		}
		var argMap map[string]interface{}
		if err := json.Unmarshal(args.Arguments, &argMap); err != nil {
			return nil, nil
		}
		modified := fixStringParams(argMap, args.ToolName) || fixNestedParams(argMap, args.ToolName)
		if !modified {
			return nil, nil
		}
		newArgs, err := json.Marshal(argMap)
		if err != nil {
			logs.Errorf("[tool:param_fix] %s: failed to marshal modified args: %v", args.ToolName, err)
			return nil, nil
		}
		logs.Infof("[tool:param_fix] %s: parameters fixed successfully", args.ToolName)
		return &tool.BeforeToolResult{ModifiedArguments: newArgs}, nil
	}
}

// fixStringParams fixes path_param and body_param if they are string-encoded JSON objects
func fixStringParams(argMap map[string]interface{}, toolName string) bool {
	modified := false
	for _, field := range []string{"path_param", "body_param"} {
		if val, ok := argMap[field].(string); ok {
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(val), &parsed); err == nil {
				argMap[field] = parsed
				modified = true
				logs.Infof("[tool:param_fix] %s: %s converted from string to object", toolName, field)
			} else {
				logs.Warnf("[tool:param_fix] %s: %s is string but not valid JSON: %v", toolName, field, err)
			}
		}
	}
	return modified
}

// fixNestedParams fixes nested string-encoded JSON values within path_param and body_param objects
func fixNestedParams(argMap map[string]interface{}, toolName string) bool {
	modified := false
	for _, field := range []string{"path_param", "body_param"} {
		objMap, ok := argMap[field].(map[string]interface{})
		if !ok {
			continue
		}
		for key, v := range objMap {
			strVal, ok := v.(string)
			if !ok {
				continue
			}
			var parsed interface{}
			if err := json.Unmarshal([]byte(strVal), &parsed); err == nil {
				objMap[key] = parsed
				modified = true
				logs.Infof("[tool:param_fix] %s: %s.%s converted from string to %T", toolName, field, key, parsed)
			}
		}
	}
	return modified
}
