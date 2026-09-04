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

package eval

import (
	"fmt"
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
)

// evalResultFields is the aiagent_run_eval columns needed by the eval results list, skipping
// large JSON columns such as eval_result/context_snapshot/eval_trace.
var evalResultFields = []string{
	"run_id", "session_id", "user", "bk_biz_id", "scene", "query", "quality_score", "redlines",
	"created_at",
}

// ListDashboardEvalResults pages the aiagent_run_eval ledger for the ops dashboard. scene and
// query are denormalized onto aiagent_run_eval (mirrored from aiagent_run at eval time), so every
// filter, sort and page in this query plan is pushed straight to MySQL with no run-table join and
// no in-memory fallback.
func (svc *service) ListDashboardEvalResults(cts *rest.Contexts) (interface{}, error) {
	if err := svc.authorizeManage(cts.Kit); err != nil {
		return nil, err
	}
	req, err := decodeDashboardEvalResultsList(cts)
	if err != nil {
		return nil, err
	}
	expr, err := buildEvalResultsExpr(req)
	if err != nil {
		logs.Errorf("build eval results filter failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	return svc.listEvalResults(cts.Kit, req, expr)
}

func decodeDashboardEvalResultsList(cts *rest.Contexts) (*proto.DashboardEvalResultsListReq, error) {
	req := new(proto.DashboardEvalResultsListReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("eval dashboard eval results list decode request failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("eval dashboard eval results list validate request failed, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	return req, nil
}

// listEvalResults counts or pages aiagent_run_eval directly via MySQL. Every display field lives
// on the eval row itself, so no join back to aiagent_run is needed.
func (svc *service) listEvalResults(kt *kit.Kit, req *proto.DashboardEvalResultsListReq,
	expr *filter.Expression) (*proto.ListResp[proto.DashboardEvalResultsListItem], error) {

	if req.Page.Count {
		countResult, err := svc.cli.DataService().Aiagent.RunEval.List(kt, &core.ListReq{
			Filter: expr, Page: req.Page,
		})
		if err != nil {
			logs.Errorf("count eval results failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		return &proto.ListResp[proto.DashboardEvalResultsListItem]{Count: countResult.Count}, nil
	}

	listResult, err := svc.cli.DataService().Aiagent.RunEval.List(kt, &core.ListReq{
		Filter: expr, Fields: evalResultFields, Page: evalResultListPage(req.Page),
	})
	if err != nil {
		logs.Errorf("list eval results failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	items := make([]proto.DashboardEvalResultsListItem, 0, len(listResult.Details))
	for _, e := range listResult.Details {
		items = append(items, toEvalResultItem(e))
	}
	return &proto.ListResp[proto.DashboardEvalResultsListItem]{Details: items}, nil
}

func toEvalResultItem(e tableaiagent.RunEvalTable) proto.DashboardEvalResultsListItem {
	return proto.DashboardEvalResultsListItem{
		CreatedAt:    string(e.CreatedAt),
		RunID:        e.RunID,
		SessionID:    e.SessionID,
		User:         e.User,
		BkBizID:      e.BkBizID,
		Scene:        string(e.Scene),
		Query:        e.Query,
		QualityScore: e.QualityScore,
		Passed:       isPassed(e.QualityScore, e.Redlines),
	}
}

// evalResultListPage fills in the eval results list's default sort (created_at DESC) when the
// caller left page.sort/page.order empty, without mutating the caller's *core.BasePage.
func evalResultListPage(page *core.BasePage) *core.BasePage {
	out := *page
	if out.Sort == "" {
		out.Sort = proto.SortCreatedAt
	}
	if out.Order == "" {
		out.Order = core.Descending
	}
	return &out
}

// buildEvalResultsExpr builds the [from, to] + every aiagent_run_eval-column filter for the eval
// results list, including passed which is a MySQL AND/OR composite because it depends on both
// quality_score and the redlines JSON array length.
func buildEvalResultsExpr(req *proto.DashboardEvalResultsListReq) (*filter.Expression, error) {
	fromCST, err := rfc3339ToCSTDateTime(req.From)
	if err != nil {
		return nil, fmt.Errorf("invalid from: %w", err)
	}
	toCST, err := rfc3339ToCSTDateTime(req.To)
	if err != nil {
		return nil, fmt.Errorf("invalid to: %w", err)
	}

	rules := make([]filter.RuleFactory, 0, 8)
	rules = append(rules, tools.RuleGreaterThanEqual("created_at", fromCST))
	rules = append(rules, tools.RuleLessThanEqual("created_at", toCST))
	for _, rule := range dashboardEvalResultsFilters(req) {
		rules = append(rules, rule)
	}
	if req.Passed != nil {
		rules = append(rules, passedRule(converter.PtrToVal(req.Passed)))
	}
	return &filter.Expression{Op: filter.And, Rules: rules}, nil
}

// passedRule expresses "passed" as quality_score >= threshold AND redlines empty, and its
// negation as the OR of the two opposite conditions, so MySQL evaluates it instead of the row
// being loaded first and checked in Go.
func passedRule(passed bool) filter.RuleFactory {
	if passed {
		return &filter.Expression{
			Op: filter.And,
			Rules: []filter.RuleFactory{
				tools.RuleGreaterThanEqual("quality_score", passThreshold()),
				jsonLengthEqual("redlines", 0),
			},
		}
	}
	return &filter.Expression{
		Op: filter.Or,
		Rules: []filter.RuleFactory{
			tools.RuleLessThan("quality_score", passThreshold()),
			tools.RuleJsonLengthGreaterThan("redlines", 0),
		},
	}
}

func jsonLengthEqual(field string, value int) *filter.AtomRule {
	return &filter.AtomRule{Field: field, Op: filter.JSONLength.Factory(), Value: value}
}

// dashboardEvalResultsFilters builds the aiagent_run_eval-column AtomRules for the eval results
// list. scene and query_like are plain column filters now that both are denormalized onto
// aiagent_run_eval.
func dashboardEvalResultsFilters(req *proto.DashboardEvalResultsListReq) []*filter.AtomRule {
	if req == nil {
		return nil
	}
	extra := make([]*filter.AtomRule, 0, 7)
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
	if req.HasRedline != nil {
		if *req.HasRedline {
			extra = append(extra, tools.RuleJsonLengthGreaterThan("redlines", 0))
		} else {
			extra = append(extra, jsonLengthEqual("redlines", 0))
		}
	}
	return extra
}
