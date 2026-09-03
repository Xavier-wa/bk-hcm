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
	"testing"
	"time"

	"hcm/pkg/criteria/constant"
)

func TestListAiagentRunEvalGapReqFromTo(t *testing.T) {
	req := &ListAiagentRunEvalGapReq{
		From: "2026-01-01T00:00:00+08:00",
		To:   "2026-01-02T00:00:00+08:00",
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("std format range must pass, err: %v", err)
	}
	from, err := req.FromTime()
	if err != nil {
		t.Fatalf("parse from failed, err: %v", err)
	}
	to, err := req.ToTime()
	if err != nil {
		t.Fatalf("parse to failed, err: %v", err)
	}
	wantFrom, err := time.Parse(constant.TimeStdFormat, req.From)
	if err != nil {
		t.Fatalf("parse want from failed, err: %v", err)
	}
	wantTo, err := time.Parse(constant.TimeStdFormat, req.To)
	if err != nil {
		t.Fatalf("parse want to failed, err: %v", err)
	}
	if !from.Equal(wantFrom) || !to.Equal(wantTo) {
		t.Fatalf("parsed range: from=%v to=%v", from, to)
	}

	req.From = "2026-01-01 00:00:00"
	if err := req.Validate(); err == nil {
		t.Fatal("datetime layout from must fail")
	}
}

func TestListAiagentRunEvalGapReqDefaultLimit(t *testing.T) {
	req := &ListAiagentRunEvalGapReq{LookbackSec: 60}
	if err := req.Validate(); err != nil {
		t.Fatalf("lookback gap req must pass, err: %v", err)
	}
	if req.Limit != constant.DefaultAiagentRunEvalGapLimit {
		t.Fatalf("limit = %d, want %d", req.Limit, constant.DefaultAiagentRunEvalGapLimit)
	}

	req.Limit = 50
	if err := req.Validate(); err != nil {
		t.Fatalf("custom limit must pass, err: %v", err)
	}
	if req.Limit != 50 {
		t.Fatalf("custom limit overwritten, got %d", req.Limit)
	}
}
