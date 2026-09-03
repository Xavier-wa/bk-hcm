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
	"context"
	"encoding/json"
	"testing"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/metrics"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/prometheus/client_golang/prometheus"
)

func TestObserveEventsSkipWithoutMeta(t *testing.T) {
	metrics.EnsureAiagentMetric()
	before := currentInflight(t)

	ObserveEvents(context.Background(), []aguievents.Event{
		aguievents.NewRunStartedEvent("t1", "r1"),
	})
	if got := currentInflight(t); got != before {
		t.Fatalf("history / no meta must skip metric, inflight = %v want %v", got, before)
	}
}

func TestObserveEventsStartedFinished(t *testing.T) {
	metrics.EnsureAiagentMetric()
	meta := NewRunMeta("r-obs", "sess", "user", enumor.IntentTypeChat, 9, nil)
	ctx := WithRunMeta(context.Background(), meta)
	before := currentInflight(t)

	ObserveEvents(ctx, []aguievents.Event{aguievents.NewRunStartedEvent("t1", "r-obs")})
	if got := currentInflight(t); got != before+1 {
		t.Fatalf("inflight after started = %v, want %v", got, before+1)
	}

	ObserveEvents(ctx, []aguievents.Event{aguievents.NewRunFinishedEvent("t1", "r-obs")})
	if got := currentInflight(t); got != before {
		t.Fatalf("inflight after finished = %v, want %v", got, before)
	}
}

func TestAfterTranslateRecordsStartedWithoutRewriting(t *testing.T) {
	metrics.EnsureAiagentMetric()
	meta := NewRunMeta("", "", "", enumor.IntentTypeChat, 0, nil)
	ctx := WithRunMeta(context.Background(), meta)
	before := currentInflight(t)
	started := aguievents.NewRunStartedEvent("t1", "r-after")

	got, err := AfterTranslate(ctx, started)
	if err != nil {
		t.Fatalf("AfterTranslate err: %v", err)
	}
	if got != started {
		t.Fatal("AfterTranslate must return the original event")
	}
	if inflight := currentInflight(t); inflight != before+1 {
		t.Fatalf("inflight after AfterTranslate started = %v, want %v", inflight, before+1)
	}
	EndRun(ctx)
}

func TestBeginRunAndEndRunPairInflight(t *testing.T) {
	metrics.EnsureAiagentMetric()
	meta := NewRunMeta("", "", "", enumor.IntentTypeChat, 0, nil)
	ctx := WithRunMeta(context.Background(), meta)
	before := currentInflight(t)

	BeginRun(ctx)
	if got := currentInflight(t); got != before+1 {
		t.Fatalf("inflight after begin = %v, want %v", got, before+1)
	}
	BeginRun(ctx)
	if got := currentInflight(t); got != before+1 {
		t.Fatalf("second begin must be idempotent, inflight = %v", got)
	}

	ObserveEvents(ctx, []aguievents.Event{aguievents.NewRunFinishedEvent("t1", "r-begin")})
	if got := currentInflight(t); got != before {
		t.Fatalf("inflight after finished = %v, want %v", got, before)
	}
	EndRun(ctx)
	if got := currentInflight(t); got != before {
		t.Fatalf("end after finished must not go negative, inflight = %v", got)
	}
}

func TestFinishedWithoutHoldDoesNotGoNegative(t *testing.T) {
	metrics.EnsureAiagentMetric()
	meta := NewRunMeta("", "", "", enumor.IntentTypeChat, 0, nil)
	ctx := WithRunMeta(context.Background(), meta)
	before := currentInflight(t)

	ObserveEvents(ctx, []aguievents.Event{aguievents.NewRunFinishedEvent("t1", "r-orphan-finish")})
	if got := currentInflight(t); got != before {
		t.Fatalf("finished without begin must not change inflight, got %v want %v", got, before)
	}
}

func TestEndRunReleasesHoldWithoutTerminalEvent(t *testing.T) {
	metrics.EnsureAiagentMetric()
	meta := NewRunMeta("", "", "", enumor.IntentTypeChat, 0, nil)
	ctx := WithRunMeta(context.Background(), meta)
	before := currentInflight(t)

	BeginRun(ctx)
	EndRun(ctx)
	if got := currentInflight(t); got != before {
		t.Fatalf("begin/end without terminal event inflight = %v, want %v", got, before)
	}
}

func TestNormalizeScene(t *testing.T) {
	if got := NormalizeScene(enumor.IntentTypeHostApply); got != enumor.IntentTypeHostApply {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeScene(""); got != "" {
		t.Fatalf("empty tag = %q, want empty", got)
	}
	if got := NormalizeScene(enumor.IntentTypeUnsupported); got != "" {
		t.Fatalf("unsupported = %q, want empty", got)
	}
	if got := NormalizeScene(enumor.IntentType("unknown")); got != "" {
		t.Fatalf("unknown = %q, want empty", got)
	}
}

func TestSceneForMetric(t *testing.T) {
	(*RunMeta)(nil).SetScene(enumor.IntentTypeChat)
	if got := (*RunMeta)(nil).SceneForMetric(); got != "" {
		t.Fatalf("nil meta = %q, want empty", got)
	}
	meta := &RunMeta{}
	if got := meta.SceneForMetric(); got != "" {
		t.Fatalf("entry scene = %q, want empty", got)
	}
	meta.SetScene(enumor.IntentTypeHostApply)
	if got := meta.SceneForMetric(); got != "host_apply" {
		t.Fatalf("scene = %q, want host_apply", got)
	}
	meta.SetScene("")
	if got := meta.SceneForMetric(); got != "" {
		t.Fatalf("cleared scene = %q, want empty", got)
	}
	if got := NewRunMeta("", "", "", enumor.IntentTypeChat, 1, nil).SceneForMetric(); got != "chat" {
		t.Fatalf("NewRunMeta scene = %q, want chat", got)
	}
}

func TestRunMetaSceneConcurrentAccess(t *testing.T) {
	meta := NewRunMeta("", "", "", enumor.IntentTypeChat, 0, nil)
	const n = 1000
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < n; i++ {
			meta.SetScene(enumor.IntentTypeHostApply)
			_ = meta.SceneForMetric()
		}
	}()
	for i := 0; i < n; i++ {
		meta.SetScene(enumor.IntentTypeResourceQuery)
		_ = meta.SceneForMetric()
	}
	<-done
}

func TestRefreshSceneFromStateDelta(t *testing.T) {
	meta := &RunMeta{}
	ctx := WithRunMeta(context.Background(), meta)
	raw, err := json.Marshal(string(enumor.IntentTypeResourceQuery))
	if err != nil {
		t.Fatalf("marshal session_tag: %v", err)
	}
	RefreshScene(ctx, map[string][]byte{constant.StateKeySessionTag: raw})
	if got := meta.SceneForMetric(); got != "resource_query" {
		t.Fatalf("got %q, want resource_query", got)
	}

	RefreshScene(context.Background(), map[string][]byte{constant.StateKeySessionTag: raw})
	RefreshScene(ctx, map[string][]byte{"other": raw})
	if got := meta.SceneForMetric(); got != "resource_query" {
		t.Fatalf("unrelated delta changed scene to %q", got)
	}
}

func TestRefreshSceneKeepsEntryWhenTagEmpty(t *testing.T) {
	meta := NewRunMeta("", "", "", enumor.IntentTypeHostApply, 0, nil)
	ctx := WithRunMeta(context.Background(), meta)
	raw, err := json.Marshal("")
	if err != nil {
		t.Fatalf("marshal empty session_tag: %v", err)
	}
	RefreshScene(ctx, map[string][]byte{constant.StateKeySessionTag: raw})
	if got := meta.SceneForMetric(); got != "host_apply" {
		t.Fatalf("got %q, want host_apply", got)
	}
}

func TestRefreshSceneKeepsEntryWhenTagUnknown(t *testing.T) {
	meta := NewRunMeta("", "", "", enumor.IntentTypeHostApply, 0, nil)
	ctx := WithRunMeta(context.Background(), meta)
	raw, err := json.Marshal(string(enumor.IntentTypeUnsupported))
	if err != nil {
		t.Fatalf("marshal session_tag: %v", err)
	}
	RefreshScene(ctx, map[string][]byte{constant.StateKeySessionTag: raw})
	if got := meta.SceneForMetric(); got != "host_apply" {
		t.Fatalf("got %q, want host_apply", got)
	}
}

func currentInflight(t *testing.T) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, mf := range families {
		if mf.GetName() != "hcm_aiagent_run_inflight" {
			continue
		}
		var sum float64
		for _, m := range mf.GetMetric() {
			sum += m.GetGauge().GetValue()
		}
		return sum
	}
	return 0
}
