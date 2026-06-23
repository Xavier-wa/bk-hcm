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

package aguievent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"hcm/pkg/criteria/constant"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/graph"
)

func TestIsToolConfirmInterruptKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{key: constant.ToolConfirmInterruptKey, want: true},
		{key: "tool_confirm:create_biz_apply:c1", want: true},
		{key: constant.HITLInterruptKey, want: false},
		{key: "fallback", want: false},
		{key: "", want: false},
	}
	for _, tc := range tests {
		if got := isToolConfirmInterruptKey(tc.key); got != tc.want {
			t.Errorf("isToolConfirmInterruptKey(%q) = %v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestBuildToolConfirmEvent(t *testing.T) {
	meta := graph.PregelStepMetadata{
		InterruptKey:   "tool_confirm:create_biz_apply:c1",
		InterruptValue: map[string]any{"tool": "create_biz_apply"},
		CheckpointID:   "ckpt-1",
		LineageID:      "lineage-1",
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	evt := &event.Event{StateDelta: map[string][]byte{graph.MetadataKeyPregel: raw}}

	name, got := buildToolConfirmEvent(context.Background(), evt)
	if got == "" {
		t.Fatalf("payload is empty, want non-empty")
	}
	// 事件名应带具体工具后缀，便于前端路由到对应卡片。
	wantName := constant.ToolConfirmCustomEventName + ".create_biz_apply"
	if name != wantName {
		t.Errorf("event name = %q, want %q", name, wantName)
	}
	if !strings.Contains(got, "ckpt-1") || !strings.Contains(got, "create_biz_apply") {
		t.Errorf("payload = %s, want checkpoint and tool info", got)
	}
}

func TestBuildToolConfirmEventNonGate(t *testing.T) {
	meta := graph.PregelStepMetadata{InterruptKey: constant.HITLInterruptKey}
	raw, _ := json.Marshal(meta)
	evt := &event.Event{StateDelta: map[string][]byte{graph.MetadataKeyPregel: raw}}

	if name, got := buildToolConfirmEvent(context.Background(), evt); got != "" || name != "" {
		t.Errorf("non-gate event = (%q, %q), want empty", name, got)
	}
}
