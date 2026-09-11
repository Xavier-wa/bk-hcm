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
	"os"
	"strings"
	"testing"

	"hcm/cmd/agent-server/logics/agent/hitl"
	authlogic "hcm/cmd/agent-server/logics/auth"
	woatypes "hcm/cmd/woa-server/types/task"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

// TestMain 初始化 cc 运行期配置：门禁在确认后构造后端 kit（core.NewBackendKit）时会读取
// 租户开关，未初始化时会直接 panic。
func TestMain(m *testing.M) {
	cc.InitRuntime(cc.TestSetting{})
	os.Exit(m.Run())
}

// confirmArgsJSON is a sample forwarded args JSON mirroring what the frontend sends back
// via forwardedProps.resumeValue on the confirm path.
const confirmArgsJSON = `{"path_param":{"bk_biz_id":100},"body_param":{"remark":"ok"}}`

// applyTestUser 是测试用的请求上下文用户名，也是门禁主动鉴权唯一认可的判定主体来源。
const applyTestUser = "ctx-user"

// applyTestCtx 构造带蓝鲸用户名的 context。门禁的主动鉴权只认上下文中的用户名，
// 空 context 会在鉴权前被 fail closed 拒绝，故凡是要走到鉴权及其之后的用例都须用它。
func applyTestCtx() context.Context {
	return authlogic.WithBKUsername(context.Background(), applyTestUser)
}

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

// applyGateStub 描述申领门禁三个注入点（主动鉴权、申请地址、woa 预检）的打桩行为。
type applyGateStub struct {
	authorized   bool
	authorizeErr error
	applyURL     string
	applyURLErr  error
	checkResult  *woatypes.CheckApplyOrderResp
	checkErr     error
}

// applyGateCalls 记录打桩门禁上各注入点的实际调用情况。
// checkCalled 用于断言「无权限时不得调用 woa 预检」，authorizeRes 用于断言鉴权资源属性。
type applyGateCalls struct {
	checkCalled    bool
	applyURLCalled bool
	authorizeRes   []meta.ResourceAttribute
	authorizeUsers []string
}

// newStubApplyGate 构造一个三个注入点全部打桩的申领门禁，使 OnResume 的全部分支
// 无需真实 IAM 与 woa-server 即可单测，并返回调用记录供断言。
func newStubApplyGate(stub applyGateStub) (*createCvmApplyGate, *applyGateCalls) {
	calls := new(applyGateCalls)
	g := &createCvmApplyGate{
		authorize: func(kt *kit.Kit, res meta.ResourceAttribute) (bool, error) {
			calls.authorizeRes = append(calls.authorizeRes, res)
			calls.authorizeUsers = append(calls.authorizeUsers, kt.User)
			return stub.authorized, stub.authorizeErr
		},
		applyURL: func(_ *kit.Kit, _ meta.ResourceAttribute) (string, error) {
			calls.applyURLCalled = true
			return stub.applyURL, stub.applyURLErr
		},
		check: func(_ context.Context, _ int64, _ *woatypes.ApplyReq) (*woatypes.CheckApplyOrderResp, error) {
			calls.checkCalled = true
			return stub.checkResult, stub.checkErr
		},
	}
	return g, calls
}

// newTestCreateCvmApplyGate 构造一个主动鉴权判定为通过、check 函数被打桩为给定 result/error 的申领门禁，
// 使鉴权通过后 OnResume 的三态行为无需真实 woa-server 即可单测。
func newTestCreateCvmApplyGate(result *woatypes.CheckApplyOrderResp, err error) *createCvmApplyGate {
	g, _ := newStubApplyGate(applyGateStub{authorized: true, checkResult: result, checkErr: err})
	return g
}

// parseDeniedResult 把 tool 结果解析为无权限拒绝结构，非 JSON 时直接失败。
func parseDeniedResult(t *testing.T, content string) applyDeniedResult {
	t.Helper()

	var rst applyDeniedResult
	if err := json.Unmarshal([]byte(content), &rst); err != nil {
		t.Fatalf("tool content is not a denied result JSON: %v, content: %s", err, content)
	}
	return rst
}

// rejectContent 断言本次结果是回退到 llm 的单条 tool 拒绝消息，并返回其内容。
func rejectContent(t *testing.T, res hitl.ResumeResult) string {
	t.Helper()

	if res.Next != enumor.CvmApplyNodeLLM {
		t.Fatalf("next = %q, want llm", res.Next)
	}
	if len(res.AppendMessages) != 1 || res.AppendMessages[0].Role != model.RoleTool {
		t.Fatalf("reject should append one tool message, got %+v", res.AppendMessages)
	}
	return res.AppendMessages[0].Content
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
	g := newCreateCvmApplyGate(nil, nil)
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
	// proceed 通过 AppendMessages 关闭原始 call 并注入带新 ID 的 tool_call。
	g := newTestCreateCvmApplyGate(&woatypes.CheckApplyOrderResp{Pass: true}, nil)
	original := newApplyProxyToolCall()
	res, err := g.OnResume(applyTestCtx(), original,
		`{"path_param":{"bk_biz_id":100},"body_param":{"remark":"edited"}}`)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Next != enumor.CvmApplyNodeTool {
		t.Errorf("next = %q, want tool", res.Next)
	}
	if len(res.AppendMessages) != 2 {
		t.Fatalf("append messages = %d, want 2 (close original + inject new call)", len(res.AppendMessages))
	}
	if res.AppendMessages[0].Role != model.RoleTool || res.AppendMessages[0].ToolID != original.ID {
		t.Fatalf("first message should close original tool call, got %+v", res.AppendMessages[0])
	}
	if res.AppendMessages[1].Role != model.RoleAssistant || len(res.AppendMessages[1].ToolCalls) != 1 {
		t.Fatalf("second message should inject new tool call, got %+v", res.AppendMessages[1])
	}
	newCall := res.AppendMessages[1].ToolCalls[0]
	if newCall.ID == "" || newCall.ID == original.ID {
		t.Errorf("new tool call id = %q, want regenerated id", newCall.ID)
	}

	var envelope struct {
		ToolName    string         `json:"tool_name"`
		SchemaToken string         `json:"schema_token"`
		Parameters  map[string]any `json:"parameters"`
	}
	if err = json.Unmarshal(newCall.Function.Arguments, &envelope); err != nil {
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
	g := newCreateCvmApplyGate(nil, nil)
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
	if g := newCreateCvmApplyGate(nil, nil); g.EventKind() != constant.ToolConfirmCreateCvmApplyInterruptKey {
		t.Errorf("EventKind = %q, want %q", g.EventKind(), constant.ToolConfirmCreateCvmApplyInterruptKey)
	}
}

func TestApplyGateOnResume(t *testing.T) {
	g := newTestCreateCvmApplyGate(&woatypes.CheckApplyOrderResp{Pass: true}, nil)
	ctx := applyTestCtx()

	t.Run("forwarded args with biz_id proceeds to tool", func(t *testing.T) {
		// forwardedProps.resumeValue contains the full confirmed args JSON (no action wrapper).
		original := newApplyToolCall()
		res, err := g.OnResume(ctx, original, confirmArgsJSON)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if res.Next != enumor.CvmApplyNodeTool {
			t.Errorf("next = %q, want tool", res.Next)
		}
		if len(res.AppendMessages) != 2 {
			t.Fatalf("append messages = %d, want 2 (close original + inject new call)",
				len(res.AppendMessages))
		}
		if res.AppendMessages[0].Role != model.RoleTool || res.AppendMessages[0].ToolID != original.ID {
			t.Errorf("first message should close original tool call, got %+v", res.AppendMessages[0])
		}
		if len(res.AppendMessages[1].ToolCalls) != 1 {
			t.Fatalf("second message should inject regenerated tool call")
		}
		if res.AppendMessages[1].ToolCalls[0].ID == original.ID {
			t.Errorf("new tool call id should differ from original %q", original.ID)
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
		// 引导用户点击确认按钮的说明由 CancelNotice 提供、hitl 节点以 assistant 消息注入，
		// 门禁自身只写回 tool 结果。
		if len(res.AppendMessages) != 1 || res.AppendMessages[0].Role != model.RoleTool {
			t.Errorf("cancel should append one tool message, got %+v", res.AppendMessages)
		}
		if res.AppendMessages[0].Content != applyCancelToolResult {
			t.Errorf("cancel tool result = %q, want %q", res.AppendMessages[0].Content, applyCancelToolResult)
		}
	})
}

// TestApplyGateCancelNotice 校验门禁实现了 hitl.CancelNoticer，且引导说明覆盖三个关键点：
// 用户输入的文字不能替代点击、必须先用文字说明、可按用户意愿重新弹出确认卡片。
// 前两点缺一不可：用户输入「确认提交」时，模型会把它当成提交许可直接重新调用提单工具，
// 于是又弹一次确认卡片，用户始终收不到需要点击按钮的说明。
func TestApplyGateCancelNotice(t *testing.T) {
	notice := newCreateCvmApplyGate(nil, nil).(hitl.CancelNoticer).CancelNotice()
	if !strings.Contains(notice, "不能作为提交依据") {
		t.Errorf("cancel notice = %q, want it to reject free text as a submit approval", notice)
	}
	if !strings.Contains(notice, "必须先用文字说明") {
		t.Errorf("cancel notice = %q, want it to require a text reply first", notice)
	}
	if !strings.Contains(notice, "重新弹出确认卡片") {
		t.Errorf("cancel notice = %q, want it to allow re-opening the confirm card", notice)
	}
	if !strings.Contains(notice, "确认提交") {
		t.Errorf("cancel notice = %q, want it to name the confirm button", notice)
	}
}

func TestApplyGateOnResumeBusinessReject(t *testing.T) {
	// 业务不通过：check 返回 Pass=false + reason，门禁应回退到 llm 并携带可读原因，不放行提单。
	const reason = "子单1可申领容量不足，需要 2 台，当前实时可用 0 台。"
	g := newTestCreateCvmApplyGate(&woatypes.CheckApplyOrderResp{Pass: false, Reason: reason}, nil)

	res, err := g.OnResume(applyTestCtx(), newApplyToolCall(), confirmArgsJSON)
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

	res, err := g.OnResume(applyTestCtx(), newApplyToolCall(), confirmArgsJSON)
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

// TestApplyGateOnResumeNoPermissionWithApplyURL 覆盖无权限主路径：门禁主动判定无权限且申请地址可生成时，
// 拒绝结果带 2030403 与非空 apply_url，且 MUST NOT 调用 woa 预检（无权限用户不得进入正式提单）。
func TestApplyGateOnResumeNoPermissionWithApplyURL(t *testing.T) {
	const wantURL = "https://iam.example.com/apply-permission?system=hcm"
	g, calls := newStubApplyGate(applyGateStub{
		authorized: false, applyURL: wantURL,
		// woa 预检桩故意返回放行，一旦被调用即可从下面的断言中暴露出来。
		checkResult: &woatypes.CheckApplyOrderResp{Pass: true},
	})

	res, err := g.OnResume(applyTestCtx(), newApplyToolCall(), applyArgsWrapped)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if calls.checkCalled {
		t.Errorf("woa check must not be called once the gate decided the user has no permission")
	}

	rst := parseDeniedResult(t, rejectContent(t, res))
	if rst.Code != errf.PermissionDenied {
		t.Errorf("code = %d, want %d", rst.Code, errf.PermissionDenied)
	}
	if rst.ApplyURL != wantURL {
		t.Errorf("apply_url = %q, want %q", rst.ApplyURL, wantURL)
	}
	// 面向用户的说明须点明账号与业务，且不得把「联系管理员」当成唯一引导。
	// 账号取自请求上下文，而非申领参数里的提单人（body 中的 bk_username 为 tester）。
	if !strings.Contains(rst.Message, applyTestUser) || !strings.Contains(rst.Message, "100") {
		t.Errorf("message = %q, want it to carry the ctx user and biz id", rst.Message)
	}
	if strings.Contains(rst.Message, "联系该业务管理员") {
		t.Errorf("message = %q, should not degrade to contacting admin when apply_url is available", rst.Message)
	}
}

// TestApplyGateOnResumeNoPermissionApplyURLUnavailable 覆盖无权限降级路径：已判定无权限但申请地址
// 失败/为空/非 http(s) 时，仍返回 2030403 与降级说明，不给空的或伪造的 apply_url，也不升格为系统异常。
func TestApplyGateOnResumeNoPermissionApplyURLUnavailable(t *testing.T) {
	tests := []struct {
		name string
		stub applyGateStub
	}{
		{name: "apply url call failed", stub: applyGateStub{applyURLErr: errors.New("iam timeout")}},
		{name: "apply url empty", stub: applyGateStub{applyURL: ""}},
		{name: "apply url not http", stub: applyGateStub{applyURL: "javascript:alert(1)"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.stub.authorized = false
			tc.stub.checkResult = &woatypes.CheckApplyOrderResp{Pass: true}
			g, calls := newStubApplyGate(tc.stub)

			res, err := g.OnResume(applyTestCtx(), newApplyToolCall(), applyArgsWrapped)
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if calls.checkCalled {
				t.Errorf("woa check must not be called on the no-permission path")
			}

			content := rejectContent(t, res)
			rst := parseDeniedResult(t, content)
			if rst.Code != errf.PermissionDenied {
				t.Errorf("code = %d, want %d", rst.Code, errf.PermissionDenied)
			}
			if rst.ApplyURL != "" {
				t.Errorf("apply_url = %q, want it omitted", rst.ApplyURL)
			}
			if !strings.Contains(rst.Message, "联系该业务管理员") {
				t.Errorf("message = %q, want the degrade guidance", rst.Message)
			}
			if strings.Contains(content, applyPreCheckFailedPrefix) {
				t.Errorf("content = %q, must not use the system-error wording", content)
			}
		})
	}
}

// TestApplyGateOnResumeAuthorizeFailed 覆盖鉴权系统失败：Authorize 报错/超时或 authorizer 未注入时
// fail closed——拒绝放行、不调 woa 预检、不生成 apply_url、不标成 2030403 引导。
func TestApplyGateOnResumeAuthorizeFailed(t *testing.T) {
	t.Run("authorize returns error", func(t *testing.T) {
		g, calls := newStubApplyGate(applyGateStub{
			authorizeErr: errors.New("iam unavailable"),
			checkResult:  &woatypes.CheckApplyOrderResp{Pass: true},
		})

		res, err := g.OnResume(applyTestCtx(), newApplyToolCall(), applyArgsWrapped)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if calls.checkCalled {
			t.Errorf("woa check must not be called when authorize itself failed")
		}
		if calls.applyURLCalled {
			t.Errorf("apply url must not be generated before the user is known to lack permission")
		}
		if content := rejectContent(t, res); content != applyAuthCheckFailed {
			t.Errorf("content = %q, want %q", content, applyAuthCheckFailed)
		}
	})

	t.Run("authorizer not injected", func(t *testing.T) {
		// 依赖未注入等同鉴权失败，同样 fail closed，不得因缺少 authorizer 放行提单。
		g := newCreateCvmApplyGate(nil, nil)
		res, err := g.OnResume(applyTestCtx(), newApplyToolCall(), applyArgsWrapped)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if content := rejectContent(t, res); content != applyAuthCheckFailed {
			t.Errorf("content = %q, want %q", content, applyAuthCheckFailed)
		}
	})
}

// TestApplyGateOnResumeNoContextUser 覆盖鉴权主体缺失：请求上下文没有蓝鲸用户名时无法判定权限，
// 门禁须 fail closed 拒绝，且不得拿申领参数里的提单人（body 中的 bk_username，由模型生成、
// 用户可在确认卡片上编辑）顶替判定主体去鉴权。
func TestApplyGateOnResumeNoContextUser(t *testing.T) {
	g, calls := newStubApplyGate(applyGateStub{
		// 三个桩都故意配成「放行」，一旦被调用即可从下面的断言中暴露出来。
		authorized: true, applyURL: "https://iam.example.com/apply",
		checkResult: &woatypes.CheckApplyOrderResp{Pass: true},
	})

	res, err := g.OnResume(context.Background(), newApplyToolCall(), applyArgsWrapped)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(calls.authorizeUsers) != 0 {
		t.Errorf("authorize must not be called without a ctx user, got users %v", calls.authorizeUsers)
	}
	if calls.checkCalled {
		t.Errorf("woa check must not be called without a ctx user")
	}
	if calls.applyURLCalled {
		t.Errorf("apply url must not be generated without a ctx user")
	}
	if content := rejectContent(t, res); content != applyAuthCheckFailed {
		t.Errorf("content = %q, want %q", content, applyAuthCheckFailed)
	}
}

// TestApplyGateOnResumeNonPermissionFailureNoApplyURL 覆盖非权限失败：主动鉴权通过后的额度/参数不通过、
// 取消确认、以及原因文案里偶然出现「权限」字样时，都不得标成无权限，也不得附带 apply_url。
func TestApplyGateOnResumeNonPermissionFailureNoApplyURL(t *testing.T) {
	tests := []struct {
		name       string
		reason     string
		cancel     bool
		wantSubstr string
	}{
		{name: "quota not enough", reason: "子单1可申领容量不足，需要 2 台，当前实时可用 0 台。",
			wantSubstr: applyPreCheckRejectPrefix},
		{name: "reason mentions permission word", reason: "申领参数校验未通过：权限组配额字段缺失，请修正后重试。",
			wantSubstr: applyPreCheckRejectPrefix},
		{name: "user cancelled", cancel: true, wantSubstr: applyCancelToolResult},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, calls := newStubApplyGate(applyGateStub{authorized: true, applyURL: "https://iam.example.com/apply",
				checkResult: &woatypes.CheckApplyOrderResp{Pass: false, Reason: tc.reason}})

			resumeValue := applyArgsWrapped
			if tc.cancel {
				resumeValue = hitl.CancelActionSignal
			}
			res, err := g.OnResume(applyTestCtx(), newApplyToolCall(), resumeValue)
			if err != nil {
				t.Fatalf("err = %v", err)
			}

			content := rejectContent(t, res)
			if !strings.Contains(content, tc.wantSubstr) {
				t.Errorf("content = %q, want it to contain %q", content, tc.wantSubstr)
			}
			if strings.Contains(content, "apply_url") {
				t.Errorf("content = %q, must not carry apply_url on a non-permission failure", content)
			}
			if strings.Contains(content, "2030403") {
				t.Errorf("content = %q, must not be marked as no-permission", content)
			}
			if calls.applyURLCalled {
				t.Errorf("apply url must not be generated on a non-permission failure")
			}
		})
	}
}

// TestApplyGateOnResumeAuthorizedProceed 覆盖放行路径：主动鉴权通过且 woa 预检通过时按现网放行，
// 结果不含无权限标识与 apply_url；同时断言鉴权资源属性为「业务 + 创建/申领 + 当前业务 ID」，
// 与 woa CheckBizApplyOrder 的鉴权点一致。
func TestApplyGateOnResumeAuthorizedProceed(t *testing.T) {
	g, calls := newStubApplyGate(applyGateStub{authorized: true, applyURL: "https://iam.example.com/apply",
		checkResult: &woatypes.CheckApplyOrderResp{Pass: true}})

	res, err := g.OnResume(applyTestCtx(), newApplyToolCall(), applyArgsWrapped)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Next != enumor.CvmApplyNodeTool {
		t.Fatalf("next = %q, want tool", res.Next)
	}
	if !calls.checkCalled {
		t.Errorf("woa check must still run as defense in depth once the gate authorized")
	}
	if calls.applyURLCalled {
		t.Errorf("apply url must not be generated on the pass path")
	}
	for _, msg := range res.AppendMessages {
		if strings.Contains(msg.Content, "apply_url") || strings.Contains(msg.Content, "2030403") {
			t.Errorf("pass path message must not carry no-permission fields, got %q", msg.Content)
		}
	}

	if len(calls.authorizeRes) != 1 {
		t.Fatalf("authorize called %d times, want 1", len(calls.authorizeRes))
	}
	got := calls.authorizeRes[0]
	if got.Basic == nil || got.Type != meta.Biz || got.Action != meta.Create {
		t.Errorf("authorize resource = %+v, want type=%q action=%q", got, meta.Biz, meta.Create)
	}
	if got.BizID != 100 {
		t.Errorf("authorize biz id = %d, want 100", got.BizID)
	}
	// 鉴权主体只能是请求上下文中的用户名；申领 body 里的提单人（tester）不得被用来鉴权。
	if calls.authorizeUsers[0] != applyTestUser {
		t.Errorf("authorize user = %q, want the ctx user %q", calls.authorizeUsers[0], applyTestUser)
	}
}

// TestApplyGateOnResumeWoaPermissionDeniedFallback 覆盖窄兜底：主动鉴权已通过但 woa 预检仍返回 2030403 时
// （两次调用之间权限被回收，或鉴权点漂移）走无权限引导，不放行、也不拼成系统异常文案。
func TestApplyGateOnResumeWoaPermissionDeniedFallback(t *testing.T) {
	const wantURL = "https://iam.example.com/apply-permission"
	g, _ := newStubApplyGate(applyGateStub{authorized: true, applyURL: wantURL,
		checkErr: errf.New(errf.PermissionDenied, "no permission")})

	res, err := g.OnResume(applyTestCtx(), newApplyToolCall(), applyArgsWrapped)
	if err != nil {
		t.Fatalf("err = %v", err)
	}

	content := rejectContent(t, res)
	if strings.Contains(content, applyPreCheckFailedPrefix) {
		t.Errorf("content = %q, must not use the system-error wording on a 2030403 fallback", content)
	}
	rst := parseDeniedResult(t, content)
	if rst.Code != errf.PermissionDenied {
		t.Errorf("code = %d, want %d", rst.Code, errf.PermissionDenied)
	}
	if rst.ApplyURL != wantURL {
		t.Errorf("apply_url = %q, want %q", rst.ApplyURL, wantURL)
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
