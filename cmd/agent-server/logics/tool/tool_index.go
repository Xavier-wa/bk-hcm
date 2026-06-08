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
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"golang.org/x/sync/errgroup"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/embedder"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

// ToolMeta holds structured metadata extracted from a single MCP tool.
type ToolMeta struct {
	Name        string      // tool name
	Description string      // tool description
	Parameters  []ParamMeta // input parameters
	Tags        []string    // tags for categorization
	SearchText  string      // pre-built full-text for indexing
}

// ParamMeta describes one tool input parameter.
type ParamMeta struct {
	Name     string // parameter name
	Type     string // parameter type
	Required bool   // whether the parameter is required
}

// ToolMatch is a scored search result.
type ToolMatch struct {
	Name  string  // tool name
	Score float64 // match score
}

// ExtractToolMeta builds a ToolMeta from a tool.Tool declaration.
func ExtractToolMeta(t tool.Tool, frameworkName string, tags []string) ToolMeta {
	return extractToolMeta(t, frameworkName, tags)
}

// extractToolMeta builds a ToolMeta from a tool.Tool declaration.
func extractToolMeta(t tool.Tool, frameworkName string, tags []string) ToolMeta {
	decl := t.Declaration()
	meta := ToolMeta{
		Name:        frameworkName,
		Description: decl.Description,
		Tags:        tags,
	}

	if decl.InputSchema != nil {
		requiredSet := make(map[string]bool, len(decl.InputSchema.Required))
		for _, r := range decl.InputSchema.Required {
			requiredSet[r] = true
		}
		for pName, pSchema := range decl.InputSchema.Properties {
			pm := ParamMeta{Name: pName, Required: requiredSet[pName]}
			if pSchema != nil {
				pm.Type = pSchema.Type
			}
			meta.Parameters = append(meta.Parameters, pm)
		}
	}

	meta.SearchText = buildSearchText(meta)
	return meta
}

// buildSearchText concatenates name, description, tags and parameter names
// into a single searchable string.
func buildSearchText(m ToolMeta) string {
	var b strings.Builder
	b.WriteString(m.Name)
	b.WriteByte(' ')
	b.WriteString(m.Description)
	for _, tag := range m.Tags {
		b.WriteByte(' ')
		b.WriteString(tag)
	}
	for _, p := range m.Parameters {
		b.WriteByte(' ')
		b.WriteString(p.Name)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Tokenizer — mixed n-gram for Chinese, whitespace/underscore split for ASCII
// ---------------------------------------------------------------------------

// tokenize splits text into search tokens.
// English: split on whitespace/underscore, lowercase.
// Chinese: unigram + bigram.
// Digits and special chars: kept as individual tokens.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var asciiWord strings.Builder

	flushASCII := func() {
		if asciiWord.Len() > 0 {
			tokens = append(tokens, asciiWord.String())
			asciiWord.Reset()
		}
	}

	var prevCJK rune
	hasPrevCJK := false

	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size

		if isCJK(r) {
			flushASCII()
			tokens = append(tokens, string(r)) // unigram
			if hasPrevCJK {
				tokens = append(tokens, string(prevCJK)+string(r)) // bigram
			}
			prevCJK = r
			hasPrevCJK = true
			continue
		}

		hasPrevCJK = false

		if r == '_' || r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			flushASCII()
			continue
		}

		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			asciiWord.WriteRune(r)
		} else {
			flushASCII()
			tokens = append(tokens, string(r))
		}
	}
	flushASCII()
	return tokens
}

// isCJK returns true for CJK Unified Ideographs ranges.
func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0x2A700 && r <= 0x2B73F) ||
		(r >= 0x2B740 && r <= 0x2B81F) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x2F800 && r <= 0x2FA1F)
}

// ---------------------------------------------------------------------------
// ToolIndex interface
// ---------------------------------------------------------------------------

// ToolIndex is the common interface for keyword / BM25 / embedding tool search indexes.
type ToolIndex interface {
	// Build loads tool metadata into the index.
	Build(ctx context.Context, tools []ToolMeta) error
	// Search queries the index and returns scored matches.
	Search(ctx context.Context, query string, topN int, scoreThreshold float64) []ToolMatch
}

// ---------------------------------------------------------------------------
// KeywordIndex — simple substring + tag exact match
// ---------------------------------------------------------------------------

// KeywordIndex scores tools by how many query tokens appear as substrings in
// the tool's SearchText, with bonus points for exact tag matches.
type KeywordIndex struct {
	tools []ToolMeta
}

// Build loads tool metadata into the KeywordIndex for subsequent substring searches.
func (idx *KeywordIndex) Build(_ context.Context, tools []ToolMeta) error {
	idx.tools = tools
	return nil
}

// Search scores tools by counting query-token substring hits in SearchText,
// with bonus points for exact tag matches, then applies topN and threshold.
func (idx *KeywordIndex) Search(_ context.Context, query string, topN int, scoreThreshold float64) []ToolMatch {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	var matches []ToolMatch
	for _, tm := range idx.tools {
		searchLower := strings.ToLower(tm.SearchText)
		var score float64
		for _, qt := range queryTokens {
			if strings.Contains(searchLower, qt) {
				score++
			}
		}
		// Bonus for exact tag match (case-insensitive).
		for _, qt := range queryTokens {
			for _, tag := range tm.Tags {
				if strings.EqualFold(qt, tag) {
					score += 2
				}
			}
		}
		if score > 0 {
			matches = append(matches, ToolMatch{Name: tm.Name, Score: score})
		}
	}

	return applyThresholdAndTopN(matches, topN, scoreThreshold)
}

// ---------------------------------------------------------------------------
// BM25Index — classic BM25 with TF-IDF + length normalization
// ---------------------------------------------------------------------------

// BM25Index implements a BM25-based full-text search index.
type BM25Index struct {
	tools    []ToolMeta
	docLen   []int              // token count per document
	avgDL    float64            // average document length
	tf       []map[string]int   // term frequency per document
	idf      map[string]float64 // inverse document frequency per term
	docCount int
}

// Build computes per-document term frequencies, document frequencies and IDF
// values required for BM25 scoring.
func (idx *BM25Index) Build(_ context.Context, tools []ToolMeta) error {
	idx.tools = tools
	idx.docCount = len(tools)
	idx.docLen = make([]int, idx.docCount)
	idx.tf = make([]map[string]int, idx.docCount)

	df := make(map[string]int) // document frequency
	totalLen := 0

	for i, tm := range tools {
		tokens := tokenize(tm.SearchText)
		idx.docLen[i] = len(tokens)
		totalLen += len(tokens)

		freq := make(map[string]int, len(tokens))
		for _, t := range tokens {
			freq[t]++
		}
		idx.tf[i] = freq

		for term := range freq {
			df[term]++
		}
	}

	if idx.docCount > 0 {
		idx.avgDL = float64(totalLen) / float64(idx.docCount)
	}

	// IDF: log((N - df + 0.5) / (df + 0.5) + 1)
	idx.idf = make(map[string]float64, len(df))
	n := float64(idx.docCount)
	for term, d := range df {
		idx.idf[term] = math.Log((n-float64(d)+0.5)/(float64(d)+0.5) + 1)
	}

	return nil
}

// Search ranks indexed tools using the BM25 algorithm against the query tokens.
func (idx *BM25Index) Search(_ context.Context, query string, topN int, scoreThreshold float64) []ToolMatch {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 || idx.docCount == 0 {
		return nil
	}

	const (
		bm25K1 = 1.2
		bm25B  = 0.75
	)

	var matches []ToolMatch
	for i := range idx.tools {
		score := 0.0
		dl := float64(idx.docLen[i])
		for _, qt := range queryTokens {
			idf, ok := idx.idf[qt]
			if !ok {
				continue
			}
			tfVal := float64(idx.tf[i][qt])
			numerator := tfVal * (bm25K1 + 1)
			denominator := tfVal + bm25K1*(1-bm25B+bm25B*dl/idx.avgDL)
			score += idf * numerator / denominator
		}
		if score > 0 {
			matches = append(matches, ToolMatch{Name: idx.tools[i].Name, Score: score})
		}
	}

	return applyThresholdAndTopN(matches, topN, scoreThreshold)
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// EmbeddingIndex — cosine similarity over pre-computed embedding vectors
// ---------------------------------------------------------------------------

// embeddingBuildConcurrency is the number of concurrent goroutines used when
// calling the Embedding API during Build.
const embeddingBuildConcurrency = 8

// EmbeddingIndex implements ToolIndex using vector cosine similarity.
// Build generates embedding vectors for all tools concurrently; Search embeds
// the query and ranks tools by cosine similarity.
type EmbeddingIndex struct {
	emb     embedder.Embedder
	tools   []ToolMeta
	vectors [][]float64
}

// NewEmbeddingIndex creates an EmbeddingIndex backed by the given embedder.
func NewEmbeddingIndex(emb embedder.Embedder) *EmbeddingIndex {
	return &EmbeddingIndex{emb: emb}
}

// Build concurrently embeds all tools and stores the resulting vectors.
// Returns an error if any embedding call fails or vectors have inconsistent dimensions.
func (idx *EmbeddingIndex) Build(ctx context.Context, tools []ToolMeta) error {
	if len(tools) == 0 {
		return nil
	}

	vectors := make([][]float64, len(tools))
	g, gCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, embeddingBuildConcurrency)

	for i, tm := range tools {
		i, tm := i, tm
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-gCtx.Done():
				return gCtx.Err()
			}
			vec, err := idx.emb.GetEmbedding(gCtx, tm.SearchText)
			if err != nil {
				return fmt.Errorf("embed tool %q: %w", tm.Name, err)
			}
			vectors[i] = vec
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	if len(vectors) > 1 {
		dim := len(vectors[0])
		for i, v := range vectors[1:] {
			if len(v) != dim {
				return fmt.Errorf("embedding dimension mismatch: tool[0] has %d dims, tool[%d] has %d dims",
					dim, i+1, len(v))
			}
		}
	}

	idx.tools = tools
	idx.vectors = vectors
	return nil
}

// Search embeds the query and returns tools sorted by cosine similarity.
// Returns nil if the index is empty or the embedding call fails.
func (idx *EmbeddingIndex) Search(ctx context.Context, query string, topN int, scoreThreshold float64) []ToolMatch {
	rid := rest.RidFromContext(ctx)
	if len(idx.vectors) == 0 {
		return nil
	}

	queryVec, err := idx.emb.GetEmbedding(ctx, query)
	if err != nil {
		logs.Warnf("embedding tool search: GetEmbedding failed: %v, rid: %s", err, rid)
		return nil
	}
	if len(queryVec) == 0 {
		return nil
	}

	var matches []ToolMatch
	for i, vec := range idx.vectors {
		if len(vec) == 0 {
			continue
		}
		score := cosineSimilarity(queryVec, vec)
		if score < 0 {
			logs.Warnf("embedding tool search: vector dimension mismatch for tool %s, fall back to full tool set, rid: %s",
				idx.tools[i].Name, rid)
			return nil
		}
		if score > 0 {
			matches = append(matches, ToolMatch{Name: idx.tools[i].Name, Score: score})
		}
	}

	return applyThresholdAndTopN(matches, topN, scoreThreshold)
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// cosineSimilarity computes the cosine similarity between two equal-length vectors.
// Returns -1 on length mismatch to signal caller to fall back to full tool set.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return -1
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// applyThresholdAndTopN sorts matches descending by score, applies score
// threshold filtering, and truncates to topN.
func applyThresholdAndTopN(matches []ToolMatch, topN int, scoreThreshold float64) []ToolMatch {
	if len(matches) == 0 {
		return nil
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	if scoreThreshold > 0 {
		maxScore := matches[0].Score
		cutoff := maxScore * scoreThreshold
		filtered := matches[:0]
		for _, m := range matches {
			if m.Score >= cutoff {
				filtered = append(filtered, m)
			}
		}
		matches = filtered
	}

	if topN > 0 && len(matches) > topN {
		matches = matches[:topN]
	}
	return matches
}
