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

package eval

import (
	proto "hcm/pkg/api/agent-server/eval"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// SyncHistory reconstructs missing aiagent_run rows from session history.
// Already-existing run_id values are skipped.
func (svc *service) SyncHistory(cts *rest.Contexts) (interface{}, error) {
	if err := svc.authorizeManage(cts.Kit); err != nil {
		return nil, err
	}
	if svc.syncer == nil {
		logs.Errorf("sync history failed, history syncer is not initialized, rid: %s", cts.Kit.Rid)
		return nil, errf.New(errf.Aborted, "history syncer is not initialized")
	}
	req := new(proto.SyncHistoryReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("sync history decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		logs.Errorf("sync history validate request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	resp, err := svc.syncer.Sync(cts.Kit, req)
	if err != nil {
		logs.Errorf("sync history failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	return resp, nil
}
