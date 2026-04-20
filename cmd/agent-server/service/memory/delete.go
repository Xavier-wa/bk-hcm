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
	"hcm/pkg/criteria/errf"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/memory"
)

// DeleteMemory deletes a single memory entry by ID.
//
// DELETE /api/v1/agent/memory/{memory_id}
func (svc *service) DeleteMemory(cts *rest.Contexts) (interface{}, error) {
	if svc.memorySvc == nil {
		return nil, errf.New(errf.PermissionDenied, "memory backend is not configured")
	}

	memID := cts.PathParameter("memory_id").String()
	if memID == "" {
		return nil, errf.New(errf.InvalidParameter, `missing path parameter "memory_id"`)
	}

	memKey := memory.Key{AppName: svc.appName, UserID: cts.Kit.User, MemoryID: memID}
	if err := svc.memorySvc.DeleteMemory(cts.Kit.Ctx, memKey); err != nil {
		logs.Errorf("delete memory %s for user %s: %v, rid: %s", memID, cts.Kit.User, err, cts.Kit.Rid)
		return nil, errf.New(errf.Unknown, "delete memory failed")
	}
	return nil, nil
}

// ClearMemories deletes all memory entries for the calling user.
//
// DELETE /api/v1/agent/memory
func (svc *service) ClearMemories(cts *rest.Contexts) (interface{}, error) {
	if svc.memorySvc == nil {
		return nil, errf.New(errf.PermissionDenied, "memory backend is not configured")
	}

	userKey := memory.UserKey{AppName: svc.appName, UserID: cts.Kit.User}
	if err := svc.memorySvc.ClearMemories(cts.Kit.Ctx, userKey); err != nil {
		logs.Errorf("clear memories for user %s: %v, rid: %s", cts.Kit.User, err, cts.Kit.Rid)
		return nil, errf.New(errf.Unknown, "clear memories failed")
	}
	return nil, nil
}
