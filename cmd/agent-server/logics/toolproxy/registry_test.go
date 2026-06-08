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
	"testing"

	"hcm/cmd/agent-server/logics/tool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolRegistry_RegisterGetUpdateDelete(t *testing.T) {
	reg := NewToolRegistry()

	meta := &ToolMetadata{
		ToolMeta: tool.ToolMeta{
			Name:        "mcp_search_code",
			Description: "搜索代码",
			Tags:        []string{"code"},
			SearchText:  "mcp_search_code 搜索代码",
		},
		Schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"query"},
		},
	}
	reg.Register(meta)

	got, err := reg.GetTool("mcp_search_code")
	require.NoError(t, err)
	assert.Equal(t, "搜索代码", got.Description)
	assert.Equal(t, []string{"code"}, got.Tags)

	meta.Description = "搜索代码库"
	require.NoError(t, reg.Update(meta))
	got, err = reg.GetTool("mcp_search_code")
	require.NoError(t, err)
	assert.Equal(t, "搜索代码库", got.Description)

	_, err = reg.GetTool("nonexistent")
	assert.Error(t, err)

	reg.Delete("mcp_search_code")
	_, err = reg.GetTool("mcp_search_code")
	assert.Error(t, err)

	assert.Empty(t, reg.ListTools())
}

func TestToolRegistry_RegisterSkipsInvalidMeta(t *testing.T) {
	reg := NewToolRegistry()

	reg.Register(nil)
	reg.Register(&ToolMetadata{})
	reg.Register(&ToolMetadata{ToolMeta: tool.ToolMeta{Name: ""}})

	assert.Empty(t, reg.ListTools())
}

func TestToolRegistry_ExportToolMetas(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&ToolMetadata{
		ToolMeta: tool.ToolMeta{Name: "tool_a", SearchText: "tool_a"},
	})
	reg.Register(&ToolMetadata{
		ToolMeta: tool.ToolMeta{Name: "tool_b", SearchText: "tool_b"},
	})

	metas := reg.ExportToolMetas()
	assert.Len(t, metas, 2)
	names := make(map[string]bool)
	for _, m := range metas {
		names[m.Name] = true
	}
	assert.True(t, names["tool_a"])
	assert.True(t, names["tool_b"])
}

func TestToolRegistry_GetToolByAlias(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&ToolMetadata{
		ToolMeta: tool.ToolMeta{Name: "mcp/search_code"},
	})
	reg.registerToolAlias("search_code", "mcp/search_code")

	got, err := reg.GetTool("search_code")
	require.NoError(t, err)
	assert.Equal(t, "mcp/search_code", got.Name)

	got, err = reg.GetTool("mcp/search_code")
	require.NoError(t, err)
	assert.Equal(t, "mcp/search_code", got.Name)
}

func TestToolRegistry_RegisterToolAlias(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&ToolMetadata{
		ToolMeta: tool.ToolMeta{Name: "mcp/search_code"},
	})
	reg.Register(&ToolMetadata{
		ToolMeta: tool.ToolMeta{Name: "other/search_code"},
	})

	reg.registerToolAlias("search_code", "mcp/search_code")
	got, err := reg.GetTool("search_code")
	require.NoError(t, err)
	assert.Equal(t, "mcp/search_code", got.Name)

	reg.registerToolAlias("search_code", "other/search_code")
	got, err = reg.GetTool("search_code")
	require.NoError(t, err)
	assert.Equal(t, "mcp/search_code", got.Name)

	reg.registerToolAlias("", "mcp/search_code")
	reg.registerToolAlias("mcp/search_code", "mcp/search_code")
	_, err = reg.GetTool("mcp/search_code")
	require.NoError(t, err)
}
