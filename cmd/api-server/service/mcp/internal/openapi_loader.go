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

package internalmcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"hcm/pkg/cc"
	"hcm/pkg/logs"

	"gopkg.in/yaml.v3"
	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"

	"github.com/getkin/kin-openapi/openapi3"
)

// ToolBackend describes how to call a tool's local backend.
//
// Method 是 HTTP 方法（大写），Path 是 api-server 本地 mux 上的完整路径
// （如 "/api/v1/cloud/cvms/list"），支持 `{var}` 模板占位由 dispatcher 替换。
type ToolBackend struct {
	Method string
	Path   string
}

// LoadResult 是 OpenAPI yaml 加载的输出。
type LoadResult struct {
	Tools    []*mcpsdk.Tool
	Backends map[string]ToolBackend
}

// LoadOpenAPISpec parses an OpenAPI 3.0 yaml file (exported from BK API Gateway
// resource list) and returns MCP tools plus their backend routing table.
//
// Filtering rules:
//   - When includeOpIDs is empty, all operations with a non-empty operationId
//     are exported.
//   - Otherwise only operations whose operationId matches an entry (exact match
//     or glob pattern, see path.Match) are exported.
//
// 解析失败、文件缺失或一个工具都没解析出来均返回 error。
func LoadOpenAPISpec(specPath string, includeOpIDs []string) (*LoadResult, error) {
	specPath = strings.TrimSpace(specPath)
	if specPath == "" {
		return nil, fmt.Errorf("openapi spec path is empty")
	}

	raw, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("read openapi spec %q failed: %w", specPath, err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse openapi spec %q failed: %w", specPath, err)
	}

	paths, ok := doc["paths"].(map[string]interface{})
	if !ok || len(paths) == 0 {
		return nil, fmt.Errorf("openapi spec %q has no `paths` section", specPath)
	}

	matcher := newOpIDMatcher(includeOpIDs)
	out := &LoadResult{Backends: make(map[string]ToolBackend)}

	// 按 path key 排序，让加载结果稳定（便于测试和日志）。
	pathKeys := make([]string, 0, len(paths))
	for k := range paths {
		pathKeys = append(pathKeys, k)
	}
	sort.Strings(pathKeys)

	for _, pathKey := range pathKeys {
		methods, ok := paths[pathKey].(map[string]interface{})
		if !ok {
			continue
		}
		for method, raw := range methods {
			op, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			tool, backend, err := buildToolFromOperation(pathKey, method, op)
			if err != nil {
				logs.Warnf("openapi loader: skip path=%s method=%s, err: %v", pathKey, method, err)
				continue
			}
			if tool == nil {
				continue
			}
			if !matcher.match(tool.Name) {
				continue
			}
			if _, exists := out.Backends[tool.Name]; exists {
				logs.Warnf("openapi loader: duplicate operationId %q, later definition ignored", tool.Name)
				continue
			}
			out.Tools = append(out.Tools, tool)
			out.Backends[tool.Name] = backend
		}
	}

	if len(out.Tools) == 0 {
		return nil, fmt.Errorf("openapi spec %q produced 0 tools (check operationId / includeOperationIDs)",
			specPath)
	}
	return out, nil
}

// buildToolFromOperation converts a single OpenAPI path operation into an MCP tool.
func buildToolFromOperation(pathKey, method string, op map[string]interface{}) (*mcpsdk.Tool, ToolBackend, error) {
	name := strings.TrimSpace(asString(op["operationId"]))
	if name == "" {
		return nil, ToolBackend{}, fmt.Errorf("missing operationId")
	}

	description := strings.TrimSpace(asString(op["description"]))
	if description == "" {
		description = strings.TrimSpace(asString(op["summary"]))
	}

	inputSchema, err := buildInputSchema(op)
	if err != nil {
		return nil, ToolBackend{}, fmt.Errorf("build inputSchema: %w", err)
	}

	if example := extractResponseExample(op); example != "" {
		description = appendExample(description, example)
	}

	backend := extractBackend(pathKey, method, op)

	tool := &mcpsdk.Tool{
		Name:           name,
		Description:    description,
		InputSchema:    inputSchema,
		RawInputSchema: rawJSON(inputSchema),
	}
	return tool, backend, nil
}

// buildInputSchema builds a grouped JSON Schema for an MCP tool, matching the
// format used by the BK API Gateway MCP proxy:
//
//	{
//	  "type": "object",
//	  "properties": {
//	    "body_param":  { ...requestBody schema... },
//	    "path_param":  { "type":"object", "properties": { "<name>": {...} }, "required": [...] }
//	  },
//	  "required": ["path_param"]   // only when path params exist
//	}
//
// body_param is omitted when the operation has no requestBody.
// path_param is omitted when the operation has no path/query parameters.
func buildInputSchema(op map[string]interface{}) (*openapi3.Schema, error) {
	top := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []interface{}{},
	}
	topProps := top["properties"].(map[string]interface{})

	// body_param — from requestBody.
	body := baseSchemaFromRequestBody(op)
	hasBody := len(body["properties"].(map[string]interface{})) > 0 ||
		len(body["required"].([]interface{})) > 0
	if hasBody {
		body["description"] = "HTTP request body in JSON format, containing the main data payload for the API request."
		topProps["body_param"] = body
	}

	// path_param — from OpenAPI parameters (path / query / header).
	pathParamProps := map[string]interface{}{}
	pathParamRequired := []interface{}{}
	if params, ok := op["parameters"].([]interface{}); ok {
		for _, item := range params {
			p, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			pName := strings.TrimSpace(asString(p["name"]))
			if pName == "" {
				continue
			}
			pSchema, _ := p["schema"].(map[string]interface{})
			merged := make(map[string]interface{})
			for k, v := range pSchema {
				merged[k] = v
			}
			if desc := strings.TrimSpace(asString(p["description"])); desc != "" {
				merged["description"] = desc
			}
			if example, ok := p["example"]; ok {
				merged["example"] = example
			}
			// path_param 的每个字段内嵌 required 字段（与网关格式一致）。
			if asBool(p["required"]) {
				merged["required"] = []interface{}{pName}
				pathParamRequired = append(pathParamRequired, pName)
			}
			pathParamProps[pName] = merged
		}
	}
	if len(pathParamProps) > 0 {
		pathParamSchema := map[string]interface{}{
			"description": "URL path parameters, used to identify specific resources in the URL path (e.g., /users/{id}).",
			"type":        "object",
			"properties":  pathParamProps,
			"required":    pathParamRequired,
		}
		topProps["path_param"] = pathParamSchema
		addRequired(top, "path_param")
	}

	// JSON 化后反序列化到 openapi3.Schema，保持类型/属性嵌套结构。
	b, err := json.Marshal(top)
	if err != nil {
		return nil, err
	}
	out := &openapi3.Schema{}
	if err := out.UnmarshalJSON(b); err != nil {
		return nil, err
	}
	return out, nil
}

// baseSchemaFromRequestBody returns the request body schema as a plain map
// （type/properties/required 三段保证存在），或者返回一个空 object schema。
func baseSchemaFromRequestBody(op map[string]interface{}) map[string]interface{} {
	defaultSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []interface{}{},
	}

	rb, ok := op["requestBody"].(map[string]interface{})
	if !ok {
		return defaultSchema
	}
	content, ok := rb["content"].(map[string]interface{})
	if !ok {
		return defaultSchema
	}
	appJSON, ok := content["application/json"].(map[string]interface{})
	if !ok {
		return defaultSchema
	}
	s, ok := appJSON["schema"].(map[string]interface{})
	if !ok {
		return defaultSchema
	}

	out := deepCopyMap(s)
	if _, ok := out["type"]; !ok {
		out["type"] = "object"
	}
	if _, ok := out["properties"]; !ok {
		out["properties"] = map[string]interface{}{}
	}
	if _, ok := out["properties"].(map[string]interface{}); !ok {
		out["properties"] = map[string]interface{}{}
	}
	if _, ok := out["required"]; !ok {
		out["required"] = []interface{}{}
	}
	return out
}

func addRequired(schema map[string]interface{}, name string) {
	req, _ := schema["required"].([]interface{})
	for _, item := range req {
		if asString(item) == name {
			return
		}
	}
	schema["required"] = append(req, name)
}

// extractResponseExample returns the 200 response example as a JSON string.
// Returns "" if no example is present.
func extractResponseExample(op map[string]interface{}) string {
	responses, ok := op["responses"].(map[string]interface{})
	if !ok {
		return ""
	}
	r200, ok := responses["200"].(map[string]interface{})
	if !ok {
		return ""
	}
	content, ok := r200["content"].(map[string]interface{})
	if !ok {
		return ""
	}
	appJSON, ok := content["application/json"].(map[string]interface{})
	if !ok {
		return ""
	}
	example, ok := appJSON["example"]
	if !ok {
		return ""
	}
	b, err := json.Marshal(example)
	if err != nil {
		return ""
	}
	return string(b)
}

func appendExample(description, example string) string {
	const header = "\n\nExample response:\n"
	if description == "" {
		return strings.TrimPrefix(header, "\n\n") + example
	}
	return description + header + example
}

var envPlaceholderRegexp = regexp.MustCompile(`\{env\.[A-Za-z0-9_]+\}`)

// extractBackend pulls method/path from x-bk-apigateway-resource.backend,
// falling back to the path key + HTTP method when not present.
func extractBackend(pathKey, method string, op map[string]interface{}) ToolBackend {
	backend := ToolBackend{
		Method: strings.ToUpper(strings.TrimSpace(method)),
		Path:   pathKey,
	}

	resource, ok := op["x-bk-apigateway-resource"].(map[string]interface{})
	if !ok {
		return backend
	}
	rb, ok := resource["backend"].(map[string]interface{})
	if !ok {
		return backend
	}
	if m := strings.TrimSpace(asString(rb["method"])); m != "" {
		backend.Method = strings.ToUpper(m)
	}
	if p := strings.TrimSpace(asString(rb["path"])); p != "" {
		backend.Path = p
	}

	// Strip leading {env.url_path_prefix} or {env.api_sub_path} placeholders.
	backend.Path = envPlaceholderRegexp.ReplaceAllString(backend.Path, "")
	if !strings.HasPrefix(backend.Path, "/") {
		backend.Path = "/" + backend.Path
	}
	return backend
}

// opIDMatcher matches operationId against a whitelist (exact + glob).
type opIDMatcher struct {
	patterns []string
}

func newOpIDMatcher(patterns []string) *opIDMatcher {
	cleaned := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	return &opIDMatcher{patterns: cleaned}
}

func (m *opIDMatcher) match(name string) bool {
	if m == nil || len(m.patterns) == 0 {
		return true
	}
	for _, pattern := range m.patterns {
		if pattern == name {
			return true
		}
		if ok, err := path.Match(pattern, name); err == nil && ok {
			return true
		}
	}
	return false
}

// internalOnlyToolFromCfg converts a cc.InternalOnlyToolSetting into an MCP tool
// plus its backend descriptor.
func internalOnlyToolFromCfg(cfg cc.InternalOnlyToolSetting) (*mcpsdk.Tool, ToolBackend, error) {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return nil, ToolBackend{}, fmt.Errorf("internalOnlyTools entry missing name")
	}
	if strings.TrimSpace(cfg.Backend.Method) == "" || strings.TrimSpace(cfg.Backend.Path) == "" {
		return nil, ToolBackend{}, fmt.Errorf("internalOnlyTools[%s] missing backend method/path", name)
	}

	schemaSrc := cfg.InputSchema
	if len(schemaSrc) == 0 {
		schemaSrc = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []interface{}{},
		}
	}
	b, err := json.Marshal(schemaSrc)
	if err != nil {
		return nil, ToolBackend{}, fmt.Errorf("internalOnlyTools[%s] marshal inputSchema: %w", name, err)
	}
	schema := &openapi3.Schema{}
	if err := schema.UnmarshalJSON(b); err != nil {
		return nil, ToolBackend{}, fmt.Errorf("internalOnlyTools[%s] decode inputSchema: %w", name, err)
	}

	tool := &mcpsdk.Tool{
		Name:           name,
		Description:    strings.TrimSpace(cfg.Description),
		InputSchema:    schema,
		RawInputSchema: rawJSON(schema),
	}
	backend := ToolBackend{
		Method: strings.ToUpper(strings.TrimSpace(cfg.Backend.Method)),
		Path:   cfg.Backend.Path,
	}
	return tool, backend, nil
}

// utility helpers ---------------------------------------------------------

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func asBool(v interface{}) bool {
	if v == nil {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// deepCopyMap returns a deep copy of a map[string]interface{}, recursing into
// nested maps and slices so we can mutate the copy without affecting the source.
func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return deepCopyMap(t)
	case []interface{}:
		cp := make([]interface{}, len(t))
		for i, item := range t {
			cp[i] = deepCopyValue(item)
		}
		return cp
	default:
		return v
	}
}

// rawJSON marshals any value to json.RawMessage, returning nil on failure.
func rawJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
