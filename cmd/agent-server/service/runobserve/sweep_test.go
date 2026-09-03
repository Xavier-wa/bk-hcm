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

package runobserve

import (
	"testing"

	"hcm/pkg/cc"
)

func TestNewSweepCronTaskDisabled(t *testing.T) {
	got, err := newSweepCronTask(cc.AiagentRunSweepConfig{Enabled: false}, nil, nil)
	if err != nil {
		t.Fatalf("disabled sweep must not error, err: %v", err)
	}
	if got != nil {
		t.Fatal("enabled=false must not register sweep task")
	}
}

func TestNewSweepCronTaskEnabled(t *testing.T) {
	cfg := cc.AiagentRunSweepConfig{Enabled: true, Interval: "10m"}
	got, err := newSweepCronTask(cfg, nil, nil)
	if err != nil {
		t.Fatalf("enabled sweep failed, err: %v", err)
	}
	if got == nil {
		t.Fatal("enabled sweep must return a task")
	}
}

func TestNextSweepLimit(t *testing.T) {
	tests := []struct {
		name    string
		scanned int
		want    int
	}{
		{name: "first page", scanned: 0, want: sweepPageLimit},
		{name: "mid round", scanned: 500, want: sweepPageLimit},
		{name: "last partial page", scanned: maxSweepPerRound - 30, want: 30},
		{name: "cap reached", scanned: maxSweepPerRound, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := nextSweepLimit(tc.scanned); got != tc.want {
				t.Fatalf("nextSweepLimit(%d) = %d, want %d", tc.scanned, got, tc.want)
			}
		})
	}
}

func TestSweepPageOutcome(t *testing.T) {
	tests := []struct {
		name             string
		listed           int
		limit            int
		scanned          int
		wantDone         bool
		wantLeftoverHint bool
	}{
		{name: "empty", listed: 0, limit: 100, scanned: 0, wantDone: true},
		{name: "short page", listed: 20, limit: 100, scanned: 20, wantDone: true},
		{name: "full page continue", listed: 100, limit: 100, scanned: 100, wantDone: false},
		{
			name: "hit cap on full page", listed: 100, limit: 100,
			scanned: maxSweepPerRound, wantDone: true, wantLeftoverHint: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			done, leftover := sweepPageOutcome(tc.listed, tc.limit, tc.scanned)
			if done != tc.wantDone || leftover != tc.wantLeftoverHint {
				t.Fatalf("got done=%t leftover=%t, want done=%t leftover=%t",
					done, leftover, tc.wantDone, tc.wantLeftoverHint)
			}
		})
	}
}
