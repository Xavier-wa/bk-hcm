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
	"slices"
	"strings"

	"hcm/cmd/agent-server/logics/tool"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// SearchToolsTool implements the search_tools meta-tool.
type SearchToolsTool struct {
	proxy *ToolProxy
}

// NewSearchToolsTool creates a SearchToolsTool bound to the given proxy.
func NewSearchToolsTool(proxy *ToolProxy) *SearchToolsTool {
	return &SearchToolsTool{proxy: proxy}
}

// Declaration returns the tool declaration for search_tools.
func (t *SearchToolsTool) Declaration() *trpctool.Declaration {
	return &trpctool.Declaration{
		Name: constant.SearchToolsToolName,
		Description: "【探索未知工具时使用】根据任务描述语义搜索最相关的 MCP 工具，返回工具列表及完整 schema。" +
			"当用户未指定具体工具名、或你不确定该用哪个工具时调用。若用户已明确工具名，请改用 get_tool_schema。",
		InputSchema: buildSearchToolsSchema(),
	}
}

type searchToolsParams struct {
	Query string
	TopK  int
	Tag   string
}

type searchToolItem struct {
	Name           string                   `json:"name"`
	Description    string                   `json:"description"`
	RelevanceScore float64                  `json:"relevance_score"`
	Tags           []string                 `json:"tags,omitempty"`
	Schema         map[string]interface{}   `json:"schema"`
	Examples       []map[string]interface{} `json:"examples,omitempty"`
	SchemaToken    string                   `json:"schema_token"`
}

// Call performs semantic search over registered MCP tools.
func (t *SearchToolsTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	rid := rest.RidFromContext(ctx)
	registry, index, buildOK := t.proxy.SearchSnapshot()
	if !buildOK || registry == nil || index == nil {
		logs.Errorf("search_tools failed, err: index not ready, rid: %s", rid)
		return buildErrorResult("embedding_error", "工具索引尚未就绪，请稍后重试", nil), nil
	}

	params, errResp := parseSearchToolsArgs(jsonArgs, rid)
	if params == nil {
		return errResp, nil
	}

	topK, searchTopK, tagFilter := resolveSearchLimits(params, t.proxy.topN)
	logs.Infof("search_tools called, query=%s, top_k=%d, tag=%s, search_top_k=%d, rid: %s",
		params.Query, topK, tagFilter, searchTopK, rid)

	matches := index.Search(ctx, params.Query, searchTopK, t.proxy.scoreThreshold)
	items := buildSearchToolItems(t.proxy, registry, matches, topK, tagFilter)

	toolNames := make([]string, len(items))
	for i, item := range items {
		toolNames[i] = item.Name
	}
	logs.Infof("search_tools success, candidates=%d, returned=%d, tool_names=%v, rid: %s",
		len(matches), len(items), toolNames, rid)
	return map[string]interface{}{
		"success": true,
		"tools":   items,
		"total":   len(items),
	}, nil
}

func parseSearchToolsArgs(jsonArgs []byte, rid string) (*searchToolsParams, any) {
	var raw struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
		Tag   string `json:"tag"`
	}
	if err := json.Unmarshal(jsonArgs, &raw); err != nil {
		logs.Errorf("search_tools failed, err: %v, args=%s, rid: %s", err, string(jsonArgs), rid)
		return nil, buildErrorResult("invalid_parameters", "parameter parse failed: "+err.Error(), nil)
	}
	params := searchToolsParams{
		Query: strings.TrimSpace(raw.Query),
		TopK:  raw.TopK,
		Tag:   strings.TrimSpace(raw.Tag),
	}
	if params.Query == "" {
		logs.Errorf("search_tools failed, err: query required, rid: %s", rid)
		return nil, buildErrorResult("invalid_parameters", "parameter query is required",
			map[string]interface{}{
				"query": "查询业务的资源申请单据",
			})
	}
	return &params, nil
}

// resolveSearchLimits clamps topK and expands searchTopK when tag filtering is enabled.
func resolveSearchLimits(params *searchToolsParams, maxTopN int) (topK, searchTopK int, tagFilter string) {
	topK = params.TopK
	if topK <= 0 {
		topK = maxTopN
	}
	if topK > maxTopN {
		topK = maxTopN
	}
	searchTopK = topK
	tagFilter = params.Tag
	if tagFilter != "" {
		searchTopK = maxTopN * 3
	}
	return topK, searchTopK, tagFilter
}

func buildSearchToolItems(proxy *ToolProxy, registry *ToolRegistry, matches []tool.ToolMatch, topK int,
	tagFilter string) []searchToolItem {

	items := make([]searchToolItem, 0, len(matches))
	for _, match := range matches {
		meta, err := registry.GetTool(match.Name)
		if err != nil {
			continue
		}
		if tagFilter != "" && len(meta.Tags) > 0 && !slices.Contains(meta.Tags, tagFilter) {
			continue
		}
		items = append(items, searchToolItem{
			Name:           meta.Name,
			Description:    meta.Description,
			RelevanceScore: match.Score,
			Tags:           meta.Tags,
			Schema:         meta.Schema,
			Examples:       meta.Examples,
			SchemaToken:    proxy.GenerateSchemaToken(meta.Name, meta.Schema),
		})
		if len(items) >= topK {
			break
		}
	}
	return items
}

func buildSearchToolsSchema() *trpctool.Schema {
	return &trpctool.Schema{
		Type: "object",
		Properties: map[string]*trpctool.Schema{
			"query": {
				Type:        "string",
				Description: "搜索意图描述，例如「查询业务的资源申请单据」",
			},
			"top_k": {
				Type:        "integer",
				Description: "返回工具数量上限，默认 5",
			},
			"tag": {
				Type:        "string",
				Description: "可选，按工具 tag 过滤",
			},
		},
		Required: []string{"query"},
	}
}
