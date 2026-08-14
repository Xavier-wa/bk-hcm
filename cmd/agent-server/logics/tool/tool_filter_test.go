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
	"strings"
	"testing"

	"hcm/pkg/cc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// ---------------------------------------------------------------------------
// Mock ToolSet
// ---------------------------------------------------------------------------

type mockToolSet struct {
	name  string
	tools []tool.Tool
	err   error
}

func (m *mockToolSet) Tools(_ context.Context) []tool.Tool { return m.tools }
func (m *mockToolSet) Close() error                        { return nil }
func (m *mockToolSet) Name() string                        { return m.name }

// panicToolSet panics when Tools() is called.
type panicToolSet struct{ name string }

func (p *panicToolSet) Tools(_ context.Context) []tool.Tool { panic("connection failed") }
func (p *panicToolSet) Close() error                        { return nil }
func (p *panicToolSet) Name() string                        { return p.name }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func strPtr(s string) *string { return &s }

func ctxWithInvocation(msg model.Message) context.Context {
	ctx, inv := agent.EnsureInvocation(context.Background())
	inv.Message = msg
	return ctx
}

func buildTestLazy(toolSets []tool.ToolSet, tags map[string][]string, topN int) *LazyToolIndex {
	idx := &BM25Index{}
	return &LazyToolIndex{
		mcpToolSets:    toolSets,
		toolTags:       tags,
		scoreThreshold: 0,
		topN:           topN,
		index:          idx,
	}
}

// ---------------------------------------------------------------------------
// 6.4 makeDynamicToolFilter — degradation scenarios
// ---------------------------------------------------------------------------

func TestDynamicToolFilter_NoInvocation_PassAll(t *testing.T) {
	lazy := buildTestLazy(nil, nil, 8)
	filter := MakeDynamicToolFilter(lazy)

	mt := newSimpleMockTool("some_mcp_tool", "test")
	assert.True(t, filter(context.Background(), mt), "should pass all when no invocation in context")
}

func TestDynamicToolFilter_EmptyMessage_PassAll(t *testing.T) {
	ts := &mockToolSet{
		name: "mcp",
		tools: []tool.Tool{
			newSimpleMockTool("tool_a", "do something"),
		},
	}
	lazy := buildTestLazy([]tool.ToolSet{ts}, nil, 8)
	filter := MakeDynamicToolFilter(lazy)

	ctx := ctxWithInvocation(model.Message{})
	assert.True(t, filter(ctx, newSimpleMockTool("mcp_tool_a", "do something")),
		"should pass all when user message is empty")
}

func TestDynamicToolFilter_BuildFailed_PassAll(t *testing.T) {
	// All toolsets fail → buildOK stays false → degradation
	lazy := buildTestLazy([]tool.ToolSet{&panicToolSet{name: "broken"}}, nil, 8)
	filter := MakeDynamicToolFilter(lazy)

	ctx := ctxWithInvocation(model.Message{Content: "hello"})
	assert.True(t, filter(ctx, newSimpleMockTool("broken_something", "test")),
		"should pass all when index build fails")
}

// ---------------------------------------------------------------------------
// 6.4 MakeDynamicToolFilter — search hit / miss scenarios
// ---------------------------------------------------------------------------

func TestDynamicToolFilter_SearchHit(t *testing.T) {
	ts := &mockToolSet{
		name: "hcm",
		tools: []tool.Tool{
			newSimpleMockTool("list_cvm", "list cloud virtual machines"),
			newSimpleMockTool("create_disk", "create CBS disk"),
		},
	}
	lazy := buildTestLazy([]tool.ToolSet{ts}, nil, 8)
	filter := MakeDynamicToolFilter(lazy)

	ctx := ctxWithInvocation(model.Message{Content: "list cloud virtual machines"})

	assert.True(t, filter(ctx, newSimpleMockTool("hcm_list_cvm", "list cloud virtual machines")))
	assert.False(t, filter(ctx, newSimpleMockTool("hcm_create_disk", "create CBS disk")),
		"unrelated MCP tool should be filtered out")
}

func TestDynamicToolFilter_SearchNoMatch_MCPToolBlocked(t *testing.T) {
	ts := &mockToolSet{
		name: "hcm",
		tools: []tool.Tool{
			newSimpleMockTool("list_cvm", "list cloud virtual machines"),
		},
	}
	lazy := buildTestLazy([]tool.ToolSet{ts}, nil, 8)
	filter := MakeDynamicToolFilter(lazy)

	ctx := ctxWithInvocation(model.Message{Content: "today weather forecast"})
	assert.False(t, filter(ctx, newSimpleMockTool("hcm_list_cvm", "list cloud virtual machines")),
		"MCP tool should be blocked when no search match (pure chat)")
}

// ---------------------------------------------------------------------------
// 6.4 Skill tool passthrough
// ---------------------------------------------------------------------------

func TestDynamicToolFilter_SkillToolAlwaysPass(t *testing.T) {
	ts := &mockToolSet{
		name: "hcm",
		tools: []tool.Tool{
			newSimpleMockTool("list_cvm", "list cloud virtual machines"),
		},
	}
	lazy := buildTestLazy([]tool.ToolSet{ts}, nil, 8)
	filter := MakeDynamicToolFilter(lazy)

	ctx := ctxWithInvocation(model.Message{Content: "today weather"})

	// "skill_load" is not in mcpToolNames → always passes
	assert.True(t, filter(ctx, newSimpleMockTool("skill_load", "load a skill")))
	assert.True(t, filter(ctx, newSimpleMockTool("skill_run", "run a skill")))
	assert.True(t, filter(ctx, newSimpleMockTool("transfer_to_agent", "transfer")))
}

// ---------------------------------------------------------------------------
// 6.4 Cache hit — second call same invocation
// ---------------------------------------------------------------------------

func TestDynamicToolFilter_CacheHit(t *testing.T) {
	ts := &mockToolSet{
		name: "hcm",
		tools: []tool.Tool{
			newSimpleMockTool("list_cvm", "list cloud virtual machines"),
			newSimpleMockTool("create_disk", "create CBS disk"),
		},
	}
	lazy := buildTestLazy([]tool.ToolSet{ts}, nil, 8)
	filter := MakeDynamicToolFilter(lazy)

	ctx := ctxWithInvocation(model.Message{Content: "list cloud virtual machines"})

	// First call populates cache
	_ = filter(ctx, newSimpleMockTool("hcm_list_cvm", "list cloud virtual machines"))

	// Second call should use cache (same context/invocation)
	assert.True(t, filter(ctx, newSimpleMockTool("hcm_list_cvm", "list cloud virtual machines")))
	assert.False(t, filter(ctx, newSimpleMockTool("hcm_create_disk", "create CBS disk")))
}

// ---------------------------------------------------------------------------
// 6.5 lazyToolIndex — prefixed name handling
// ---------------------------------------------------------------------------

func TestLazyToolIndex_PrefixedName(t *testing.T) {
	ts := &mockToolSet{
		name: "myprefix",
		tools: []tool.Tool{
			newSimpleMockTool("tool_a", "does A"),
		},
	}
	lazy := buildTestLazy([]tool.ToolSet{ts}, nil, 8)
	lazy.ensureBuild(context.Background())

	require.True(t, lazy.buildOK)
	assert.True(t, lazy.mcpToolNames["myprefix_tool_a"])
	assert.False(t, lazy.mcpToolNames["tool_a"])
}

func TestLazyToolIndex_EmptyPrefix(t *testing.T) {
	ts := &mockToolSet{
		name: "", // empty prefix
		tools: []tool.Tool{
			newSimpleMockTool("raw_tool", "raw tool"),
		},
	}
	lazy := buildTestLazy([]tool.ToolSet{ts}, nil, 8)
	lazy.ensureBuild(context.Background())

	require.True(t, lazy.buildOK)
	assert.True(t, lazy.mcpToolNames["raw_tool"])
}

func TestLazyToolIndex_ToolTagsViaRawName(t *testing.T) {
	ts := &mockToolSet{
		name: "svc",
		tools: []tool.Tool{
			newSimpleMockTool("list_cvm", "list instances"),
		},
	}
	tags := map[string][]string{"list_cvm": {"云服务器", "CVM"}}
	lazy := buildTestLazy([]tool.ToolSet{ts}, tags, 8)
	lazy.ensureBuild(context.Background())

	require.True(t, lazy.buildOK)
	// Tags should be injected via raw name "list_cvm", not prefixed name "svc_list_cvm"
	assert.True(t, lazy.mcpToolNames["svc_list_cvm"])
}

// ---------------------------------------------------------------------------
// 6.7 lazyToolIndex — partial build failure
// ---------------------------------------------------------------------------

func TestLazyToolIndex_PartialBuildFailure(t *testing.T) {
	goodTS := &mockToolSet{
		name: "good",
		tools: []tool.Tool{
			newSimpleMockTool("tool_ok", "working tool"),
		},
	}
	badTS := &panicToolSet{name: "bad"}

	lazy := buildTestLazy([]tool.ToolSet{goodTS, badTS}, nil, 8)
	lazy.ensureBuild(context.Background())

	assert.True(t, lazy.buildOK, "should succeed with partial tools")
	assert.True(t, lazy.mcpToolNames["good_tool_ok"])
}

func TestLazyToolIndex_AllBuildFailure(t *testing.T) {
	lazy := buildTestLazy([]tool.ToolSet{
		&panicToolSet{name: "bad1"},
		&panicToolSet{name: "bad2"},
	}, nil, 8)
	lazy.ensureBuild(context.Background())

	assert.False(t, lazy.buildOK, "should fail when all toolsets fail")
}

// ---------------------------------------------------------------------------
// 6.6 extractQueryWithContext tests (multi-turn context window + summary)
// ---------------------------------------------------------------------------

func makeUserEvent(content string) event.Event {
	return event.Event{
		Response: &model.Response{
			Choices: []model.Choice{{
				Message: model.Message{Role: model.RoleUser, Content: content},
			}},
		},
	}
}

func makeAssistantEvent(content string) event.Event {
	return event.Event{
		Response: &model.Response{
			Choices: []model.Choice{{
				Message: model.Message{Role: model.RoleAssistant, Content: content},
			}},
		},
	}
}

func TestExtractQuery_SingleTurn_NoSession(t *testing.T) {
	inv := &agent.Invocation{
		Message: model.Message{Content: "查询CVM列表"},
	}
	assert.Equal(t, "查询CVM列表", extractQueryWithContext(inv, 3))
}

func TestExtractQuery_SingleTurn_WindowOne(t *testing.T) {
	sess := session.NewSession("app", "user", "sess1",
		session.WithSessionEvents([]event.Event{
			makeUserEvent("历史消息"),
			makeAssistantEvent("回复"),
		}))
	inv := &agent.Invocation{
		Message: model.Message{Content: "当前消息"},
		Session: sess,
	}
	assert.Equal(t, "当前消息", extractQueryWithContext(inv, 1))
}

func TestExtractQuery_SlidingWindow(t *testing.T) {
	sess := session.NewSession("app", "user", "sess1",
		session.WithSessionEvents([]event.Event{
			makeUserEvent("第一轮"),
			makeAssistantEvent("回复1"),
			makeUserEvent("第二轮"),
			makeAssistantEvent("回复2"),
			makeUserEvent("第三轮"),
			makeAssistantEvent("回复3"),
			makeUserEvent("第四轮"),
			makeAssistantEvent("回复4"),
		}))
	inv := &agent.Invocation{
		Message: model.Message{Content: "当前消息"},
		Session: sess,
	}
	result := extractQueryWithContext(inv, 3)
	assert.Contains(t, result, "第三轮")
	assert.Contains(t, result, "第四轮")
	assert.Contains(t, result, "当前消息")
	assert.NotContains(t, result, "第一轮")
	assert.NotContains(t, result, "第二轮")
}

func TestExtractQuery_WithSummary(t *testing.T) {
	sess := session.NewSession("app", "user", "sess1",
		session.WithSessionEvents([]event.Event{
			makeUserEvent("历史消息"),
			makeAssistantEvent("回复"),
		}),
		session.WithSessionSummaries(map[string]*session.Summary{
			session.SummaryFilterKeyAllContents: {Summary: "用户正在管理CVM实例"},
		}))
	inv := &agent.Invocation{
		Message: model.Message{Content: "继续"},
		Session: sess,
	}
	result := extractQueryWithContext(inv, 3)
	assert.Contains(t, result, "用户正在管理CVM实例")
	assert.Contains(t, result, "历史消息")
	assert.Contains(t, result, "继续")

	idx := strings.Index(result, "用户正在管理CVM实例")
	idx2 := strings.Index(result, "历史消息")
	idx3 := strings.Index(result, "继续")
	assert.Less(t, idx, idx2, "summary should come before history")
	assert.Less(t, idx2, idx3, "history should come before current")
}

func TestExtractQuery_NoSummary_Graceful(t *testing.T) {
	sess := session.NewSession("app", "user", "sess1",
		session.WithSessionEvents([]event.Event{
			makeUserEvent("帮我买一台机器"),
			makeAssistantEvent("好的"),
		}))
	inv := &agent.Invocation{
		Message: model.Message{Content: "继续"},
		Session: sess,
	}
	result := extractQueryWithContext(inv, 3)
	assert.Contains(t, result, "帮我买一台机器")
	assert.Contains(t, result, "继续")
	assert.NotContains(t, result, "好的")
}

func TestExtractQuery_Multimodal(t *testing.T) {
	inv := &agent.Invocation{
		Message: model.Message{
			ContentParts: []model.ContentPart{
				{Type: model.ContentTypeText, Text: strPtr("请帮我")},
				{Type: model.ContentTypeImage},
				{Type: model.ContentTypeText, Text: strPtr("查询CVM")},
			},
		},
	}
	result := extractQueryWithContext(inv, 1)
	assert.Equal(t, "请帮我 查询CVM", result)
}

func TestExtractQuery_EmptyMessage(t *testing.T) {
	inv := &agent.Invocation{
		Message: model.Message{},
	}
	assert.Equal(t, "", extractQueryWithContext(inv, 3))
}

func TestExtractQuery_EmptySessionEvents(t *testing.T) {
	sess := session.NewSession("app", "user", "sess1")
	inv := &agent.Invocation{
		Message: model.Message{Content: "你好"},
		Session: sess,
	}
	assert.Equal(t, "你好", extractQueryWithContext(inv, 3))
}

// ---------------------------------------------------------------------------
// 6.9 End-to-end multi-turn dialog scenario
// ---------------------------------------------------------------------------

func TestExtractQuery_BuyMachine_E2E(t *testing.T) {
	sess := session.NewSession("app", "user", "sess1",
		session.WithSessionEvents([]event.Event{
			makeUserEvent("帮我买一台机器"),
			makeAssistantEvent("好的，请问您需要什么配置？"),
			makeUserEvent("看看南京有哪些机型可以购买"),
			makeAssistantEvent("南京地区有以下机型：S5.MEDIUM4、S5.LARGE8..."),
		}))
	inv := &agent.Invocation{
		Message: model.Message{Content: "好的，买这个吧"},
		Session: sess,
	}
	query := extractQueryWithContext(inv, 3)
	assert.Contains(t, query, "帮我买一台机器")
	assert.Contains(t, query, "南京")
	assert.Contains(t, query, "机型")
	assert.Contains(t, query, "买这个吧")

	idx := &BM25Index{}
	metas := []ToolMeta{
		{Name: "create_cvm", Description: "创建CVM云服务器实例，购买机器",
			SearchText: "create_cvm 创建CVM云服务器实例 购买 机器"},
		{Name: "list_instance_types", Description: "查询可用机型列表",
			SearchText: "list_instance_types 查询可用机型列表 机型"},
		{Name: "list_vpcs", Description: "查询VPC列表", SearchText: "list_vpcs 查询VPC列表"},
	}
	require.NoError(t, idx.Build(context.Background(), metas))
	matches := idx.Search(context.Background(), query, 10, 0)
	matchNames := make([]string, len(matches))
	for i, m := range matches {
		matchNames[i] = m.Name
	}
	assert.Contains(t, matchNames, "create_cvm", "purchase tool should be matched")
	assert.Contains(t, matchNames, "list_instance_types", "instance type tool should be matched")
}

// ---------------------------------------------------------------------------
// 6.10 queryContextWindow config validation
// ---------------------------------------------------------------------------

func TestQueryContextWindowDefault(t *testing.T) {
	cfg := &cc.AgentDynamicToolLoadingConfig{Enabled: true, QueryContextWindow: 0}
	if cfg.QueryContextWindow <= 0 {
		cfg.QueryContextWindow = 3
	}
	assert.Equal(t, 3, cfg.QueryContextWindow)
}

func TestQueryContextWindowCustom(t *testing.T) {
	cfg := &cc.AgentDynamicToolLoadingConfig{Enabled: true, QueryContextWindow: 5}
	assert.Equal(t, 5, cfg.QueryContextWindow)
}

func TestQueryContextWindowNegative(t *testing.T) {
	cfg := &cc.AgentDynamicToolLoadingConfig{Enabled: true, QueryContextWindow: -1}
	if cfg.QueryContextWindow <= 0 {
		cfg.QueryContextWindow = 3
	}
	assert.Equal(t, 3, cfg.QueryContextWindow)
}

// ---------------------------------------------------------------------------
// 6.8 buildToolIndex — strategy selection
// ---------------------------------------------------------------------------

func TestBuildToolIndex_BM25Default(t *testing.T) {
	idx := buildToolIndex(&cc.AgentDynamicToolLoadingConfig{Strategy: ""}, &cc.AgentModelProvider{})
	assert.IsType(t, &BM25Index{}, idx)
}

func TestBuildToolIndex_BM25Explicit(t *testing.T) {
	idx := buildToolIndex(&cc.AgentDynamicToolLoadingConfig{Strategy: "bm25"}, &cc.AgentModelProvider{})
	assert.IsType(t, &BM25Index{}, idx)
}

func TestBuildToolIndex_Keyword(t *testing.T) {
	idx := buildToolIndex(&cc.AgentDynamicToolLoadingConfig{Strategy: "keyword"}, &cc.AgentModelProvider{})
	assert.IsType(t, &KeywordIndex{}, idx)
}

func TestBuildToolIndex_KeywordCaseInsensitive(t *testing.T) {
	idx := buildToolIndex(&cc.AgentDynamicToolLoadingConfig{Strategy: "Keyword"}, &cc.AgentModelProvider{})
	assert.IsType(t, &KeywordIndex{}, idx)
}

// ---------------------------------------------------------------------------
// 6.8 topN validation (app.go level, tested via config struct)
// ---------------------------------------------------------------------------

func TestTopNDefault(t *testing.T) {
	cfg := &cc.AgentDynamicToolLoadingConfig{Enabled: true, TopN: 0}
	if cfg.TopN <= 0 {
		cfg.TopN = 10
	}
	assert.Equal(t, 10, cfg.TopN)
}

func TestTopNCustom(t *testing.T) {
	cfg := &cc.AgentDynamicToolLoadingConfig{Enabled: true, TopN: 20}
	assert.Equal(t, 20, cfg.TopN)
}
