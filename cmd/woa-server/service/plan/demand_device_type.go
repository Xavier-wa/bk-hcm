/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
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
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/slice"
)

// ListBizResPlanDemandWithDeviceTypes list resource plan demand with device types.
func (s *service) ListBizResPlanDemandWithDeviceTypes(cts *rest.Contexts) (interface{}, error) {
	bizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	req := new(ptypes.ListResPlanDemandWithDeviceTypesReq)
	if err = cts.DecodeInto(req); err != nil {
		logs.Errorf("failed to decode list res plan demand with device types request, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err = req.Validate(); err != nil {
		logs.Errorf("failed to validate list res plan demand with device types request, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// 权限校验
	bkBizIDs, err := s.bizLogics.ListAuthorizedBiz(cts.Kit)
	if err != nil {
		logs.Errorf("failed to list authorized biz, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	if !slice.IsItemInSlice(bkBizIDs, bizID) {
		return &ptypes.ListResPlanDemandWithDeviceTypesResp{
			Details: make([]*ptypes.ListResPlanDemandWithDeviceTypesItem, 0),
		}, nil
	}

	req.BkBizID = bizID

	resp, err := s.planController.ListResPlanDemandWithDeviceTypes(cts.Kit, req)
	if err != nil {
		logs.Errorf("failed to list res plan demand with device types, err: %v, req: %+v, rid: %s",
			err, *req, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return resp, nil
}
