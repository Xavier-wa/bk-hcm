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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"hcm/cmd/agent-server/logics/auth"
	"hcm/cmd/agent-server/logics/tool"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/knowledge/embedder"
	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// ToolMetadata extends tool.ToolMeta with schema and usage fields.
type ToolMetadata struct {
	tool.ToolMeta
	Schema      map[string]interface{}   `json:"schema"`
	Examples    []map[string]interface{} `json:"examples,omitempty"`
	UsageCount  int                      `json:"usage_count,omitempty"`
	SuccessRate float64                  `json:"success_rate,omitempty"`
}

// ToolProxy manages MCP tool metadata, embedding index, and meta-tools.
type ToolProxy struct {
	registry       *ToolRegistry
	searchIndex    tool.ToolIndex
	toolTags       map[string][]string
	actualTools    map[string]trpctool.CallableTool
	mcpToolSets    *tool.MCPToolSet
	emb            embedder.Embedder
	topN           int
	scoreThreshold float64

	buildOK bool

	refreshMu   sync.RWMutex
	stopRefresh context.CancelFunc

	loadToken func(kt *kit.Kit) (string, error)

	// schemaTokenSecret is a process-lifetime random secret used to generate and verify schema tokens.
	// Tokens are HMAC-SHA256 signatures over (toolName + schemaContent), ensuring the LLM must
	// call search_tools or get_tool_schema before execute_tool in every interaction.
	schemaTokenSecret string

	searchTool  *SearchToolsTool
	schemaTool  *GetToolSchemaTool
	executeTool *ExecuteToolTool
	proxySet    *staticToolSet
}

// newSchemaTokenSecret generates a cryptographically random secret for schema token signing.
func newSchemaTokenSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("hcm-agent-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// GenerateSchemaToken returns a short HMAC-SHA256 token derived from the tool name and schema.
// The token is deterministic for a given (toolName, schema, secret) triple, allowing stateless
// verification in execute_tool without any session storage.
func (p *ToolProxy) GenerateSchemaToken(toolName string, schema map[string]interface{}) string {
	schemaBytes, _ := json.Marshal(schema)
	mac := hmac.New(sha256.New, []byte(p.schemaTokenSecret))
	mac.Write([]byte(toolName))
	mac.Write(schemaBytes)
	return hex.EncodeToString(mac.Sum(nil))[:constant.ProxySchemaTokenLen]
}

// VerifySchemaToken checks whether the provided token matches the expected token for the tool.
func (p *ToolProxy) VerifySchemaToken(toolName string, schema map[string]interface{}, token string) bool {
	if token == "" {
		return false
	}
	expected := p.GenerateSchemaToken(toolName, schema)
	return hmac.Equal([]byte(expected), []byte(token))
}

// NewToolProxy constructs a ToolProxy without loading MCP tools.
func NewToolProxy(mcpToolSets *tool.MCPToolSet, emb embedder.Embedder, toolProxyCfg *cc.AgentToolProxyConfig,
	loadToken func(kt *kit.Kit) (string, error)) *ToolProxy {

	index := tool.NewEmbeddingIndex(emb)
	p := &ToolProxy{
		searchIndex:       index,
		toolTags:          toolProxyCfg.ToolTags,
		mcpToolSets:       mcpToolSets,
		emb:               emb,
		topN:              toolProxyCfg.TopN,
		scoreThreshold:    toolProxyCfg.ScoreThreshold,
		loadToken:         loadToken,
		schemaTokenSecret: newSchemaTokenSecret(),
	}

	p.searchTool = NewSearchToolsTool(p)
	p.schemaTool = NewGetToolSchemaTool(p)
	p.executeTool = NewExecuteToolTool(p)
	p.proxySet = &staticToolSet{
		name: constant.ProxyToolSetName,
		tools: []trpctool.Tool{
			p.searchTool,
			p.schemaTool,
			p.executeTool,
		},
	}
	return p
}

// Build loads MCP tools and builds the embedding index synchronously.
func (p *ToolProxy) Build(kt *kit.Kit) error {
	if p.loadToken != nil {
		token, err := p.loadToken(kt)
		if err != nil {
			p.refreshMu.Lock()
			p.buildOK = false
			p.refreshMu.Unlock()
			return fmt.Errorf("load init access token: %v", err)
		}
		kt.Ctx = auth.WithAccessToken(kt.Ctx, token)
	}

	registry, actualTools, err := loadTools(kt, p.mcpToolSets, p.toolTags)
	if err != nil {
		p.refreshMu.Lock()
		p.buildOK = false
		p.refreshMu.Unlock()
		return err
	}

	searchIndex := buildSearchIndex(kt, registry.ExportToolMetas(), p.emb)

	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()
	p.registry = registry
	p.searchIndex = searchIndex
	p.actualTools = actualTools
	p.buildOK = true
	logs.Infof("tool proxy: build success, %d tools indexed, rid: %s", len(actualTools), kt.Rid)
	return nil
}

// buildForTest loads tools without reading global cc config (unit tests only).
func (p *ToolProxy) buildForTest(kt *kit.Kit) error {
	registry, actualTools, err := loadTools(kt, p.mcpToolSets, map[string][]string{})
	if err != nil {
		p.refreshMu.Lock()
		p.buildOK = false
		p.refreshMu.Unlock()
		return err
	}
	searchIndex := buildSearchIndex(kt, registry.ExportToolMetas(), p.emb)

	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()
	p.registry = registry
	p.searchIndex = searchIndex
	p.actualTools = actualTools
	p.buildOK = true
	return nil
}

// StartRefreshLoop starts a background goroutine to refresh the tool registry and index.
func (p *ToolProxy) StartRefreshLoop(kt *kit.Kit, interval time.Duration) error {
	if interval <= 0 || p.loadToken == nil {
		return fmt.Errorf("invalid interval or load token")
	}

	refreshCtx, cancel := context.WithCancel(kt.Ctx)
	p.stopRefresh = cancel
	newKit := kt.NewSubKitWithCtx(refreshCtx)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-refreshCtx.Done():
				return
			case <-ticker.C:
				p.refresh(newKit)
			}
		}
	}()

	return nil
}

func (p *ToolProxy) refresh(kt *kit.Kit) {
	if p.loadToken != nil {
		token, err := p.loadToken(kt)
		if err != nil {
			logs.Warnf("tool proxy refresh: load access token failed: %v, rid: %s", err, kt.Rid)
			return
		}
		kt.Ctx = auth.WithAccessToken(kt.Ctx, token)
	}

	registry, actualTools, err := loadTools(kt, p.mcpToolSets, p.toolTags)
	if err != nil {
		logs.Warnf("tool proxy refresh: load tools failed: %v, rid: %s", err, kt.Rid)
		return
	}
	searchIndex := buildSearchIndex(kt, registry.ExportToolMetas(), p.emb)

	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()
	p.registry = registry
	p.searchIndex = searchIndex
	p.actualTools = actualTools
	p.buildOK = true
	logs.Infof("tool proxy refresh success, %d tools indexed, rid: %s", len(actualTools), kt.Rid)
}

// StopRefresh stops the background refresh loop if running.
func (p *ToolProxy) StopRefresh() {
	if p.stopRefresh != nil {
		p.stopRefresh()
	}
}

// IsBuildOK reports whether the latest build succeeded.
func (p *ToolProxy) IsBuildOK() bool {
	p.refreshMu.RLock()
	defer p.refreshMu.RUnlock()
	return p.buildOK
}

// GetProxyToolSet returns the static ToolSet containing the 3 meta-tools.
func (p *ToolProxy) GetProxyToolSet() trpctool.ToolSet {
	return p.proxySet
}

// RegistrySnapshot returns registry under read lock for meta-tools.
func (p *ToolProxy) RegistrySnapshot() *ToolRegistry {
	p.refreshMu.RLock()
	defer p.refreshMu.RUnlock()
	return p.registry
}

// SearchSnapshot returns search index under read lock.
func (p *ToolProxy) SearchSnapshot() (*ToolRegistry, tool.ToolIndex, bool) {
	p.refreshMu.RLock()
	defer p.refreshMu.RUnlock()
	return p.registry, p.searchIndex, p.buildOK
}

// CallTool calls the actual tool.
func (p *ToolProxy) CallTool(ctx context.Context, toolName string, jsonArgs []byte) (any, error) {
	p.refreshMu.RLock()
	defer p.refreshMu.RUnlock()

	rid := rest.RidFromContext(ctx)
	actual, ok := p.actualTools[toolName]
	if !ok {
		logs.Errorf("tool %s not found, rid: %s", toolName, rid)
		return nil, fmt.Errorf("tool %s not found", toolName)
	}
	return actual.Call(ctx, jsonArgs)
}

func buildSearchIndex(kt *kit.Kit, metas []tool.ToolMeta, emb embedder.Embedder) tool.ToolIndex {
	keywordIndex := &tool.KeywordIndex{}
	if err := keywordIndex.Build(kt.Ctx, metas); err != nil {
		logs.Warnf("tool proxy: build keyword index failed: %v, rid: %s", err, kt.Rid)
	}

	embeddingIndex := tool.NewEmbeddingIndex(emb)
	if err := embeddingIndex.Build(kt.Ctx, metas); err != nil {
		logs.Warnf("tool proxy: build embedding index failed, fallback to keyword index: %v, rid: %s", err, kt.Rid)
		return keywordIndex
	}
	return &fallbackToolIndex{
		primary:  embeddingIndex,
		fallback: keywordIndex,
	}
}

type fallbackToolIndex struct {
	primary  tool.ToolIndex
	fallback tool.ToolIndex
}

func (idx *fallbackToolIndex) Build(ctx context.Context, metas []tool.ToolMeta) error {
	if err := idx.primary.Build(ctx, metas); err != nil {
		return err
	}
	return idx.fallback.Build(ctx, metas)
}

func (idx *fallbackToolIndex) Search(ctx context.Context, query string, topN int,
	scoreThreshold float64) []tool.ToolMatch {
	matches := idx.primary.Search(ctx, query, topN, scoreThreshold)
	if len(matches) > 0 {
		return matches
	}
	return idx.fallback.Search(ctx, query, topN, scoreThreshold)
}

// staticToolSet is an in-memory ToolSet for meta-tools.
type staticToolSet struct {
	name  string
	tools []trpctool.Tool
}

// Name returns the toolset name.
func (s *staticToolSet) Name() string {
	return s.name
}

// Tools returns the registered meta-tools.
func (s *staticToolSet) Tools(_ context.Context) []trpctool.Tool {
	return s.tools
}

// Close is a no-op for static meta-tools.
func (s *staticToolSet) Close() error {
	return nil
}
