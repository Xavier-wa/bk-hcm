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
	"testing"

	proto "hcm/pkg/api/agent-server/eval"
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/converter"
)

func TestToEvalResultItem(t *testing.T) {
	e := tableaiagent.RunEvalTable{
		RunID: "r1", SessionID: "s1", User: "u1", BkBizID: 100,
		Scene: enumor.IntentTypeHostApply, Query: "apply a cvm",
		QualityScore: 90, Redlines: types.StringArray{},
	}
	item := toEvalResultItem(e)
	if item.RunID != "r1" || item.SessionID != "s1" || item.User != "u1" || item.BkBizID != 100 {
		t.Fatalf("identity fields must come straight from the eval row, got %+v", item)
	}
	if item.Scene != string(enumor.IntentTypeHostApply) || item.Query != "apply a cvm" {
		t.Fatalf("scene/query must come straight from the eval row (no run join), got %+v", item)
	}
	if item.QualityScore != 90 || !item.Passed {
		t.Fatalf("quality/passed mismatch, got %+v", item)
	}
}

func TestDashboardEvalResultsFilters(t *testing.T) {
	got := dashboardEvalResultsFilters(&proto.DashboardEvalResultsListReq{
		RunID: "r1", SessionID: "s1", Users: []string{"u1", "u2"}, BkBizIDs: []int64{100, 200},
	})
	if len(got) != 4 {
		t.Fatalf("want run_id/session_id/users/bk_biz_ids rules, got %d", len(got))
	}
	if len(dashboardEvalResultsFilters(&proto.DashboardEvalResultsListReq{})) != 0 {
		t.Fatal("empty filters must be empty")
	}
}

func TestDashboardEvalResultsFiltersSceneAndQueryLike(t *testing.T) {
	got := dashboardEvalResultsFilters(&proto.DashboardEvalResultsListReq{
		PeriodFilter: proto.PeriodFilter{Scene: "chat"}, QueryLike: "  hello  ",
	})
	if len(got) != 2 {
		t.Fatalf("want scene + query_like rules pushed to SQL, got %d: %+v", len(got), got)
	}
	if got[0].Field != "scene" || got[0].Op != filter.Equal.Factory() || got[0].Value != "chat" {
		t.Fatalf("scene must be a plain equal rule on aiagent_run_eval, got %+v", got[0])
	}
	if got[1].Field != "query" || got[1].Op != filter.ContainsInsensitive.Factory() || got[1].Value != "hello" {
		t.Fatalf("query_like must be a trimmed cis rule on aiagent_run_eval, got %+v", got[1])
	}

	if len(dashboardEvalResultsFilters(&proto.DashboardEvalResultsListReq{QueryLike: "   "})) != 0 {
		t.Fatal("blank query_like must not push a rule")
	}
}

func TestDashboardEvalResultsFiltersHasRedline(t *testing.T) {
	has := dashboardEvalResultsFilters(&proto.DashboardEvalResultsListReq{
		HasRedline: converter.ValToPtr(true),
	})
	if len(has) != 1 || has[0].Field != "redlines" || has[0].Op != filter.JSONLengthGreaterThan.Factory() {
		t.Fatalf("has_redline=true must push json_length_gt(redlines, 0), got %+v", has)
	}

	none := dashboardEvalResultsFilters(&proto.DashboardEvalResultsListReq{
		HasRedline: converter.ValToPtr(false),
	})
	if len(none) != 1 || none[0].Field != "redlines" || none[0].Op != filter.JSONLength.Factory() ||
		none[0].Value != 0 {
		t.Fatalf("has_redline=false must push json_length(redlines) = 0, got %+v", none)
	}
}

func TestBuildEvalResultsExprPushesFiltersToSQL(t *testing.T) {
	req := &proto.DashboardEvalResultsListReq{
		PeriodFilter: proto.PeriodFilter{
			From: "2026-08-16T00:00:00+08:00", To: "2026-08-23T23:59:59+08:00", Scene: "chat",
		},
		RunID: "r1", QueryLike: "hello",
	}
	expr, err := buildEvalResultsExpr(req)
	if err != nil {
		t.Fatalf("valid rfc3339 range must not error, err: %v", err)
	}
	// from, to, run_id, scene, query_like (all dashboardEvalResultsFilters) = 5 rules, no passed
	// rule appended.
	if len(expr.Rules) != 5 {
		t.Fatalf("want from/to/run_id/scene/query_like rules pushed to SQL, got %d: %+v",
			len(expr.Rules), expr.Rules)
	}
	if expr.Op != filter.And {
		t.Fatalf("eval results filter must be AND-combined, got %s", expr.Op)
	}

	if _, err := buildEvalResultsExpr(&proto.DashboardEvalResultsListReq{
		PeriodFilter: proto.PeriodFilter{From: "bad-from", To: "2026-08-23T23:59:59+08:00"},
	}); err == nil {
		t.Fatal("invalid from must error instead of building a bad filter")
	}
}

func TestBuildEvalResultsExprPassedAppendsComposite(t *testing.T) {
	req := &proto.DashboardEvalResultsListReq{
		PeriodFilter: proto.PeriodFilter{
			From: "2026-08-16T00:00:00+08:00", To: "2026-08-23T23:59:59+08:00",
		},
		Passed: converter.ValToPtr(true),
	}
	expr, err := buildEvalResultsExpr(req)
	if err != nil {
		t.Fatalf("valid rfc3339 range must not error, err: %v", err)
	}
	// from, to, plus one passed composite rule pushed to SQL instead of filtered after loading rows.
	if len(expr.Rules) != 3 {
		t.Fatalf("want from/to/passed rules, got %d: %+v", len(expr.Rules), expr.Rules)
	}
	passedExpr, ok := expr.Rules[2].(*filter.Expression)
	if !ok || passedExpr.Op != filter.And || len(passedExpr.Rules) != 2 {
		t.Fatalf("passed=true must be an AND composite, got %+v", expr.Rules[2])
	}
}

func TestPassedRule(t *testing.T) {
	passedExpr, ok := passedRule(true).(*filter.Expression)
	if !ok || passedExpr.Op != filter.And || len(passedExpr.Rules) != 2 {
		t.Fatalf("passed=true must AND quality_score>=threshold with empty redlines, got %+v",
			passedRule(true))
	}

	failedExpr, ok := passedRule(false).(*filter.Expression)
	if !ok || failedExpr.Op != filter.Or || len(failedExpr.Rules) != 2 {
		t.Fatalf("passed=false must OR quality_score<threshold with non-empty redlines, got %+v",
			passedRule(false))
	}
}

func TestJSONLengthEqual(t *testing.T) {
	rule := jsonLengthEqual("redlines", 0)
	if rule.Field != "redlines" || rule.Op != filter.JSONLength.Factory() || rule.Value != 0 {
		t.Fatalf("want redlines json_length = 0, got %+v", rule)
	}
}

func TestEvalResultListPageDefaults(t *testing.T) {
	original := &core.BasePage{Start: 10, Limit: 20}
	got := evalResultListPage(original)
	if got.Sort != proto.SortCreatedAt || got.Order != core.Descending {
		t.Fatalf("empty sort/order must default to created_at DESC, got %+v", got)
	}
	if original.Sort != "" || original.Order != "" {
		t.Fatalf("evalResultListPage must not mutate the caller's page, got %+v", original)
	}

	explicit := &core.BasePage{Sort: proto.SortQualityScore, Order: core.Ascending}
	if got := evalResultListPage(explicit); got.Sort != proto.SortQualityScore || got.Order != core.Ascending {
		t.Fatalf("explicit sort/order must be kept, got %+v", got)
	}
}
