/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package eval

import (
	"strings"

	proto "hcm/pkg/api/agent-server/eval"
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"
)

// feedbackListFields is the aiagent_run_feedback columns needed by the feedback list.
var feedbackListFields = []string{
	"run_id", "session_id", "user", "bk_biz_id", "scene", "query", "reaction", "tags", "comment",
	"created_at",
}

// ListDashboardFeedback pages the aiagent_run_feedback ledger for the ops dashboard. scene/query
// are denormalized onto aiagent_run_feedback (mirrored from aiagent_run at feedback time), so
// every filter, sort and page in this query plan is pushed straight to MySQL with no in-memory
// fallback. quality_score/passed are display-only fields looked up from aiagent_run_eval by
// run_id after paging; the list does not support filtering or sorting by them (see
// proto.DashboardFeedbackListReq).
func (svc *service) ListDashboardFeedback(cts *rest.Contexts) (interface{}, error) {
	if err := svc.authorizeManage(cts.Kit); err != nil {
		return nil, err
	}
	req := new(proto.DashboardFeedbackListReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("eval dashboard feedback list decode request failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("eval dashboard feedback list validate request failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	expr, err := periodExpr("created_at", req.From, req.To, "", dashboardFeedbackFilters(req)...)
	if err != nil {
		logs.Errorf("build feedback period filter failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	return svc.listFeedbacks(cts.Kit, req.Page, expr)
}

// listFeedbacks counts or pages aiagent_run_feedback directly via MySQL, then loads
// quality_score/passed for just the returned detail page's run_ids for display.
func (svc *service) listFeedbacks(kt *kit.Kit, page *core.BasePage, expr *filter.Expression) (
	*proto.ListResp[proto.DashboardFeedbackListItem], error) {

	if page.Count {
		countResult, err := svc.cli.DataService().Aiagent.Feedback.List(kt, &core.ListReq{
			Filter: expr, Page: page,
		})
		if err != nil {
			logs.Errorf("count feedbacks failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		return &proto.ListResp[proto.DashboardFeedbackListItem]{Count: countResult.Count}, nil
	}

	listResult, err := svc.cli.DataService().Aiagent.Feedback.List(kt, &core.ListReq{
		Filter: expr, Fields: feedbackListFields, Page: feedbackListPage(page),
	})
	if err != nil {
		logs.Errorf("list feedbacks failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	evalMap, err := svc.loadEvalsByRunIDs(kt, feedbackRunIDs(listResult.Details))
	if err != nil {
		return nil, err
	}
	items := make([]proto.DashboardFeedbackListItem, 0, len(listResult.Details))
	for _, fb := range listResult.Details {
		items = append(items, dashboardFeedbackItem(fb, evalMap))
	}
	return &proto.ListResp[proto.DashboardFeedbackListItem]{Details: items}, nil
}

func feedbackRunIDs(feedbacks []tableaiagent.FeedbackTable) []string {
	return slice.Unique(slice.Map(feedbacks, func(fb tableaiagent.FeedbackTable) string {
		return fb.RunID
	}))
}

// feedbackListPage fills in the feedback list's default sort (created_at DESC) when the caller
// left page.sort/page.order empty, without mutating the caller's *core.BasePage.
func feedbackListPage(page *core.BasePage) *core.BasePage {
	out := *page
	if out.Sort == "" {
		out.Sort = proto.SortCreatedAt
	}
	if out.Order == "" {
		out.Order = core.Descending
	}
	return &out
}

// loadEvalsByRunIDs batches aiagent_run_eval rows by run_id, used to attach display-only
// quality_score/passed onto feedback rows.
func (svc *service) loadEvalsByRunIDs(kt *kit.Kit, ids []string) (
	map[string]tableaiagent.RunEvalTable, error) {

	out := make(map[string]tableaiagent.RunEvalTable, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range slice.Split(ids, int(core.DefaultMaxPageLimit)) {
		result, err := svc.cli.DataService().Aiagent.RunEval.List(kt, &core.ListReq{
			Filter: tools.ContainersExpression("run_id", chunk),
			Page:   &core.BasePage{Start: 0, Limit: core.DefaultMaxPageLimit},
		})
		if err != nil {
			logs.Errorf("list evals by run ids failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		for _, e := range result.Details {
			out[e.RunID] = e
		}
	}
	return out, nil
}

// dashboardFeedbackFilters builds the aiagent_run_feedback-column AtomRules for the feedback
// list. Every field here is a plain column filter, pushed straight to MySQL.
func dashboardFeedbackFilters(req *proto.DashboardFeedbackListReq) []*filter.AtomRule {
	if req == nil {
		return nil
	}
	extra := make([]*filter.AtomRule, 0, 8)
	if req.RunID != "" {
		extra = append(extra, tools.RuleEqual("run_id", req.RunID))
	}
	if req.SessionID != "" {
		extra = append(extra, tools.RuleEqual("session_id", req.SessionID))
	}
	if len(req.Users) > 0 {
		extra = append(extra, tools.RuleIn("user", req.Users))
	}
	if len(req.BkBizIDs) > 0 {
		extra = append(extra, tools.RuleIn("bk_biz_id", req.BkBizIDs))
	}
	if req.Scene != "" {
		extra = append(extra, tools.RuleEqual("scene", req.Scene))
	}
	if queryLike := strings.TrimSpace(req.QueryLike); queryLike != "" {
		extra = append(extra, tools.RuleCis("query", queryLike))
	}
	if req.Reaction != "" {
		extra = append(extra, tools.RuleEqual("reaction", string(req.Reaction)))
	}
	if req.Tag != "" {
		extra = append(extra, tools.RuleJSONContains("tags", req.Tag))
	}
	return extra
}

// dashboardFeedbackItem converts one aiagent_run_feedback row into a list item, attaching
// display-only quality_score/passed from evalMap when present.
func dashboardFeedbackItem(fb tableaiagent.FeedbackTable,
	evalMap map[string]tableaiagent.RunEvalTable) proto.DashboardFeedbackListItem {

	item := proto.DashboardFeedbackListItem{
		CreatedAt: string(fb.CreatedAt),
		RunID:     fb.RunID,
		SessionID: fb.SessionID,
		User:      fb.User,
		BkBizID:   fb.BkBizID,
		Scene:     string(fb.Scene),
		Query:     fb.Query,
		Reaction:  fb.Reaction,
		Tags:      tagsOrEmpty(fb.Tags),
		Comment:   converter.PtrToVal(fb.Comment),
	}
	if ev, ok := evalMap[fb.RunID]; ok {
		q := ev.QualityScore
		p := isPassed(ev.QualityScore, ev.Redlines)
		item.QualityScore = &q
		item.Passed = &p
	}
	return item
}

func tagsOrEmpty(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
