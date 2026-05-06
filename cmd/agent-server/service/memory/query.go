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

package memory

import (
	proto "hcm/pkg/api/agent-server/memory"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/memory"
)

// ListMemories returns the stored memory entries for the calling user.
// Supports optional ?query= parameter for semantic search.
//
// GET /api/v1/agent/memory
func (svc *service) ListMemories(cts *rest.Contexts) (interface{}, error) {
	if svc.memorySvc == nil {
		return nil, errf.New(errf.PermissionDenied, "memory backend is not configured")
	}

	if err := svc.authorizer.AuthorizeWithPerm(cts.Kit,
		meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AgentAssistant, Action: meta.Find}}); err != nil {
		logs.Errorf("agent auth: permission denied, user: %s, err: %v, rid: %s", cts.Kit.User, err, cts.Kit.Rid)
		return nil, errf.New(errf.PermissionDenied, "permission denied")
	}

	userKey := memory.UserKey{AppName: svc.appName, UserID: cts.Kit.User}

	query := cts.Request.QueryParameter("query")
	var entries []*memory.Entry
	var err error
	if query != "" {
		entries, err = svc.memorySvc.SearchMemories(cts.Kit.Ctx, userKey, query)
	} else {
		entries, err = svc.memorySvc.ReadMemories(cts.Kit.Ctx, userKey, 0)
	}
	if err != nil {
		logs.Errorf("list memories for user %s: %v, rid: %s", cts.Kit.User, err, cts.Kit.Rid)
		return nil, errf.New(errf.Unknown, "list memories failed")
	}

	resp := &proto.ListMemoriesResp{
		Memories: make([]proto.MemoryEntry, 0, len(entries)),
		Total:    len(entries),
	}
	for _, e := range entries {
		if e == nil || e.Memory == nil {
			continue
		}
		item := proto.MemoryEntry{
			ID:     e.ID,
			Memory: e.Memory.Memory,
			Topics: e.Memory.Topics,
		}
		if !e.CreatedAt.IsZero() {
			item.CreatedAt = e.CreatedAt.Format(constant.TimeStdFormat)
		}
		if !e.UpdatedAt.IsZero() {
			item.UpdatedAt = e.UpdatedAt.Format(constant.TimeStdFormat)
		}
		resp.Memories = append(resp.Memories, item)
	}
	return resp, nil
}
