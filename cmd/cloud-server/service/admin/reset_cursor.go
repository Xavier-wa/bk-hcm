/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2025 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package admin

import (
	"context"

	"hcm/cmd/cloud-server/logics/tenant"
	"hcm/cmd/cloud-server/service/sync"
	apiccsync "hcm/pkg/api/cloud-server/cc-sync"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// ResetCCWatchCursor resets cc event cursor to latest.
// Target tenant is from request header X-Bk-Tenant-Id.
// The reset takes effect on the next watch loop iteration via a reset flag,
// avoiding race with in-flight cursor commits.
func (s *adminService) ResetCCWatchCursor(cts *rest.Contexts) (any, error) {
	req := new(apiccsync.ResetWatchCursorReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("decode reset cc watch cursor req failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("validate reset cc watch cursor req failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	if s.ccWatcher == nil {
		logs.Errorf("cc watcher is not initialized, cloud resource sync is disabled, rid: %s", cts.Kit.Rid)
		return nil, errf.New(errf.Aborted, "cc watcher is not initialized, cloud resource sync is disabled")
	}

	if err := tenant.CheckTenantExist(cts.Kit, s.client.DataService(), cts.Kit.TenantID); err != nil {
		return nil, err
	}

	logs.Infof("reset cc watch event cursor start, resource: %s, tenant: %s, operator: %s, rid: %s",
		req.Resource, cts.Kit.TenantID, cts.Kit.User, cts.Kit.Rid)

	preCursor, err := s.ccWatcher.ResetEventCursor(cts.Kit, req.Resource)
	if err != nil {
		logs.Errorf("reset cc watch event cursor failed, err: %v, resource: %s, tenant: %s, rid: %s",
			err, req.Resource, cts.Kit.TenantID, cts.Kit.Rid)
		return nil, err
	}

	logs.Infof("reset cc watch event cursor success, pre cursor: %s, resource: %s, tenant: %s, rid: %s",
		preCursor, req.Resource, cts.Kit.TenantID, cts.Kit.Rid)

	// 主机补偿同步异步触发，cc 增量同步只涉及 ziyan/other 两个厂商。
	// 互斥判断在同步编排内逐账号进行，进行中的账号自动跳过。
	asyncKt := cts.Kit.NewSubKitWithCtx(context.Background()).WithAsyncSource()
	go sync.SyncHostsByTenant(asyncKt, []enumor.Vendor{enumor.TCloudZiyan, enumor.Other}, s.client)
	logs.Infof("trigger host sync, tenant: %s, rid: %s", cts.Kit.TenantID, cts.Kit.Rid)

	return &apiccsync.ResetWatchCursorResp{PreCursor: preCursor}, nil
}
