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
	"testing"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/constant"
)

func TestSyncHistoryReqValidate(t *testing.T) {
	req := &SyncHistoryReq{}
	if err := req.Validate(); err == nil {
		t.Fatal("empty session_codes must fail")
	}

	req = &SyncHistoryReq{SessionCodes: []string{""}}
	if err := req.Validate(); err == nil {
		t.Fatal("empty session_code item must fail")
	}

	over := make([]string, constant.MaxHistorySyncSessionLimit+1)
	for i := range over {
		over[i] = fmt.Sprintf("s%d", i)
	}
	req = &SyncHistoryReq{SessionCodes: over}
	if err := req.Validate(); err == nil {
		t.Fatal("over max session_codes must fail")
	}

	req = &SyncHistoryReq{SessionCodes: []string{"s1", "s1"}}
	if err := req.Validate(); err != nil {
		t.Fatalf("duplicate session_codes must pass, err: %v", err)
	}
	if len(req.SessionCodes) != 1 || req.SessionCodes[0] != "s1" {
		t.Fatalf("duplicate session_codes must unique, got %v", req.SessionCodes)
	}

	req = &SyncHistoryReq{SessionCodes: []string{"s1"}}
	if err := req.Validate(); err != nil {
		t.Fatalf("valid session_codes must pass, err: %v", err)
	}

	longCode := strings.Repeat("a", 129)
	req = &SyncHistoryReq{SessionCodes: []string{longCode}}
	if err := req.Validate(); err == nil {
		t.Fatal("session_code over 128 must fail")
	}
}

func TestReevalReqValidate(t *testing.T) {
	req := &ReevalReq{}
	if err := req.Validate(); err == nil {
		t.Fatal("missing overwrite must fail")
	}

	overwrite := false
	req = &ReevalReq{Overwrite: &overwrite}
	if err := req.Validate(); err != nil {
		t.Fatalf("overwrite=false must pass, err: %v", err)
	}

	overwrite = true
	req = &ReevalReq{Overwrite: &overwrite}
	if err := req.Validate(); err != nil {
		t.Fatalf("overwrite=true must pass, err: %v", err)
	}
}

func TestDashboardListReqValidateCountPage(t *testing.T) {
	period := PeriodFilter{From: "2026-08-01T00:00:00Z", To: "2026-08-02T00:00:00Z"}
	evalReq := &DashboardEvalResultsListReq{PeriodFilter: period, Page: core.NewCountPage()}
	if err := evalReq.Validate(); err != nil {
		t.Fatalf("eval results count page must pass, err: %v", err)
	}
	if evalReq.Page.Limit != 0 {
		t.Fatalf("eval results count page limit must remain zero, got %d", evalReq.Page.Limit)
	}

	feedbackReq := &DashboardFeedbackListReq{PeriodFilter: period, Page: core.NewCountPage()}
	if err := feedbackReq.Validate(); err != nil {
		t.Fatalf("feedback count page must pass, err: %v", err)
	}
	if feedbackReq.Page.Limit != 0 {
		t.Fatalf("feedback count page limit must remain zero, got %d", feedbackReq.Page.Limit)
	}
}
