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
	"hcm/pkg/criteria/errf"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/memory"
)

// AddMemory adds a new memory entry for the calling user.
//
// POST /api/v1/agent/memory
func (svc *service) AddMemory(cts *rest.Contexts) (interface{}, error) {
	if svc.memorySvc == nil {
		return nil, errf.New(errf.PermissionDenied, "memory backend is not configured")
	}

	req := new(proto.AddMemoryReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	userKey := memory.UserKey{AppName: svc.appName, UserID: cts.Kit.User}
	if err := svc.memorySvc.AddMemory(cts.Kit.Ctx, userKey, req.Memory, req.Topics); err != nil {
		logs.Errorf("add memory for user %s: %v, rid: %s", cts.Kit.User, err, cts.Kit.Rid)
		return nil, errf.New(errf.Unknown, "add memory failed")
	}
	return nil, nil
}
