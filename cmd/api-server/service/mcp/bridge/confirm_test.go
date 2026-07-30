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

package bridge

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"hcm/cmd/api-server/service/mcp/ingress"
	"hcm/pkg/criteria/constant"

	"trpc.group/trpc-go/trpc-agent-go/graph"
)

func TestBuildConfirmResumeValue(t *testing.T) {
	if _, ok := buildConfirmResumeValue(nil); ok {
		t.Fatal("nil confirm should not resume")
	}
	if _, ok := buildConfirmResumeValue(&ingress.ConfirmRequest{Action: constant.MCPConfirmActionCancel}); ok {
		t.Fatal("cancel should not set forwarded resume")
	}
	got, ok := buildConfirmResumeValue(&ingress.ConfirmRequest{Action: constant.MCPConfirmActionConfirm})
	if !ok || got != "{}" {
		t.Fatalf("empty confirm args = %q ok=%v, want {}", got, ok)
	}
	got, ok = buildConfirmResumeValue(&ingress.ConfirmRequest{
		Action: constant.MCPConfirmActionConfirm,
		Args:   map[string]any{"path_param": map[string]any{"bk_biz_id": float64(1)}},
	})
	if !ok {
		t.Fatal("confirm with args should set resume")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("resume JSON invalid: %v", err)
	}
}

func TestExtractPendingConfirmFromMetadata(t *testing.T) {
	pregel := graph.PregelStepMetadata{
		InterruptKey:   constant.ToolConfirmCreateCvmApplyInterruptKey + constant.InterruptKeySeparator + "c1",
		InterruptValue: map[string]any{"tool": "create_biz_apply", "data": map[string]any{"n": 1}},
		CheckpointID:   "ck-1",
		LineageID:      "ctx-1",
	}
	raw, err := json.Marshal(pregel)
	if err != nil {
		t.Fatal(err)
	}
	meta := map[string]any{
		"state_delta": map[string]any{
			graph.MetadataKeyPregel: map[string]any{
				"encoding": "bytes",
				"payload":  base64.StdEncoding.EncodeToString(raw),
			},
		},
	}
	pc := extractPendingConfirmFromMetadata(meta)
	if pc == nil {
		t.Fatal("expected pending confirm")
	}
	if pc.Kind != constant.ToolConfirmCustomEventName {
		t.Fatalf("kind = %q", pc.Kind)
	}
	if pc.Tool != "create_biz_apply" {
		t.Fatalf("tool = %q", pc.Tool)
	}
	if pc.Data["n"] != float64(1) && pc.Data["n"] != 1 {
		t.Fatalf("data = %#v", pc.Data)
	}
}

// encodePregelMetadata 将 PregelStepMetadata 编码成 bridge 期望的 state_delta 元数据结构。
func encodePregelMetadata(t *testing.T, pregel graph.PregelStepMetadata) map[string]any {
	t.Helper()
	raw, err := json.Marshal(pregel)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"state_delta": map[string]any{
			graph.MetadataKeyPregel: map[string]any{
				"encoding": "bytes",
				"payload":  base64.StdEncoding.EncodeToString(raw),
			},
		},
	}
}

func TestExtractPendingConfirm_AccountSelect(t *testing.T) {
	meta := encodePregelMetadata(t, graph.PregelStepMetadata{
		InterruptKey: constant.AccountSelectInterruptKey + constant.InterruptKeySeparator + "3",
		InterruptValue: map[string]any{
			"type":    constant.AccountSelectInterruptKey,
			"options": []any{map[string]any{"account_id": "0000002b", "account_name": "ziyan"}},
		},
	})
	pc := extractPendingConfirmFromMetadata(meta)
	if pc == nil {
		t.Fatal("expected pending confirm for account_select")
	}
	if pc.Kind != constant.AccountSelectInterruptKey {
		t.Fatalf("kind = %q, want %q", pc.Kind, constant.AccountSelectInterruptKey)
	}
	// 非 tool.confirm：payload 应还原到 Data，供向用户展示可选项。
	if _, ok := pc.Data["options"]; !ok {
		t.Fatalf("expected options in data, got %#v", pc.Data)
	}
	content := formatConfirmContent(pc, "")
	if !strings.Contains(content, constant.MCPSelectPendingMessage) {
		t.Fatalf("account_select content should use select guidance, got: %s", content)
	}
	if strings.Contains(content, "confirm.args") {
		t.Fatalf("account_select content should not mention confirm.args, got: %s", content)
	}
}

func TestExtractPendingConfirm_AfterToolHITL(t *testing.T) {
	meta := encodePregelMetadata(t, graph.PregelStepMetadata{
		InterruptKey: constant.AfterToolHITLRecommendSelectInterruptKey + constant.InterruptKeySeparator + "5",
		InterruptValue: map[string]any{
			"recommendations": []any{map[string]any{"instance_type": "S3.MEDIUM4"}},
		},
	})
	pc := extractPendingConfirmFromMetadata(meta)
	if pc == nil {
		t.Fatal("expected pending confirm for after_tool_hitl")
	}
	if !strings.HasPrefix(pc.Kind, constant.AfterToolHITLRecommendSelectInterruptKey) {
		t.Fatalf("kind = %q, want prefix %q", pc.Kind, constant.AfterToolHITLRecommendSelectInterruptKey)
	}
	if _, ok := pc.Data["recommendations"]; !ok {
		t.Fatalf("expected recommendations in data, got %#v", pc.Data)
	}
	content := formatConfirmContent(pc, "")
	if !strings.Contains(content, "S3.MEDIUM4") {
		t.Fatalf("after_tool_hitl content should render recommendation payload, got: %s", content)
	}
}

func TestFormatConfirmContent_ToolConfirmKeepsArgsGuidance(t *testing.T) {
	pc := &pendingConfirm{
		Kind: constant.ToolConfirmCustomEventName,
		Tool: "create_biz_apply",
		Data: map[string]any{"count": float64(2)},
	}
	content := formatConfirmContent(pc, "")
	if !strings.Contains(content, constant.MCPConfirmPendingMessage) {
		t.Fatalf("tool.confirm content should use confirm guidance, got: %s", content)
	}
	if !strings.Contains(content, "confirm.args") {
		t.Fatalf("tool.confirm content should mention confirm.args, got: %s", content)
	}
}

func TestBuildSendMessageParams_Confirm(t *testing.T) {
	params := buildSendMessageParams(&ingress.SendMessageRequest{
		Text:      "确认提交",
		ContextID: "ctx-1",
		Confirm: &ingress.ConfirmRequest{
			Action: constant.MCPConfirmActionConfirm,
			Args:   map[string]any{"body_param": map[string]any{"count": 2}},
		},
	})
	resume, _ := params.Message.Metadata[constant.StateKeyForwardedResumeValue].(string)
	if resume == "" {
		t.Fatal("expected forwarded_resume_value in metadata")
	}
	if params.Message.Metadata[constant.ForwardedPropResumeValue] != resume {
		t.Fatal("resumeValue alias missing")
	}
}
