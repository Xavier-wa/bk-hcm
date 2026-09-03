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
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/kit"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

func TestTargetTranscriptEmptySkipsEval(t *testing.T) {
	if !(CandidateSet{}).TargetTranscriptEmpty("r1") {
		t.Fatal("missing candidates must be empty")
	}
	set := candidateSetOf([]CandidateRun{{Run: tableaiagent.RunTable{RunID: "r1"}}})
	if !set.TargetTranscriptEmpty("r1") {
		t.Fatal("empty items must skip eval")
	}
	set = candidateSetOf([]CandidateRun{{
		Run:        tableaiagent.RunTable{RunID: "r1"},
		Transcript: Transcript{Items: []TranscriptItem{{Type: enumor.AiagentTranscriptItemUser, Text: "hi"}}},
	}})
	if set.TargetTranscriptEmpty("r1") {
		t.Fatal("non-empty transcript must not skip")
	}
}

func candidateSetOf(ordered []CandidateRun) CandidateSet {
	byID := make(map[string]CandidateRun, len(ordered))
	for _, c := range ordered {
		byID[c.Run.RunID] = c
	}
	return CandidateSet{Ordered: ordered, ByID: byID}
}

func TestSkipExistingEval(t *testing.T) {
	existingEval := &dsaiagent.GetAiagentRunEvalResult{ID: "e1"}
	if !skipExistingEval(existingEval, false) {
		t.Fatal("existing row without overwrite must skip")
	}
	if skipExistingEval(existingEval, true) {
		t.Fatal("overwrite must not skip")
	}
	if skipExistingEval(nil, false) {
		t.Fatal("missing row must not skip")
	}
}

func TestShouldOverwriteEval(t *testing.T) {
	existingEval := &dsaiagent.GetAiagentRunEvalResult{ID: "e1"}
	cases := []struct {
		name         string
		overwrite    bool
		existingEval *dsaiagent.GetAiagentRunEvalResult
		want         bool
	}{
		{name: "overwrite existing", overwrite: true, existingEval: existingEval, want: true},
		{name: "overwrite missing", overwrite: true, existingEval: nil, want: false},
		{name: "create existing", overwrite: false, existingEval: existingEval, want: false},
		{name: "empty id", overwrite: true, existingEval: &dsaiagent.GetAiagentRunEvalResult{}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldOverwriteEval(tc.overwrite, tc.existingEval); got != tc.want {
				t.Fatalf("shouldOverwriteEval() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildSnapshotOmitsItems(t *testing.T) {
	candidates := []CandidateRun{
		{
			Run:        tableaiagent.RunTable{RunID: "r1"},
			Role:       enumor.AiagentEvalSnapshotRoleTarget,
			Transcript: Transcript{Items: []TranscriptItem{{Type: enumor.AiagentTranscriptItemUser, Text: "q"}}},
		},
	}
	snap := buildSnapshot(candidates, "r1", "r1", map[string]string{"r1": "brief"})
	if len(snap.Runs) != 1 || snap.Runs[0].Brief != "brief" || snap.Runs[0].RunID != "r1" {
		t.Fatalf("snapshot: %+v", snap)
	}
	rt := reflect.TypeOf(SnapshotRun{})
	if _, ok := rt.FieldByName("Items"); ok {
		t.Fatal("snapshot run must not carry items")
	}
}

func TestDetachFromRequestCancelSurvivesParentCancel(t *testing.T) {
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), constant.RidKey, "rid-value"))
	kt := &kit.Kit{Rid: "rid-1234567890abcd", Ctx: parent}
	detached := detachFromRequestCancel(kt)
	cancel()
	if detached.Ctx.Err() != nil {
		t.Fatalf("detached ctx should not be cancelled, err: %v", detached.Ctx.Err())
	}
	if detached.Rid != kt.Rid {
		t.Fatalf("rid must stay the same, got %s", detached.Rid)
	}
	if got := detached.Ctx.Value(constant.RidKey); got != "rid-value" {
		t.Fatalf("ctx value must be kept, got %v", got)
	}
}

type stubEvalModel struct {
	content          string
	reasoningContent string
	emptyCh          bool
}

func (m *stubEvalModel) GenerateContent(_ context.Context, _ *trpcmodel.Request) (
	<-chan *trpcmodel.Response, error) {

	ch := make(chan *trpcmodel.Response, 1)
	if !m.emptyCh {
		ch <- &trpcmodel.Response{
			Choices: []trpcmodel.Choice{{
				Message: trpcmodel.Message{
					Content:          m.content,
					ReasoningContent: m.reasoningContent,
				},
			}},
		}
	}
	close(ch)
	return ch, nil
}

func (m *stubEvalModel) Info() trpcmodel.Info {
	return trpcmodel.Info{Name: "stub-eval"}
}

func TestCallEvalModelAbortedWhenCtxCanceledAndChannelEmpty(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := callEvalModel(ctx, &stubEvalModel{emptyCh: true}, "sys", "user", 16, 0.2)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("want aborted canceled, got %v", err)
	}
	if !strings.Contains(err.Error(), "eval model aborted") {
		t.Fatalf("want aborted prefix, got %v", err)
	}
}

func TestCallEvalModelEmptyContent(t *testing.T) {
	_, err := callEvalModel(context.Background(), &stubEvalModel{emptyCh: true}, "sys", "user", 16, 0.2)
	if err == nil || err.Error() != "eval model returned empty content" {
		t.Fatalf("want empty content, got %v", err)
	}
}

func TestCallEvalModelUsesReasoningContent(t *testing.T) {
	out, err := callEvalModel(context.Background(), &stubEvalModel{reasoningContent: "{\"ok\":true}"},
		"sys", "user", 16, 0.2)
	if err != nil {
		t.Fatalf("callEvalModel: %v", err)
	}
	if out != "{\"ok\":true}" {
		t.Fatalf("got %q", out)
	}
}
