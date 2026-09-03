/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may obtain a copy of the License at http://opensource.org/licenses/MIT
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

// Package eval implements process-local model evaluation for aiagent runs.
package eval

import (
	"hcm/pkg/criteria/enumor"
	tableaiagent "hcm/pkg/dal/table/aiagent"
)

// Transcript is the ledger conversation payload stored on aiagent_run.
type Transcript struct {
	Items []TranscriptItem `json:"items"`
}

// TranscriptItem is one time-ordered bubble in a run.
type TranscriptItem struct {
	// Type 气泡类型：user / assistant / tool / custom。
	Type enumor.AiagentTranscriptItemType `json:"type"`
	// Text user/assistant 正文。tool/custom 为空。
	Text string `json:"text,omitempty"`
	// ToolName 工具名，仅 type=tool。
	ToolName string `json:"tool_name,omitempty"`
	// ToolArgs 工具入参原文，仅 type=tool。
	ToolArgs string `json:"tool_args,omitempty"`
	// ToolResult 工具返回原文，仅 type=tool。
	ToolResult string `json:"tool_result,omitempty"`
	// Name custom 事件名（HITL 等），仅 type=custom。
	Name string `json:"name,omitempty"`
	// Value custom 事件载荷，结构随 Name 变化，原样存 AG-UI CustomEvent.Value。
	Value any `json:"value,omitempty"`
}

// CandidateRun 是评估候选集里的一轮账本（时间正序：早 → 晚）。
// 候选集 = 同一 session_code、created_at 不晚于被评轮、最多 contextRunLimit 条，且必含被评轮。
type CandidateRun struct {
	Run        tableaiagent.RunTable
	Query      string
	Transcript Transcript
	Role       enumor.AiagentEvalSnapshotRole
}

// CandidateSet 是一次评估的候选集。
// Ordered 给阶段一/二按时间扫；ByID 按 run_id 取被评轮，避免每次 for 检索。
type CandidateSet struct {
	Ordered []CandidateRun
	ByID    map[string]CandidateRun
}

// Target 按 run_id 取一轮。
func (s CandidateSet) Target(runID string) (CandidateRun, bool) {
	c, ok := s.ByID[runID]
	return c, ok
}

// TargetTranscriptEmpty 被评轮不在集内，或其 transcript.items 为空。
func (s CandidateSet) TargetTranscriptEmpty(runID string) bool {
	c, ok := s.ByID[runID]
	return !ok || len(c.Transcript.Items) == 0
}

// SnapshotRun is one lightweight row stored on eval.context_snapshot.
type SnapshotRun struct {
	RunID string                         `json:"run_id"`
	Role  enumor.AiagentEvalSnapshotRole `json:"role"`
	Brief string                         `json:"brief"`
}

// ContextSnapshot is stored on aiagent_run_eval without items.
type ContextSnapshot struct {
	Runs []SnapshotRun `json:"runs"`
}

// DimScores is judge 0-5 scores keyed by dimension name. Weights live in eval yaml.
type DimScores map[string]int

// JudgeScopeResult is the stage-1 JSON.
type JudgeScopeResult struct {
	StartRunID string `json:"start_run_id"`
}

// JudgeRubricResult is the stage-2 JSON before code scoring.
type JudgeRubricResult struct {
	Dims       DimScores         `json:"dims"`
	Redlines   []string          `json:"redlines"`
	ReasonCode string            `json:"reason_code"`
	Summary    string            `json:"summary"`
	Briefs     map[string]string `json:"briefs"`
}

// EvalResult is persisted on aiagent_run_eval.eval_result.
type EvalResult struct {
	Dims         DimScores         `json:"dims"`
	ProcessScore int               `json:"process_score"`
	OutcomeScore int               `json:"outcome_score"`
	QualityScore int               `json:"quality_score"`
	Redlines     []string          `json:"redlines"`
	ReasonCode   string            `json:"reason_code"`
	Summary      string            `json:"summary"`
	Briefs       map[string]string `json:"briefs"`
}

// EvalTrace is persisted for debug; omitted from detail by default.
type EvalTrace struct {
	ScopePrompt  string `json:"scope_prompt,omitempty"`
	ScopeOutput  string `json:"scope_output,omitempty"`
	RubricPrompt string `json:"rubric_prompt,omitempty"`
	RubricOutput string `json:"rubric_output,omitempty"`
}

// ScoreResult is the code-computed percent scores.
type ScoreResult struct {
	Process int
	Outcome int
	Quality int
}
