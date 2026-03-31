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

package plan

import (
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// SyncBudgetOperatorByTime 同步 admin 单据提报人信息。
func (s *service) SyncBudgetOperatorByTime(cts *rest.Contexts) (interface{}, error) {
	req := new(ptypes.BudgetOperatorSyncReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("decode budget operator sync request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("invalid budget operator sync request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.ZiYanResPlan, Action: meta.Update}}
	if err := s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		logs.Errorf("authorize budget operator sync failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	start, end, _ := req.TimeRange()
	resp, err := s.planController.SyncBudgetOperatorByTime(cts.Kit, start, end)
	if err != nil {
		logs.Errorf("sync budget operator failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return resp, errf.NewFromErr(errf.Aborted, err)
	}

	return resp, nil

}
