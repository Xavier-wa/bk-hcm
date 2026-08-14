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

package a2a

import (
	"context"
	"testing"

	"hcm/pkg/criteria/constant"

	"trpc.group/trpc-go/trpc-a2a-go/protocol"
	"trpc.group/trpc-go/trpc-agent-go/graph"
)

func TestNormalizeForwardedResumeValue_StringAndObject(t *testing.T) {
	meta := map[string]any{
		constant.ForwardedPropResumeValue: map[string]any{"action": "confirm"},
	}
	normalizeForwardedResumeValue(meta, "rid")
	got, ok := meta[constant.StateKeyForwardedResumeValue].(string)
	if !ok || got != `{"action":"confirm"}` {
		t.Fatalf("forwarded = %v (%T), want JSON string", meta[constant.StateKeyForwardedResumeValue],
			meta[constant.StateKeyForwardedResumeValue])
	}
	if _, exists := meta[constant.ForwardedPropResumeValue]; exists {
		t.Fatal("resumeValue alias should be removed after normalize")
	}

	meta2 := map[string]any{constant.StateKeyForwardedResumeValue: `{"k":1}`}
	normalizeForwardedResumeValue(meta2, "rid")
	if meta2[constant.StateKeyForwardedResumeValue] != `{"k":1}` {
		t.Fatalf("string value changed unexpectedly: %v", meta2[constant.StateKeyForwardedResumeValue])
	}
}

func TestEnrichA2AAutoResume_WithoutSaverStillNormalizes(t *testing.T) {
	ctxID := "ctx-1"
	msg := protocol.NewMessageWithContext(
		protocol.MessageRoleUser,
		[]protocol.Part{protocol.NewTextPart("确认提交")},
		nil,
		&ctxID,
	)
	msg.Metadata = map[string]any{
		constant.ForwardedPropResumeValue: `{"path_param":{"bk_biz_id":1}}`,
	}
	enrichA2AAutoResume(context.Background(), nil, &msg)
	if msg.Metadata[graph.CfgKeyLineageID] != ctxID {
		t.Fatalf("lineage_id = %v, want %s", msg.Metadata[graph.CfgKeyLineageID], ctxID)
	}
	if msg.Metadata[constant.StateKeyForwardedResumeValue] != `{"path_param":{"bk_biz_id":1}}` {
		t.Fatalf("forwarded resume not normalized: %v", msg.Metadata[constant.StateKeyForwardedResumeValue])
	}
	if _, ok := msg.Metadata[graph.CfgKeyCheckpointID]; ok {
		t.Fatal("checkpoint_id should not be set without saver")
	}
}

func TestForwardedResumeValueString(t *testing.T) {
	// structured confirm payload (JSON object string) is returned as-is (trimmed).
	meta := map[string]any{constant.StateKeyForwardedResumeValue: `  {"k":1}  `}
	if got := forwardedResumeValueString(meta); got != `{"k":1}` {
		t.Fatalf("structured forwarded = %q, want %q", got, `{"k":1}`)
	}

	// plain string (e.g. account_id) is returned as-is.
	meta = map[string]any{constant.StateKeyForwardedResumeValue: "0000002b"}
	if got := forwardedResumeValueString(meta); got != "0000002b" {
		t.Fatalf("plain forwarded = %q, want %q", got, "0000002b")
	}

	// absent / non-string / nil metadata all yield empty string so the caller falls back
	// to message text.
	if got := forwardedResumeValueString(nil); got != "" {
		t.Fatalf("nil metadata forwarded = %q, want empty", got)
	}
	if got := forwardedResumeValueString(map[string]any{}); got != "" {
		t.Fatalf("missing forwarded = %q, want empty", got)
	}
	if got := forwardedResumeValueString(
		map[string]any{constant.StateKeyForwardedResumeValue: 123}); got != "" {
		t.Fatalf("non-string forwarded = %q, want empty", got)
	}
}

func TestA2AMessageText(t *testing.T) {
	msg := protocol.NewMessage(
		protocol.MessageRoleUser,
		[]protocol.Part{
			protocol.NewTextPart("hello"),
			protocol.NewTextPart(" world"),
		},
	)
	if got := a2aMessageText(&msg); got != "hello world" {
		t.Fatalf("text = %q, want %q", got, "hello world")
	}
	if got := a2aMessageText(nil); got != "" {
		t.Fatalf("nil message text = %q", got)
	}
}
