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

package metrics

import (
	"strings"
	"testing"
	"time"

	"hcm/pkg/criteria/enumor"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestAiagentMetricRules(t *testing.T) {
	EnsureAiagentMetric()

	if isAiagentRunTotalState(enumor.AiagentRunStateRunning) {
		t.Fatal("running must not be a run_total state")
	}
	IncAiagentRunTotal(123, enumor.AiagentRunStateRunning, "chat")
	assertNoRunTotalState(t, string(enumor.AiagentRunStateRunning))

	IncAiagentRunTotal(123, enumor.AiagentRunStateFinished, "chat")
	if got := testutil.ToFloat64(aiagent.runTotal.With(prometheus.Labels{
		LabelState:     string(enumor.AiagentRunStateFinished),
		LabelBKCCBizID: "123",
		LabelScene:     "chat",
	})); got < 1 {
		t.Fatalf("run_total finished = %v, want >= 1", got)
	}

	if isAiagentRunTotalState(enumor.AiagentRunState("cancel")) {
		t.Fatal("cancel must not be a run_total state")
	}
	IncAiagentRunTotal(123, enumor.AiagentRunState("cancel"), "chat")
	assertNoRunTotalState(t, "cancel")

	inflight := aiagent.runInflight.With(prometheus.Labels{LabelBKCCBizID: "123"})
	beforeInflight := testutil.ToFloat64(inflight)
	IncAiagentRunCancel(123, "chat")
	if testutil.ToFloat64(inflight) != beforeInflight {
		t.Fatal("cancel must not change inflight")
	}
	if got := testutil.ToFloat64(aiagent.runCancel.With(prometheus.Labels{
		LabelBKCCBizID: "123",
		LabelScene:     "chat",
	})); got < 1 {
		t.Fatalf("run_cancel = %v, want >= 1", got)
	}

	beforeDuration := testutil.CollectAndCount(aiagent.runDuration)
	ObserveAiagentRunDuration(123, enumor.AiagentRunState("cancel"), "chat", time.Second)
	ObserveAiagentRunDuration(123, enumor.AiagentRunStateUnknown, "chat", time.Second)
	if testutil.CollectAndCount(aiagent.runDuration) != beforeDuration {
		t.Fatal("duration must only accept finished/error")
	}
	ObserveAiagentRunDuration(123, enumor.AiagentRunStateFinished, "chat", time.Second)
	ObserveAiagentRunDuration(123, enumor.AiagentRunStateError, "chat", time.Second)
	if testutil.CollectAndCount(aiagent.runDuration) <= beforeDuration {
		t.Fatal("duration must record finished and error")
	}
	assertDurationHasBizID(t, "123")
}

func TestAiagentMetricNoWriteFail(t *testing.T) {
	EnsureAiagentMetric()

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, mf := range families {
		name := mf.GetName()
		if strings.Contains(name, "write_fail") || strings.Contains(name, "run_observation_write_fail") {
			t.Fatalf("unexpected write fail metric: %s", name)
		}
	}
}

func TestAiagentRunInflightByBizID(t *testing.T) {
	EnsureAiagentMetric()

	bizA := aiagent.runInflight.With(prometheus.Labels{LabelBKCCBizID: "11"})
	bizB := aiagent.runInflight.With(prometheus.Labels{LabelBKCCBizID: "22"})
	beforeA := testutil.ToFloat64(bizA)
	beforeB := testutil.ToFloat64(bizB)

	AddAiagentRunInflight(11, 1)
	AddAiagentRunInflight(22, 1)
	if got := testutil.ToFloat64(bizA); got != beforeA+1 {
		t.Fatalf("biz 11 inflight = %v, want %v", got, beforeA+1)
	}
	if got := testutil.ToFloat64(bizB); got != beforeB+1 {
		t.Fatalf("biz 22 inflight = %v, want %v", got, beforeB+1)
	}

	AddAiagentRunInflight(11, -1)
	if got := testutil.ToFloat64(bizA); got != beforeA {
		t.Fatalf("biz 11 after release = %v, want %v", got, beforeA)
	}
	if got := testutil.ToFloat64(bizB); got != beforeB+1 {
		t.Fatalf("biz 22 must stay held, inflight = %v", got)
	}
	AddAiagentRunInflight(22, -1)
}

func TestAiagentEmptySceneLabel(t *testing.T) {
	EnsureAiagentMetric()

	IncAiagentRunTotal(321, enumor.AiagentRunStateFinished, "")
	if got := testutil.ToFloat64(aiagent.runTotal.With(prometheus.Labels{
		LabelState:     string(enumor.AiagentRunStateFinished),
		LabelBKCCBizID: "321",
		LabelScene:     "",
	})); got < 1 {
		t.Fatalf("empty scene run_total = %v, want >= 1", got)
	}

	IncAiagentRunCancel(321, "")
	if got := testutil.ToFloat64(aiagent.runCancel.With(prometheus.Labels{
		LabelBKCCBizID: "321",
		LabelScene:     "",
	})); got < 1 {
		t.Fatalf("empty scene run_cancel = %v, want >= 1", got)
	}
}

func assertDurationHasBizID(t *testing.T, bizID string) {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	name := Namespace + "_" + AiagentSubSys + "_run_duration_seconds"
	found := false
	for _, mf := range families {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == LabelBKCCBizID && lp.GetValue() == bizID {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("run_duration_seconds must expose bkcc_biz_id=%s", bizID)
	}
}

func assertNoRunTotalState(t *testing.T, state string) {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, mf := range families {
		if mf.GetName() != Namespace+"_"+AiagentSubSys+"_run_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == LabelState && lp.GetValue() == state {
					t.Fatalf("run_total must not expose state=%s", state)
				}
			}
		}
	}
}
