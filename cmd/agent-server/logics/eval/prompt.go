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
	"encoding/json"
	"fmt"
	"strings"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

func buildScopePrompt(template string, candidates []CandidateRun, targetRunID string) string {
	type row struct {
		RunID string `json:"run_id"`
		Query string `json:"query"`
	}
	rows := make([]row, 0, len(candidates))
	for _, w := range candidates {
		rows = append(rows, row{RunID: w.Run.RunID, Query: w.Query})
	}
	payload, _ := json.Marshal(rows)
	return fmt.Sprintf("%s\n\n# Target run_id\n%s\n\n# Window queries\n%s\n",
		template, targetRunID, string(payload))
}

func buildRubricPrompt(template string, candidates []CandidateRun, startRunID, targetRunID string) string {
	type row struct {
		RunID      string     `json:"run_id"`
		Role       string     `json:"role"`
		Query      string     `json:"query"`
		Transcript Transcript `json:"transcript"`
	}
	rows := make([]row, 0, len(candidates))
	started := startRunID == ""
	for _, w := range candidates {
		if w.Run.RunID == startRunID {
			started = true
		}
		if !started {
			continue
		}
		rows = append(rows, row{
			RunID:      w.Run.RunID,
			Role:       string(w.Role),
			Query:      w.Query,
			Transcript: w.Transcript,
		})
		if w.Run.RunID == targetRunID {
			break
		}
	}
	payload, _ := json.Marshal(rows)
	return fmt.Sprintf("%s\n\n# Target run_id\n%s\n\n# Window transcripts\n%s\n",
		template, targetRunID, string(payload))
}

func parseScopeOutput(raw string, candidates []CandidateRun, targetRunID string) (startRunID string, illegal bool) {
	var parsed JudgeScopeResult
	if err := decodeJudgeJSON(raw, &parsed); err != nil || parsed.StartRunID == "" {
		return targetRunID, true
	}
	allowed := make(map[string]int, len(candidates))
	targetIdx := -1
	for i, w := range candidates {
		allowed[w.Run.RunID] = i
		if w.Run.RunID == targetRunID {
			targetIdx = i
		}
	}
	idx, ok := allowed[parsed.StartRunID]
	if !ok || (targetIdx >= 0 && idx > targetIdx) {
		return targetRunID, true
	}
	return parsed.StartRunID, false
}

func parseRubricOutput(kt *kit.Kit, raw string) (*JudgeRubricResult, error) {
	var parsed JudgeRubricResult
	if err := decodeJudgeJSON(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse rubric json: %w", err)
	}
	if parsed.Briefs == nil {
		parsed.Briefs = map[string]string{}
	}
	parsed.Redlines = filterRedlines(kt, parsed.Redlines)
	parsed.ReasonCode = normalizeReasonCode(kt, parsed.ReasonCode)
	return &parsed, nil
}

// filterRedlines 逐项过滤红线，只丢非法值。
// 整组作废会把判官已经识别出的合法红线一起抹掉，让一次有问题的对话看起来干净。
func filterRedlines(kt *kit.Kit, raw []string) []string {
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if err := enumor.AiagentEvalRedline(item).Validate(); err != nil {
			logs.Warnf("drop invalid eval redline: %s, rid: %s", item, kt.Rid)
			continue
		}
		out = append(out, item)
	}
	return out
}

// normalizeReasonCode 非法 reason_code 降级为 ok。
// 降级本身是静默的，会把判官发现的问题伪装成「无问题」，因此必须留日志。
func normalizeReasonCode(kt *kit.Kit, raw string) string {
	if err := enumor.AiagentEvalReasonCode(raw).Validate(); err != nil {
		logs.Warnf("fallback invalid eval reason_code: %s, rid: %s", raw, kt.Rid)
		return string(enumor.AiagentEvalReasonOK)
	}
	return raw
}

// decodeJudgeJSON 解析判官输出。
// 判官偶尔会在 JSON 前后带上「以下是评估结果」之类的说明，只削 markdown 围栏会解析失败；
// 阶段二解析失败等于整轮评估作废（token 已经烧掉），因此再退一步截取首个 '{' 到末个 '}' 重试。
func decodeJudgeJSON(raw string, out any) error {
	s := stripJSONFence(raw)
	err := json.Unmarshal([]byte(s), out)
	if err == nil {
		return nil
	}
	body, ok := sliceJSONObject(s)
	if !ok {
		return err
	}
	return json.Unmarshal([]byte(body), out)
}

func sliceJSONObject(s string) (string, bool) {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return "", false
	}
	return s[start : end+1], true
}

func stripJSONFence(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
