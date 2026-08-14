/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable or agreed to in writing,
 * software distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package cvmapply

import (
	"context"
	"testing"

	"hcm/cmd/agent-server/logics/auth"
	corecloud "hcm/pkg/api/core/cloud"
	protocloud "hcm/pkg/api/data-service/cloud"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
	inmemory "trpc.group/trpc-go/trpc-agent-go/session/inmemory"
)

const testAppName = "hcm-agent"

// newTestSessionSvc 返回一个内存版 session.Service（测试用），并预创建给定身份的会话。
func newTestSessionSvc(t *testing.T, appName, userID, sessionID string) session.Service {
	t.Helper()
	svc := inmemory.NewSessionService()
	if _, err := svc.CreateSession(context.Background(), session.Key{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	}, nil); err != nil {
		t.Fatalf("create test session failed: %v", err)
	}
	return svc
}

// newTestCtx 构造带蓝鲸用户名的 context（与 BuildAGUISessionKey 读取侧一致）。
func newTestCtx(userID string) context.Context {
	return auth.WithBKUsername(context.Background(), userID)
}

// newTestInvocation 构造带 RuntimeState lineageID 的 invocation；session 可为 nil。
func newTestInvocation(sessionID string, withSession bool) *trpcagent.Invocation {
	inv := &trpcagent.Invocation{
		RunOptions: trpcagent.RunOptions{
			RuntimeState: map[string]any{
				graph.CfgKeyLineageID: sessionID,
			},
		},
	}
	if withSession {
		inv.Session = &session.Session{
			AppName: testAppName,
			UserID:  "ignored-session-user",
			ID:      sessionID,
			State:   session.StateMap{},
		}
	}
	return inv
}

// 场景一/二：选中账号写入 RuntimeState 并落库到 session 后端（统一 key）。
func TestInjectAccountIDToRuntimeState_PersistToBackend(t *testing.T) {
	const userID, sessionID, accountID = "user1", "sess1", "acc-100"
	svc := newTestSessionSvc(t, testAppName, userID, sessionID)
	inv := newTestInvocation(sessionID, true)
	ctx := newTestCtx(userID)

	injectAccountIDToRuntimeState(ctx, inv, accountID, svc, testAppName)

	if got := inv.RunOptions.RuntimeState[constant.SessionAccountIDTempKey]; got != accountID {
		t.Fatalf("runtime state account_id = %v, want %v", got, accountID)
	}
	reused, ok := getSelectedAccountIDFromSession(ctx, inv, svc, testAppName)
	if !ok || reused != accountID {
		t.Fatalf("persisted account not found: got=%q ok=%v, want=%q", reused, ok, accountID)
	}
}

// 场景：inv.Session == nil，仅 RuntimeState 含 lineageID 时仍能落库并读回。
func TestInjectAndGet_NilSessionUsesLineageID(t *testing.T) {
	const userID, sessionID, accountID = "user1", "lineage-sess", "acc-nil-sess"
	svc := newTestSessionSvc(t, testAppName, userID, sessionID)
	inv := newTestInvocation(sessionID, false) // Session == nil
	ctx := newTestCtx(userID)

	injectAccountIDToRuntimeState(ctx, inv, accountID, svc, testAppName)

	got, ok := getSelectedAccountIDFromSession(ctx, inv, svc, testAppName)
	if !ok || got != accountID {
		t.Fatalf("nil Session should still persist via lineageID: got=%q ok=%v, want=%q",
			got, ok, accountID)
	}
}

// 场景三：重新提单（新 invocation）读取已存账号并命中。
func TestGetSelectedAccountIDFromSession_ReuseAcrossRuns(t *testing.T) {
	const userID, sessionID, accountID = "user1", "sess1", "acc-200"
	svc := newTestSessionSvc(t, testAppName, userID, sessionID)
	ctx := newTestCtx(userID)

	first := newTestInvocation(sessionID, true)
	injectAccountIDToRuntimeState(ctx, first, accountID, svc, testAppName)

	second := newTestInvocation(sessionID, true)
	got, ok := getSelectedAccountIDFromSession(ctx, second, svc, testAppName)
	if !ok || got != accountID {
		t.Fatalf("reapply should reuse persisted account: got=%q ok=%v, want=%q", got, ok, accountID)
	}
}

func TestGetSelectedAccountIDFromSession_NilService(t *testing.T) {
	inv := newTestInvocation("sess1", true)
	ctx := newTestCtx("user1")
	if got, ok := getSelectedAccountIDFromSession(ctx, inv, nil, testAppName); ok || got != "" {
		t.Fatalf("nil session service should return (_, false): got=%q ok=%v", got, ok)
	}
}

func TestGetSelectedAccountIDFromSession_NoPersistedAccount(t *testing.T) {
	const userID, sessionID = "user1", "sess1"
	svc := newTestSessionSvc(t, testAppName, userID, sessionID)
	inv := newTestInvocation(sessionID, true)
	ctx := newTestCtx(userID)
	if got, ok := getSelectedAccountIDFromSession(ctx, inv, svc, testAppName); ok || got != "" {
		t.Fatalf("empty backend should return (_, false): got=%q ok=%v", got, ok)
	}
}

func TestInjectAccountIDToRuntimeState_NilServiceSkipPersist(t *testing.T) {
	inv := newTestInvocation("sess1", true)
	ctx := newTestCtx("user1")
	injectAccountIDToRuntimeState(ctx, inv, "acc-x", nil, testAppName)
	if got := inv.RunOptions.RuntimeState[constant.SessionAccountIDTempKey]; got != "acc-x" {
		t.Fatalf("runtime state should still be set when session service is nil: got=%v", got)
	}
}

// 后端无账号但 RuntimeState 已回填时，tryReuseAccountID 仍命中。
func TestTryReuseAccountID_RuntimeStateFallback(t *testing.T) {
	const userID, sessionID, accountID = "user1", "sess-rt", "acc-rt"
	svc := newTestSessionSvc(t, testAppName, userID, sessionID)
	inv := newTestInvocation(sessionID, true)
	inv.RunOptions.RuntimeState[constant.SessionAccountIDTempKey] = accountID
	ctx := newTestCtx(userID)

	got, ok := tryReuseAccountID(ctx, inv, svc, testAppName, nil)
	if !ok || got != accountID {
		t.Fatalf("should reuse RuntimeState account: got=%q ok=%v, want=%q", got, ok, accountID)
	}
}

// 双源皆空时返回 false。
func TestTryReuseAccountID_BothEmpty(t *testing.T) {
	const userID, sessionID = "user1", "sess-empty"
	svc := newTestSessionSvc(t, testAppName, userID, sessionID)
	inv := newTestInvocation(sessionID, true)
	ctx := newTestCtx(userID)

	got, ok := tryReuseAccountID(ctx, inv, svc, testAppName, nil)
	if ok || got != "" {
		t.Fatalf("both empty should return (_, false): got=%q ok=%v", got, ok)
	}
}

// 子图 resume 无 forwarded 时，从 interrupt 返回的 userInput（account_id）解析并持久化。
func TestResolveSelectedAccountID_FromUserInput(t *testing.T) {
	accounts := []*protocloud.AccountBizRelWithAccount{
		{BaseAccount: corecloud.BaseAccount{
			ID: "0000002b", Name: "ziyan-hcm-test", Vendor: enumor.TCloudZiyan,
		}},
	}
	inv := newTestInvocation("sess1", false)
	ctx := newTestCtx("user1")

	got := resolveSelectedAccountID(ctx, inv, "0000002b", accounts)
	if got != "0000002b" {
		t.Fatalf("resolve from userInput = %q, want 0000002b", got)
	}
}

// isAccountUsable：仅当账号仍在当前业务列表且 vendor 受支持时可复用（TAPD 136108620 AC-004）。
func TestIsAccountUsable(t *testing.T) {
	accounts := []*protocloud.AccountBizRelWithAccount{
		{BaseAccount: corecloud.BaseAccount{ID: "acc-ziyan", Vendor: enumor.TCloudZiyan}},
		{BaseAccount: corecloud.BaseAccount{ID: "acc-tcloud", Vendor: enumor.TCloud}},
	}
	cases := []struct {
		name      string
		accountID string
		want      bool
	}{
		{name: "usable ziyan account", accountID: "acc-ziyan", want: true},
		{name: "unsupported vendor not usable", accountID: "acc-tcloud", want: false},
		{name: "missing account not usable", accountID: "acc-gone", want: false},
		{name: "empty account not usable", accountID: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAccountUsable(tc.accountID, accounts); got != tc.want {
				t.Fatalf("isAccountUsable(%q) = %v, want %v", tc.accountID, got, tc.want)
			}
		})
	}
}

// graph State 含 account_id 时 tryReuseAccountID 第三源命中。
func TestTryReuseAccountID_StateFallback(t *testing.T) {
	const userID, sessionID, accountID = "user1", "sess-state", "acc-state"
	svc := newTestSessionSvc(t, testAppName, userID, sessionID)
	inv := newTestInvocation(sessionID, true)
	ctx := newTestCtx(userID)
	state := graph.State{constant.SessionAccountIDTempKey: accountID}

	got, ok := tryReuseAccountID(ctx, inv, svc, testAppName, state)
	if !ok || got != accountID {
		t.Fatalf("should reuse state account: got=%q ok=%v, want=%q", got, ok, accountID)
	}
}

// filterEnabledAccounts 仅保留 tcloud-ziyan 可用账号。
func TestFilterEnabledAccounts(t *testing.T) {
	accounts := []*protocloud.AccountBizRelWithAccount{
		{BaseAccount: corecloud.BaseAccount{ID: "z1", Name: "ziyan-1", Vendor: enumor.TCloudZiyan}},
		{BaseAccount: corecloud.BaseAccount{ID: "t1", Name: "tcloud-1", Vendor: enumor.TCloud}},
		{BaseAccount: corecloud.BaseAccount{ID: "z2", Name: "ziyan-2", Vendor: enumor.TCloudZiyan}},
	}
	enabled := filterEnabledAccounts(accounts)
	if len(enabled) != 2 || enabled[0].ID != "z1" || enabled[1].ID != "z2" {
		t.Fatalf("filterEnabledAccounts = %+v, want [z1 z2]", enabled)
	}
}

// 账号总数为 1 时自动选中并落库，不弹选择中断。
func TestRouteByAccountCount_SingleAccountAutoSelect(t *testing.T) {
	const userID, sessionID = "user1", "sess-single"
	svc := newTestSessionSvc(t, testAppName, userID, sessionID)
	inv := newTestInvocation(sessionID, true)
	ctx := newTestCtx(userID)
	accounts := []*protocloud.AccountBizRelWithAccount{
		{BaseAccount: corecloud.BaseAccount{ID: "0000002b", Name: "ziyan-hcm-test", Vendor: enumor.TCloudZiyan}},
	}

	got, err := routeByAccountCount(ctx, nil, inv, svc, testAppName, accounts)
	if err != nil {
		t.Fatalf("routeByAccountCount err: %v", err)
	}
	st, ok := got.(graph.State)
	if !ok {
		t.Fatalf("result type = %T, want graph.State", got)
	}
	if st[constant.SessionAccountIDTempKey] != "0000002b" {
		t.Fatalf("auto-selected account_id = %v, want 0000002b", st[constant.SessionAccountIDTempKey])
	}
	if st[constant.AccountSelectNextNodeKey] != enumor.CvmApplyNodeLLM {
		t.Fatalf("next node = %v, want llm", st[constant.AccountSelectNextNodeKey])
	}
	if reused, ok := getSelectedAccountIDFromSession(ctx, inv, svc, testAppName); !ok || reused != "0000002b" {
		t.Fatalf("auto-selected account not persisted: got=%q ok=%v", reused, ok)
	}
}

// 账号总数为 0 时路由 fallback，由主图兜底回复。
func TestRouteByAccountCount_ZeroAccountRouteFallback(t *testing.T) {
	const userID, sessionID = "user1", "sess-zero"
	svc := newTestSessionSvc(t, testAppName, userID, sessionID)
	inv := newTestInvocation(sessionID, true)
	ctx := newTestCtx(userID)

	got, err := routeByAccountCount(ctx, nil, inv, svc, testAppName, nil)
	if err != nil {
		t.Fatalf("routeByAccountCount err: %v", err)
	}
	st, ok := got.(graph.State)
	if !ok {
		t.Fatalf("result type = %T, want graph.State", got)
	}
	if st[constant.AccountSelectNextNodeKey] != enumor.CvmApplyNodeFallback {
		t.Fatalf("next node = %v, want fallback", st[constant.AccountSelectNextNodeKey])
	}
}

// 有账号但一个都不支持申领时路由 fallback：继续弹卡片会让用户面对全禁用选项、点不动也退不出。
func TestRouteByAccountCount_NoEnabledAccountRouteFallback(t *testing.T) {
	cases := []struct {
		name     string
		accounts []*protocloud.AccountBizRelWithAccount
	}{
		{
			name: "single non-ziyan account",
			accounts: []*protocloud.AccountBizRelWithAccount{
				{BaseAccount: corecloud.BaseAccount{ID: "t1", Name: "tcloud-1", Vendor: enumor.TCloud}},
			},
		},
		{
			name: "multiple accounts all non-ziyan",
			accounts: []*protocloud.AccountBizRelWithAccount{
				{BaseAccount: corecloud.BaseAccount{ID: "t1", Name: "tcloud-1", Vendor: enumor.TCloud}},
				{BaseAccount: corecloud.BaseAccount{ID: "a1", Name: "aws-1", Vendor: enumor.Aws}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const userID, sessionID = "user1", "sess-no-enabled"
			svc := newTestSessionSvc(t, testAppName, userID, sessionID)
			inv := newTestInvocation(sessionID, true)
			ctx := newTestCtx(userID)

			got, err := routeByAccountCount(ctx, nil, inv, svc, testAppName, tc.accounts)
			if err != nil {
				t.Fatalf("routeByAccountCount err: %v", err)
			}
			st, ok := got.(graph.State)
			if !ok {
				t.Fatalf("result type = %T, want graph.State", got)
			}
			if st[constant.AccountSelectNextNodeKey] != enumor.CvmApplyNodeFallback {
				t.Fatalf("next node = %v, want fallback", st[constant.AccountSelectNextNodeKey])
			}
			// 无权限文案须落成 assistant 消息：主图 fallback 只认消息尾部的 assistant 回复，
			// 子图的路由键不会经 output mapper 回填主图。
			msgs, _ := st[graph.StateKeyMessages].([]trpcmodel.Message)
			if len(msgs) != 1 || msgs[0].Role != trpcmodel.RoleAssistant ||
				msgs[0].Content != constant.NoPermissionFallbackMessage {
				t.Fatalf("fallback message = %+v, want single assistant message with no-permission text", msgs)
			}
			if _, exist := st[constant.SessionAccountIDTempKey]; exist {
				t.Fatalf("unusable account must not be selected: %v", st[constant.SessionAccountIDTempKey])
			}
			if _, exist := inv.RunOptions.RuntimeState[constant.SessionAccountIDTempKey]; exist {
				t.Fatalf("unusable account must not be injected into runtime state")
			}
			if persisted, ok := getSelectedAccountIDFromSession(ctx, inv, svc, testAppName); ok {
				t.Fatalf("unusable account must not be persisted, got %q", persisted)
			}
		})
	}
}

// isAccountEnabled 是账号可用性的唯一判据：仅自研云账号可用，nil 账号不可用。
func TestIsAccountEnabled(t *testing.T) {
	if !isAccountEnabled(&protocloud.AccountBizRelWithAccount{
		BaseAccount: corecloud.BaseAccount{ID: "z1", Vendor: enumor.TCloudZiyan}}) {
		t.Fatalf("tcloud-ziyan account should be enabled")
	}
	if isAccountEnabled(&protocloud.AccountBizRelWithAccount{
		BaseAccount: corecloud.BaseAccount{ID: "t1", Vendor: enumor.TCloud}}) {
		t.Fatalf("non-ziyan account must not be enabled")
	}
	if isAccountEnabled(nil) {
		t.Fatalf("nil account must not be enabled")
	}
}

// 自由文本未命中任一账号选项时返回空，供 default 分支跳过下传空账号。
func TestMatchAccountIDFromUserInput_NoMatch(t *testing.T) {
	accounts := []*protocloud.AccountBizRelWithAccount{
		{BaseAccount: corecloud.BaseAccount{ID: "0000002b", Name: "ziyan-hcm-test", Vendor: enumor.TCloudZiyan}},
	}
	if got := matchAccountIDFromUserInput("选择自研云账号", accounts); got != "" {
		t.Fatalf("unmatched free text should return empty, got %q", got)
	}
}

// 后端有账号时优先用后端。
func TestTryReuseAccountID_PreferBackend(t *testing.T) {
	const userID, sessionID, backendID, runtimeID = "user1", "sess-pref", "acc-backend", "acc-runtime"
	svc := newTestSessionSvc(t, testAppName, userID, sessionID)
	inv := newTestInvocation(sessionID, true)
	ctx := newTestCtx(userID)

	injectAccountIDToRuntimeState(ctx, inv, backendID, svc, testAppName)
	inv.RunOptions.RuntimeState[constant.SessionAccountIDTempKey] = runtimeID

	got, ok := tryReuseAccountID(ctx, inv, svc, testAppName, nil)
	if !ok || got != backendID {
		t.Fatalf("should prefer backend account: got=%q ok=%v, want=%q", got, ok, backendID)
	}
}

// isEnabledAccountID 仅对业务下可用（tcloud-ziyan）账号返回 true（select_account 校验的核心谓词）。
func TestIsEnabledAccountID(t *testing.T) {
	accounts := []*protocloud.AccountBizRelWithAccount{
		{BaseAccount: corecloud.BaseAccount{ID: "z1", Vendor: enumor.TCloudZiyan}},
		{BaseAccount: corecloud.BaseAccount{ID: "t1", Vendor: enumor.TCloud}},
	}
	if !isEnabledAccountID(accounts, "z1") {
		t.Fatalf("z1 (tcloud-ziyan) should be enabled")
	}
	if isEnabledAccountID(accounts, "t1") {
		t.Fatalf("t1 (non-ziyan) must not be enabled")
	}
	if isEnabledAccountID(accounts, "not-exist") {
		t.Fatalf("unknown id must not be enabled")
	}
}

// 回归：select_account 工具以 nil state 调用 parseBkBizID，且 resume 后 RuntimeState 里的
// bk_biz_id 经 checkpoint JSON 反序列化为 float64（非 int64）。修复前会落入
// util.GetInt64ByInterface(nil) 触发反射空指针 panic（线上 "node tool panic" 根因）。
func TestParseBkBizID_ResumeFloat64RuntimeStateNilState(t *testing.T) {
	inv := newTestInvocation("sess-biz", true)
	// checkpoint 恢复后 bk_biz_id 为 float64
	inv.RunOptions.RuntimeState[constant.SessionBkBizIDStateKey] = float64(2005000)
	ctx := newTestCtx("user1")

	if got := parseBkBizID(ctx, inv, nil); got != 2005000 {
		t.Fatalf("parseBkBizID with float64 runtime state = %d, want 2005000", got)
	}
}

// parseBkBizID 在 RuntimeState 与 state 均无 bk_biz_id 时返回 0 而非 panic（nil 值安全）。
func TestParseBkBizID_MissingReturnsZeroNoPanic(t *testing.T) {
	inv := newTestInvocation("sess-biz-missing", true)
	ctx := newTestCtx("user1")

	if got := parseBkBizID(ctx, inv, nil); got != 0 {
		t.Fatalf("parseBkBizID with missing bk_biz_id = %d, want 0", got)
	}
}

// parseBkBizID 优先读 RuntimeState 的 int64（每轮 fresh 场景），命中即返回。
func TestParseBkBizID_Int64RuntimeState(t *testing.T) {
	inv := newTestInvocation("sess-biz-int", true)
	inv.RunOptions.RuntimeState[constant.SessionBkBizIDStateKey] = int64(300)
	ctx := newTestCtx("user1")

	if got := parseBkBizID(ctx, inv, nil); got != 300 {
		t.Fatalf("parseBkBizID with int64 runtime state = %d, want 300", got)
	}
}

// parseBkBizID 在 RuntimeState 缺失时回退 graph State（account_select 节点场景）。
func TestParseBkBizID_FallbackToGraphState(t *testing.T) {
	inv := newTestInvocation("sess-biz-state", true)
	ctx := newTestCtx("user1")
	state := graph.State{constant.SessionBkBizIDStateKey: float64(777)}

	if got := parseBkBizID(ctx, inv, state); got != 777 {
		t.Fatalf("parseBkBizID fallback to graph state = %d, want 777", got)
	}
}

// 回归：自由文本选账号（经 select_account 落库，等价于 injectAccountIDToRuntimeState）后，
// 同会话下一轮以新 invocation 复用已选账号，不再重复弹账号选择（复现本次问题场景）。
func TestRegression_SelectedAccountReusedNextTurn(t *testing.T) {
	const userID, sessionID, accountID = "user1", "sess-reg", "0000002b"
	svc := newTestSessionSvc(t, testAppName, userID, sessionID)
	ctx := newTestCtx(userID)

	run1 := newTestInvocation(sessionID, true)
	injectAccountIDToRuntimeState(ctx, run1, accountID, svc, testAppName)

	run2 := newTestInvocation(sessionID, true)
	got, ok := tryReuseAccountID(ctx, run2, svc, testAppName, nil)
	if !ok || got != accountID {
		t.Fatalf("next turn should reuse selected account: got=%q ok=%v, want=%q", got, ok, accountID)
	}
}

// 账号失效后必须同时清掉 RuntimeState 与 session 后端：前者会让 account_gate 误判为已选账号
// 而放行申领类工具，后者会在下一轮 run 起点被重新回填。
func TestClearSelectedAccount_ClearsRuntimeStateAndBackend(t *testing.T) {
	const userID, sessionID, accountID = "user1", "sess-clear", "acc-stale"
	svc := newTestSessionSvc(t, testAppName, userID, sessionID)
	inv := newTestInvocation(sessionID, true)
	ctx := newTestCtx(userID)

	injectAccountIDToRuntimeState(ctx, inv, accountID, svc, testAppName)
	clearSelectedAccount(ctx, inv, svc, testAppName)

	if got, exist := inv.RunOptions.RuntimeState[constant.SessionAccountIDTempKey]; exist {
		t.Fatalf("runtime state account_id should be removed, got %v", got)
	}
	if got, ok := getSelectedAccountIDFromSession(ctx, inv, svc, testAppName); ok || got != "" {
		t.Fatalf("backend account_id should be cleared: got=%q ok=%v", got, ok)
	}
	if got, ok := tryReuseAccountID(ctx, inv, svc, testAppName, nil); ok || got != "" {
		t.Fatalf("cleared account must not be reusable: got=%q ok=%v", got, ok)
	}
}
