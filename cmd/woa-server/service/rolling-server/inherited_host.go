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

// Package rollingserver ...
package rollingserver

import (
	"time"

	woaserver "hcm/pkg/api/woa-server"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"
)

// ListBizInheritedHosts 业务视角查询可继承的固资候选，按机型族分组返回。
func (s *service) ListBizInheritedHosts(cts *rest.Contexts) (any, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		logs.Errorf("failed to get bk_biz_id from path, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	if bkBizID <= 0 {
		return nil, errf.New(errf.InvalidParameter, "bk_biz_id is invalid")
	}

	err = s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access}, BizID: bkBizID})
	if err != nil {
		logs.Errorf("list biz inherited hosts auth failed, err: %v, bkBizID: %d, rid: %s", err, bkBizID, cts.Kit.Rid)
		return nil, err
	}

	req := new(woaserver.ListInheritedHostsReq)
	if err = cts.DecodeInto(req); err != nil {
		logs.Errorf("failed to decode list biz inherited hosts request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	req.BkBizID = bkBizID
	if err = req.Validate(); err != nil {
		logs.Errorf("failed to validate list biz inherited hosts request, err: %v, req: %+v, rid: %s", err, req,
			cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	return s.listInheritedHosts(cts.Kit, req)
}

// ListInheritedHosts 资源视角查询可继承的固资候选，按机型族分组返回。
func (s *service) ListInheritedHosts(cts *rest.Contexts) (any, error) {
	req := new(woaserver.ListInheritedHostsReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("failed to decode list inherited hosts request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("failed to validate list inherited hosts request, err: %v, req: %+v, rid: %s", err, req,
			cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	err := s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.ZiYanResource, Action: meta.Create}, BizID: req.BkBizID,
	})
	if err != nil {
		logs.Errorf("list inherited hosts auth failed, err: %v, bkBizID: %d, rid: %s", err, req.BkBizID, cts.Kit.Rid)
		return nil, err
	}

	return s.listInheritedHosts(cts.Kit, req)
}

func (s *service) listInheritedHosts(kt *kit.Kit, req *woaserver.ListInheritedHostsReq) (
	*woaserver.ListInheritedHostsResp, error) {

	hostMap, err := s.rollingServerLogic.ListInheritedHosts(kt, req.BkBizID, req.Region, req.DeviceFamilies)
	if err != nil {
		logs.Errorf("list inherited hosts failed, err: %v, bkBizID: %d, region: %s, deviceFamilies: %v, rid: %s",
			err, req.BkBizID, req.Region, req.DeviceFamilies, kt.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	// 分组按请求入参的机型族顺序返回，未查到候选的机型族返回空列表
	now := time.Now()
	groups := make([]*woaserver.InheritedHostGroup, 0, len(req.DeviceFamilies))
	for _, deviceFamily := range req.DeviceFamilies {
		hosts := hostMap[deviceFamily]
		candidates := make([]*woaserver.InheritedHostCandidate, 0, len(hosts))
		for _, host := range hosts {
			candidates = append(candidates, &woaserver.InheritedHostCandidate{
				InheritedHost: cvt.PtrToVal(host),
				IsRecommended: isInheritedHostRecommended(now, host.BillingStartTime),
			})
		}

		groups = append(groups, &woaserver.InheritedHostGroup{DeviceFamily: deviceFamily, Hosts: candidates})
	}

	return &woaserver.ListInheritedHostsResp{Info: groups}, nil
}

// isInheritedHostRecommended 判断继承固资候选是否为推荐项。
// 套餐计费起始时间距当前时间已满 RsInheritedHostRecommendMonths 个月才推荐，满足条件的候选可以有多条；
// 计费起始时间缺失时无法判断，不作为推荐项。
func isInheritedHostRecommended(now time.Time, billingStartTime time.Time) bool {
	if billingStartTime.IsZero() {
		return false
	}

	return billingStartTime.AddDate(0, constant.RsInheritedHostRecommendMonths, 0).Before(now)
}
