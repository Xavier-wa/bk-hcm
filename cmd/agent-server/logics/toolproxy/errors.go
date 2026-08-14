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

import "strings"

// toolErrorResult is the structured error payload returned by meta-tools.
type toolErrorResult struct {
	Success bool `json:"success"`
	Error   struct {
		Type           string                 `json:"type"`
		Message        string                 `json:"message"`
		Suggestion     map[string]interface{} `json:"suggestion,omitempty"`
		RequiredSchema map[string]interface{} `json:"required_schema,omitempty"`
	} `json:"error"`
}

// toolSuccessResult wraps a successful meta-tool payload.
type toolSuccessResult struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Tools   interface{} `json:"tools,omitempty"`
	Total   int         `json:"total,omitempty"`
	Schema  interface{} `json:"schema,omitempty"`
}

func buildErrorResult(errType, message string, suggestion map[string]interface{}) toolErrorResult {
	resp := toolErrorResult{Success: false}
	resp.Error.Type = errType
	resp.Error.Message = message
	if len(suggestion) > 0 {
		resp.Error.Suggestion = suggestion
	}
	return resp
}

// buildErrorResultWithSchema builds an error result that includes the tool's full schema.
// Use this when parameter validation fails or schema_token is invalid, so the LLM can
// immediately correct its parameters without an additional get_tool_schema round trip.
func buildErrorResultWithSchema(errType, message string, schema map[string]interface{}) toolErrorResult {
	resp := toolErrorResult{Success: false}
	resp.Error.Type = errType
	resp.Error.Message = message +
		"。完整 schema 已附在 required_schema 中，请按 schema 格式重新构造 parameters 并携带有效 schema_token 后重试。"
	resp.Error.RequiredSchema = schema
	return resp
}

func buildSuccessResult(result interface{}) toolSuccessResult {
	return toolSuccessResult{Success: true, Result: result}
}

func isPermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{"permission", "权限", "403", "forbidden", "denied", "unauthorized"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}
