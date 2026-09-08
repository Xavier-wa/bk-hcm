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

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
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

func TestReduceAGUIEventsUserMessageCustom(t *testing.T) {
	evt := aguievents.NewCustomEvent(constant.AGUIUserMessageCustomEventName, aguievents.WithValue(map[string]any{
		"id": "m1", "role": "user", "content": "from track",
	}))
	got := ReduceAGUIEvents([]aguievents.Event{evt})
	if len(got.Items) != 1 {
		t.Fatalf("items = %+v", got.Items)
	}
	if got.Items[0].Type != enumor.AiagentTranscriptItemUser || got.Items[0].Text != "from track" {
		t.Fatalf("item = %+v", got.Items[0])
	}
}
