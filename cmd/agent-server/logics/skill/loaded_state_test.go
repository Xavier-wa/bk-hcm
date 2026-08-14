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

package skill

import (
	"context"
	"sort"
	"testing"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/session"
	inmemory "trpc.group/trpc-go/trpc-agent-go/session/inmemory"
	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
)

const (
	testAgentName  = "hcm-agent"
	testUserID     = "user1"
	testSessionID  = "sess1"
	applySkillName = "ziyan-cvm-apply"
)

// newLoadedSkillCtx 构造一个带 session 的 invocation context，并把 loadedSkills 标记为已加载。
func newLoadedSkillCtx(t *testing.T, loadedSkills ...string) (context.Context, *trpcagent.Invocation) {
	t.Helper()

	svc := inmemory.NewSessionService()
	key := session.Key{AppName: testAgentName, UserID: testUserID, SessionID: testSessionID}
	if _, err := svc.CreateSession(context.Background(), key, nil); err != nil {
		t.Fatalf("create test session failed: %v", err)
	}

	sess := &session.Session{
		AppName: testAgentName,
		UserID:  testUserID,
		ID:      testSessionID,
		State:   session.StateMap{},
	}
	for _, name := range loadedSkills {
		sess.SetState(skillpkg.LoadedKey(testAgentName, name), []byte("1"))
		sess.SetState(skillpkg.DocsKey(testAgentName, name), []byte("*"))
	}

	inv := &trpcagent.Invocation{Session: sess, SessionService: svc}
	return trpcagent.NewInvocationContext(context.Background(), inv), inv
}

// 场景切换后旧场景的 skill 必须不再被注入：清除之后 collectLoadedSkills 读不到任何已加载 skill。
func TestClearLoadedSkills(t *testing.T) {
	ctx, inv := newLoadedSkillCtx(t, applySkillName)

	if got := collectLoadedSkills(testAgentName, inv.Session.State); len(got) != 1 ||
		got[0] != applySkillName {

		t.Fatalf("loaded skills before clear = %v, want [%s]", got, applySkillName)
	}

	ClearLoadedSkills(ctx, testAgentName)

	if got := collectLoadedSkills(testAgentName, inv.Session.State); len(got) != 0 {
		t.Fatalf("loaded skills after clear = %v, want empty", got)
	}
	docsKey := skillpkg.DocsKey(testAgentName, applySkillName)
	if got := inv.Session.State[docsKey]; len(got) != 0 {
		t.Fatalf("selected docs after clear = %q, want empty", got)
	}
}

// 其他 agent 的 skill 状态不属于本 agent 的作用域，清除时不得被波及。
func TestClearLoadedSkillsKeepsOtherAgentState(t *testing.T) {
	ctx, inv := newLoadedSkillCtx(t, applySkillName)

	otherKey := skillpkg.LoadedKey("other-agent", applySkillName)
	inv.Session.SetState(otherKey, []byte("1"))

	ClearLoadedSkills(ctx, testAgentName)

	if got := inv.Session.State[otherKey]; len(got) == 0 {
		t.Fatalf("other agent loaded skill must be kept, got %q", got)
	}
}

// 无 invocation / session 时只记日志、不 panic（例如单测或异常上下文）。
func TestClearLoadedSkillsWithoutSession(t *testing.T) {
	ClearLoadedSkills(context.Background(), testAgentName)
}

// collectLoadedSkills 必须按值判空：清除以写空值表达，key 仍然存在。
func TestCollectLoadedSkillsSkipsClearedEntries(t *testing.T) {
	state := session.StateMap{
		skillpkg.LoadedKey(testAgentName, applySkillName):        []byte{},
		skillpkg.LoadedKey(testAgentName, "hcm-resource-search"): []byte("1"),
	}

	got := collectLoadedSkills(testAgentName, state)
	sort.Strings(got)
	if len(got) != 1 || got[0] != "hcm-resource-search" {
		t.Fatalf("loaded skills = %v, want [hcm-resource-search]", got)
	}
}
