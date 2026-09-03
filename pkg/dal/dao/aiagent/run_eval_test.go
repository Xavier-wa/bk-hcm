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

package aiagent

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/tools/times"
)

func TestBuildGapSQLLeftJoinNoJSONExtract(t *testing.T) {
	now := time.Date(2026, 6, 1, 15, 4, 5, 0, time.Local)
	sql, args, err := buildGapSQL(&AiagentRunEvalGapOption{Lookback: time.Hour, Limit: 10}, now)
	if err != nil {
		t.Fatalf("build gap sql failed, err: %v", err)
	}
	if !strings.Contains(sql, "LEFT JOIN") {
		t.Fatalf("gap sql must left join eval, sql: %s", sql)
	}
	if !strings.Contains(sql, "e.run_id IS NULL") {
		t.Fatalf("gap sql must filter missing eval, sql: %s", sql)
	}
	if strings.Contains(sql, "JSON_EXTRACT") {
		t.Fatal("gap sql must not use JSON_EXTRACT")
	}
	if strings.Contains(sql, "'finished'") || strings.Contains(sql, "'running'") {
		t.Fatalf("status filter must use named placeholders, sql: %s", sql)
	}
	if !strings.Contains(sql, "r.status IN (:term_status_0") {
		t.Fatalf("status filter missing named placeholders, sql: %s", sql)
	}
	for i, status := range enumor.TerminalAiagentRunStatuses() {
		key := fmt.Sprintf("term_status_%d", i)
		if args[key] != string(status) {
			t.Fatalf("args[%s] = %v, want %s", key, args[key], status)
		}
	}
	wantSince := times.FormatDateTimeCST(now.Add(-time.Hour))
	if args["since"] != wantSince {
		t.Fatalf("lookback since: %v, want %s", args["since"], wantSince)
	}
}

func TestBuildGapSQLRangeUsesCSTWallClock(t *testing.T) {
	from := time.Date(2026, 8, 15, 16, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 16, 16, 0, 0, 0, time.UTC)
	_, args, err := buildGapSQL(&AiagentRunEvalGapOption{From: from, To: to, Count: true}, time.Now())
	if err != nil {
		t.Fatalf("utc range gap sql failed, err: %v", err)
	}
	if args["from_tm"] != "2026-08-16 00:00:00" || args["to_tm"] != "2026-08-17 00:00:00" {
		t.Fatalf("utc range must format as CST, args: %+v", args)
	}
}

func TestBuildGapSQLCountAndRange(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	to := time.Date(2026, 1, 2, 0, 0, 0, 0, loc)
	sql, args, err := buildGapSQL(&AiagentRunEvalGapOption{
		From: from, To: to, Count: true,
	}, time.Now())
	if err != nil {
		t.Fatalf("range gap sql failed, err: %v", err)
	}
	if !strings.Contains(sql, "SELECT COUNT(*)") {
		t.Fatalf("count sql missing, sql: %s", sql)
	}
	if args["from_tm"] != times.FormatDateTimeCST(from) ||
		args["to_tm"] != times.FormatDateTimeCST(to) {
		t.Fatalf("range args: %+v", args)
	}
}

func TestBuildGapSQLDefaultLimit(t *testing.T) {
	_, args, err := buildGapSQL(&AiagentRunEvalGapOption{Lookback: time.Hour}, time.Now())
	if err != nil {
		t.Fatalf("default limit gap sql failed, err: %v", err)
	}
	if args["limit"] != uint(constant.DefaultAiagentRunEvalGapLimit) {
		t.Fatalf("limit = %v, want %d", args["limit"], constant.DefaultAiagentRunEvalGapLimit)
	}
}

func TestBuildGapSQLLookbackRequired(t *testing.T) {
	_, _, err := buildGapSQL(&AiagentRunEvalGapOption{}, time.Now())
	if errf.Error(err).Code != errf.InvalidParameter {
		t.Fatalf("empty lookback must be invalid, err: %v", err)
	}
}
