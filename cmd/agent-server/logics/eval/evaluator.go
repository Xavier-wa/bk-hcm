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

package eval

import (
	"context"
	"fmt"
	"strings"

	"hcm/cmd/agent-server/logics/prompt"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/metrics"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

// Evaluator runs the two-stage judge and writes aiagent_run_eval.
type Evaluator struct {
	cli         *client.ClientSet
	loader      *ContextLoader
	model       trpcmodel.Model
	promptStore *prompt.Store
}

// NewEvaluator creates an Evaluator.
func NewEvaluator(cli *client.ClientSet, loader *ContextLoader, model trpcmodel.Model,
	promptStore *prompt.Store) *Evaluator {

	return &Evaluator{cli: cli, loader: loader, model: model, promptStore: promptStore}
}

// Evaluate scores one terminal run. overwrite=true updates the existing row.
func (e *Evaluator) Evaluate(kt *kit.Kit, runID string, overwrite bool) {
	if e == nil || kt == nil {
		return
	}
	// /agui SSE 结束会 cancel 请求 ctx；判官和写 eval 必须活过用户连接。
	// WithoutCancel 保留 rid / username 等 ctx value，只去掉取消。
	kt = detachFromRequestCancel(kt)
	if err := e.evaluate(kt, runID, overwrite); err != nil {
		logs.Errorf("evaluate run failed, err: %v, run_id: %s, rid: %s", err, runID, kt.Rid)
		metrics.IncAiagentEvalTotal(metrics.AiagentEvalResultFail)
		return
	}
}

// detachFromRequestCancel returns a kit whose ctx survives parent cancellation.
func detachFromRequestCancel(kt *kit.Kit) *kit.Kit {
	parent := kt.Ctx
	if parent == nil {
		parent = context.Background()
	}
	return kt.NewSubKitWithCtx(context.WithoutCancel(parent))
}

func (e *Evaluator) evaluate(kt *kit.Kit, runID string, overwrite bool) error {
	prep, err := e.prepareEvalRun(kt, runID, overwrite)
	if err != nil || prep == nil {
		return err
	}

	evalCfg := cc.AgentServer().Eval
	// eval trace 用于记录两阶段评估中的对模型的输入输出
	trace := EvalTrace{}
	startRunID := e.stageOne(kt, prep.candidates.Ordered, runID, evalCfg, &trace)
	rubric, err := e.stageTwo(kt, prep.candidates.Ordered, startRunID, runID, evalCfg, &trace)
	if err != nil {
		logs.Errorf("eval stage2 failed, err: %v, run_id: %s, rid: %s", err, runID, kt.Rid)
		return err
	}

	score := e.computeScore(kt, rubric.Dims, evalCfg, runID)
	req, err := e.buildEvalPersistReq(kt, evalPersistInput{
		runID: runID, startRunID: startRunID, candidates: prep.candidates,
		rubric: rubric, score: score, evalCfg: evalCfg, trace: &trace,
	})
	if err != nil {
		return err
	}
	written, err := e.saveEvalResult(kt, req, overwrite, prep.existingEval)
	if err != nil {
		return err
	}
	if !written {
		return nil
	}
	metrics.IncAiagentEvalTotal(metrics.AiagentEvalResultSuccess)
	logs.Infof("evaluate run success, run_id: %s, quality: %d, rid: %s", runID, score.Quality, kt.Rid)
	return nil
}

type evalPrep struct {
	existingEval *dsaiagent.GetAiagentRunEvalResult
	candidates   CandidateSet
}

// prepareEvalRun loads the existing eval row and candidate window.
// A nil prep with a nil error means the run should be skipped.
func (e *Evaluator) prepareEvalRun(kt *kit.Kit, runID string, overwrite bool) (*evalPrep, error) {
	existingEval, err := e.cli.DataService().Aiagent.RunEval.Get(kt, runID)
	if err != nil && errf.Error(err).Code != errf.RecordNotFound {
		logs.Errorf("evaluate run failed, err: %v, run_id: %s, rid: %s", err, runID, kt.Rid)
		return nil, err
	}
	if skipExistingEval(existingEval, overwrite) {
		logs.Infof("skip eval, already exists, run_id: %s, rid: %s", runID, kt.Rid)
		return nil, nil
	}

	// candidates：同一会话、不晚于被评轮的最近 N 条（含被评轮），时间正序。
	// 阶段一再从中截 start_run_id；N = eval.contextRunLimit。
	candidates, err := e.loader.LoadCandidates(kt, runID)
	if err != nil {
		logs.Errorf("load candidates failed, err: %v, run_id: %s, rid: %s", err, runID, kt.Rid)
		return nil, err
	}
	if candidates.TargetTranscriptEmpty(runID) {
		logs.Warnf("skip eval, empty transcript, run_id: %s, rid: %s", runID, kt.Rid)
		metrics.IncAiagentEvalTotal(metrics.AiagentEvalResultSkip)
		return nil, nil
	}
	return &evalPrep{existingEval: existingEval, candidates: candidates}, nil
}

type evalPersistInput struct {
	runID      string
	startRunID string
	candidates CandidateSet
	rubric     *JudgeRubricResult
	score      ScoreResult
	evalCfg    cc.AgentEvalConfig
	trace      *EvalTrace
}

func (e *Evaluator) buildEvalPersistReq(kt *kit.Kit, in evalPersistInput) (
	*dsaiagent.CreateAiagentRunEvalReq, error) {

	target, _ := in.candidates.Target(in.runID)
	briefs := fillBriefs(in.candidates.Ordered, in.startRunID, in.runID, in.rubric.Briefs)
	req := &dsaiagent.CreateAiagentRunEvalReq{
		RunID:         in.runID,
		SessionID:     e.loader.resolveThreadID(kt, target.Run.SessionCode),
		User:          target.Run.User,
		BkBizID:       target.Run.BkBizID,
		StartRunID:    in.startRunID,
		ProcessScore:  in.score.Process,
		OutcomeScore:  in.score.Outcome,
		QualityScore:  in.score.Quality,
		Redlines:      in.rubric.Redlines,
		ReasonCode:    toReasonCode(in.rubric.ReasonCode),
		RubricVersion: in.evalCfg.RubricVersion,
	}
	evalResult := EvalResult{
		Dims:         in.rubric.Dims,
		ProcessScore: in.score.Process,
		OutcomeScore: in.score.Outcome,
		QualityScore: in.score.Quality,
		Redlines:     in.rubric.Redlines,
		ReasonCode:   in.rubric.ReasonCode,
		Summary:      in.rubric.Summary,
		Briefs:       briefs,
	}
	snapshot := buildSnapshot(in.candidates.Ordered, in.startRunID, in.runID, briefs)
	return req, fillEvalJSONFields(kt, req, evalResult, snapshot, *in.trace)
}

func fillEvalJSONFields(kt *kit.Kit, req *dsaiagent.CreateAiagentRunEvalReq, evalResult EvalResult,
	snapshot ContextSnapshot, trace EvalTrace) error {

	var err error
	if req.EvalResult, err = types.NewJsonField(evalResult); err != nil {
		logs.Errorf("marshal eval result failed, err: %v, run_id: %s, rid: %s", err, req.RunID, kt.Rid)
		return err
	}
	if req.ContextSnapshot, err = types.NewJsonField(snapshot); err != nil {
		logs.Errorf("marshal eval context snapshot failed, err: %v, run_id: %s, rid: %s",
			err, req.RunID, kt.Rid)
		return err
	}
	if req.EvalTrace, err = types.NewJsonField(trace); err != nil {
		logs.Errorf("marshal eval trace failed, err: %v, run_id: %s, rid: %s", err, req.RunID, kt.Rid)
		return err
	}
	return nil
}

// saveEvalResult writes the eval row. written=false means a concurrent create already exists.
func (e *Evaluator) saveEvalResult(kt *kit.Kit, req *dsaiagent.CreateAiagentRunEvalReq, overwrite bool,
	existingEval *dsaiagent.GetAiagentRunEvalResult) (written bool, err error) {

	if shouldOverwriteEval(overwrite, existingEval) {
		err = e.cli.DataService().Aiagent.RunEval.Overwrite(kt,
			&dsaiagent.OverwriteAiagentRunEvalReq{CreateAiagentRunEvalReq: *req})
		if err != nil {
			logs.Errorf("overwrite eval failed, err: %v, run_id: %s, rid: %s", err, req.RunID, kt.Rid)
			return false, err
		}
		return true, nil
	}

	if _, err = e.cli.DataService().Aiagent.RunEval.Create(kt, req); err == nil {
		return true, nil
	}
	if errf.Error(err).Code == errf.RecordDuplicated {
		logs.Infof("skip eval, duplicated run_id, run_id: %s, rid: %s", req.RunID, kt.Rid)
		return false, nil
	}
	logs.Errorf("create eval failed, err: %v, run_id: %s, rid: %s", err, req.RunID, kt.Rid)
	return false, err
}

// 阶段一截取参与评估的历史轮
func (e *Evaluator) stageOne(kt *kit.Kit, candidates []CandidateRun, runID string, cfg cc.AgentEvalConfig,
	trace *EvalTrace) string {

	template := e.promptContent(constant.EvalScopePromptKey)
	prompt := buildScopePrompt(template, candidates, runID)
	trace.ScopePrompt = prompt
	out, err := callEvalModel(kt.Ctx, e.model, template, prompt, cfg.Model.MaxTokens, cfg.Model.Temperature)
	if err != nil {
		logs.Warnf("eval stage1 failed, err: %v, run_id: %s, rid: %s", err, runID, kt.Rid)
		metrics.IncAiagentEvalScopeFallback(metrics.AiagentEvalScopeFallbackFail)
		return runID
	}

	trace.ScopeOutput = out
	start, illegal := parseScopeOutput(out, candidates, runID)
	if illegal {
		logs.Warnf("eval stage1 illegal start_run_id, fallback to target, run_id: %s, rid: %s",
			runID, kt.Rid)
		metrics.IncAiagentEvalScopeFallback(metrics.AiagentEvalScopeFallbackIllegal)
	}
	return start
}

// 阶段二评分
func (e *Evaluator) stageTwo(kt *kit.Kit, candidates []CandidateRun, startRunID, runID string,
	cfg cc.AgentEvalConfig, trace *EvalTrace) (*JudgeRubricResult, error) {

	template := e.promptContent(constant.EvalRubricPromptKey)
	prompt := buildRubricPrompt(template, candidates, startRunID, runID)
	trace.RubricPrompt = prompt
	out, err := callEvalModel(kt.Ctx, e.model, template, prompt, cfg.Model.MaxTokens, cfg.Model.Temperature)
	if err != nil {
		logs.Errorf("eval stage2 failed, err: %v, run_id: %s, rid: %s", err, runID, kt.Rid)
		return nil, err
	}
	trace.RubricOutput = out
	return parseRubricOutput(kt, out)
}

// computeScore 计算百分制得分，并对判官漏给的维度留日志。
// 漏给的维度按不适用剔除，若日志里频繁出现同一维度，说明是判官偷懒而不是真的不适用。
func (e *Evaluator) computeScore(kt *kit.Kit, dims DimScores, cfg cc.AgentEvalConfig,
	runID string) ScoreResult {

	if missing := MissingDims(dims, cfg.ProcessWeights, cfg.OutcomeWeights); len(missing) > 0 {
		logs.Warnf("eval dims missing, treated as not applicable, dims: %v, run_id: %s, rid: %s",
			missing, runID, kt.Rid)
	}
	return Compute(dims, cfg.ProcessWeights, cfg.OutcomeWeights)
}

func fillBriefs(candidates []CandidateRun, startRunID, targetRunID string, briefs map[string]string) map[string]string {
	if briefs == nil {
		briefs = map[string]string{}
	}
	started := startRunID == ""
	for _, w := range candidates {
		if w.Run.RunID == startRunID {
			started = true
		}
		if !started {
			continue
		}
		if briefs[w.Run.RunID] == "" {
			briefs[w.Run.RunID] = fallbackBrief(w.Transcript)
		}
		if w.Run.RunID == targetRunID {
			break
		}
	}
	return briefs
}

func buildSnapshot(candidates []CandidateRun, startRunID, targetRunID string, briefs map[string]string) ContextSnapshot {
	snap := ContextSnapshot{Runs: make([]SnapshotRun, 0)}
	started := startRunID == ""
	for _, w := range candidates {
		if w.Run.RunID == startRunID {
			started = true
		}
		if !started {
			continue
		}
		snap.Runs = append(snap.Runs, SnapshotRun{
			RunID: w.Run.RunID,
			Role:  w.Role,
			Brief: briefs[w.Run.RunID],
		})
		if w.Run.RunID == targetRunID {
			break
		}
	}
	return snap
}

func toReasonCode(raw string) enumor.AiagentEvalReasonCode {
	c := enumor.AiagentEvalReasonCode(raw)
	if err := c.Validate(); err != nil {
		return enumor.AiagentEvalReasonOK
	}
	return c
}

func skipExistingEval(existingEval *dsaiagent.GetAiagentRunEvalResult, overwrite bool) bool {
	return existingEval != nil && existingEval.ID != "" && !overwrite
}

func shouldOverwriteEval(overwrite bool, existingEval *dsaiagent.GetAiagentRunEvalResult) bool {
	return overwrite && existingEval != nil && existingEval.ID != ""
}

func (e *Evaluator) promptContent(key string) string {
	if e == nil || e.promptStore == nil {
		return ""
	}
	entry, ok := e.promptStore.Get(key)
	if !ok {
		return ""
	}
	return entry.Content
}

// callEvalModel calls the eval model once and returns concatenated content.
func callEvalModel(ctx context.Context, mdl trpcmodel.Model, systemPrompt, userPrompt string,
	maxTokens int, temperature float64) (string, error) {

	if mdl == nil {
		return "", fmt.Errorf("eval model is nil")
	}
	temp := temperature
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(systemPrompt),
			trpcmodel.NewUserMessage(userPrompt),
		},
		GenerationConfig: trpcmodel.GenerationConfig{
			Stream:      false,
			MaxTokens:   &maxTokens,
			Temperature: &temp,
		},
	}
	respCh, err := mdl.GenerateContent(ctx, req)
	if err != nil {
		return "", fmt.Errorf("generate content: %w", err)
	}
	var sb strings.Builder
	for resp := range respCh {
		if resp.Error != nil {
			return "", fmt.Errorf("model response error: %s", resp.Error.Message)
		}
		for _, choice := range resp.Choices {
			if choice.Message.Content != "" {
				sb.WriteString(choice.Message.Content)
				continue
			}
			if choice.Delta.Content != "" {
				sb.WriteString(choice.Delta.Content)
				continue
			}
			if choice.Message.ReasoningContent != "" {
				sb.WriteString(choice.Message.ReasoningContent)
			}
		}
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("eval model aborted: %w", err)
		}
		logs.Warnf("eval model returned empty content")
		return "", fmt.Errorf("eval model returned empty content")
	}
	return out, nil
}
