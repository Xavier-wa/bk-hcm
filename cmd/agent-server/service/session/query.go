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

package session

import (
	"time"
	"unicode/utf8"

	proto "hcm/pkg/api/agent-server/session"
	"hcm/pkg/api/core"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

// ListSessions queries the current user's session list.
//
// POST /api/v1/agent/sessions/list
func (svc *service) ListSessions(cts *rest.Contexts) (interface{}, error) {
	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AgentAssistant, Action: meta.Find}}
	return svc.listSessions(cts, constant.UnassignedBiz, authRes)
}

// BizListSessions queries the current user's session list in a specific business.
//
// POST /api/v1/agent/bizs/{bk_biz_id}/sessions/list
func (svc *service) BizListSessions(cts *rest.Contexts) (interface{}, error) {
	bizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	if bizID <= 0 {
		return nil, errf.Newf(errf.InvalidParameter, "bk_biz_id must be greater than 0")
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AgentAssistant, Action: meta.Find}, BizID: bizID}
	return svc.listSessions(cts, bizID, authRes)
}

func (svc *service) listSessions(cts *rest.Contexts, bkBizID int64, authRes meta.ResourceAttribute) (
	interface{}, error) {

	if svc.cli == nil {
		return nil, errf.New(errf.UnHealthy, "data service client is not configured")
	}

	req := new(core.ListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := svc.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		logs.Errorf("list sessions: permission denied, user: %s, bk_biz_id: %d, err: %v, rid: %s",
			cts.Kit.User, bkBizID, err, cts.Kit.Rid)
		return nil, errf.New(errf.PermissionDenied, "permission denied")
	}

	filterRules := make([]*filter.AtomRule, 0)
	filterRules = append(filterRules, tools.RuleEqual("user", cts.Kit.User))
	if bkBizID != constant.UnassignedBiz {
		filterRules = append(filterRules, tools.RuleEqual("bk_biz_id", bkBizID))
	}
	combined := tools.ExpressionAnd(filterRules...)

	var err error
	if req.Filter != nil {
		combined, err = tools.And(combined, req.Filter)
		if err != nil {
			logs.Errorf("list sessions failed, filter invalid, err: %v, user: %s, bk_biz_id: %d, filter: %+v, rid: %s",
				err, cts.Kit.User, bkBizID, req.Filter, cts.Kit.Rid)
			return nil, errf.NewFromErr(errf.InvalidParameter, err)
		}
	}

	listReq := &core.ListReq{
		Filter: combined,
		Page:   req.Page,
		Fields: req.Fields,
	}

	result, err := svc.cli.DataService().Aiagent.Session.List(cts.Kit, listReq)
	if err != nil {
		logs.Errorf("list sessions failed, user: %s, bk_biz_id: %d, err: %v, rid: %s", cts.Kit.User,
			bkBizID, err, cts.Kit.Rid)
		return nil, err
	}

	return &proto.ListSessionsResult{
		Count:   result.Count,
		Details: result.Details,
	}, nil
}

// GetContextStats computes and serves the context consumption statistics
// for a given session identified by its session_code path parameter.
func (svc *service) GetContextStats(cts *rest.Contexts) (interface{}, error) {
	sessionCode := cts.PathParameter("session_code").String()
	if sessionCode == "" {
		return nil, errf.New(errf.InvalidParameter, `missing path parameter "session_code"`)
	}

	if err := svc.authorizer.AuthorizeWithPerm(cts.Kit,
		meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AgentAssistant, Action: meta.Find}}); err != nil {
		logs.Errorf("agent auth: permission denied, user: %s, err: %v, rid: %s", cts.Kit.User, err, cts.Kit.Rid)
		return nil, errf.New(errf.PermissionDenied, "permission denied")
	}

	sessionMeta, err := svc.resolver.ResolveMeta(cts.Kit, sessionCode)
	if err != nil {
		logs.Errorf("context-stats: resolve session_code %s failed: %v, rid: %s", sessionCode, err, cts.Kit.Rid)
		return nil, err
	}
	if sessionMeta.User != cts.Kit.User {
		logs.Errorf("get context stats failed, session user mismatch, session_code: %s, session_user: %s, "+
			"user: %s, rid: %s", sessionCode, sessionMeta.User, cts.Kit.User, cts.Kit.Rid)
		return nil, errf.New(errf.PermissionDenied, "permission denied")
	}

	key := session.Key{
		AppName:   svc.appName,
		UserID:    cts.Kit.User,
		SessionID: sessionMeta.ThreadID,
	}

	sess, err := svc.sessionSvc.GetSession(cts.Kit.Ctx, key)
	if err != nil {
		logs.Errorf("context-stats: get session %+v failed: %v, rid: %s", key, err, cts.Kit.Rid)
		return nil, err
	}

	return buildContextStatsResponse(sess), nil
}

// buildContextStatsResponse assembles the stats response from a loaded session
// and the operator-configured summary thresholds.
func buildContextStatsResponse(sess *session.Session) *proto.ContextStatsResponse {
	events := sess.GetEvents()
	totalEvents := len(events)

	// Determine the cutoff: events strictly after this timestamp are "unsummarized".
	cutoff := parseSummaryCutoff(sess)
	unsummarized := filterUnsummarizedEvents(events, cutoff)

	hasSummary, lastSummaryAt := latestSummaryInfo(sess)

	var lastEventAt string
	if totalEvents > 0 {
		lastEventAt = events[totalEvents-1].Timestamp.Format(time.RFC3339)
	}

	var lastSummaryStr string
	if !lastSummaryAt.IsZero() {
		lastSummaryStr = lastSummaryAt.Format(time.RFC3339)
	}

	cfg := cc.AgentServer().Storage.Session.Summary

	policy := cfg.Policy
	if policy == "" {
		policy = "any"
	}

	thresholds := &proto.SummaryThresholds{
		Enabled:        cfg.Enabled,
		Policy:         policy,
		EventThreshold: cfg.EventThreshold,
		TokenThreshold: cfg.TokenThreshold,
		IdleThreshold:  cfg.IdleThreshold,
	}

	return &proto.ContextStatsResponse{
		TotalEvents:        totalEvents,
		UnsummarizedEvents: len(unsummarized),
		EstimatedTokens:    estimateTokens(unsummarized),
		HasSummary:         hasSummary,
		LastSummaryAt:      lastSummaryStr,
		LastEventAt:        lastEventAt,
		Thresholds:         thresholds,
	}
}

// parseSummaryCutoff reads the framework-internal state key that records the
// timestamp of the last event included in a summary.
func parseSummaryCutoff(sess *session.Session) time.Time {
	raw, ok := sess.GetState(constant.SessionStateLastIncludedTS)
	if !ok || len(raw) == 0 {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, string(raw))
	if err != nil {
		return time.Time{}
	}
	return t
}

func filterUnsummarizedEvents(events []event.Event, cutoff time.Time) []event.Event {
	if cutoff.IsZero() {
		return events
	}
	result := make([]event.Event, 0, len(events)/2)
	for i := range events {
		if events[i].Timestamp.After(cutoff) {
			result = append(result, events[i])
		}
	}
	return result
}

// estimateTokens approximates the token count of a set of events using the same
// heuristic as the framework's SimpleTokenCounter (rune count / 4).
func estimateTokens(events []event.Event) int {
	totalRunes := 0
	for i := range events {
		e := &events[i]
		if e.Response == nil {
			continue
		}
		for _, choice := range e.Choices {
			msg := choice.Message
			totalRunes += utf8.RuneCountInString(msg.Content)
			totalRunes += utf8.RuneCountInString(msg.ReasoningContent)
			for _, part := range msg.ContentParts {
				if part.Text != nil {
					totalRunes += utf8.RuneCountInString(*part.Text)
				}
			}
			for _, tc := range msg.ToolCalls {
				totalRunes += utf8.RuneCountInString(tc.Function.Name)
				totalRunes += utf8.RuneCount(tc.Function.Arguments)
			}
		}
	}
	return int(float64(totalRunes) / constant.ApproxRunesPerToken)
}

// latestSummaryInfo inspects the session's Summaries map and returns whether
// any summary exists and the most recent summary update time.
func latestSummaryInfo(sess *session.Session) (hasSummary bool, latest time.Time) {
	sess.SummariesMu.RLock()
	defer sess.SummariesMu.RUnlock()

	for _, s := range sess.Summaries {
		if s != nil && s.Summary != "" {
			if s.UpdatedAt.After(latest) {
				latest = s.UpdatedAt
			}
		}
	}
	return !latest.IsZero(), latest
}
