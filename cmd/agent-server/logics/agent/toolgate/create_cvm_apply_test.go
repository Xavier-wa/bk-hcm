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

package toolgate

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"hcm/cmd/agent-server/logics/agent/hitl"
	agenttool "hcm/cmd/agent-server/logics/tool"
	woatypes "hcm/cmd/woa-server/types/task"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

// confirmArgsJSON is a sample forwarded args JSON mirroring what the frontend sends back
// via forwardedProps.resumeValue on the confirm path.
const confirmArgsJSON = `{"path_param":{"bk_biz_id":100},"body_param":{"remark":"ok"}}`

const applyArgsWrapped = `{
  "path_param": {"bk_biz_id": 100},
  "body_param": {
    "bk_username": "tester",
    "require_type": 1,
    "expect_time": "2026-06-10",
    "remark": "压测扩容",
    "suborders": [
      {"resource_type": "QCLOUDCVM", "replicas": 2,
       "spec": {"region": "ap-guangzhou", "zone": "ap-guangzhou-3", "device_type": "S5.LARGE8"}}
    ]
  }
}`

// newTestCreateCvmApplyGate 构造一个 check 函数被打桩为给定 result/error 的申领门禁，
// 使 OnResume 的三态行为无需真实 woa-server 即可单测。
func newTestCreateCvmApplyGate(result *woatypes.CheckApplyOrderResp, err error) *createCvmApplyGate {
	return &createCvmApplyGate{
		check: func(_ context.Context, _ int64, _ *woatypes.ApplyReq) (*woatypes.CheckApplyOrderResp, error) {
			return result, err
		},
	}
}

func newApplyToolCall() *model.ToolCall {
	return &model.ToolCall{
		ID: "call_1",
		Function: model.FunctionDefinitionParam{
			Name:      constant.ToolNameCreateBizApply,
			Arguments: []byte(applyArgsWrapped),
		},
	}
}

// newApplyProxyToolCall 将申领参数包装进 proxy execute_tool 信封，模拟 LLM 现在通过
// tool_proxy_execute_tool 调用真实 create_biz_apply MCP 工具的方式。
func newApplyProxyToolCall() *model.ToolCall {
	envelope := `{"tool_name":"bkhcm-devhk/create_biz_apply","parameters":` + applyArgsWrapped +
		`,"schema_token":"tok-123"}`
	return &model.ToolCall{
		ID: "call_proxy_1",
		Function: model.FunctionDefinitionParam{
			Name:      constant.ProxyExecuteToolFullName,
			Arguments: []byte(envelope),
		},
	}
}

func TestApplyGateBuildPayloadProxyWrapped(t *testing.T) {
	g := newCreateCvmApplyGate(nil)
	raw, err := g.BuildPayload(context.Background(), newApplyProxyToolCall())
	if err != nil {
		t.Fatalf("BuildPayload err = %v", err)
	}
	payload, ok := raw.(*ConfirmPayload)
	if !ok {
		t.Fatalf("payload type = %T, want *ConfirmPayload", raw)
	}
	if payload.Tool != constant.ToolNameCreateBizApply {
		t.Errorf("tool = %q", payload.Tool)
	}
	// proxy 信封下，确认卡片数据应来自 parameters，仍携带 body_param 包装。
	if _, ok := payload.Data["body_param"]; !ok {
		t.Errorf("data should unwrap proxy parameters and keep body_param, got %v", payload.Data)
	}
}

func TestApplyGateOnResumeProxyEditedArgsPreservesEnvelope(t *testing.T) {
	// proxy 信封下编辑参数后，回写必须保留 tool_name / schema_token，仅替换 parameters，
	// 否则 execute_tool 的 schema_token 校验会拒绝提单。
	// forwardedProps.resumeValue carries the confirmed args JSON directly (no action wrapper).
	g := newTestCreateCvmApplyGate(&woatypes.CheckApplyOrderResp{Pass: true}, nil)
	res, err := g.OnResume(context.Background(), newApplyProxyToolCall(),
		`{"path_param":{"bk_biz_id":100},"body_param":{"remark":"edited"}}`)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Next != enumor.CvmApplyNodeTool {
		t.Errorf("next = %q, want tool", res.Next)
	}
	if len(res.MessageOps) != 1 {
		t.Fatalf("message ops = %d, want 1", len(res.MessageOps))
	}
	op, ok := res.MessageOps[0].(agenttool.ReplaceToolCallArgs)
	if !ok {
		t.Fatalf("op type = %T, want tool.ReplaceToolCallArgs", res.MessageOps[0])
	}

	var envelope struct {
		ToolName    string         `json:"tool_name"`
		SchemaToken string         `json:"schema_token"`
		Parameters  map[string]any `json:"parameters"`
	}
	if err = json.Unmarshal(op.NewArgs, &envelope); err != nil {
		t.Fatalf("rewritten args not a valid envelope: %v", err)
	}
	if envelope.ToolName != "bkhcm-devhk/create_biz_apply" || envelope.SchemaToken != "tok-123" {
		t.Errorf("envelope must preserve tool_name/schema_token, got %+v", envelope)
	}
	body, _ := envelope.Parameters["body_param"].(map[string]any)
	if body["remark"] != "edited" {
		t.Errorf("parameters not replaced with edited args, got %+v", envelope.Parameters)
	}
}

func TestApplyGateBuildPayload(t *testing.T) {
	g := newCreateCvmApplyGate(nil)
	raw, err := g.BuildPayload(context.Background(), newApplyToolCall())
	if err != nil {
		t.Fatalf("BuildPayload err = %v", err)
	}
	payload, ok := raw.(*ConfirmPayload)
	if !ok {
		t.Fatalf("payload type = %T, want *ConfirmPayload", raw)
	}
	if payload.Tool != constant.ToolNameCreateBizApply {
		t.Errorf("tool = %q", payload.Tool)
	}
	if _, ok := payload.Data["body_param"]; !ok {
		t.Errorf("data should passthrough body_param wrapper, got %v", payload.Data)
	}
}

func TestApplyGateEventKind(t *testing.T) {
	if g := newCreateCvmApplyGate(nil); g.EventKind() != constant.ToolConfirmInterruptKey {
		t.Errorf("EventKind = %q, want %q", g.EventKind(), constant.ToolConfirmInterruptKey)
	}
}

func TestApplyGateOnResume(t *testing.T) {
	g := newTestCreateCvmApplyGate(&woatypes.CheckApplyOrderResp{Pass: true}, nil)
	ctx := context.Background()

	t.Run("forwarded args with biz_id proceeds to tool", func(t *testing.T) {
		// forwardedProps.resumeValue contains the full confirmed args JSON (no action wrapper).
		res, err := g.OnResume(ctx, newApplyToolCall(), confirmArgsJSON)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if res.Next != enumor.CvmApplyNodeTool {
			t.Errorf("next = %q, want tool", res.Next)
		}
		if len(res.MessageOps) != 1 {
			t.Fatalf("message ops = %d, want 1 (rewrite with confirmed args)", len(res.MessageOps))
		}
		if _, ok := res.MessageOps[0].(agenttool.ReplaceToolCallArgs); !ok {
			t.Errorf("op type = %T, want tool.ReplaceToolCallArgs", res.MessageOps[0])
		}
	})

	t.Run("cancel signal rejects to llm with tool message", func(t *testing.T) {
		// hitl.CancelActionSignal is synthesised by the HITL node when no forwardedProps present.
		res, err := g.OnResume(ctx, newApplyToolCall(), hitl.CancelActionSignal)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if res.Next != enumor.CvmApplyNodeLLM {
			t.Errorf("next = %q, want llm", res.Next)
		}
		if !res.ClearUserInput {
			t.Errorf("cancel should clear user input")
		}
		if len(res.AppendMessages) != 1 || res.AppendMessages[0].Role != model.RoleTool {
			t.Errorf("cancel should append one tool message, got %+v", res.AppendMessages)
		}
	})
}

func TestApplyGateOnResumeBusinessReject(t *testing.T) {
	// 业务不通过：check 返回 Pass=false + reason，门禁应回退到 llm 并携带可读原因，不放行提单。
	const reason = "子单1可申领容量不足，需要 2 台，当前实时可用 0 台。"
	g := newTestCreateCvmApplyGate(&woatypes.CheckApplyOrderResp{Pass: false, Reason: reason}, nil)

	res, err := g.OnResume(context.Background(), newApplyToolCall(), confirmArgsJSON)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Next != enumor.CvmApplyNodeLLM {
		t.Errorf("next = %q, want llm", res.Next)
	}
	if len(res.AppendMessages) != 1 {
		t.Fatalf("append messages = %d, want 1", len(res.AppendMessages))
	}
	if !strings.Contains(res.AppendMessages[0].Content, reason) {
		t.Errorf("reject message %q should carry reason %q", res.AppendMessages[0].Content, reason)
	}
}

func TestApplyGateOnResumeSystemError(t *testing.T) {
	// 系统异常：check 返回 error，门禁应回退到 llm 并提示校验失败，不放行提单。
	g := newTestCreateCvmApplyGate(nil, errors.New("woa down"))

	res, err := g.OnResume(context.Background(), newApplyToolCall(), confirmArgsJSON)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Next != enumor.CvmApplyNodeLLM {
		t.Errorf("next = %q, want llm", res.Next)
	}
	if len(res.AppendMessages) != 1 {
		t.Fatalf("append messages = %d, want 1", len(res.AppendMessages))
	}
}

func TestApplyGateOnResumeMissingBizID(t *testing.T) {
	// bk_biz_id 缺失：无法确定所属业务，门禁应回退到 llm 拒绝提单，不静默放行。
	g := newTestCreateCvmApplyGate(&woatypes.CheckApplyOrderResp{Pass: true}, nil)
	tc := &model.ToolCall{
		ID: "call_no_biz",
		Function: model.FunctionDefinitionParam{
			Name:      constant.ToolNameCreateBizApply,
			Arguments: []byte(`{"body_param":{"bk_username":"u","suborders":[]}}`),
		},
	}

	// forwarded args without path_param/bk_biz_id so the gate falls back to the original
	// tool call args, which also lack bk_biz_id.
	res, err := g.OnResume(context.Background(), tc, `{}`)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Next != enumor.CvmApplyNodeLLM {
		t.Errorf("next = %q, want llm", res.Next)
	}
}

func TestExtractApplyPathParam(t *testing.T) {
	tests := []struct {
		name   string
		args   map[string]any
		want   int64
		wantOk bool
	}{
		{name: "number", args: map[string]any{"path_param": map[string]any{"bk_biz_id": float64(100)}},
			want: 100, wantOk: true},
		{name: "string", args: map[string]any{"path_param": map[string]any{"bk_biz_id": "200"}},
			want: 200, wantOk: true},
		{name: "missing path_param", args: map[string]any{"body_param": map[string]any{}}, wantOk: false},
		{name: "missing bk_biz_id", args: map[string]any{"path_param": map[string]any{}}, wantOk: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractApplyPathParam(tc.args)
			if ok != tc.wantOk || (ok && got != tc.want) {
				t.Errorf("extractApplyPathParam() = (%d, %v), want (%d, %v)", got, ok, tc.want, tc.wantOk)
			}
		})
	}
}

func TestExtractApplyBody(t *testing.T) {
	wrapped := map[string]any{"body_param": map[string]any{"bk_username": "u"}}
	if body := extractApplyBody(wrapped); body["bk_username"] != "u" {
		t.Errorf("wrapped body not unwrapped: %v", body)
	}

	flat := map[string]any{"bk_username": "u"}
	if body := extractApplyBody(flat); body["bk_username"] != "u" {
		t.Errorf("flat body altered: %v", body)
	}
}
