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
	"fmt"
	"sync"

	"hcm/cmd/agent-server/logics/tool"
	"hcm/pkg/kit"
	"hcm/pkg/logs"

	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// ToolRegistry maintains tool metadata with concurrent-safe access.
// Canonical tool names use the {toolsetName}_{rawName} convention; aliases map
// unprefixed rawName to canonical when globally unique (e.g. skill-specified names).
type ToolRegistry struct {
	mu      sync.RWMutex
	tools   map[string]*ToolMetadata
	aliases map[string]string
}

// NewToolRegistry creates an empty ToolRegistry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:   make(map[string]*ToolMetadata),
		aliases: make(map[string]string),
	}
}

// Register adds or replaces a tool in the registry.
func (r *ToolRegistry) Register(meta *ToolMetadata) {
	if meta == nil || meta.Name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[meta.Name] = meta
}

// registerToolAlias maps rawName to frameworkName when unprefixed lookup is unambiguous.
// Callable lookup uses frameworkName only; registry.GetTool resolves aliases before execute.
func (r *ToolRegistry) registerToolAlias(alias, name string) {
	if alias == "" || alias == name {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.aliases[alias]; ok && existing != name {
		logs.Warnf("skip tool alias %q: conflicts with %q and %q", alias, existing, name)
		return
	}
	r.aliases[alias] = name
}

// Update updates an existing tool metadata entry.
func (r *ToolRegistry) Update(meta *ToolMetadata) error {
	if meta == nil || meta.Name == "" {
		return fmt.Errorf("tool metadata is nil or name is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tools[meta.Name]; !ok {
		return fmt.Errorf("tool %s not found", meta.Name)
	}
	r.tools[meta.Name] = meta
	return nil
}

// Delete removes a tool from the registry.
func (r *ToolRegistry) Delete(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// GetTool returns metadata for the given tool name.
// Accepts canonical names ({toolset}_{raw}) or unprefixed rawName when registered as an alias.
func (r *ToolRegistry) GetTool(name string) (*ToolMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	canonical, ok := r.lookupFrameworkNameLocked(name)
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return r.tools[canonical], nil
}

func (r *ToolRegistry) lookupFrameworkNameLocked(name string) (string, bool) {
	if _, ok := r.tools[name]; ok {
		return name, true
	}
	if canonical, ok := r.aliases[name]; ok {
		if _, ok := r.tools[canonical]; ok {
			return canonical, true
		}
	}
	return "", false
}

// ListTools returns a snapshot of all registered tools.
func (r *ToolRegistry) ListTools() []*ToolMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*ToolMetadata, 0, len(r.tools))
	for _, meta := range r.tools {
		result = append(result, meta)
	}
	return result
}

// ExportToolMetas exports []tool.ToolMeta for index building.
func (r *ToolRegistry) ExportToolMetas() []tool.ToolMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	metas := make([]tool.ToolMeta, 0, len(r.tools))
	for _, meta := range r.tools {
		metas = append(metas, meta.ToolMeta)
	}
	return metas
}

// loadTools loads MCP tools into a new registry and callable-tool index.
func loadTools(kt *kit.Kit, mcpToolSets *tool.MCPToolSet, toolTags map[string][]string) (
	*ToolRegistry, map[string]trpctool.CallableTool, error) {

	if mcpToolSets == nil {
		return nil, nil, fmt.Errorf("mcp toolsets is nil")
	}

	if toolTags == nil {
		toolTags = map[string][]string{}
	}

	logs.Infof("load tools start, toolset_count: %d, rid: %s", len(mcpToolSets.TS), kt.Rid)

	registry := NewToolRegistry()
	actualTools := make(map[string]trpctool.CallableTool)
	for _, ts := range mcpToolSets.TS {
		// context 必须包含 access_token 或 bk_ticket
		tools, err := safeGetTools(kt.Ctx, ts)
		if err != nil {
			logs.Warnf("load tools skip toolset, prefix: %s, err: %v, rid: %s", ts.Name(), err, kt.Rid)
			continue
		}
		prefix := ts.Name()
		logs.Infof("load tools from toolset, prefix: %q, tool_count: %d, rid: %s", prefix, len(tools), kt.Rid)
		for _, t := range tools {
			rawName := t.Declaration().Name
			frameworkName := rawName
			if prefix != "" {
				frameworkName = prefix + "/" + rawName
			}

			tags := toolTags[rawName]
			meta := tool.ExtractToolMeta(t, frameworkName, tags)
			toolWithMeta := &ToolMetadata{
				ToolMeta: meta,
				Schema:   inputSchemaToMap(t.Declaration().InputSchema),
			}

			registry.Register(toolWithMeta)
			registry.registerToolAlias(rawName, frameworkName)
			if c, ok := t.(trpctool.CallableTool); ok {
				actualTools[frameworkName] = c
			}

			logs.Infof("register tool, raw_name: %q, framework_name: %q, tags: %v, rid: %s",
				rawName, frameworkName, tags, kt.Rid)
		}
	}

	logs.Infof("load tools success, tool_count: %d, callable_count: %d, alias_count: %d, rid: %s",
		len(registry.tools), len(actualTools), len(registry.aliases), kt.Rid)

	return registry, actualTools, nil
}

// safeGetTools wraps ts.Tools(ctx) in a recover to handle panics from broken MCP connections.
func safeGetTools(ctx context.Context, ts trpctool.ToolSet) (tools []trpctool.Tool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return ts.Tools(ctx), nil
}

func inputSchemaToMap(schema *trpctool.Schema) map[string]interface{} {
	if schema == nil {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	result := map[string]interface{}{
		"type": schema.Type,
	}
	if schema.Description != "" {
		result["description"] = schema.Description
	}
	if len(schema.Properties) > 0 {
		props := make(map[string]interface{}, len(schema.Properties))
		for name, s := range schema.Properties {
			if s == nil {
				continue
			}
			props[name] = inputSchemaToMap(s)
		}
		result["properties"] = props
	}
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}
	if schema.Items != nil {
		result["items"] = inputSchemaToMap(schema.Items)
	}
	if schema.AdditionalProperties != nil {
		result["additionalProperties"] = schema.AdditionalProperties
	}
	if schema.Default != nil {
		result["default"] = schema.Default
	}
	if len(schema.Enum) > 0 {
		result["enum"] = schema.Enum
	}
	if schema.Ref != "" {
		result["$ref"] = schema.Ref
	}
	if len(schema.Defs) > 0 {
		defs := make(map[string]interface{}, len(schema.Defs))
		for name, s := range schema.Defs {
			if s == nil {
				continue
			}
			defs[name] = inputSchemaToMap(s)
		}
		result["$defs"] = defs
	}
	return result
}
