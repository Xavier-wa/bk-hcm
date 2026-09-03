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
	"encoding/json"
	"strings"
	"testing"
	"time"

	proto "hcm/pkg/api/agent-server/eval"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/kit"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

func TestSkipHistoryTrack(t *testing.T) {
	tr := Transcript{Items: []TranscriptItem{
		{Type: enumor.AiagentTranscriptItemUser, Text: "hi"},
	}}
	existing := map[string]struct{}{"old": {}}
	cases := []struct {
		name  string
		runID string
		tr    Transcript
		want  string
	}{
		{name: "empty run_id", want: skipReasonEmptyRunID},
		{name: "too long", runID: strings.Repeat("a", constant.AiagentRunIDMaxLen+1), tr: tr,
			want: skipReasonRunIDTooLong},
		{name: "empty transcript", runID: "r1", want: skipReasonEmptyTranscript},
		{name: "already exists", runID: "old", tr: tr, want: skipReasonAlreadyExists},
		{name: "ok", runID: "r1", tr: tr},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := skipHistoryTrack(tc.runID, tc.tr, existing)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatOccurredAt(t *testing.T) {
	if got := formatOccurredAt(time.Time{}); got != "" {
		t.Fatalf("zero time must be empty, got %q", got)
	}
	ts := time.Date(2026, 9, 3, 1, 2, 3, 0, time.UTC)
	got := formatOccurredAt(ts)
	if got == "" {
		t.Fatal("non-zero time must format")
	}
	if _, err := time.Parse(constant.DateTimeLayout, got); err != nil {
		t.Fatalf("format must be datetime layout, got %q err %v", got, err)
	}
}

func TestNewHistorySyncerNilClient(t *testing.T) {
	s := NewHistorySyncer(nil, nil, "app")
	if s == nil {
		t.Fatal("NewHistorySyncer must return instance")
	}
	_, err := s.Sync(kit.New(), &proto.SyncHistoryReq{SessionCodes: []string{"s1"}})
	if err == nil {
		t.Fatal("nil client must fail")
	}
}

func TestStatusFromAGUITrackEvents(t *testing.T) {
	finished, err := aguievents.NewRunFinishedEvent("t1", "r-ok").ToJSON()
	if err != nil {
		t.Fatalf("marshal finished: %v", err)
	}
	runErr, err := aguievents.NewRunErrorEvent("boom", aguievents.WithRunID("r-err")).ToJSON()
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	started, err := aguievents.NewRunStartedEvent("t1", "r-open").ToJSON()
	if err != nil {
		t.Fatalf("marshal started: %v", err)
	}
	startedThenErr, err := aguievents.NewRunStartedEvent("t1", "r-no-id").ToJSON()
	if err != nil {
		t.Fatalf("marshal started then err: %v", err)
	}
	errNoRunID, err := aguievents.NewRunErrorEvent("fail").ToJSON()
	if err != nil {
		t.Fatalf("marshal error without run id: %v", err)
	}
	lastWinsStart, err := aguievents.NewRunFinishedEvent("t1", "r-last").ToJSON()
	if err != nil {
		t.Fatalf("marshal last-wins start: %v", err)
	}
	lastWinsEnd, err := aguievents.NewRunErrorEvent("later", aguievents.WithRunID("r-last")).ToJSON()
	if err != nil {
		t.Fatalf("marshal last-wins end: %v", err)
	}

	events := []session.TrackEvent{
		{Payload: json.RawMessage(`{not json`)},
		{Payload: finished},
		{Payload: runErr},
		{Payload: started},
		{Payload: startedThenErr},
		{Payload: errNoRunID},
		{Payload: lastWinsStart},
		{Payload: lastWinsEnd},
	}
	got := statusFromAGUITrackEvents(events)
	assertStatus(t, got, "r-ok", enumor.AiagentRunStatusFinished)
	assertStatus(t, got, "r-err", enumor.AiagentRunStatusError)
	assertStatus(t, got, "r-open", enumor.AiagentRunStatusUnknown)
	assertStatus(t, got, "r-no-id", enumor.AiagentRunStatusError)
	assertStatus(t, got, "r-last", enumor.AiagentRunStatusError)
}

func TestHistoryRunStatus(t *testing.T) {
	byRun := map[string]enumor.AiagentRunStatus{
		"r1": enumor.AiagentRunStatusFinished,
	}
	if got := historyRunStatus("r1", byRun); got != enumor.AiagentRunStatusFinished {
		t.Fatalf("got %s, want finished", got)
	}
	if got := historyRunStatus("missing", byRun); got != enumor.AiagentRunStatusUnknown {
		t.Fatalf("missing run must be unknown, got %s", got)
	}
	if got := historyRunStatus("r1", nil); got != enumor.AiagentRunStatusUnknown {
		t.Fatalf("nil map must be unknown, got %s", got)
	}
}

func TestCountMissingSessions(t *testing.T) {
	found := []tableaiagent.SessionTable{{SessionCode: "s1"}}
	n := countMissingSessions(kit.New(), []string{"s1", "s2", "s3"}, found)
	if n != 2 {
		t.Fatalf("missing count = %d, want 2", n)
	}
}

func TestTrackRunIDs(t *testing.T) {
	got := trackRunIDs(map[string]SessionTrack{
		"":   {},
		"r1": {},
		"r2": {},
	})
	if len(got) != 2 {
		t.Fatalf("got %d run ids, want 2", len(got))
	}
}

func assertStatus(t *testing.T, got map[string]enumor.AiagentRunStatus, runID string,
	want enumor.AiagentRunStatus) {

	t.Helper()
	if got[runID] != want {
		t.Fatalf("run %s status = %s, want %s", runID, got[runID], want)
	}
}
