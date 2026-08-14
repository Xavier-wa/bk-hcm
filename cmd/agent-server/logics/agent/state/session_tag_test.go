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

package state

import (
	"context"
	"errors"
	"testing"

	"hcm/cmd/agent-server/logics/auth"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
)

const (
	testThreadID = "thread-1"
	testUser     = "tester"
)

// stubTagUpdater 记录收到的回写请求，并可注入错误以覆盖失败路径。
type stubTagUpdater struct {
	reqs []*dsaiagent.UpdateAiagentSessionReq
	err  error
}

func (s *stubTagUpdater) Update(_ *kit.Kit, req *dsaiagent.UpdateAiagentSessionReq) error {
	s.reqs = append(s.reqs, req)
	return s.err
}

// ctxWithThread 构造节点内可解析出 threadID 的 context；threadID 为空时模拟 RuntimeState 缺键。
func ctxWithThread(threadID, user string) context.Context {
	var ctx context.Context = context.Background()
	if user != "" {
		ctx = auth.WithBKUsername(ctx, user)
	}

	rs := map[string]any{}
	if threadID != "" {
		rs[graph.CfgKeyLineageID] = threadID
	}
	inv := &trpcagent.Invocation{RunOptions: trpcagent.RunOptions{RuntimeState: rs}}
	return trpcagent.NewInvocationContext(ctx, inv)
}

// 正常路径：请求以 threadID 定位会话，写入新标签，Reviser 取 ctx 中的蓝鲸用户名。
func TestPersistSessionTag_Success(t *testing.T) {
	updater := &stubTagUpdater{}

	PersistSessionTag(ctxWithThread(testThreadID, testUser), updater, enumor.IntentTypeResourceQuery)

	if len(updater.reqs) != 1 {
		t.Fatalf("update called %d times, want 1", len(updater.reqs))
	}
	req := updater.reqs[0]
	if req.ID != testThreadID {
		t.Errorf("req.ID = %q, want %q", req.ID, testThreadID)
	}
	if req.SessionTag != enumor.IntentTypeResourceQuery {
		t.Errorf("req.SessionTag = %q, want %q", req.SessionTag, enumor.IntentTypeResourceQuery)
	}
	if req.Reviser != testUser {
		t.Errorf("req.Reviser = %q, want %q", req.Reviser, testUser)
	}
}

// Reviser 是必填字段：ctx 缺蓝鲸用户名时必须落到后端操作用户，不能留空导致请求被校验拒绝。
func TestPersistSessionTag_ReviserFallsBackToBackendUser(t *testing.T) {
	updater := &stubTagUpdater{}

	PersistSessionTag(ctxWithThread(testThreadID, ""), updater, enumor.IntentTypeHostApply)

	if len(updater.reqs) != 1 {
		t.Fatalf("update called %d times, want 1", len(updater.reqs))
	}
	if got := updater.reqs[0].Reviser; got != constant.BackendOperationUserKey {
		t.Errorf("req.Reviser = %q, want %q", got, constant.BackendOperationUserKey)
	}
}

// 防御路径：未注入客户端、以及解析不出 threadID 时都不得发起回写，且不得 panic。
func TestPersistSessionTag_SkipsWithoutUpdaterOrThreadID(t *testing.T) {
	t.Run("nil updater", func(t *testing.T) {
		PersistSessionTag(ctxWithThread(testThreadID, testUser), nil, enumor.IntentTypeHostApply)
	})

	t.Run("missing thread id", func(t *testing.T) {
		updater := &stubTagUpdater{}
		PersistSessionTag(ctxWithThread("", testUser), updater, enumor.IntentTypeHostApply)
		if len(updater.reqs) != 0 {
			t.Errorf("update called %d times, want 0", len(updater.reqs))
		}
	})

	t.Run("no invocation in context", func(t *testing.T) {
		updater := &stubTagUpdater{}
		PersistSessionTag(context.Background(), updater, enumor.IntentTypeHostApply)
		if len(updater.reqs) != 0 {
			t.Errorf("update called %d times, want 0", len(updater.reqs))
		}
	})
}

// 回写失败只记日志、不向调用方抛错，调用点因此无法阻断本轮 run。
func TestPersistSessionTag_SwallowsUpdateError(t *testing.T) {
	updater := &stubTagUpdater{err: errors.New("data-service unavailable")}

	PersistSessionTag(ctxWithThread(testThreadID, testUser), updater, enumor.IntentTypeHostApply)

	if len(updater.reqs) != 1 {
		t.Fatalf("update called %d times, want 1", len(updater.reqs))
	}
}
