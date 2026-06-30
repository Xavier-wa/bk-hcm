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

package hitl

import (
	"context"
	"testing"

	agenttool "hcm/cmd/agent-server/logics/tool"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

func TestBuildInterruptKey(t *testing.T) {
	sep := constant.InterruptKeySeparator
	wantConfirm := constant.ToolConfirmInterruptKey + sep + "create_biz_apply" + sep + "c1"
	if got := buildHITLInterruptKey(constant.ToolConfirmInterruptKey, "create_biz_apply", "c1"); got != wantConfirm {
		t.Errorf("buildInterruptKey() = %q, want %q", got, wantConfirm)
	}
	wantHITL := constant.HITLInterruptKey + sep + "create_biz_apply" + sep + "c1"
	if got := buildHITLInterruptKey(constant.HITLInterruptKey, "create_biz_apply", "c1"); got != wantHITL {
		t.Errorf("buildInterruptKey() = %q, want %q", got, wantHITL)
	}
}

func TestBuildResumeDelta(t *testing.T) {
	t.Run("proceed to tool with message ops", func(t *testing.T) {
		res := ResumeResult{
			Next:       enumor.CvmApplyNodeTool,
			MessageOps: []graph.MessageOp{agenttool.ReplaceToolCallArgs{ToolID: "c1", NewArgs: []byte(`{}`)}},
		}
		delta := buildResumeDelta(res, "create_biz_apply", "rid")
		if delta[constant.StateKeyHITLRoute] != enumor.CvmApplyNodeTool {
			t.Errorf("route = %v, want tool", delta[constant.StateKeyHITLRoute])
		}
		if _, ok := delta[graph.StateKeyMessages].([]graph.MessageOp); !ok {
			t.Errorf("messages delta = %T, want []graph.MessageOp", delta[graph.StateKeyMessages])
		}
	})

	t.Run("reject to llm clears input and appends message", func(t *testing.T) {
		res := ResumeResult{
			Next:           enumor.CvmApplyNodeLLM,
			ClearUserInput: true,
			AppendMessages: []model.Message{{Role: model.RoleTool, ToolID: "c1", Content: "cancelled"}},
		}
		delta := buildResumeDelta(res, "create_biz_apply", "rid")
		if delta[constant.StateKeyHITLRoute] != enumor.CvmApplyNodeLLM {
			t.Errorf("route = %v, want llm", delta[constant.StateKeyHITLRoute])
		}
		if v, ok := delta[graph.StateKeyUserInput].(string); !ok || v != "" {
			t.Errorf("user input = %v, want empty", delta[graph.StateKeyUserInput])
		}
		if _, ok := delta[graph.StateKeyMessages].([]model.Message); !ok {
			t.Errorf("messages delta = %T, want []model.Message", delta[graph.StateKeyMessages])
		}
	})

	t.Run("unknown next falls back to llm", func(t *testing.T) {
		delta := buildResumeDelta(ResumeResult{Next: enumor.CvmApplyNode("bogus")}, "x", "rid")
		if delta[constant.StateKeyHITLRoute] != enumor.CvmApplyNodeLLM {
			t.Errorf("route = %v, want llm", delta[constant.StateKeyHITLRoute])
		}
	})
}

func TestExtractHandledToolCall(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewHumanConfirmHandler())

	t.Run("found", func(t *testing.T) {
		msgs := []model.Message{
			{Role: model.RoleUser, Content: "hi"},
			{Role: model.RoleAssistant, ToolCalls: []model.ToolCall{
				{ID: "c1", Function: model.FunctionDefinitionParam{Name: constant.HumanConfirmToolName}},
			}},
		}
		if tc := extractHandledToolCall(msgs, reg); tc == nil || tc.ID != "c1" {
			t.Errorf("extractHandledToolCall() = %v, want c1", tc)
		}
	})

	t.Run("stops at user message", func(t *testing.T) {
		msgs := []model.Message{
			{Role: model.RoleAssistant, ToolCalls: []model.ToolCall{
				{ID: "c1", Function: model.FunctionDefinitionParam{Name: constant.HumanConfirmToolName}},
			}},
			{Role: model.RoleUser, Content: "new turn"},
		}
		if tc := extractHandledToolCall(msgs, reg); tc != nil {
			t.Errorf("extractHandledToolCall() = %v, want nil", tc)
		}
	})

	t.Run("non handled tool", func(t *testing.T) {
		msgs := []model.Message{
			{Role: model.RoleAssistant, ToolCalls: []model.ToolCall{
				{ID: "c", Function: model.FunctionDefinitionParam{Name: "other_tool"}},
			}},
		}
		if tc := extractHandledToolCall(msgs, reg); tc != nil {
			t.Errorf("extractHandledToolCall() = %v, want nil", tc)
		}
	})
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	reg.Register(nil)
	if reg.Has(constant.HumanConfirmToolName) {
		t.Errorf("empty registry should not have handler")
	}
	reg.Register(NewHumanConfirmHandler())
	if !reg.Has(constant.HumanConfirmToolName) {
		t.Errorf("registry should have human_confirm handler")
	}
	if _, ok := reg.Lookup(constant.HumanConfirmToolName); !ok {
		t.Errorf("lookup should find human_confirm handler")
	}
}

func TestResolveStructuredResumeValue(t *testing.T) {
	orderJSON := `{"path_param":{"bk_biz_id":"213"},"body_param":{"bk_username":"u"}}`

	t.Run("prefer interrupt resume over stale runtime state", func(t *testing.T) {
		got := resolveStructuredResumeValue(orderJSON, context.Background())
		if got != orderJSON {
			t.Errorf("got %q, want order JSON", got)
		}
	})

	t.Run("free text is not structured", func(t *testing.T) {
		if got := resolveStructuredResumeValue("确认提交", context.Background()); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("account id string is not structured", func(t *testing.T) {
		if isJSONObjectString("0000002b") {
			t.Error("account id should not be treated as JSON object")
		}
	})
}

func TestIsJSONObjectString(t *testing.T) {
	if !isJSONObjectString(`{"action":"cancel"}`) {
		t.Error("JSON object should match")
	}
	if isJSONObjectString(`"plain"`) {
		t.Error("JSON string should not match")
	}
	if isJSONObjectString("确认提交") {
		t.Error("free text should not match")
	}
}
