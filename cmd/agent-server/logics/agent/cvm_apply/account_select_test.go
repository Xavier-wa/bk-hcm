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
