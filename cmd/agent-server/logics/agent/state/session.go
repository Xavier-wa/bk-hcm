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

// Package state provides the session state management logic.
package state

import (
	"context"
	"time"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

// PersistStateToService calls svc.UpdateSessionState to write stateMap into the session backend (e.g. MySQL).
// It also updates the in-memory sess.State for consistency within the current process.
func PersistStateToService(ctx context.Context, stateMap session.StateMap) {
	rid := rest.RidFromContext(ctx)
	inv, ok := trpcagent.InvocationFromContext(ctx)
	if !ok || inv == nil || inv.Session == nil || inv.SessionService == nil {
		logs.Warnf("[session_state] no invocation or session in context, skip write state into session "+
			"(ok=%v inv=%v session=%v, service=%v), rid: %s",
			ok, inv != nil, inv != nil && inv.Session != nil, inv != nil && inv.SessionService != nil, rid)
		return
	}
	sess := inv.Session
	svc := inv.SessionService

	// Update in-memory first for consistency within the current invocation.
	for k, v := range stateMap {
		sess.SetState(k, v)
	}

	// 框架对 state 的更新存在并发且未进行加锁，如果 after callback 执行的过快，add event 对 state 的更新可能会产生覆盖，因此这里等待
	time.Sleep(constant.SessionStateUpdateConcurrentWait)

	key := session.Key{
		AppName:   sess.AppName,
		UserID:    sess.UserID,
		SessionID: sess.ID,
	}
	if err := svc.UpdateSessionState(ctx, key, stateMap); err != nil {
		logs.Errorf("[session_state] UpdateSessionState failed: key=%+v err=%v, rid: %s", key, err, rid)
	}
}
