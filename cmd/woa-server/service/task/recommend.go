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
	"sort"

	planlogics "hcm/cmd/woa-server/logics/plan"
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/api/core"
	dt "hcm/pkg/api/core/cloud/device-type"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	woaserver "hcm/pkg/api/woa-server"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/thirdparty/cvmapi"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"
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
	userRows, err := s.listUserRecommend(cts.Kit, bkBizID, req.BkUsername, req.Limit)
	if err != nil {
		logs.Errorf("list user recommend failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	for _, row := range userRows {
		result.Items = append(result.Items, &woaserver.ApplyRecommendTopElem{
			RequireType: row.RequireType,
			Region:      row.Region,
			DeviceType:  row.DeviceType,
			ImageID:     row.ImageID,
			Count:       row.Count,
			Source:      enumor.ApplyRecommendSourceUser,
		})
	}

	remain := req.Limit - len(result.Items)
	if remain <= 0 {
		return result, nil
	}

	// Step 2: query biz recommend table to supplement
	bizRows, err := s.listBizRecommend(cts.Kit, bkBizID, req.Limit)
	if err != nil {
		logs.Errorf("list biz recommend failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	// Step 3: deduplicate biz rows by (require_type, region, device_type, image_id) against user rows
	userSet := make(map[string]struct{}, len(userRows))
	for _, row := range userRows {
		key := buildRecommendKey(row.RequireType, row.Region, row.DeviceType, row.ImageID)
		userSet[key] = struct{}{}
	}

	for _, row := range bizRows {
		if remain <= 0 {
			break
		}
		key := buildRecommendKey(row.RequireType, row.Region, row.DeviceType, row.ImageID)
		if _, exists := userSet[key]; exists {
			continue
		}
		result.Items = append(result.Items, &woaserver.ApplyRecommendTopElem{
			RequireType: row.RequireType,
			Region:      row.Region,
			DeviceType:  row.DeviceType,
			ImageID:     row.ImageID,
			Count:       row.Count,
			Source:      enumor.ApplyRecommendSourceBiz,
		})
		remain--
	}

	return result, nil
}

func (s *service) listUserRecommend(kt *kit.Kit, bkBizID int64, bkUsername string, limit int,
	extraRules ...filter.RuleFactory) ([]*cvmapplytable.ZiyanCvmApplyUserRecommend, error) {

	filterExpr := tools.AllExpression()
	filterExpr.Rules = append(filterExpr.Rules,
		tools.RuleEqual("bk_biz_id", bkBizID),
		tools.RuleEqual("bk_username", bkUsername),
	)
	filterExpr.Rules = append(filterExpr.Rules, extraRules...)

	req := &cvmapplyproto.ZiyanCvmApplyUserRecommendListReq{
		Filter: filterExpr,
		Page: &core.BasePage{
			Start: 0,
			Limit: uint(limit),
			Sort:  "count",
			Order: core.Descending,
		},
	}

	resp, err := s.client.DataService().TCloudZiyan.ZiyanCvmApplyUserRecommend.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("list user recommend failed, err: %v, biz: %d, user: %s, rid: %s", err, bkBizID, bkUsername,
			kt.Rid)
		return nil, err
	}

	return resp.Details, nil
}

func (s *service) listBizRecommend(kt *kit.Kit, bkBizID int64, limit int,
	extraRules ...filter.RuleFactory) ([]*cvmapplytable.ZiyanCvmApplyBizRecommend, error) {

	filterExpr := tools.AllExpression()
	filterExpr.Rules = append(filterExpr.Rules, tools.RuleEqual("bk_biz_id", bkBizID))
	filterExpr.Rules = append(filterExpr.Rules, extraRules...)

	req := &cvmapplyproto.ZiyanCvmApplyBizRecommendListReq{
		Filter: filterExpr,
		Page: &core.BasePage{
			Start: 0,
			Limit: uint(limit),
			Sort:  "count",
			Order: core.Descending,
		},
	}

	resp, err := s.client.DataService().TCloudZiyan.ZiyanCvmApplyBizRecommend.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("list biz recommend failed, err: %v, biz: %d, rid: %s", err, bkBizID, kt.Rid)
		return nil, err
	}

	return resp.Details, nil
}

func buildRecommendKey(requireType enumor.RequireType, region, deviceType, imageID string) string {
	return fmt.Sprintf("%d|%s|%s|%s", requireType, region, deviceType, imageID)
}

// staticRecommendCandidate 静态推荐候选（归一化 user/biz 两表行）
type staticRecommendCandidate struct {
	requireType enumor.RequireType
	region      string
	deviceType  string
	imageID     string
	count       int
	source      enumor.ApplyRecommendSource
}

// GetBizApplyRecommendByStatic returns online recommendation plans built on static recommend + stock.
func (s *service) GetBizApplyRecommendByStatic(cts *rest.Contexts) (interface{}, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		logs.Errorf("failed to get bk_biz_id from path, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	if bkBizID <= 0 {
		return nil, errf.New(errf.InvalidParameter, "bk_biz_id is invalid")
	}
	req := new(woaserver.ApplyRecommendByStaticReq)
	if err = cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err = req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access}, BizID: bkBizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		logs.Errorf("failed to authorize biz access, bizID: %d, err: %v, rid: %s", bkBizID, err, cts.Kit.Rid)
		return nil, err
	}

	applyNum := cc.WoaServer().ApplyRecommend.DefaultApplyNum
	if req.Replicas != nil {
		applyNum = cvt.PtrToVal(req.Replicas)
	}

	candidates, err := s.listStaticCandidates(cts.Kit, bkBizID, req)
	if err != nil {
		logs.Errorf("list static recommend candidates failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	candidates, err = s.filterCandidatesByCapacity(cts.Kit, candidates, applyNum, req.Zone)
	if err != nil {
		logs.Errorf("filter candidates by capacity failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	return &woaserver.ApplyRecommendByStaticResp{Items: assembleStaticPlans(candidates, req, applyNum)}, nil
}

// listStaticCandidates 查静态推荐候选：user 优先 biz 补足，A 类入参收窄，按四元组去重。
func (s *service) listStaticCandidates(kt *kit.Kit, bkBizID int64,
	req *woaserver.ApplyRecommendByStaticReq) ([]*staticRecommendCandidate, error) {

	rules := buildStaticFilterRules(req)
	fetchLimit := cc.WoaServer().ApplyRecommend.MaxRows
	userRows, err := s.listUserRecommend(kt, bkBizID, req.BkUsername, fetchLimit, rules...)
	if err != nil {
		return nil, err
	}
	candidates := make([]*staticRecommendCandidate, 0, len(userRows))
	userSet := make(map[string]struct{}, len(userRows))
	for _, row := range userRows {
		userSet[buildRecommendKey(row.RequireType, row.Region, row.DeviceType, row.ImageID)] = struct{}{}
		candidates = append(candidates, &staticRecommendCandidate{
			requireType: row.RequireType, region: row.Region, deviceType: row.DeviceType,
			imageID: row.ImageID, count: row.Count, source: enumor.ApplyRecommendSourceUser,
		})
	}
	bizRows, err := s.listBizRecommend(kt, bkBizID, fetchLimit, rules...)
	if err != nil {
		return nil, err
	}
	for _, row := range bizRows {
		if _, ok := userSet[buildRecommendKey(row.RequireType, row.Region, row.DeviceType, row.ImageID)]; ok {
			continue
		}
		candidates = append(candidates, &staticRecommendCandidate{
			requireType: row.RequireType, region: row.Region, deviceType: row.DeviceType,
			imageID: row.ImageID, count: row.Count, source: enumor.ApplyRecommendSourceBiz,
		})
	}
	return candidates, nil
}

// buildStaticFilterRules 构建 A 类过滤条件（入参非空者）。
func buildStaticFilterRules(req *woaserver.ApplyRecommendByStaticReq) []filter.RuleFactory {
	rules := make([]filter.RuleFactory, 0)
	if req.RequireType != nil {
		rules = append(rules, tools.RuleEqual("require_type", *req.RequireType))
	}
	if req.Region != "" {
		rules = append(rules, tools.RuleEqual("region", req.Region))
	}
	if req.DeviceType != "" {
		rules = append(rules, tools.RuleEqual("device_type", req.DeviceType))
	}
	if req.ImageID != "" {
		rules = append(rules, tools.RuleEqual("image_id", req.ImageID))
	}
	return rules
}

// capacityTriple 库存校验的输入三元组（不含镜像维度）。
type capacityTriple struct {
	requireType enumor.RequireType
	region      string
	deviceType  string
}

// filterCandidatesByCapacity 库存校验：免校验需求类型直接保留，其余按静态表 capacity ≥ 申请数量过滤。
func (s *service) filterCandidatesByCapacity(kt *kit.Kit, candidates []*staticRecommendCandidate,
	applyNum int, zone string) ([]*staticRecommendCandidate, error) {

	needCheck := make([]capacityTriple, 0, len(candidates))
	for _, c := range candidates {
		if !c.requireType.NotNeedVerifyCapacity() {
			needCheck = append(needCheck, capacityTriple{c.requireType, c.region, c.deviceType})
		}
	}
	satisfied := make(map[string]bool)
	if len(needCheck) > 0 {
		var err error
		satisfied, err = s.queryCapacitySatisfied(kt, needCheck, applyNum, zone)
		if err != nil {
			return nil, err
		}
	}
	result := make([]*staticRecommendCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.requireType.NotNeedVerifyCapacity() {
			result = append(result, c)
			continue
		}
		if satisfied[buildCapacityKey(c.requireType, c.region, c.deviceType)] {
			result = append(result, c)
		}
	}
	return result, nil
}

// queryCapacitySatisfied 批量查静态库存表，返回满足 capacity ≥ 申请数量的 (require_type|region|device_type) 集合。
func (s *service) queryCapacitySatisfied(kt *kit.Kit, triples []capacityTriple,
	applyNum int, zone string) (map[string]bool, error) {

	rtSet, regionSet, dtSet := map[enumor.RequireType]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	requireTypes, regions, deviceTypes := make([]enumor.RequireType, 0), make([]string, 0), make([]string, 0)
	for _, c := range triples {
		if _, ok := rtSet[c.requireType]; !ok {
			rtSet[c.requireType] = struct{}{}
			requireTypes = append(requireTypes, c.requireType)
		}
		if _, ok := regionSet[c.region]; !ok {
			regionSet[c.region] = struct{}{}
			regions = append(regions, c.region)
		}
		if _, ok := dtSet[c.deviceType]; !ok {
			dtSet[c.deviceType] = struct{}{}
			deviceTypes = append(deviceTypes, c.deviceType)
		}
	}
	expr := tools.AllExpression()
	expr.Rules = append(expr.Rules, tools.RuleIn("require_type", requireTypes), tools.RuleIn("region", regions))
	deviceTypeExpr := tools.ExpressionOr()
	for _, batch := range slice.Split(deviceTypes, int(filter.DefaultMaxInLimit)) {
		deviceTypeExpr.Rules = append(deviceTypeExpr.Rules, tools.RuleIn("device_type", batch))
	}
	expr.Rules = append(expr.Rules, deviceTypeExpr)
	if zone != "" {
		expr.Rules = append(expr.Rules, tools.RuleEqual("zone", zone))
	}
	satisfied := make(map[string]bool)
	listReq := &core.ListReq{Filter: expr, Page: core.NewDefaultBasePage()}
	for {
		resp, err := s.client.DataService().Global.DeviceCapacity.List(kt, listReq)
		if err != nil {
			logs.Errorf("list device capacity failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		for _, one := range resp.Details {
			if one.Capacity != nil && cvt.PtrToVal(one.Capacity) >= int64(applyNum) {
				satisfied[buildCapacityKey(one.RequireType, one.Region, one.DeviceType)] = true
			}
		}
		if len(resp.Details) < int(listReq.Page.Limit) {
			break
		}
		listReq.Page.Start += uint32(listReq.Page.Limit)
	}
	return satisfied, nil
}

// assembleStaticPlans 组装方案：每候选 1 个子单，按候选顺序（user 优先 biz、count 倒序）取前 N。
func assembleStaticPlans(candidates []*staticRecommendCandidate, req *woaserver.ApplyRecommendByStaticReq,
	applyNum int) []*woaserver.ApplyRecommendItem {

	zone := cvmapi.CvmZoneAll
	var resAssign *enumor.ResAssign
	if req.Zone != "" {
		// 指定具体 zone → res_assign 不适用(nil)
		zone = req.Zone
	} else {
		ra := enumor.ResPriorityResAssign
		if req.ResAssign != nil {
			ra = cvt.PtrToVal(req.ResAssign)
		}
		resAssign = &ra
	}
	items := make([]*woaserver.ApplyRecommendItem, 0, len(candidates))
	for _, c := range candidates {
		if len(items) >= req.Limit {
			break
		}
		items = append(items, &woaserver.ApplyRecommendItem{
			Source: c.source,
			Suborder: &woaserver.ApplyRecommendSuborder{
				RequireType: c.requireType,
				Region:      c.region,
				Zone:        zone,
				DeviceType:  c.deviceType,
				ImageID:     c.imageID,
				ResAssign:   resAssign,
				Replicas:    applyNum,
				ChargeType:  cvmapi.ChargeTypePrePaid,
				SystemDisk: enumor.DiskSpec{
					DiskType: enumor.DiskPremium, DiskSize: constant.RecommendSystemDiskSize,
					DiskNum: constant.RecommendDiskNum,
				},
				DataDisk: []enumor.DiskSpec{{
					DiskType: enumor.DiskPremium, DiskSize: constant.RecommendDataDiskSize,
					DiskNum: constant.RecommendDiskNum,
				}},
			},
		})
	}
	return items
}

// buildCapacityKey 构建库存校验的去重 key（不含镜像维度）。
func buildCapacityKey(requireType enumor.RequireType, region, deviceType string) string {
	return fmt.Sprintf("%d|%s|%s", requireType, region, deviceType)
}

// planRecommendCandidate 预测推荐候选（按预测内/外余量归一）。
type planRecommendCandidate struct {
	requireType enumor.RequireType
	region      string
	deviceType  string
	remainCore  int64
	chargeType  cvmapi.ChargeType
}

// GetBizApplyRecommendByPlan returns online recommendation plans built on forecast remain + stock.
func (s *service) GetBizApplyRecommendByPlan(cts *rest.Contexts) (interface{}, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		logs.Errorf("failed to get bk_biz_id from path, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	if bkBizID <= 0 {
		return nil, errf.New(errf.InvalidParameter, "bk_biz_id is invalid")
	}
	req := new(woaserver.ApplyRecommendByPlanReq)
	if err = cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err = req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access}, BizID: bkBizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		logs.Errorf("failed to authorize biz access, bizID: %d, err: %v, rid: %s", bkBizID, err, cts.Kit.Rid)
		return nil, err
	}

	requireType := enumor.RequireTypeRegular
	if req.RequireType != nil {
		requireType = cvt.PtrToVal(req.RequireType)
	}
	applyNum := cc.WoaServer().ApplyRecommend.DefaultApplyNum
	if req.Replicas != nil {
		applyNum = cvt.PtrToVal(req.Replicas)
	}

	candidates, err := s.listPlanCandidates(cts, bkBizID, requireType, req, applyNum)
	if err != nil {
		logs.Errorf("list plan recommend candidates failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	candidates, err = s.filterPlanCandidatesByCapacity(cts, candidates, applyNum, req.Zone)
	if err != nil {
		logs.Errorf("filter plan candidates by capacity failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	items, err := s.assemblePlanItems(cts, candidates, req, applyNum)
	if err != nil {
		logs.Errorf("assemble plan recommend items failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	return &woaserver.ApplyRecommendByStaticResp{Items: items}, nil
}

// listPlanCandidates 预测匹配：预测内优先，全部 region 预测内无候选则全局回退预测外；按余量门槛过滤。
func (s *service) listPlanCandidates(cts *rest.Contexts, bkBizID int64, requireType enumor.RequireType,
	req *woaserver.ApplyRecommendByPlanReq, applyNum int) ([]*planRecommendCandidate, error) {

	_, prodMaxAvailable, err := s.planLogics.GetProdResRemainPoolMatch(cts.Kit, bkBizID, requireType, "")
	if err != nil {
		logs.Errorf("get prod res remain pool match failed, err: %v, biz: %d, rid: %s", err, bkBizID, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}
	deviceTypeMap, err := s.planLogics.GetAllDeviceTypeMap(cts.Kit)
	if err != nil {
		logs.Errorf("get all device type map failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}
	regions := determinePlanRegions(req.Region, prodMaxAvailable)

	// 预测内优先（包年包月）
	candidates, err := s.collectPlanCandidates(cts, bkBizID, requireType, regions, enumor.PlanTypeCodeInPlan,
		cvmapi.ChargeTypePrePaid, prodMaxAvailable, deviceTypeMap, req.DeviceType, applyNum)
	if err != nil {
		return nil, err
	}
	if len(candidates) > 0 {
		return candidates, nil
	}
	// 全局回退预测外（按量计费）
	return s.collectPlanCandidates(cts, bkBizID, requireType, regions, enumor.PlanTypeCodeOutPlan,
		cvmapi.ChargeTypePostPaidByHour, prodMaxAvailable, deviceTypeMap, req.DeviceType, applyNum)
}

// determinePlanRegions 确定预测匹配的 region 集合：入参非空取该 region，否则取预测池内全部 region 去重。
func determinePlanRegions(region string, pool planlogics.ResPlanPoolMatch) []string {
	if region != "" {
		return []string{region}
	}
	seen := make(map[string]struct{})
	regions := make([]string, 0)
	for key := range pool {
		if key.RegionID == "" {
			continue
		}
		if _, ok := seen[key.RegionID]; ok {
			continue
		}
		seen[key.RegionID] = struct{}{}
		regions = append(regions, key.RegionID)
	}
	return regions
}

// collectPlanCandidates 按 planType 逐 region 收集可用机型候选，按机型过滤与余量门槛过滤。
func (s *service) collectPlanCandidates(cts *rest.Contexts, bkBizID int64, requireType enumor.RequireType,
	regions []string, planType enumor.PlanTypeCode, chargeType cvmapi.ChargeType, pool planlogics.ResPlanPoolMatch,
	deviceTypeMap map[string]dt.DistinctDeviceType, deviceTypeFilter string, applyNum int) (
	[]*planRecommendCandidate, error) {

	candidates := make([]*planRecommendCandidate, 0)
	for _, region := range regions {
		avlReq := &ptypes.GetCvmChargeTypeDeviceTypeReq{BkBizID: bkBizID, RequireType: requireType, Region: region}
		avlDeviceTypes, err := s.planLogics.GetPlanTypeAvlDeviceTypesV2(cts.Kit, planType, avlReq, pool)
		if err != nil {
			logs.Errorf("get plan type avl device types failed, err: %v, region: %s, rid: %s", err, region, cts.Kit.Rid)
			return nil, errf.NewFromErr(errf.Aborted, err)
		}
		for _, d := range avlDeviceTypes {
			if !d.Available {
				continue
			}
			if deviceTypeFilter != "" && d.DeviceType != deviceTypeFilter {
				continue
			}
			cpuCore := deviceTypeMap[d.DeviceType].CpuCore
			if cpuCore <= 0 || d.RemainCore < int64(applyNum)*cpuCore {
				continue
			}
			candidates = append(candidates, &planRecommendCandidate{
				requireType: requireType, region: region, deviceType: d.DeviceType,
				remainCore: d.RemainCore, chargeType: chargeType,
			})
		}
	}
	return candidates, nil
}

// filterPlanCandidatesByCapacity 库存校验：免校验需求类型直接保留，其余按静态表 capacity ≥ 申请数量过滤。
func (s *service) filterPlanCandidatesByCapacity(cts *rest.Contexts, candidates []*planRecommendCandidate,
	applyNum int, zone string) ([]*planRecommendCandidate, error) {

	needCheck := make([]capacityTriple, 0, len(candidates))
	for _, c := range candidates {
		if !c.requireType.NotNeedVerifyCapacity() {
			needCheck = append(needCheck, capacityTriple{c.requireType, c.region, c.deviceType})
		}
	}
	satisfied := make(map[string]bool)
	if len(needCheck) > 0 {
		var err error
		satisfied, err = s.queryCapacitySatisfied(cts.Kit, needCheck, applyNum, zone)
		if err != nil {
			return nil, err
		}
	}
	result := make([]*planRecommendCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.requireType.NotNeedVerifyCapacity() {
			result = append(result, c)
			continue
		}
		if satisfied[buildCapacityKey(c.requireType, c.region, c.deviceType)] {
			result = append(result, c)
		}
	}
	return result, nil
}

// assemblePlanItems 组装方案：按余量倒序、去重(region|device_type)、补镜像与默认值，取前 limit。
func (s *service) assemblePlanItems(cts *rest.Contexts, candidates []*planRecommendCandidate,
	req *woaserver.ApplyRecommendByPlanReq, applyNum int) ([]*woaserver.ApplyRecommendItem, error) {

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].remainCore > candidates[j].remainCore
	})

	zone, resAssign := resolvePlanZoneResAssign(req.Zone, req.ResAssign)
	seen := make(map[string]struct{})
	items := make([]*woaserver.ApplyRecommendItem, 0, req.Limit)
	for _, c := range candidates {
		if len(items) >= req.Limit {
			break
		}
		dedupKey := buildCapacityKey(c.requireType, c.region, c.deviceType)
		if _, ok := seen[dedupKey]; ok {
			continue
		}
		seen[dedupKey] = struct{}{}
		imageID, err := s.resolvePlanImageID(cts, c, req.ImageID)
		if err != nil {
			return nil, err
		}
		items = append(items, &woaserver.ApplyRecommendItem{
			Suborder: buildPlanSuborder(c, zone, resAssign, imageID, applyNum),
		})
	}
	return items, nil
}

// resolvePlanZoneResAssign 可用区与资源分配方式联动：未传 zone→全部(all)+res_assign；传了 zone→该 zone+不适用(nil)。
func resolvePlanZoneResAssign(reqZone string, reqResAssign *enumor.ResAssign) (string, *enumor.ResAssign) {
	if reqZone != "" {
		return reqZone, nil
	}
	ra := enumor.ResPriorityResAssign
	if reqResAssign != nil {
		ra = cvt.PtrToVal(reqResAssign)
	}
	return cvmapi.CvmZoneAll, &ra
}

// buildPlanSuborder 组装单个子单方案，计费模式随命中预测池来源。
func buildPlanSuborder(c *planRecommendCandidate, zone string, resAssign *enumor.ResAssign, imageID string,
	applyNum int) *woaserver.ApplyRecommendSuborder {

	return &woaserver.ApplyRecommendSuborder{
		RequireType: c.requireType,
		Region:      c.region,
		Zone:        zone,
		DeviceType:  c.deviceType,
		ImageID:     imageID,
		ResAssign:   resAssign,
		Replicas:    applyNum,
		ChargeType:  c.chargeType,
		SystemDisk: enumor.DiskSpec{
			DiskType: enumor.DiskPremium, DiskSize: constant.RecommendSystemDiskSize,
			DiskNum: constant.RecommendDiskNum,
		},
		DataDisk: []enumor.DiskSpec{{
			DiskType: enumor.DiskPremium, DiskSize: constant.RecommendDataDiskSize,
			DiskNum: constant.RecommendDiskNum,
		}},
	}
}

// resolvePlanImageID 镜像优先级：入参 → 历史子单回查 → 默认 DftImageID
func (s *service) resolvePlanImageID(cts *rest.Contexts, c *planRecommendCandidate, reqImageID string) (string, error) {
	if reqImageID != "" {
		return reqImageID, nil
	}
	imageID, err := s.querySuborderLatestImageID(cts, c.requireType, c.region, c.deviceType)
	if err != nil {
		return "", err
	}
	if imageID == "" {
		imageID = cvmapi.DftImageID
	}
	return imageID, nil
}

// querySuborderLatestImageID 回查历史子单按 (require_type,region,device_type) 最近一次非空 image_id。
func (s *service) querySuborderLatestImageID(cts *rest.Contexts, requireType enumor.RequireType,
	region, deviceType string) (string, error) {

	expr := tools.AllExpression()
	expr.Rules = append(expr.Rules,
		tools.RuleEqual("require_type", requireType),
		tools.RuleEqual("region", region),
		tools.RuleEqual("device_type", deviceType),
		tools.RuleNotEqual("image_id", ""),
	)
	listReq := &cvmapplyproto.ZiyanCvmApplySuborderListReq{
		Filter: expr,
		Page:   &core.BasePage{Start: 0, Limit: 1, Sort: "created_at", Order: core.Descending},
		Fields: []string{"image_id"},
	}
	resp, err := s.client.DataService().TCloudZiyan.ZiyanCvmApplySuborder.List(cts.Kit.Ctx, cts.Kit.Header(), listReq)
	if err != nil {
		logs.Errorf("list suborder for image id failed, err: %v, region: %s, deviceType: %s, rid: %s",
			err, region, deviceType, cts.Kit.Rid)
		return "", err
	}
	if len(resp.Details) == 0 {
		return "", nil
	}
	return resp.Details[0].ImageID, nil
}

// splitPlanKey 预测余量族级占用 key。
type splitPlanKey struct {
	region         string
	technicalClass string
	coreType       string
	planType       enumor.PlanTypeCode
}

// splitCapacityKey 库存占用 key。
type splitCapacityKey struct {
	requireType enumor.RequireType
	region      string
	deviceType  string
	zone        string
}

// GetBizApplyRecommendSplitSubOrder splits a confirmed plan into a main order draft with suborders.
func (s *service) GetBizApplyRecommendSplitSubOrder(cts *rest.Contexts) (interface{}, error) {
	bkBizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		logs.Errorf("failed to get bk_biz_id from path, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	if bkBizID <= 0 {
		return nil, errf.New(errf.InvalidParameter, "bk_biz_id is invalid")
	}
	req := new(woaserver.ApplyRecommendSplitSubOrderReq)
	if err = cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err = req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access}, BizID: bkBizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		logs.Errorf("failed to authorize biz access, bizID: %d, err: %v, rid: %s", bkBizID, err, cts.Kit.Rid)
		return nil, err
	}

	suborders, err := s.splitApplyOrder(cts.Kit, bkBizID, req)
	if err != nil {
		logs.Errorf("split apply order failed, err: %v, biz: %d, rid: %s", err, bkBizID, cts.Kit.Rid)
		return nil, err
	}
	return &woaserver.ApplyRecommendSplitSubOrderResp{Suborders: suborders}, nil
}

// splitApplyOrder 串联：占用预处理 → 预测余量 → 库存封顶 → 数量分配 → 组装子单。
func (s *service) splitApplyOrder(kt *kit.Kit, bkBizID int64, req *woaserver.ApplyRecommendSplitSubOrderReq) (
	[]*woaserver.ApplyRecommendSuborder, error) {

	deviceTypeMap, err := s.planLogics.GetAllDeviceTypeMap(kt)
	if err != nil {
		logs.Errorf("get all device type map failed, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}
	cpuCore := deviceTypeMap[req.DeviceType].CpuCore
	if cpuCore <= 0 {
		logs.Warnf("device type %s cpu core invalid, return empty, rid: %s", req.DeviceType, kt.Rid)
		return []*woaserver.ApplyRecommendSuborder{}, nil
	}

	occupiedCore, err := buildOccupiedCore(kt, deviceTypeMap, req.RequireType, req.OccupiedSuborders)
	if err != nil {
		logs.Errorf("build occupied core failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	availInPlan, availOutPlan, err := s.computePlanAvailable(kt, bkBizID, req, deviceTypeMap, cpuCore, occupiedCore)
	if err != nil {
		return nil, err
	}

	occupiedCapacity := buildOccupiedCapacity(req.OccupiedSuborders)
	availCapacity, skipCapacity, err := s.computeAvailableCapacity(kt, req, occupiedCapacity)
	if err != nil {
		return nil, err
	}

	inPlan, outPlan := allocateSplit(int64(req.Replicas), availInPlan, availOutPlan, availCapacity, skipCapacity)
	return assembleSplitSuborders(req, inPlan, outPlan), nil
}

// buildOccupiedCore 构建族级预测占用映射；裁撤合并到同一族级 key。
func buildOccupiedCore(kt *kit.Kit, deviceTypeMap map[string]dt.DistinctDeviceType, requireType enumor.RequireType,
	occupied []*woaserver.ApplyRecommendSuborder) (map[splitPlanKey]int64, error) {

	result := make(map[splitPlanKey]int64)
	for _, sub := range occupied {
		info, ok := deviceTypeMap[sub.DeviceType]
		if !ok || info.CpuCore <= 0 {
			logs.Errorf("device type is invalid: %s, rid: %s", sub.DeviceType, kt.Rid)
			return nil, fmt.Errorf("device type %s is invalid", sub.DeviceType)
		}
		key := splitPlanKey{
			region:         sub.Region,
			technicalClass: info.TechnicalClass,
			coreType:       string(info.CoreType),
			planType:       normalizePlanType(requireType, sub.ChargeType),
		}
		result[key] += int64(sub.Replicas) * info.CpuCore
	}
	return result, nil
}

// buildOccupiedCapacity 构建机型级库存占用映射（按 require_type|region|device_type|zone 聚合台数）。
func buildOccupiedCapacity(occupied []*woaserver.ApplyRecommendSuborder) map[splitCapacityKey]int64 {
	result := make(map[splitCapacityKey]int64)
	for _, sub := range occupied {
		key := splitCapacityKey{
			requireType: sub.RequireType,
			region:      sub.Region,
			deviceType:  sub.DeviceType,
			zone:        sub.Zone,
		}
		result[key] += int64(sub.Replicas)
	}
	return result
}

// normalizePlanType 由计费模式映射预测内/外；裁撤忽略内外、统一归到 Ignore 单池 key。
func normalizePlanType(requireType enumor.RequireType, chargeType cvmapi.ChargeType) enumor.PlanTypeCode {
	// 机房裁撤需要忽略预测内、预测外 --story=121848852
	if requireType == enumor.RequireTypeDissolve {
		return enumor.PlanTypeCodeIgnore
	}
	return chargeType.ToPlanType()
}

// computePlanAvailable 计算预测内/外可满足台数（已扣族级占用）；非预测类型 availInPlan=count。
func (s *service) computePlanAvailable(kt *kit.Kit, bkBizID int64, req *woaserver.ApplyRecommendSplitSubOrderReq,
	deviceTypeMap map[string]dt.DistinctDeviceType, cpuCore int64, occupiedCore map[splitPlanKey]int64) (
	int64, int64, error) {

	if !req.RequireType.NeedVerifyResPlan() {
		return int64(req.Replicas), 0, nil
	}

	_, prodMaxAvailable, err := s.planLogics.GetProdResRemainPoolMatch(kt, bkBizID, req.RequireType, "")
	if err != nil {
		logs.Errorf("get prod res remain pool match failed, err: %v, biz: %d, rid: %s", err, bkBizID, kt.Rid)
		return 0, 0, errf.NewFromErr(errf.Aborted, err)
	}
	info := deviceTypeMap[req.DeviceType]

	// 机房裁撤：预测内外合并为单池（PlanTypeCodeIgnore），只算一次。
	if req.RequireType == enumor.RequireTypeDissolve {
		avail, err := s.computePlanTypeAvailable(kt, bkBizID, req, info, cpuCore, enumor.PlanTypeCodeIgnore,
			prodMaxAvailable, occupiedCore)
		if err != nil {
			logs.Errorf("compute plan type available failed, err: %v, biz: %d, rid: %s", err, bkBizID, kt.Rid)
			return 0, 0, err
		}
		return avail, 0, nil
	}

	availInPlan, err := s.computePlanTypeAvailable(kt, bkBizID, req, info, cpuCore, enumor.PlanTypeCodeInPlan,
		prodMaxAvailable, occupiedCore)
	if err != nil {
		logs.Errorf("compute plan type available failed, err: %v, biz: %d, rid: %s", err, bkBizID, kt.Rid)
		return 0, 0, err
	}
	availOutPlan, err := s.computePlanTypeAvailable(kt, bkBizID, req, info, cpuCore, enumor.PlanTypeCodeOutPlan,
		prodMaxAvailable, occupiedCore)
	if err != nil {
		logs.Errorf("compute plan type available failed, err: %v, biz: %d, rid: %s", err, bkBizID, kt.Rid)
		return 0, 0, err
	}
	return availInPlan, availOutPlan, nil
}

// computePlanTypeAvailable 取指定预测池下本机型可满足台数（已扣同族级占用）。
func (s *service) computePlanTypeAvailable(kt *kit.Kit, bkBizID int64,
	req *woaserver.ApplyRecommendSplitSubOrderReq, info dt.DistinctDeviceType, cpuCore int64,
	planType enumor.PlanTypeCode, pool planlogics.ResPlanPoolMatch, occupiedCore map[splitPlanKey]int64) (
	int64, error) {

	remainCore, err := s.getDeviceRemainCore(kt, bkBizID, req, planType, pool)
	if err != nil {
		logs.Errorf("get device remain core failed, err: %v, planType: %s, rid: %s", err, planType, kt.Rid)
		return 0, err
	}
	group := splitPlanKey{req.Region, info.TechnicalClass, string(info.CoreType), planType}
	return max(remainCore-occupiedCore[group], 0) / cpuCore, nil
}

// getDeviceRemainCore 取本机型在指定预测池的余量核数；不可用或未命中返回 0。
func (s *service) getDeviceRemainCore(kt *kit.Kit, bkBizID int64, req *woaserver.ApplyRecommendSplitSubOrderReq,
	planType enumor.PlanTypeCode, pool planlogics.ResPlanPoolMatch) (int64, error) {

	avlReq := &ptypes.GetCvmChargeTypeDeviceTypeReq{BkBizID: bkBizID, RequireType: req.RequireType, Region: req.Region}
	avlDeviceTypes, err := s.planLogics.GetPlanTypeAvlDeviceTypesV2(kt, planType, avlReq, pool)
	if err != nil {
		logs.Errorf("get plan type avl device types failed, err: %v, planType: %s, rid: %s",
			err, planType, kt.Rid)
		return 0, errf.NewFromErr(errf.Aborted, err)
	}
	for _, d := range avlDeviceTypes {
		if d.Available && d.DeviceType == req.DeviceType {
			return d.RemainCore, nil
		}
	}
	return 0, nil
}

// computeAvailableCapacity 取可用库存（库存上限已扣机型级占用）；绿通跳过库存校验返回 skip=true。
//
// 占用扣减模型：zone=all 的占用视为浮动占用（任何 zone 都可能落，统一扣）；具体 zone 的占用只扣到对应 zone。
//   - req 指定具体 zone Z：可用 = capacity(Z) − 占用(Z) − 浮动占用(all)
//   - req=all：各 zone 扣本 zone 具体占用并截断 0 后求和，再扣浮动占用(all)
func (s *service) computeAvailableCapacity(kt *kit.Kit, req *woaserver.ApplyRecommendSplitSubOrderReq,
	occupiedCapacity map[splitCapacityKey]int64) (int64, bool, error) {

	if req.RequireType.NotNeedVerifyCapacity() {
		return 0, true, nil
	}
	capMap, err := s.querySplitCapacityLimit(kt, req.RequireType, req.Region, req.DeviceType, req.Zone)
	if err != nil {
		return 0, false, err
	}
	occupiedAll := occupiedCapacity[buildSplitCapacityKey(req, cvmapi.CvmZoneAll)]

	if req.Zone != cvmapi.CvmZoneAll {
		avail := capMap[req.Zone] - occupiedCapacity[buildSplitCapacityKey(req, req.Zone)] - occupiedAll
		return max(avail, 0), false, nil
	}

	// zone=all 不钉死可用区，可跨 zone 分摊下单，故取各 zone 有效容量之和；
	// 每个 zone 先扣本 zone 具体占用并截断 0，避免某 zone 超额占用透支其他 zone。
	var totalAvail int64
	for zone, capVal := range capMap {
		zoneAvail := capVal - occupiedCapacity[buildSplitCapacityKey(req, zone)]
		totalAvail += max(zoneAvail, 0)
	}
	return max(totalAvail-occupiedAll, 0), false, nil
}

// buildSplitCapacityKey 以指定 zone 构造库存占用 key（require_type/region/device_type 取自 req）。
func buildSplitCapacityKey(req *woaserver.ApplyRecommendSplitSubOrderReq, zone string) splitCapacityKey {
	return splitCapacityKey{
		requireType: req.RequireType,
		region:      req.Region,
		deviceType:  req.DeviceType,
		zone:        zone,
	}
}

// querySplitCapacityLimit 查 device_capacity 返回各 zone 容量上限：zone=all 返回 region 下所有 zone，具体 zone 仅返回该 zone。
func (s *service) querySplitCapacityLimit(kt *kit.Kit, requireType enumor.RequireType,
	region, deviceType, zone string) (map[string]int64, error) {

	expr := tools.AllExpression()
	expr.Rules = append(expr.Rules,
		tools.RuleEqual("require_type", requireType),
		tools.RuleEqual("region", region),
		tools.RuleEqual("device_type", deviceType),
	)
	if zone != cvmapi.CvmZoneAll {
		expr.Rules = append(expr.Rules, tools.RuleEqual("zone", zone))
	}
	capMap := make(map[string]int64)
	listReq := &core.ListReq{Filter: expr, Page: core.NewDefaultBasePage()}
	for {
		resp, err := s.client.DataService().Global.DeviceCapacity.List(kt, listReq)
		if err != nil {
			logs.Errorf("list device capacity failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		for _, one := range resp.Details {
			if c := cvt.PtrToVal(one.Capacity); c > capMap[one.Zone] {
				capMap[one.Zone] = c
			}
		}
		if len(resp.Details) < int(listReq.Page.Limit) {
			break
		}
		listReq.Page.Start += uint32(listReq.Page.Limit)
	}
	return capMap, nil
}

// allocateSplit 数量分配：先按预测内→外分配，再用库存上限按内→外顺序对总量封顶。
func allocateSplit(count, availInPlan, availOutPlan, availCapacity int64, skipCapacity bool) (int64, int64) {
	inPlan := min(count, availInPlan)
	outPlan := min(count-inPlan, availOutPlan)
	if skipCapacity {
		return inPlan, outPlan
	}
	if inPlan+outPlan <= availCapacity {
		return inPlan, outPlan
	}
	if availCapacity <= inPlan {
		return availCapacity, 0
	}
	return inPlan, availCapacity - inPlan
}

func assembleSplitSuborders(req *woaserver.ApplyRecommendSplitSubOrderReq,
	inPlan, outPlan int64) []*woaserver.ApplyRecommendSuborder {

	suborders := make([]*woaserver.ApplyRecommendSuborder, 0, 2)
	if inPlan > 0 {
		suborders = append(suborders, buildSplitSuborder(req, cvmapi.ChargeTypePrePaid, int(inPlan)))
	}
	if outPlan > 0 {
		suborders = append(suborders, buildSplitSuborder(req, cvmapi.ChargeTypePostPaidByHour, int(outPlan)))
	}
	return suborders
}

// buildSplitSuborder 按入参原样透传组装单个子单，仅计费模式与申请数量由拆分推导。
func buildSplitSuborder(req *woaserver.ApplyRecommendSplitSubOrderReq, chargeType cvmapi.ChargeType,
	applyNum int) *woaserver.ApplyRecommendSuborder {

	resAssign := req.ResAssign
	return &woaserver.ApplyRecommendSuborder{
		RequireType: req.RequireType,
		Region:      req.Region,
		Zone:        req.Zone,
		DeviceType:  req.DeviceType,
		ImageID:     req.ImageID,
		ResAssign:   &resAssign,
		Replicas:    applyNum,
		ChargeType:  chargeType,
		SystemDisk:  req.SystemDisk,
		DataDisk:    req.DataDisk,
	}
}
