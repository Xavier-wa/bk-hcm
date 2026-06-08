/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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

package task

import (
	"fmt"

	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	woaserver "hcm/pkg/api/woa-server"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// GetBizApplyRecommendTop returns the top apply recommendations for a user.
func (s *service) GetBizApplyRecommendTop(cts *rest.Contexts) (interface{}, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		logs.Errorf("failed to get bk_biz_id from path, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	if bkBizID <= 0 {
		return nil, errf.New(errf.InvalidParameter, "bk_biz_id is invalid")
	}
	req := new(woaserver.ApplyRecommendTopReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access}, BizID: bkBizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		logs.Errorf("failed to authorize biz access, bizID: %d, err: %v, rid: %s", bkBizID, err, cts.Kit.Rid)
		return nil, err
	}

	result := &woaserver.ApplyRecommendTopResp{
		Items: make([]*woaserver.ApplyRecommendTopElem, 0),
	}
	// Step 1: query user recommend table
	userRows, err := s.listUserRecommend(cts, bkBizID, req.BkUsername, req.Limit)
	if err != nil {
		logs.Errorf("list user recommend failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	for _, row := range userRows {
		result.Items = append(result.Items, &woaserver.ApplyRecommendTopElem{
			RequireType: row.RequireType,
			Region:      row.Region,
			DeviceType:  row.DeviceType,
			Count:       row.Count,
			Source:      enumor.ApplyRecommendSourceUser,
		})
	}

	remain := req.Limit - len(result.Items)
	if remain <= 0 {
		return result, nil
	}

	// Step 2: query biz recommend table to supplement
	bizRows, err := s.listBizRecommend(cts, bkBizID, req.Limit)
	if err != nil {
		logs.Errorf("list biz recommend failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	// Step 3: deduplicate biz rows by (require_type, region, device_type) against user rows
	userSet := make(map[string]struct{}, len(userRows))
	for _, row := range userRows {
		key := buildTripleKey(row.RequireType, row.Region, row.DeviceType)
		userSet[key] = struct{}{}
	}

	for _, row := range bizRows {
		if remain <= 0 {
			break
		}
		key := buildTripleKey(row.RequireType, row.Region, row.DeviceType)
		if _, exists := userSet[key]; exists {
			continue
		}
		result.Items = append(result.Items, &woaserver.ApplyRecommendTopElem{
			RequireType: row.RequireType,
			Region:      row.Region,
			DeviceType:  row.DeviceType,
			Count:       row.Count,
			Source:      enumor.ApplyRecommendSourceBiz,
		})
		remain--
	}

	return result, nil
}

func (s *service) listUserRecommend(cts *rest.Contexts, bkBizID int64, bkUsername string, limit int) (
	[]*cvmapplytable.ZiyanCvmApplyUserRecommend, error) {

	filterExpr := tools.AllExpression()
	filterExpr.Rules = append(filterExpr.Rules,
		tools.RuleEqual("bk_biz_id", bkBizID),
		tools.RuleEqual("bk_username", bkUsername),
	)

	req := &cvmapplyproto.ZiyanCvmApplyUserRecommendListReq{
		Filter: filterExpr,
		Page: &core.BasePage{
			Start: 0,
			Limit: uint(limit),
			Sort:  "count",
			Order: core.Descending,
		},
	}

	resp, err := s.client.DataService().TCloudZiyan.ZiyanCvmApplyUserRecommend.List(cts.Kit.Ctx, cts.Kit.Header(), req)
	if err != nil {
		logs.Errorf("list user recommend failed, err: %v, biz: %d, user: %s, rid: %s", err, bkBizID, bkUsername,
			cts.Kit.Rid)
		return nil, err
	}

	return resp.Details, nil
}

func (s *service) listBizRecommend(cts *rest.Contexts, bkBizID int64, limit int) (
	[]*cvmapplytable.ZiyanCvmApplyBizRecommend, error) {

	req := &cvmapplyproto.ZiyanCvmApplyBizRecommendListReq{
		Filter: tools.EqualExpression("bk_biz_id", bkBizID),
		Page: &core.BasePage{
			Start: 0,
			Limit: uint(limit),
			Sort:  "count",
			Order: core.Descending,
		},
	}

	resp, err := s.client.DataService().TCloudZiyan.ZiyanCvmApplyBizRecommend.List(cts.Kit.Ctx, cts.Kit.Header(), req)
	if err != nil {
		logs.Errorf("list biz recommend failed, err: %v, biz: %d, rid: %s", err, bkBizID, cts.Kit.Rid)
		return nil, err
	}

	return resp.Details, nil
}

func buildTripleKey(requireType enumor.RequireType, region, deviceType string) string {
	return fmt.Sprintf("%d|%s|%s", requireType, region, deviceType)
}
