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
	"testing"
	"time"

	"hcm/pkg/criteria/enumor"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"trpc.group/trpc-go/trpc-agent-go/event"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

func TestReduceAGUIEventsKeepsUserDropsThinking(t *testing.T) {
	evts := []aguievents.Event{
		aguievents.NewThinkingStartEvent(),
		aguievents.NewTextMessageStartEvent("m1", aguievents.WithRole("user")),
		aguievents.NewTextMessageContentEvent("m1", "hello"),
		aguievents.NewTextMessageEndEvent("m1"),
		aguievents.NewThinkingEndEvent(),
	}
	got := ReduceAGUIEvents(evts)
	if len(got.Items) != 1 {
		t.Fatalf("items = %+v", got.Items)
	}
	if got.Items[0].Type != enumor.AiagentTranscriptItemUser || got.Items[0].Text != "hello" {
		t.Fatalf("item = %+v", got.Items[0])
	}
}

func TestSplitSessionTracksGroupsByInvocation(t *testing.T) {
	t1 := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	events := []event.Event{
		{
			InvocationID: "run-1",
			Timestamp:    t1,
			Response: &trpcmodel.Response{
				Choices: []trpcmodel.Choice{{
					Message: trpcmodel.Message{Role: trpcmodel.RoleUser, Content: "hi"},
				}},
			},
		},
		{
			InvocationID: "run-1",
			Timestamp:    t2,
			Response: &trpcmodel.Response{
				Choices: []trpcmodel.Choice{{
					Message: trpcmodel.Message{Role: trpcmodel.RoleAssistant, Content: "ok"},
				}},
			},
		},
		{
			InvocationID: "run-2",
			Timestamp:    t2.Add(time.Minute),
			Response: &trpcmodel.Response{
				Choices: []trpcmodel.Choice{{
					Message: trpcmodel.Message{Role: trpcmodel.RoleUser, Content: "next"},
				}},
			},
		},
	}
	got := SplitSessionTracks(events)
	if len(got) != 2 {
		t.Fatalf("tracks = %d, want 2", len(got))
	}
	run1 := got["run-1"]
	if len(run1.Transcript.Items) != 2 {
		t.Fatalf("run-1 items = %+v", run1.Transcript.Items)
	}
	if !run1.StartedAt.Equal(t1) || !run1.EndedAt.Equal(t2) {
		t.Fatalf("run-1 times = %v %v", run1.StartedAt, run1.EndedAt)
	}
	if SplitSessionEvents(events)["run-2"].Items[0].Text != "next" {
		t.Fatalf("SplitSessionEvents should keep transcript wrapper")
	}
}
