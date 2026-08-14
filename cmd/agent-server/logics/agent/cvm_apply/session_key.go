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

package cvmapply

import (
	"context"

	"hcm/cmd/agent-server/logics/auth"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

// BuildAGUISessionKey 构造 session 后端的定位键（AppName + UserID + SessionID）。
//
// 读写已选账号必须共用此 key。不可依赖 inv.Session.*：子图 InputMapper 会过滤
// StateKeySession，子图节点上 inv.Session 可能为空，导致 key 不一致、写读对不上。
func BuildAGUISessionKey(ctx context.Context, appName, threadID string) session.Key {
	return BuildAGUISessionKeyWithUser(appName, resolveUserID(ctx, nil), threadID)
}

// BuildAGUISessionKeyWithUser 显式指定 userID 构造 key（account_select 写入侧用，可回退 inv.Session.UserID）。
func BuildAGUISessionKeyWithUser(appName, userID, threadID string) session.Key {
	return session.Key{
		AppName:   appName,
		UserID:    userID,
		SessionID: threadID,
	}
}

// resolveUserID 解析 session 用户标识，须与 AG-UI UserIDResolver 结果一致。
// 优先 ctx 中的蓝鲸用户名；子图节点 ctx 可能缺失时回退 inv.Session.UserID（clone 父 inv 保留）。
func resolveUserID(ctx context.Context, inv *trpcagent.Invocation) string {
	if u := auth.BKUsernameFromContext(ctx); u != "" {
		return u
	}
	if inv != nil && inv.Session != nil && inv.Session.UserID != "" {
		return inv.Session.UserID
	}
	return ""
}

// resolveThreadID 解析 thread / lineage ID。
// 优先 RuntimeState[lineage_id]（makeRunOptionResolver 每轮写入，子图可用）；
// 回退 inv.Session.ID（仅父图可靠）。
func resolveThreadID(inv *trpcagent.Invocation) string {
	if inv != nil && inv.RunOptions.RuntimeState != nil {
		if id, ok := inv.RunOptions.RuntimeState[graph.CfgKeyLineageID].(string); ok && id != "" {
			return id
		}
	}
	if inv != nil && inv.Session != nil && inv.Session.ID != "" {
		return inv.Session.ID
	}
	return ""
}

// sessionKeyFromInvocation 按统一规则构造 session.Key；threadID 或 userID 缺失时返回空 key。
func sessionKeyFromInvocation(ctx context.Context, inv *trpcagent.Invocation, appName string) session.Key {
	rid := rest.RidFromContext(ctx)
	threadID := resolveThreadID(inv)
	if threadID == "" {
		logs.Errorf("account_select: empty thread id for session key, app=%s, rid: %s", appName, rid)
		return session.Key{}
	}
	key := BuildAGUISessionKeyWithUser(appName, resolveUserID(ctx, inv), threadID)
	if key.UserID == "" {
		logs.Errorf("account_select: empty user id for session key, app=%s thread=%s, rid: %s",
			appName, threadID, rid)
	}
	return key
}
