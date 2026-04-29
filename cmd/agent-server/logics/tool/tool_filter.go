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
	"fmt"
	"strings"
	"sync"

	"hcm/cmd/agent-server/logics/embedding"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// ---------------------------------------------------------------------------
// lazyToolIndex — sync.Once lazy-built index over MCP ToolSets
// ---------------------------------------------------------------------------

// LazyToolIndex is a lazy-built index over MCP ToolSets.
type LazyToolIndex struct {
	mcpToolSets        []tool.ToolSet
	toolTags           map[string][]string
	scoreThreshold     float64
	topN               int
	queryContextWindow int

	once         sync.Once
	index        ToolIndex
	mcpToolNames map[string]bool // framework-prefixed names of all MCP tools
	buildOK      bool
}

// NewLazyToolIndex creates a new LazyToolIndex.
func NewLazyToolIndex(toolset *MCPToolSet, cfg *cc.AgentDynamicToolLoadingConfig,
	embedProvider *cc.AgentModelProvider) *LazyToolIndex {

	return &LazyToolIndex{
		mcpToolSets:        toolset.TS,
		toolTags:           cfg.ToolTags,
		scoreThreshold:     cfg.ScoreThreshold,
		topN:               cfg.TopN,
		queryContextWindow: cfg.QueryContextWindow,
		index:              buildToolIndex(cfg, embedProvider),
	}
}

// ensureBuild constructs the index on first call. Thread-safe via sync.Once.
func (l *LazyToolIndex) ensureBuild(ctx context.Context) {
	l.once.Do(func() {
		var metas []ToolMeta
		l.mcpToolNames = make(map[string]bool)

		for _, ts := range l.mcpToolSets {
			tools, err := safeGetTools(ctx, ts)
			if err != nil {
				logs.Warnf("dynamic tool loading: skip toolset %q: %v", ts.Name(), err)
				continue
			}
			prefix := ts.Name()
			for _, t := range tools {
				rawName := t.Declaration().Name
				frameworkName := rawName
				if prefix != "" {
					frameworkName = prefix + "_" + rawName
				}

				tags := l.toolTags[rawName]
				meta := extractToolMeta(t, tags)
				meta.Name = frameworkName
				meta.SearchText = buildSearchText(meta)

				metas = append(metas, meta)
				l.mcpToolNames[frameworkName] = true
			}
		}

		if len(metas) == 0 {
			logs.Errorf("dynamic tool loading: no tools extracted from any MCP toolset, index not built")
			return
		}

		if err := l.index.Build(ctx, metas); err != nil {
			logs.Errorf("dynamic tool loading: failed to build index (%d tools): %v", len(metas), err)
			return
		}
		l.buildOK = true
		logs.Infof("dynamic tool loading: tool index built: %d tools indexed", len(metas))
	})
}

// safeGetTools wraps ts.Tools(ctx) in a recover to handle panics from broken MCP connections.
func safeGetTools(ctx context.Context, ts tool.ToolSet) (tools []tool.Tool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return ts.Tools(ctx), nil
}

// ---------------------------------------------------------------------------
// makeDynamicToolFilter — returns tool.FilterFunc for Per-Run injection
// ---------------------------------------------------------------------------

// filterCacheEntry wraps the cached retrieval result.
// passAll == true means system-error degradation (all tools pass).
// passAll == false means whitelist holds the search result (may be empty).
type filterCacheEntry struct {
	passAll   bool
	whitelist map[string]bool
}

// MakeDynamicToolFilter constructs the ToolFilter function that performs
// per-invocation tool retrieval and caching.
func MakeDynamicToolFilter(lazy *LazyToolIndex) tool.FilterFunc {
	return func(ctx context.Context, t tool.Tool) bool {
		toolName := t.Declaration().Name

		// (a) Get Invocation — system error degrades to pass-all.
		inv, ok := agent.InvocationFromContext(ctx)
		if !ok || inv == nil {
			return true
		}

		// (b) Check cache hit.
		if entry, ok := agent.GetStateValue[filterCacheEntry](inv, constant.RetrievedToolsCacheKey); ok {
			if entry.passAll {
				return true
			}
			if !lazy.mcpToolNames[toolName] {
				return true // Skill tool → pass
			}
			return entry.whitelist[toolName]
		}

		// (c) Not an MCP tool → pass (but still need to do search for other MCP tools).
		// We do the search first, then check.

		// (d) First call: extract query, search, build whitelist, cache.
		query := extractQueryWithContext(inv, lazy.queryContextWindow)
		if query == "" {
			inv.SetState(constant.RetrievedToolsCacheKey, filterCacheEntry{passAll: true})
			return true
		}

		lazy.ensureBuild(ctx)
		if !lazy.buildOK {
			inv.SetState(constant.RetrievedToolsCacheKey, filterCacheEntry{passAll: true})
			return true
		}

		matches := lazy.index.Search(ctx, query, lazy.topN, lazy.scoreThreshold)
		if matches == nil {
			// Search failed (e.g., embedding dimension mismatch) → fall back to all tools
			inv.SetState(constant.RetrievedToolsCacheKey, filterCacheEntry{passAll: true})
			return true
		}

		whitelist := make(map[string]bool, len(matches))
		for _, m := range matches {
			whitelist[m.Name] = true
		}
		inv.SetState(constant.RetrievedToolsCacheKey, filterCacheEntry{whitelist: whitelist})

		if len(matches) > 0 {
			names := make([]string, len(matches))
			for i, m := range matches {
				names[i] = m.Name
			}
			logs.Infof("dynamic tool filter: query=%q matched %d tools: %v",
				query, len(matches), names)
		} else {
			logs.Infof("dynamic tool filter: query=%q matched 0 tools", query)
		}

		if !lazy.mcpToolNames[toolName] {
			return true // Skill tool → pass
		}
		return whitelist[toolName]
	}
}

// extractQueryWithContext builds a search query from a sliding window of recent
// user messages and an optional session summary. This ensures that short
// follow-ups like "继续" or "可以" still carry enough context to match tools.
//
// Query structure: [Summary] + [historical user messages...] + [current message]
func extractQueryWithContext(inv *agent.Invocation, contextWindow int) string {
	current := extractMessageText(inv.Message)

	if contextWindow <= 1 || inv.Session == nil {
		return current
	}

	events := inv.Session.GetEvents()
	history := extractRecentUserMessages(events, contextWindow-1)

	var segments []string

	if summary := extractSessionSummary(inv.Session); summary != "" {
		segments = append(segments, summary)
	}

	segments = append(segments, history...)
	segments = append(segments, current)

	return strings.TrimSpace(strings.Join(segments, " "))
}

// extractMessageText extracts plain text from a model.Message, supporting both
// plain-text (Content) and multimodal (ContentParts) formats.
func extractMessageText(msg model.Message) string {
	if msg.Content != "" {
		return msg.Content
	}
	var parts []string
	for _, p := range msg.ContentParts {
		if p.Type == model.ContentTypeText && p.Text != nil {
			parts = append(parts, *p.Text)
		}
	}
	return strings.Join(parts, " ")
}

// extractRecentUserMessages scans session events in reverse and returns
// up to limit user message texts in chronological (oldest-first) order.
func extractRecentUserMessages(events []event.Event, limit int) []string {
	var result []string
	for i := len(events) - 1; i >= 0 && len(result) < limit; i-- {
		e := &events[i]
		if e.Response == nil || !e.IsUserMessage() {
			continue
		}
		text := extractEventUserText(e)
		if text != "" {
			result = append(result, text)
		}
	}
	// Reverse to chronological order.
	for l, r := 0, len(result)-1; l < r; l, r = l+1, r-1 {
		result[l], result[r] = result[r], result[l]
	}
	return result
}

// extractEventUserText extracts the text content from a user-message event.
func extractEventUserText(e *event.Event) string {
	var parts []string
	for _, c := range e.Choices {
		msg := c.Message
		if msg.Role != model.RoleUser {
			msg = c.Delta
		}
		if msg.Role != model.RoleUser {
			continue
		}
		if msg.Content != "" {
			parts = append(parts, msg.Content)
		} else {
			for _, p := range msg.ContentParts {
				if p.Type == model.ContentTypeText && p.Text != nil {
					parts = append(parts, *p.Text)
				}
			}
		}
	}
	return strings.Join(parts, " ")
}

// extractSessionSummary returns the global session summary text, or empty
// string if unavailable.
func extractSessionSummary(sess *session.Session) string {
	if sess == nil {
		return ""
	}
	sess.SummariesMu.RLock()
	defer sess.SummariesMu.RUnlock()
	if sess.Summaries == nil {
		return ""
	}
	s, ok := sess.Summaries[session.SummaryFilterKeyAllContents]
	if !ok || s == nil {
		return ""
	}
	return strings.TrimSpace(s.Summary)
}

// ---------------------------------------------------------------------------
// buildToolIndex — factory for ToolIndex based on strategy config
// ---------------------------------------------------------------------------

func buildToolIndex(cfg *cc.AgentDynamicToolLoadingConfig, provider *cc.AgentModelProvider) ToolIndex {
	switch strings.ToLower(strings.TrimSpace(cfg.Strategy)) {
	case "keyword":
		return &KeywordIndex{}
	case "embedding":
		emb := embedding.BuildEmbeddingClient(provider, &cfg.Embedding)
		return NewEmbeddingIndex(emb)
	default:
		return &BM25Index{}
	}
}
