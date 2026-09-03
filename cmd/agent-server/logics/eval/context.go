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

	"hcm/pkg/api/core"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"

	"trpc.group/trpc-go/trpc-agent-go/session"
)

const briefRuneLimit = 200

// ContextLoader 为被评轮组装评估候选集（同一会话最近若干轮 + transcript）。
type ContextLoader struct {
	cli        *client.ClientSet
	sessionSvc session.Service
	appName    string
}

// NewContextLoader creates a ContextLoader.
func NewContextLoader(cli *client.ClientSet, sessionSvc session.Service, appName string) *ContextLoader {
	return &ContextLoader{cli: cli, sessionSvc: sessionSvc, appName: appName}
}

// LoadCandidates 拉取被评轮所在会话的评估候选集。
//
// 范围：同一 session_code，created_at <= 被评轮，按时间倒序取 contextRunLimit 条再翻成正序；
// 结果必含被评轮。账本 transcript 为空时会从 session 轨道补写。
func (l *ContextLoader) LoadCandidates(kt *kit.Kit, targetRunID string) (CandidateSet, error) {
	target, err := l.getRun(kt, targetRunID)
	if err != nil {
		return CandidateSet{}, err
	}

	limit := cc.AgentServer().Eval.ContextRunLimit
	rows, err := l.listCandidates(kt, target.SessionCode, string(target.CreatedAt), uint(limit))
	if err != nil {
		return CandidateSet{}, err
	}
	found := false
	for _, r := range rows {
		if r.RunID == target.RunID {
			found = true
			break
		}
	}
	if !found {
		rows = append(rows, *target)
	}

	track := l.loadTrackByTarget(kt, target)
	set := CandidateSet{
		Ordered: make([]CandidateRun, 0, len(rows)),
		ByID:    make(map[string]CandidateRun, len(rows)),
	}
	for i := range rows {
		r := rows[i]
		// 优先用账本已落库的 transcript；终态 CAS 时一般已经写好。
		tr := decodeTranscript(string(r.Transcript))
		// 迟到 FINISHED、create 失败后补写、或候选集里更早一轮当时没带上正文时，账本可能仍空。
		// 此时从 session 轨道表切轮还原，并 PatchTranscript（不刷新 updated_at）回写该轮，避免下次再拉轨道。
		if len(tr.Items) == 0 {
			if patched, ok := track[r.RunID]; ok && len(patched.Items) > 0 {
				tr = patched
				if err := l.patchTranscript(kt, r.RunID, FirstUserText(tr), tr); err != nil {
					logs.Warnf("patch transcript failed, err: %v, run_id: %s, rid: %s",
						err, r.RunID, kt.Rid)
				}
			}
		}
		role := enumor.AiagentEvalSnapshotRoleContext
		if r.RunID == target.RunID {
			role = enumor.AiagentEvalSnapshotRoleTarget
		}
		query := r.Query
		if query == "" {
			query = FirstUserText(tr)
		}
		item := CandidateRun{Run: r, Query: query, Transcript: tr, Role: role}
		set.Ordered = append(set.Ordered, item)
		set.ByID[item.Run.RunID] = item
	}
	return set, nil
}

func (l *ContextLoader) getRun(kt *kit.Kit, runID string) (*tableaiagent.RunTable, error) {
	result, err := l.cli.DataService().Aiagent.Run.List(kt, &core.ListReq{
		Filter: tools.EqualExpression("run_id", runID),
		Page:   &core.BasePage{Start: 0, Limit: 1},
	})
	if err != nil {
		logs.Errorf("list aiagent run failed, err: %v, run_id: %s, rid: %s", err, runID, kt.Rid)
		return nil, err
	}
	if result == nil || len(result.Details) == 0 {
		logs.Errorf("aiagent run not found, run_id: %s, rid: %s", runID, kt.Rid)
		return nil, errf.New(errf.RecordNotFound, "run not found")
	}
	return &result.Details[0], nil
}

func (l *ContextLoader) listCandidates(kt *kit.Kit, sessionCode, createdAt string, limit uint) (
	[]tableaiagent.RunTable, error) {

	expr := tools.ExpressionAnd(
		tools.RuleEqual("session_code", sessionCode),
		tools.RuleLessThanEqual("created_at", createdAt),
	)
	result, err := l.cli.DataService().Aiagent.Run.List(kt, &core.ListReq{
		Filter: expr,
		Page: &core.BasePage{
			Start: 0, Limit: limit, Sort: "created_at", Order: core.Descending,
		},
	})
	if err != nil {
		logs.Errorf("list candidate runs failed, err: %v, session_code: %s, rid: %s",
			err, sessionCode, kt.Rid)
		return nil, err
	}
	// 要的是「不晚于目标轮的最近 N 条」，必须 DESC+LIMIT；再反转成时间正序给判官。
	// 若直接 ASC LIMIT N 会拿到会话最早的 N 条，而不是目标轮前最近的窗口。
	details := result.Details
	for i, j := 0, len(details)-1; i < j; i, j = i+1, j-1 {
		details[i], details[j] = details[j], details[i]
	}
	return details, nil
}

func (l *ContextLoader) loadTrackByTarget(kt *kit.Kit, target *tableaiagent.RunTable) map[string]Transcript {
	if l.sessionSvc == nil || target == nil {
		return nil
	}
	sessID := l.resolveThreadID(kt, target.SessionCode)
	if sessID == "" {
		return nil
	}
	key := session.Key{AppName: l.appName, UserID: target.User, SessionID: sessID}
	sess, err := l.sessionSvc.GetSession(kt.Ctx, key)
	if err != nil || sess == nil {
		logs.Warnf("get session track failed, err: %v, session_code: %s, rid: %s",
			err, target.SessionCode, kt.Rid)
		return nil
	}
	return SplitSessionEvents(sess.GetEvents())
}

func (l *ContextLoader) patchTranscript(kt *kit.Kit, runID, query string, tr Transcript) error {
	raw, err := json.Marshal(tr)
	if err != nil {
		return err
	}
	return l.cli.DataService().Aiagent.Run.PatchTranscript(kt, &dsaiagent.PatchAiagentRunTranscriptReq{
		RunID:      runID,
		Query:      query,
		Transcript: types.JsonField(raw),
	})
}

func (l *ContextLoader) resolveThreadID(kt *kit.Kit, sessionCode string) string {
	result, err := l.cli.DataService().Aiagent.Session.List(kt, &dsaiagent.ListAiagentSessionReq{
		Filter: tools.EqualExpression("session_code", sessionCode),
		Page:   &core.BasePage{Start: 0, Limit: 1},
	})
	if err != nil {
		logs.Warnf("list session for thread id failed, err: %v, session_code: %s, rid: %s",
			err, sessionCode, kt.Rid)
		return ""
	}
	if result == nil || len(result.Details) == 0 {
		logs.Warnf("session not found for thread id, session_code: %s, rid: %s", sessionCode, kt.Rid)
		return ""
	}
	if result.Details[0].ThreadID != "" {
		return result.Details[0].ThreadID
	}
	return result.Details[0].ID
}

func fallbackBrief(tr Transcript) string {
	return LastAssistantText(tr, briefRuneLimit)
}
