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

package logics

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// ---------------------------------------------------------------------------
// Mock tool for testing
// ---------------------------------------------------------------------------

type mockTool struct {
	decl *tool.Declaration
}

func (m *mockTool) Declaration() *tool.Declaration { return m.decl }

func newMockTool(name, desc string, params map[string]*tool.Schema, required []string) *mockTool {
	return &mockTool{decl: &tool.Declaration{
		Name:        name,
		Description: desc,
		InputSchema: &tool.Schema{
			Type:       "object",
			Properties: params,
			Required:   required,
		},
	}}
}

func newSimpleMockTool(name, desc string) *mockTool {
	return &mockTool{decl: &tool.Declaration{
		Name:        name,
		Description: desc,
	}}
}

// ---------------------------------------------------------------------------
// Mock Embedder for EmbeddingIndex tests
// ---------------------------------------------------------------------------

type mockEmbedder struct {
	vectors map[string][]float64
	err     error
}

func (m *mockEmbedder) GetEmbedding(_ context.Context, text string) ([]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	if v, ok := m.vectors[text]; ok {
		return v, nil
	}
	return nil, nil
}

func (m *mockEmbedder) GetEmbeddingWithUsage(_ context.Context, text string) ([]float64, map[string]any, error) {
	v, err := m.GetEmbedding(context.Background(), text)
	return v, nil, err
}

func (m *mockEmbedder) GetDimensions() int { return 3 }

// ---------------------------------------------------------------------------
// 6.1 extractToolMeta tests
// ---------------------------------------------------------------------------

func TestExtractToolMeta_WithTags(t *testing.T) {
	mt := newMockTool("list_cvm", "列出云服务器实例", map[string]*tool.Schema{
		"region": {Type: "string"},
		"limit":  {Type: "integer"},
	}, []string{"region"})

	meta := extractToolMeta(mt, []string{"云服务器", "CVM"})

	assert.Equal(t, "list_cvm", meta.Name)
	assert.Equal(t, "列出云服务器实例", meta.Description)
	assert.Equal(t, []string{"云服务器", "CVM"}, meta.Tags)
	assert.Len(t, meta.Parameters, 2)

	paramMap := make(map[string]ParamMeta)
	for _, p := range meta.Parameters {
		paramMap[p.Name] = p
	}
	assert.True(t, paramMap["region"].Required)
	assert.Equal(t, "string", paramMap["region"].Type)
	assert.False(t, paramMap["limit"].Required)

	assert.Contains(t, meta.SearchText, "list_cvm")
	assert.Contains(t, meta.SearchText, "列出云服务器实例")
	assert.Contains(t, meta.SearchText, "云服务器")
	assert.Contains(t, meta.SearchText, "CVM")
	assert.Contains(t, meta.SearchText, "region")
}

func TestExtractToolMeta_NoTags(t *testing.T) {
	mt := newSimpleMockTool("ping", "health check")
	meta := extractToolMeta(mt, nil)

	assert.Equal(t, "ping", meta.Name)
	assert.Nil(t, meta.Tags)
	assert.Empty(t, meta.Parameters)
	assert.Contains(t, meta.SearchText, "ping")
	assert.Contains(t, meta.SearchText, "health check")
}

func TestExtractToolMeta_NilInputSchema(t *testing.T) {
	mt := &mockTool{decl: &tool.Declaration{
		Name:        "no_schema",
		Description: "tool with nil schema",
	}}
	meta := extractToolMeta(mt, nil)
	assert.Empty(t, meta.Parameters)
}

// ---------------------------------------------------------------------------
// 6.1 buildSearchText tests
// ---------------------------------------------------------------------------

func TestBuildSearchText(t *testing.T) {
	meta := ToolMeta{
		Name:        "create_disk",
		Description: "创建云硬盘",
		Tags:        []string{"CBS", "磁盘"},
		Parameters:  []ParamMeta{{Name: "size"}, {Name: "type"}},
	}
	text := buildSearchText(meta)

	assert.Contains(t, text, "create_disk")
	assert.Contains(t, text, "创建云硬盘")
	assert.Contains(t, text, "CBS")
	assert.Contains(t, text, "磁盘")
	assert.Contains(t, text, "size")
	assert.Contains(t, text, "type")
}

// ---------------------------------------------------------------------------
// 6.1 tokenize tests
// ---------------------------------------------------------------------------

func TestTokenize_EnglishSplit(t *testing.T) {
	tokens := tokenize("list_cvm instances")
	assert.Contains(t, tokens, "list")
	assert.Contains(t, tokens, "cvm")
	assert.Contains(t, tokens, "instances")
}

func TestTokenize_ChineseNGram(t *testing.T) {
	tokens := tokenize("云服务器")
	assert.Contains(t, tokens, "云")
	assert.Contains(t, tokens, "服")
	assert.Contains(t, tokens, "务")
	assert.Contains(t, tokens, "器")
	assert.Contains(t, tokens, "云服")
	assert.Contains(t, tokens, "服务")
	assert.Contains(t, tokens, "务器")
}

func TestTokenize_MixedChinaEnglish(t *testing.T) {
	tokens := tokenize("CVM 云服务器 list")
	assert.Contains(t, tokens, "cvm") // lowercased
	assert.Contains(t, tokens, "list")
	assert.Contains(t, tokens, "云")
	assert.Contains(t, tokens, "云服")
}

func TestTokenize_SpecialChars(t *testing.T) {
	tokens := tokenize("a+b=c")
	assert.Contains(t, tokens, "a")
	assert.Contains(t, tokens, "+")
	assert.Contains(t, tokens, "b")
	assert.Contains(t, tokens, "=")
	assert.Contains(t, tokens, "c")
}

func TestTokenize_Empty(t *testing.T) {
	assert.Empty(t, tokenize(""))
	assert.Empty(t, tokenize("   "))
}

// ---------------------------------------------------------------------------
// 6.2 KeywordIndex tests
// ---------------------------------------------------------------------------

func sampleToolMetas() []ToolMeta {
	return []ToolMeta{
		{Name: "list_cvm", Description: "list cloud virtual machines", Tags: []string{"CVM", "云服务器"}, SearchText: "list_cvm list cloud virtual machines CVM 云服务器"},
		{Name: "create_disk", Description: "create CBS disk", Tags: []string{"CBS", "磁盘"}, SearchText: "create_disk create CBS disk CBS 磁盘"},
		{Name: "get_vpc", Description: "get VPC details", Tags: []string{"VPC", "网络"}, SearchText: "get_vpc get VPC details VPC 网络"},
		{Name: "resize_cvm", Description: "resize cloud virtual machine", Tags: []string{"CVM"}, SearchText: "resize_cvm resize cloud virtual machine CVM"},
	}
}

func TestKeywordIndex_Hit(t *testing.T) {
	idx := &KeywordIndex{}
	require.NoError(t, idx.Build(context.Background(), sampleToolMetas()))

	results := idx.Search(context.Background(), "cvm cloud", 10, 0)
	require.NotEmpty(t, results)
	assert.Equal(t, "list_cvm", results[0].Name)
}

func TestKeywordIndex_Miss(t *testing.T) {
	idx := &KeywordIndex{}
	require.NoError(t, idx.Build(context.Background(), sampleToolMetas()))

	results := idx.Search(context.Background(), "weather forecast", 10, 0)
	assert.Empty(t, results)
}

func TestKeywordIndex_TagBonus(t *testing.T) {
	idx := &KeywordIndex{}
	require.NoError(t, idx.Build(context.Background(), sampleToolMetas()))

	// "CVM" matches as both substring and tag → should score higher than simple match
	results := idx.Search(context.Background(), "CVM", 10, 0)
	require.True(t, len(results) >= 2)
	// list_cvm and resize_cvm both have CVM tag, should be top
	topNames := make(map[string]bool)
	for _, r := range results[:2] {
		topNames[r.Name] = true
	}
	assert.True(t, topNames["list_cvm"] || topNames["resize_cvm"])
}

func TestKeywordIndex_TopN(t *testing.T) {
	idx := &KeywordIndex{}
	require.NoError(t, idx.Build(context.Background(), sampleToolMetas()))

	results := idx.Search(context.Background(), "cloud", 1, 0)
	assert.Len(t, results, 1)
}

func TestKeywordIndex_ScoreThreshold(t *testing.T) {
	idx := &KeywordIndex{}
	metas := []ToolMeta{
		{Name: "a", SearchText: "cloud virtual machine cvm server"},
		{Name: "b", SearchText: "cloud"},
	}
	require.NoError(t, idx.Build(context.Background(), metas))

	// "cloud virtual machine" has 3 tokens; "a" matches all 3, "b" matches 1.
	// With threshold 0.8: cutoff = 3 * 0.8 = 2.4, "b" (score=1) should be filtered out.
	results := idx.Search(context.Background(), "cloud virtual machine", 10, 0.8)
	require.Len(t, results, 1)
	assert.Equal(t, "a", results[0].Name)
}

func TestKeywordIndex_EmptyQuery(t *testing.T) {
	idx := &KeywordIndex{}
	require.NoError(t, idx.Build(context.Background(), sampleToolMetas()))

	assert.Nil(t, idx.Search(context.Background(), "", 10, 0))
}

// ---------------------------------------------------------------------------
// 6.3 BM25Index tests
// ---------------------------------------------------------------------------

func TestBM25Index_SearchOrdering(t *testing.T) {
	idx := &BM25Index{}
	require.NoError(t, idx.Build(context.Background(), sampleToolMetas()))

	results := idx.Search(context.Background(), "list cvm cloud virtual machines", 10, 0)
	require.NotEmpty(t, results)
	// list_cvm matches most tokens: "list", "cvm", "cloud", "virtual", "machines"
	assert.Equal(t, "list_cvm", results[0].Name)
}

func TestBM25Index_TopN(t *testing.T) {
	idx := &BM25Index{}
	require.NoError(t, idx.Build(context.Background(), sampleToolMetas()))

	results := idx.Search(context.Background(), "cvm cloud", 1, 0)
	assert.Len(t, results, 1)
}

func TestBM25Index_ScoreThreshold(t *testing.T) {
	idx := &BM25Index{}
	metas := []ToolMeta{
		{Name: "high", SearchText: "alpha beta gamma delta epsilon"},
		{Name: "low", SearchText: "alpha zeta"},
	}
	require.NoError(t, idx.Build(context.Background(), metas))

	// Query "alpha beta gamma" → "high" should score much higher than "low".
	// With high threshold, "low" should be filtered.
	results := idx.Search(context.Background(), "alpha beta gamma", 10, 0.7)
	require.NotEmpty(t, results)
	assert.Equal(t, "high", results[0].Name)
	for _, r := range results {
		assert.NotEqual(t, "low", r.Name, "low-scoring result should be filtered by threshold")
	}
}

func TestBM25Index_EmptyIndex(t *testing.T) {
	idx := &BM25Index{}
	require.NoError(t, idx.Build(context.Background(), nil))

	assert.Nil(t, idx.Search(context.Background(), "anything", 10, 0))
}

func TestBM25Index_EmptyQuery(t *testing.T) {
	idx := &BM25Index{}
	require.NoError(t, idx.Build(context.Background(), sampleToolMetas()))

	assert.Nil(t, idx.Search(context.Background(), "", 10, 0))
}

func TestBM25Index_NoMatch(t *testing.T) {
	idx := &BM25Index{}
	require.NoError(t, idx.Build(context.Background(), sampleToolMetas()))

	results := idx.Search(context.Background(), "zzzznotfound xxxxunknown", 10, 0)
	assert.Empty(t, results)
}

// ---------------------------------------------------------------------------
// applyThresholdAndTopN edge cases
// ---------------------------------------------------------------------------

func TestApplyThresholdAndTopN_EmptyMatches(t *testing.T) {
	assert.Nil(t, applyThresholdAndTopN(nil, 10, 0))
	assert.Nil(t, applyThresholdAndTopN([]ToolMatch{}, 10, 0))
}

func TestApplyThresholdAndTopN_TopNZero(t *testing.T) {
	matches := []ToolMatch{{Name: "a", Score: 3}, {Name: "b", Score: 1}}
	result := applyThresholdAndTopN(matches, 0, 0)
	assert.Len(t, result, 2)
}

// ---------------------------------------------------------------------------
// 6.2 cosineSimilarity tests
// ---------------------------------------------------------------------------

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	assert.InDelta(t, 0.0, cosineSimilarity(a, b), 1e-9)
}

func TestCosineSimilarity_SameDirection(t *testing.T) {
	a := []float64{1, 2, 3}
	b := []float64{2, 4, 6}
	assert.InDelta(t, 1.0, cosineSimilarity(a, b), 1e-9)
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{-1, 0, 0}
	assert.InDelta(t, -1.0, cosineSimilarity(a, b), 1e-9)
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{1, 2, 3}
	assert.Equal(t, 0.0, cosineSimilarity(a, b))
}

func TestCosineSimilarity_UnitVector(t *testing.T) {
	a := []float64{1.0 / math.Sqrt(3), 1.0 / math.Sqrt(3), 1.0 / math.Sqrt(3)}
	b := []float64{1.0 / math.Sqrt(3), 1.0 / math.Sqrt(3), 1.0 / math.Sqrt(3)}
	assert.InDelta(t, 1.0, cosineSimilarity(a, b), 1e-9)
}

func TestCosineSimilarity_PanicOnLengthMismatch(t *testing.T) {
	assert.Panics(t, func() {
		cosineSimilarity([]float64{1, 2}, []float64{1, 2, 3})
	})
}

// ---------------------------------------------------------------------------
// 6.3 EmbeddingIndex tests
// ---------------------------------------------------------------------------

func TestEmbeddingIndex_BuildAndSearch_Normal(t *testing.T) {
	emb := &mockEmbedder{
		vectors: map[string][]float64{
			"list cvm":    {1, 0, 0},
			"create disk": {0, 1, 0},
			"get vpc":     {0, 0, 1},
			// query
			"show me cvm": {0.9, 0.1, 0},
		},
	}
	idx := NewEmbeddingIndex(emb)
	metas := []ToolMeta{
		{Name: "list_cvm", SearchText: "list cvm"},
		{Name: "create_disk", SearchText: "create disk"},
		{Name: "get_vpc", SearchText: "get vpc"},
	}
	require.NoError(t, idx.Build(context.Background(), metas))

	results := idx.Search(context.Background(), "show me cvm", 2, 0)
	require.NotEmpty(t, results)
	assert.Equal(t, "list_cvm", results[0].Name, "most similar tool should rank first")
}

func TestEmbeddingIndex_Build_DimensionMismatch(t *testing.T) {
	emb := &mockEmbedder{
		vectors: map[string][]float64{
			"tool a": {1, 0, 0},
			"tool b": {0, 1}, // wrong dimension
		},
	}
	idx := NewEmbeddingIndex(emb)
	metas := []ToolMeta{
		{Name: "a", SearchText: "tool a"},
		{Name: "b", SearchText: "tool b"},
	}
	err := idx.Build(context.Background(), metas)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dimension mismatch")
}

func TestEmbeddingIndex_Build_EmbedderError(t *testing.T) {
	emb := &mockEmbedder{err: errors.New("api error")}
	idx := NewEmbeddingIndex(emb)
	metas := []ToolMeta{{Name: "a", SearchText: "tool a"}}
	err := idx.Build(context.Background(), metas)
	require.Error(t, err)
}

func TestEmbeddingIndex_Search_EmbedderError(t *testing.T) {
	emb := &mockEmbedder{
		vectors: map[string][]float64{
			"tool a": {1, 0, 0},
		},
	}
	idx := NewEmbeddingIndex(emb)
	require.NoError(t, idx.Build(context.Background(), []ToolMeta{{Name: "a", SearchText: "tool a"}}))

	// Make embedder fail for queries
	emb.err = errors.New("api error")
	results := idx.Search(context.Background(), "some query", 10, 0)
	assert.Nil(t, results)
}

func TestEmbeddingIndex_Search_EmptyIndex(t *testing.T) {
	emb := &mockEmbedder{vectors: map[string][]float64{}}
	idx := NewEmbeddingIndex(emb)
	// Build not called (vectors empty)
	results := idx.Search(context.Background(), "query", 10, 0)
	assert.Nil(t, results)
}

func TestEmbeddingIndex_Build_EmptyTools(t *testing.T) {
	emb := &mockEmbedder{vectors: map[string][]float64{}}
	idx := NewEmbeddingIndex(emb)
	require.NoError(t, idx.Build(context.Background(), nil))
	assert.Nil(t, idx.Search(context.Background(), "query", 10, 0))
}
