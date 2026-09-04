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
	"os"
	"testing"

	proto "hcm/pkg/api/agent-server/eval"
	"hcm/pkg/api/core"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/enumor"
	tableaiagent "hcm/pkg/dal/table/aiagent"
)

func TestMain(m *testing.M) {
	s := &cc.AgentServerSetting{}
	s.Eval.PassThreshold = 80
	cc.InitRuntime(s)
	os.Exit(m.Run())
}

func TestDashboardFeedbackItem(t *testing.T) {
	fb := tableaiagent.FeedbackTable{
		RunID: "r1", SessionID: "s1", Reaction: enumor.FeedbackReactionDislike,
		CreatedAt: "2026-08-16 10:00:00", Scene: "host_apply", Query: "申领主机",
		Tags: []string{"factual_error"},
	}
	q40 := 40
	evalMap := map[string]tableaiagent.RunEvalTable{
		"r1": {RunID: "r1", QualityScore: q40, Redlines: []string{"overpromise"}},
	}
	item := dashboardFeedbackItem(fb, evalMap)
	if item.Scene != "host_apply" || item.Query != "申领主机" {
		t.Fatalf("scene/query must come straight from the feedback row (no run join), got %+v", item)
	}
	if item.QualityScore == nil || *item.QualityScore != q40 {
		t.Fatalf("quality_score must be attached from evalMap, got %+v", item)
	}
	if item.Passed == nil || *item.Passed {
		t.Fatalf("passed must be attached and false given a redline, got %+v", item)
	}

	noEval := dashboardFeedbackItem(fb, map[string]tableaiagent.RunEvalTable{})
	if noEval.QualityScore != nil || noEval.Passed != nil {
		t.Fatalf("quality_score/passed must stay nil without a matching eval row, got %+v", noEval)
	}
}

func TestFeedbackRunIDs(t *testing.T) {
	got := feedbackRunIDs([]tableaiagent.FeedbackTable{{RunID: "r1"}, {RunID: "r2"}, {RunID: "r1"}})
	if len(got) != 2 || got[0] != "r1" || got[1] != "r2" {
		t.Fatalf("run ids must be unique and order-preserved, got %v", got)
	}
}

func TestFeedbackListPageDefaults(t *testing.T) {
	original := &core.BasePage{Start: 10, Limit: 20}
	got := feedbackListPage(original)
	if got.Sort != proto.SortCreatedAt || got.Order != core.Descending {
		t.Fatalf("empty sort/order must default to created_at DESC, got %+v", got)
	}
	if original.Sort != "" || original.Order != "" {
		t.Fatalf("feedbackListPage must not mutate the caller's page, got %+v", original)
	}

	explicit := &core.BasePage{Sort: proto.SortCreatedAt, Order: core.Ascending}
	if got := feedbackListPage(explicit); got.Sort != proto.SortCreatedAt || got.Order != core.Ascending {
		t.Fatalf("explicit sort/order must be kept, got %+v", got)
	}
}

func TestDashboardFeedbackListReqRejectsInvalidReaction(t *testing.T) {
	req := proto.DashboardFeedbackListReq{
		PeriodFilter: proto.PeriodFilter{From: "2026-08-01T00:00:00Z", To: "2026-08-02T00:00:00Z"},
		Reaction:     enumor.FeedbackReactionLike,
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("valid req must pass, err: %v", err)
	}
	req.Reaction = "up"
	if err := req.Validate(); err == nil {
		t.Fatal("reaction=up must fail")
	}
	// page.sort is not whitelisted at the API layer: any column name is accepted here and an
	// unsupported one only fails later at the MySQL layer (unknown column in order clause).
	req.Reaction = enumor.FeedbackReactionLike
	req.Page = &core.BasePage{Start: 0, Limit: 20, Sort: proto.SortQualityScore}
	if err := req.Validate(); err != nil {
		t.Fatalf("sort is not validated here, err: %v", err)
	}
}

func TestDashboardFeedbackFilters(t *testing.T) {
	got := dashboardFeedbackFilters(&proto.DashboardFeedbackListReq{
		Users: []string{"u1", "u2"}, BkBizIDs: []int64{100, 200}, RunID: "r1",
	})
	if len(got) != 3 {
		t.Fatalf("want run_id/users/bk_biz_ids rules, got %d", len(got))
	}
	if len(dashboardFeedbackFilters(&proto.DashboardFeedbackListReq{})) != 0 {
		t.Fatal("empty filters must be empty")
	}

	withSceneQuery := dashboardFeedbackFilters(&proto.DashboardFeedbackListReq{
		PeriodFilter: proto.PeriodFilter{Scene: "chat"}, QueryLike: "  hello  ",
	})
	if len(withSceneQuery) != 2 {
		t.Fatalf("want scene + query_like rules pushed to SQL, got %d: %+v",
			len(withSceneQuery), withSceneQuery)
	}
	if withSceneQuery[0].Field != "scene" || withSceneQuery[0].Value != "chat" {
		t.Fatalf("scene must be a plain equal rule, got %+v", withSceneQuery[0])
	}
	if withSceneQuery[1].Field != "query" || withSceneQuery[1].Value != "hello" {
		t.Fatalf("query_like must be a trimmed cis rule, got %+v", withSceneQuery[1])
	}
}
