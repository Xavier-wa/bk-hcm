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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	ptypes "hcm/cmd/woa-server/types/plan"
	tasktypes "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	dt "hcm/pkg/api/core/cloud/device-type"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	rpproto "hcm/pkg/api/data-service/resource-plan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	rpd "hcm/pkg/dal/table/resource-plan/res-plan-demand"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/thirdparty/api-gateway/bkbase"
	"hcm/pkg/thirdparty/api-gateway/finops"
	"hcm/pkg/thirdparty/cvmapi"
	"hcm/pkg/tools/concurrence"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/maps"
	"hcm/pkg/tools/slice"
	"hcm/pkg/tools/times"

	"github.com/shopspring/decimal"
)

// ListResPlanDemandAndOverview list res plan demand and overview.
func (c *Controller) ListResPlanDemandAndOverview(kt *kit.Kit, req *ptypes.ListResPlanDemandReq) (
	*ptypes.ListResPlanDemandResp, error) {

	// 需要将期望到货时间扩展至整个需求周期月，查询全部的预测并分配消耗情况后，再进行筛选。以确保无论筛选条件如何，每条预测消耗量的一致
	listAllReq, err := c.extendResPlanListReq(kt, req)
	if err != nil {
		logs.Errorf("failed to convert list request to list all request, err: %v, req: %+v, rid: %s", err, *req,
			kt.Rid)
		return nil, err
	}

	// 获取demand列表
	demandList, _, err := c.listAllResPlanDemand(kt, listAllReq)
	if err != nil {
		logs.Errorf("failed to list res plan demand, err: %v, req: %+v, rid: %s", err, *listAllReq, kt.Rid)
		return nil, err
	}

	bkBizIDs := make([]int64, 0)
	for _, demand := range demandList {
		bkBizIDs = append(bkBizIDs, demand.BkBizID)
	}

	// 获取当月预测消耗历史，聚合为 ResPlanConsumePool
	startDay, endDay, err := listAllReq.ExpectTimeRange.GetTimeDate()
	if err != nil {
		logs.Errorf("failed to parse date range, err: %v, date range: %s - %s, rid: %s", err, req.ExpectTimeRange.Start,
			req.ExpectTimeRange.End, kt.Rid)
		return nil, err
	}
	prodConsumePool, err := c.GetProdResConsumePoolV2(kt, bkBizIDs, startDay, endDay)
	if err != nil {
		logs.Errorf("failed to get biz resource consume pool v2, bkBizIDs: %v, err: %v, rid: %s", bkBizIDs, err, kt.Rid)
		return nil, err
	}

	// 获取各个机型的核心数
	deviceTypeMap, err := c.deviceTypesMap.GetDeviceTypes(kt)
	if err != nil {
		logs.Errorf("get device type map failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	planAppliedCore := convResConsumePoolToExpendMap(kt, prodConsumePool, deviceTypeMap)

	// 清洗数据，计算overview
	overview, rst, err := c.convResPlanDemandRespAndFilter(kt, req, demandList, planAppliedCore, deviceTypeMap)
	if err != nil {
		logs.Errorf("failed to convert res plan demand table to response item, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 只返回总数，不返回详情及统计
	if req.Page.Count {
		return &ptypes.ListResPlanDemandResp{
			Count: uint64(len(rst)),
		}, nil
	}

	// planAppliedCore 未用尽，给出警告
	for key, appliedCPUCore := range planAppliedCore {
		if appliedCPUCore > 0 {
			logs.Warnf("plan applied core have not been used up, applied_key: %+v, remained_cpu_core: %d, rid: %s",
				key, appliedCPUCore, kt.Rid)
		}
	}

	// 数据分页
	// TODO 注意如果后续支持排序，不能在dao层直接排序，会影响消耗记录的对应关系，需要在这里额外进行
	rst = pageResPlanDemands(req.Page, rst)
	return &ptypes.ListResPlanDemandResp{
		Overview: overview,
		Details:  rst,
	}, nil
}

func pageResPlanDemands(page *core.BasePage, demands []*ptypes.ListResPlanDemandItem) []*ptypes.ListResPlanDemandItem {
	if page.Start >= uint32(len(demands)) {
		return []*ptypes.ListResPlanDemandItem{}
	}

	offset := int(page.Start + uint32(page.Limit))
	if offset > len(demands) {
		offset = len(demands)
	}
	return demands[int(page.Start):offset]
}

// extendResPlanListReq to ensure that res plan data and expend data can be aligned.
// extend expect_time to the entire demand month and remove other request params outside bk_biz.
func (c *Controller) extendResPlanListReq(kt *kit.Kit, req *ptypes.ListResPlanDemandReq) (*ptypes.ListResPlanDemandReq,
	error) {

	startT, endT, err := req.ExpectTimeRange.GetTimeDate()
	if err != nil {
		logs.Errorf("failed to parse date range, err: %v, date range: %s - %s, rid: %s", err,
			req.ExpectTimeRange.Start, req.ExpectTimeRange.End, kt.Rid)
		return nil, err
	}

	startDemandTimeRange, err := c.demandTime.GetDemandDateRangeInMonth(kt, startT)
	if err != nil {
		logs.Errorf("failed to get start demand date range in month, err: %v, start: %s, rid: %s", err, startT,
			kt.Rid)
		return nil, err
	}
	endDemandTimeRange, err := c.demandTime.GetDemandDateRangeInMonth(kt, endT)
	if err != nil {
		logs.Errorf("failed to get end demand date range in month, err: %v, end: %s, rid: %s", err, endT,
			kt.Rid)
		return nil, err
	}

	return &ptypes.ListResPlanDemandReq{
		BkBizIDs:       req.BkBizIDs,
		OpProductIDs:   req.OpProductIDs,
		PlanProductIDs: req.PlanProductIDs,
		CoreTypes:      req.CoreTypes,
		DeviceFamilies: req.DeviceFamilies,
		ExpectTimeRange: &times.DateRange{
			Start: startDemandTimeRange.Start,
			End:   endDemandTimeRange.End,
		},
		Page: core.NewDefaultBasePage(),
	}, nil
}

// convResConsumePoolToExpendMap 将 ResConsumePool 转为以 ResPlanDemandExpendKey 为 key 的 map
// 因为 ResConsumePool 精确指定了deviceType，因此在list时无法进行模糊匹配，需要进行转化后使用
func convResConsumePoolToExpendMap(kt *kit.Kit, pool ResPlanConsumePool,
	deviceTypes map[string]dt.DistinctDeviceType) map[ptypes.ResPlanDemandExpendKey]int64 {

	consumeMap := make(map[ptypes.ResPlanDemandExpendKey]int64)

	for key, cpuCore := range pool {
		if _, ok := deviceTypes[key.DeviceType]; !ok {
			logs.Warnf("device type %s not found, rid: %s", key.DeviceType, kt.Rid)
			continue
		}

		expendKey := ptypes.ResPlanDemandExpendKey{
			DemandClass:   key.DemandClass,
			BkBizID:       key.BkBizID,
			PlanType:      key.PlanType,
			AvailableTime: ptypes.AvailableMonth(key.AvailableTime),
			DeviceFamily:  deviceTypes[key.DeviceType].DeviceFamily,
			CoreType:      string(deviceTypes[key.DeviceType].CoreType),
			ObsProject:    key.ObsProject,
			RegionID:      key.RegionID,
		}

		consumeMap[expendKey] += cpuCore
	}

	return consumeMap
}

func (c *Controller) demandBelongListReq(kt *kit.Kit, demandItem *ptypes.ListResPlanDemandItem,
	req *ptypes.ListResPlanDemandReq) (bool, error) {

	if !req.CheckDemandIDs(demandItem.DemandID) {
		return false, nil
	}
	if !req.CheckObsProjects(demandItem.ObsProject) {
		return false, nil
	}
	if !req.CheckDemandClasses(demandItem.DemandClass) {
		return false, nil
	}
	if !req.CheckDeviceClasses(demandItem.DeviceClass) {
		return false, nil
	}
	if !req.CheckDeviceTypes(demandItem.DeviceType) {
		return false, nil
	}
	if !req.CheckRegionIDs(demandItem.RegionID) {
		return false, nil
	}
	if !req.CheckZoneIDs(demandItem.ZoneID) {
		return false, nil
	}
	if !req.CheckPlanTypes(demandItem.PlanType) {
		return false, nil
	}

	// 筛选本月到期，即期望交付时间在本月内的
	if req.ExpiringOnly {
		monthRange, err := c.demandTime.GetDemandDateRangeInMonth(kt, time.Now())
		if err != nil {
			logs.Errorf("failed to get demand date range in month, err: %v, rid: %s", err, kt.Rid)
			return false, err
		}
		if demandItem.ExpectTime < monthRange.Start || demandItem.ExpectTime > monthRange.End {
			return false, nil
		}
		// 筛选本月即将到期的需求时，不展示已经耗尽的需求
		if demandItem.AppliedCpuCore == demandItem.TotalCpuCore {
			return false, nil
		}
	}
	if req.ExpectTimeRange != nil {
		if demandItem.ExpectTime < req.ExpectTimeRange.Start || demandItem.ExpectTime > req.ExpectTimeRange.End {
			return false, nil
		}
	}

	return true, nil
}

// convResPlanDemandRespAndFilter convert res plan demand table to res plan demand response item,
// and filter by request params.
func (c *Controller) convResPlanDemandRespAndFilter(kt *kit.Kit, req *ptypes.ListResPlanDemandReq,
	planTables []rpd.ResPlanDemandTable, planAppliedCore map[ptypes.ResPlanDemandExpendKey]int64,
	deviceTypes map[string]dt.DistinctDeviceType) (*ptypes.ListResPlanDemandOverview, []*ptypes.ListResPlanDemandItem,
	error) {

	overview := &ptypes.ListResPlanDemandOverview{}
	demandDetails := make([]*ptypes.ListResPlanDemandItem, 0, len(planTables))

	for _, demand := range planTables {
		expectDateStr, err := times.TransTimeStrWithLayout(strconv.Itoa(demand.ExpectTime), constant.DateLayoutCompact,
			constant.DateLayout)
		if err != nil {
			logs.Errorf("failed to parse demand expect time, err: %v, expect_time: %d, rid: %s", err,
				demand.ExpectTime, kt.Rid)
			return nil, nil, err
		}
		// 短租项目需要提供预期退回时间字段
		returnTimePtr := new(string)
		if demand.ObsProject == enumor.ObsProjectShortLease {
			returnTime, err := times.TransTimeStrWithLayout(strconv.Itoa(demand.ReturnPlanTime),
				constant.DateLayoutCompact, constant.DateLayout)
			if err != nil {
				logs.Warnf("failed to parse demand return plan time, err: %v, return_plan_time: %d, rid: %s", err,
					demand.ReturnPlanTime, kt.Rid)
			} else {
				returnTimePtr = &returnTime
			}
		}
		demandItem := convListResPlanDemandItemByTable(demand, expectDateStr, returnTimePtr)
		demandKey, err := c.getDemandExpendKeyFromTable(kt, demand, expectDateStr, deviceTypes)
		if err != nil {
			logs.Errorf("failed to get demand expend key, err: %v, demand id: %s, rid: %s", err, demand.ID,
				kt.Rid)
			return nil, nil, err
		}

		// 对一个预测，如果未通过固定的planType消耗完，尝试匹配不带planType的申领记录（目前主要是升降配类型）
		matchPlanType := []enumor.PlanTypeCode{demandKey.PlanType, enumor.PlanTypeCodeIgnore}
		for _, planType := range matchPlanType {
			demandKey.PlanType = planType
			// 计算已消耗/剩余的核心数和实例数
			if allAppliedCpuCore, ok := planAppliedCore[demandKey]; ok {
				demandAppliedCpuCore := min(allAppliedCpuCore, demandItem.RemainedCpuCore)
				demandItem.AppliedCpuCore += demandAppliedCpuCore
				planAppliedCore[demandKey] -= demandAppliedCpuCore

				deviceCpuCore := decimal.NewFromInt(deviceTypes[demandItem.DeviceType].CpuCore)
				deviceMemory := decimal.NewFromInt(deviceTypes[demandItem.DeviceType].Memory)
				demandItem.AppliedOS = decimal.NewFromInt(demandItem.AppliedCpuCore).Div(deviceCpuCore)
				demandItem.AppliedMemory = demandItem.AppliedOS.Mul(deviceMemory).IntPart()
			}
			demandItem.RemainedOS = demandItem.TotalOS.Sub(demandItem.AppliedOS)
			demandItem.RemainedCpuCore = demandItem.TotalCpuCore - demandItem.AppliedCpuCore
			demandItem.RemainedMemory = demandItem.TotalMemory - demandItem.AppliedMemory
			if demandItem.RemainedCpuCore <= 0 {
				break
			}
		}

		// 不在筛选范围内的，过滤
		belong, err := c.demandBelongListReq(kt, demandItem, req)
		if err != nil {
			logs.Errorf("failed to check demand belong, err: %v, demand: %s, rid: %s", err, *demandItem, kt.Rid)
			return nil, nil, err
		}
		if !belong {
			continue
		}

		// 更新demand状态并过滤状态的查询条件
		demandItem = c.setDemandStatus(kt, demand.ID, cvt.PtrToVal(demand.Locked), demandItem)
		if len(req.Statuses) > 0 && !slices.Contains(req.Statuses, demandItem.Status) {
			continue
		}

		// 计算overview
		calcDemandListOverview(overview, demandItem, demand.PlanType)
		demandDetails = append(demandDetails, demandItem)
	}
	return overview, demandDetails, nil
}

func (c *Controller) setDemandStatus(kt *kit.Kit, demandID string, demandLockedStatus enumor.CrpDemandLockStatus,
	demandItem *ptypes.ListResPlanDemandItem) *ptypes.ListResPlanDemandItem {

	// 计算demand状态，can_apply（可申领）、not_ready（未到申领时间）、expired（已过期）
	status, demandRange, err := c.demandTime.GetDemandStatusByExpectTime(kt, demandItem.ExpectTime)
	if err != nil {
		logs.Warnf("failed to get demand status, err: %v, demand_id: %s, rid: %s", err, demandID, kt.Rid)
	} else {
		demandItem.SetStatus(status)
		demandItem.CanApplyTime = demandRange.Start // 可申领时间
		demandItem.ExpiredTime = demandRange.End    // 截止申领时间
	}

	// 目前即将过期核心数的逻辑等于可申领数（当月申领、当月过期）
	if status == enumor.DemandStatusCanApply {
		demandItem.ExpiringCpuCore = demandItem.RemainedCpuCore
	}
	if demandLockedStatus == enumor.CrpDemandLocked {
		demandItem.Status = enumor.DemandStatusLocked
	}
	demandItem.StatusName = demandItem.Status.Name()

	return demandItem
}

func calcDemandListOverview(overview *ptypes.ListResPlanDemandOverview, demandItem *ptypes.ListResPlanDemandItem,
	planType enumor.PlanTypeCode) {

	overview.TotalCpuCore += demandItem.TotalCpuCore
	overview.TotalAppliedCore += demandItem.AppliedCpuCore
	overview.ExpiringCpuCore += demandItem.ExpiringCpuCore
	if planType.InPlan() {
		overview.InPlanCpuCore += demandItem.TotalCpuCore
		overview.InPlanAppliedCpuCore += demandItem.AppliedCpuCore
	} else {
		overview.OutPlanCpuCore += demandItem.TotalCpuCore
		overview.OutPlanAppliedCpuCore += demandItem.AppliedCpuCore
	}
}

func (c *Controller) getDemandExpendKeyFromTable(kt *kit.Kit, demand rpd.ResPlanDemandTable, expectTime string,
	deviceTypeMap map[string]dt.DistinctDeviceType) (ptypes.ResPlanDemandExpendKey, error) {

	t, err := time.Parse(constant.DateLayout, expectTime)
	if err != nil {
		logs.Errorf("failed to parse demand expect time, err: %v, expect_time: %s, rid: %s", err,
			demand.ExpectTime, kt.Rid)
		return ptypes.ResPlanDemandExpendKey{}, err
	}

	availableYear, availableMonth, err := c.demandTime.GetDemandYearMonth(kt, t)
	if err != nil {
		logs.Errorf("failed to parse demand available time, err: %v, expect_time: %s, rid: %s", err,
			demand.ExpectTime, kt.Rid)
		return ptypes.ResPlanDemandExpendKey{}, err
	}

	resPlanDemandExpendKey := ptypes.ResPlanDemandExpendKey{
		DemandClass:   demand.DemandClass,
		BkBizID:       demand.BkBizID,
		PlanType:      demand.PlanType,
		AvailableTime: ptypes.NewAvailableMonth(availableYear, availableMonth),
		DeviceFamily:  deviceTypeMap[demand.DeviceType].DeviceFamily,
		CoreType:      string(deviceTypeMap[demand.DeviceType].CoreType),
		ObsProject:    demand.ObsProject,
		RegionID:      demand.RegionID,
	}
	// TODO
	// 机房裁撤需要忽略预测内、预测外 --story=121848852
	if enumor.IsDissolveObsProjectForResPlan(demand.ObsProject) {
		resPlanDemandExpendKey.PlanType = ""
	}

	return resPlanDemandExpendKey, nil
}

func convListResPlanDemandItemByTable(table rpd.ResPlanDemandTable, expectTime string,
	returnPlanTime *string) *ptypes.ListResPlanDemandItem {

	return &ptypes.ListResPlanDemandItem{
		DemandID:         table.ID,
		BkBizID:          table.BkBizID,
		BkBizName:        table.BkBizName,
		OpProductID:      table.OpProductID,
		OpProductName:    table.OpProductName,
		PlanProductID:    table.PlanProductID,
		PlanProductName:  table.PlanProductName,
		DemandClass:      table.DemandClass,
		DemandResType:    table.DemandResType,
		ExpectTime:       expectTime,
		ReturnPlanTime:   returnPlanTime,
		DeviceClass:      table.DeviceClass,
		DeviceType:       table.DeviceType,
		TotalOS:          table.OS.Decimal,
		AppliedOS:        decimal.NewFromInt(0),
		RemainedOS:       table.OS.Decimal,
		TotalCpuCore:     cvt.PtrToVal(table.CpuCore),
		AppliedCpuCore:   0,
		RemainedCpuCore:  cvt.PtrToVal(table.CpuCore),
		TotalMemory:      cvt.PtrToVal(table.Memory),
		TotalDiskSize:    cvt.PtrToVal(table.DiskSize),
		RemainedDiskSize: cvt.PtrToVal(table.DiskSize),
		RegionID:         table.RegionID,
		RegionName:       table.RegionName,
		ZoneID:           table.ZoneID,
		ZoneName:         table.ZoneName,
		AreaName:         table.AreaName,
		ResMode:          table.ResMode,
		PlanType:         table.PlanType.Name(),
		ObsProject:       table.ObsProject,
		TechnicalClass:   table.TechnicalClass,
		TicketID:         table.TicketID,
		DeviceFamily:     table.DeviceFamily,
		CoreType:         table.CoreType,
		DiskType:         table.DiskType,
		DiskTypeName:     table.DiskType.Name(),
		DiskIO:           table.DiskIO,
		Creator:          table.Creator,
		Reviser:          table.Reviser,
	}
}

// GetResPlanDemandDetail get demand detail
func (c *Controller) GetResPlanDemandDetail(kt *kit.Kit, demandID string, bkBizIDs []int64) (
	*ptypes.GetPlanDemandDetailResp, error) {
	return c.resFetcher.GetResPlanDemandDetail(kt, demandID, bkBizIDs)
}

func (c *Controller) convListResPlanDemandTimeFilter(kt *kit.Kit, expiringOnly bool, expectTimeRange *times.DateRange) (
	[]*filter.AtomRule, error) {

	listRules := make([]*filter.AtomRule, 0)

	// 筛选本月到期，即期望交付时间在本月内的
	if expiringOnly {
		monthRange, err := c.demandTime.GetDemandDateRangeInMonth(kt, time.Now())
		if err != nil {
			logs.Errorf("failed to get demand date range in month, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		startExpTime, err := times.ConvStrTimeToInt(monthRange.Start, constant.DateLayout)
		if err != nil {
			logs.Errorf("failed to parse month range, err: %v, month_range: %v, rid: %s", err, monthRange, kt.Rid)
			return nil, err
		}
		endExpTime, err := times.ConvStrTimeToInt(monthRange.End, constant.DateLayout)
		if err != nil {
			logs.Errorf("failed to parse month range, err: %v, month_range: %v, rid: %s", err, monthRange, kt.Rid)
			return nil, err
		}
		listRules = append(listRules, tools.RuleGreaterThanEqual("expect_time", startExpTime))
		listRules = append(listRules, tools.RuleLessThanEqual("expect_time", endExpTime))
	}
	if expectTimeRange != nil {
		startExpTime, err := times.ConvStrTimeToInt(expectTimeRange.Start, constant.DateLayout)
		if err != nil {
			logs.Errorf("failed to parse start expect time, err: %v, range_start: %s, rid: %s", err,
				expectTimeRange.Start, kt.Rid)
			return nil, err
		}
		endExpTime, err := times.ConvStrTimeToInt(expectTimeRange.End, constant.DateLayout)
		if err != nil {
			logs.Errorf("failed to parse end expect time, err: %v, range_end: %s, rid: %s", err,
				expectTimeRange.End, kt.Rid)
			return nil, err
		}
		listRules = append(listRules, tools.RuleGreaterThanEqual("expect_time", startExpTime))
		listRules = append(listRules, tools.RuleLessThanEqual("expect_time", endExpTime))
	}

	return listRules, nil
}

func (c *Controller) convAllResPlanDemandListOpt(kt *kit.Kit, req *ptypes.ListResPlanDemandReq) ([]*filter.AtomRule,
	error) {

	listRules := make([]*filter.AtomRule, 0)
	if len(req.BkBizIDs) > 0 {
		listRules = append(listRules, tools.RuleIn("bk_biz_id", req.BkBizIDs))
	}
	if len(req.OpProductIDs) > 0 {
		listRules = append(listRules, tools.RuleIn("op_product_id", req.OpProductIDs))
	}
	if len(req.PlanProductIDs) > 0 {
		listRules = append(listRules, tools.RuleIn("plan_product_id", req.PlanProductIDs))
	}
	if len(req.DemandIDs) > 0 {
		listRules = append(listRules, tools.RuleIn("id", req.DemandIDs))
	}
	if len(req.ObsProjects) > 0 {
		listRules = append(listRules, tools.RuleIn("obs_project", req.ObsProjects))
	}
	if len(req.DemandClasses) > 0 {
		listRules = append(listRules, tools.RuleIn("demand_class", req.DemandClasses))
	}
	if len(req.DeviceFamilies) > 0 {
		listRules = append(listRules, tools.RuleIn("device_family", req.DeviceFamilies))
	}
	if len(req.CoreTypes) > 0 {
		listRules = append(listRules, tools.RuleIn("core_type", req.CoreTypes))
	}
	if len(req.DeviceClasses) > 0 {
		listRules = append(listRules, tools.RuleIn("device_class", req.DeviceClasses))
	}
	if len(req.DeviceTypes) > 0 {
		listRules = append(listRules, tools.RuleIn("device_type", req.DeviceTypes))
	}
	if len(req.RegionIDs) > 0 {
		listRules = append(listRules, tools.RuleIn("region_id", req.RegionIDs))
	}
	if len(req.ZoneIDs) > 0 {
		listRules = append(listRules, tools.RuleIn("zone_id", req.ZoneIDs))
	}
	if len(req.PlanTypes) > 0 {
		planTypeCodes := make([]enumor.PlanTypeCode, len(req.PlanTypes))
		for _, planType := range req.PlanTypes {
			planTypeCodes = append(planTypeCodes, planType.GetCode())
		}
		listRules = append(listRules, tools.RuleIn("plan_type", planTypeCodes))
	}

	timeRules, err := c.convListResPlanDemandTimeFilter(kt, req.ExpiringOnly, req.ExpectTimeRange)
	if err != nil {
		logs.Errorf("failed to convert list res plan demand time filter, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	listRules = append(listRules, timeRules...)

	return listRules, nil
}

// listAllResPlanDemand list all res plan demand by request.
// Note that only count and sort is used in the req.Page.
func (c *Controller) listAllResPlanDemand(kt *kit.Kit, req *ptypes.ListResPlanDemandReq) ([]rpd.ResPlanDemandTable,
	uint64, error) {

	listRules, err := c.convAllResPlanDemandListOpt(kt, req)
	if err != nil {
		logs.Errorf("failed to convert list res plan demand filter, err: %v, rid: %s", err, kt.Rid)
		return nil, 0, err
	}

	listPage := &core.BasePage{
		Start: 0,
		Limit: core.DefaultMaxPageLimit,
		Sort:  req.Page.Sort,
		Order: req.Page.Order,
	}
	if req.Page.Count {
		listPage = req.Page
	}

	listReq := &rpproto.ResPlanDemandListReq{
		ListReq: core.ListReq{
			Filter: tools.ExpressionAnd(listRules...),
			Page:   listPage,
		},
	}

	result := make([]rpd.ResPlanDemandTable, 0)
	for {
		rst, err := c.client.DataService().Global.ResourcePlan.ListResPlanDemand(kt, listReq)
		if err != nil {
			logs.Errorf("failed to list local res plan demand, err: %v, rid: %s", err, kt.Rid)
			return nil, 0, err
		}

		if req.Page.Count {
			return nil, rst.Count, nil
		}

		result = append(result, rst.Details...)

		if len(rst.Details) < int(listReq.Page.Limit) {
			break
		}
		listReq.Page.Start += uint32(listReq.Page.Limit)
	}

	return result, 0, nil
}

// RepairResPlanDemandFromTicket 给定时间范围，从范围内的历史单据还原预测
func (c *Controller) RepairResPlanDemandFromTicket(kt *kit.Kit, bkBizIDs []int64,
	ticketTimeRange times.DateRange) error {

	start := time.Now()
	logs.Infof("start repair res plan demand from ticket, bk_biz_ids: %v, rangeStart: %s, rangeEnd: %s, time: %v, "+
		"rid: %s", bkBizIDs, ticketTimeRange.Start, ticketTimeRange.End, start, kt.Rid)

	// 捞取时间范围内的所有订单
	listTicketRules := make([]filter.RuleFactory, 0)

	drFilter, err := tools.DateRangeExpression("submitted_at", &ticketTimeRange)
	if err != nil {
		logs.Errorf("failed to build ticket time range filter, err: %v, time_range: %v, rid: %s", err,
			ticketTimeRange, kt.Rid)
		return err
	}
	listTicketRules = append(listTicketRules, drFilter)

	if len(bkBizIDs) > 0 {
		listTicketRules = append(listTicketRules, tools.RuleIn("bk_biz_id", bkBizIDs))
	}

	listTicketFilter, err := tools.And(listTicketRules...)
	if err != nil {
		logs.Errorf("failed to build ticket filter, err: %v, bk_biz_ids: %v, time_range: %v, rid: %s", err,
			bkBizIDs, ticketTimeRange, kt.Rid)
		return err
	}

	allTickets, err := c.resFetcher.ListAllResPlanTicket(kt, listTicketFilter)
	if err != nil {
		logs.Errorf("failed to list all res plan ticket, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	// 从订单还原预测变更信息
	if err := c.dispatcher.ApplyResPlanDemandChangeFromRPTickets(kt, allTickets); err != nil {
		logs.Errorf("failed to apply res plan demand change from res plan ticket, err: %v, bk_biz_ids: %v, "+
			"rangeStart: %s, rangeEnd: %s, rid: %s", err, bkBizIDs, ticketTimeRange.Start, ticketTimeRange.End, kt.Rid)
		return err
	}

	end := time.Now()
	logs.Infof("end repair res plan demand from ticket, bk_biz_ids: %v, rangeStart: %s, rangeEnd: %s, time: %v, "+
		"cost: %ds, rid: %s", bkBizIDs, ticketTimeRange.Start, ticketTimeRange.End, end, end.Sub(start).Seconds(),
		kt.Rid)
	return nil
}

// QueryIEGDemands query IEG crp demands.
func (c *Controller) QueryIEGDemands(kt *kit.Kit, req *QueryIEGDemandsReq) ([]*cvmapi.CvmCbsPlanQueryItem, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	queryReq := buildIEGDemandQueryReq(req)
	return c.fetchAllCvmCbsPlans(kt, queryReq)
}

// buildIEGDemandQueryReq builds CvmCbsPlanQueryReq from QueryIEGDemandsReq.
func buildIEGDemandQueryReq(req *QueryIEGDemandsReq) *cvmapi.CvmCbsPlanQueryReq {
	queryReq := &cvmapi.CvmCbsPlanQueryReq{
		ReqMeta: cvmapi.ReqMeta{
			Id:      cvmapi.CvmId,
			JsonRpc: cvmapi.CvmJsonRpc,
			Method:  cvmapi.CvmCbsPlanQueryMethod,
		},
		Params: &cvmapi.CvmCbsPlanQueryParam{
			Page: &cvmapi.Page{
				Start: 0,
				Size:  int(core.DefaultMaxPageLimit),
			},
			BgName: []string{cvmapi.CvmCbsPlanQueryBgName},
		},
	}

	// append filter parameters.
	if req.ExpectTimeRange != nil {
		queryReq.Params.UseTime = &cvmapi.UseTime{
			Start: req.ExpectTimeRange.Start,
			End:   req.ExpectTimeRange.End,
		}
	}

	if len(req.CrpDemandIDs) > 0 {
		queryReq.Params.DemandIdList = req.CrpDemandIDs
	}

	if len(req.CrpSns) > 0 {
		queryReq.Params.OrderIdList = req.CrpSns
	}

	if len(req.DeviceClasses) > 0 {
		queryReq.Params.InstanceType = req.DeviceClasses
	}

	if len(req.PlanProdNames) > 0 {
		queryReq.Params.PlanProductName = req.PlanProdNames
	}

	if len(req.OpProdNames) > 0 {
		queryReq.Params.ProductName = req.OpProdNames
	}

	if len(req.ObsProjects) > 0 {
		queryReq.Params.ProjectName = req.ObsProjects
	}

	if len(req.RegionNames) > 0 {
		queryReq.Params.CityName = req.RegionNames
	}

	if len(req.ZoneNames) > 0 {
		queryReq.Params.ZoneName = req.ZoneNames
	}

	// 技术分类
	if len(req.TechnicalClasses) > 0 {
		queryReq.Params.TechnicalClass = req.TechnicalClasses
	}

	return queryReq
}

// fetchAllCvmCbsPlans fetches all pages of cvm cbs plan data.
func (c *Controller) fetchAllCvmCbsPlans(kt *kit.Kit, queryReq *cvmapi.CvmCbsPlanQueryReq) (
	[]*cvmapi.CvmCbsPlanQueryItem, error) {

	result := make([]*cvmapi.CvmCbsPlanQueryItem, 0)
	for start := 0; ; start += int(core.DefaultMaxPageLimit) {
		queryReq.Params.Page.Start = start
		rst, err := c.crpCli.QueryCvmCbsPlans(kt.Ctx, kt.Header(), queryReq)
		if err != nil {
			return nil, err
		}

		if rst.Error.Code != 0 {
			logs.Errorf("failed to query cvm cbs plans, err: %s, crp_trace: %s, rid: %s", rst.Error.Message,
				rst.TraceId, kt.Rid)
			return nil, errors.New(rst.Error.Message)
		}

		result = append(result, rst.Result.Data...)

		if len(rst.Result.Data) < int(core.DefaultMaxPageLimit) {
			break
		}
	}

	return result, nil
}

// GetProdResPlanPool get op product resource plan pool.
func (c *Controller) GetProdResPlanPool(kt *kit.Kit, prodID int64) (ResPlanPool, error) {
	// get op product all unlocked crp demand ids.
	opt, err := tools.And(
		tools.RuleEqual("locked", int8(enumor.CrpDemandUnLocked)),
		tools.RuleEqual("op_product_id", prodID),
	)
	if err != nil {
		logs.Errorf("failed to and filter expression, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// get product all unlocked crp demand details.
	demands, err := c.listCrpDemandDetails(kt, opt)
	if err != nil {
		logs.Errorf("failed to list all crp demand details, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// construct resource plan pool.
	pool := make(ResPlanPool)
	strUnionFind := NewStrUnionFind()
	for _, demand := range demands {
		strUnionFind.Add(demand.InstanceModel)
	}

	for _, demand := range demands {
		// merge match device type.
		deviceType := demand.InstanceModel
		wildcardSource := strUnionFind.Elements()
		matches, err := c.IsDeviceMatched(kt, wildcardSource, demand.InstanceModel)
		if err != nil {
			logs.Errorf("failed to check device matched, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		for idx, match := range matches {
			if match {
				strUnionFind.Union(wildcardSource[idx], demand.InstanceModel)
				deviceType = strUnionFind.Find(demand.InstanceModel)
				break
			}
		}

		key := ResPlanPoolKey{
			PlanType:      enumor.PlanType(demand.InPlan).ToAnotherPlanType(),
			AvailableTime: NewAvailableTime(demand.Year, time.Month(demand.Month)),
			DeviceType:    deviceType,
			ObsProject:    demand.ProjectName,
			RegionName:    demand.CityName,
			ZoneName:      demand.ZoneName,
		}

		pool[key] += demand.PlanCoreAmount
	}

	return pool, nil
}

// listCrpDemandDetails list crp resource plan demand details.
func (c *Controller) listCrpDemandDetails(kt *kit.Kit, opt *filter.Expression) ([]*cvmapi.CvmCbsPlanQueryItem, error) {
	if opt == nil {
		return nil, errors.New("expression is nil")
	}

	// get op product all unlocked crp demand ids.
	listOpt := &types.ListOption{
		Fields: []string{"crp_demand_id"},
		Filter: opt,
		Page:   core.NewDefaultBasePage(),
	}

	// get all crp demand ids.
	crpDemandIDs := make([]int64, 0)
	for {
		rst, err := c.dao.ResPlanCrpDemand().List(kt, listOpt)
		if err != nil {
			logs.Errorf("failed to list resource plan crp demand, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		for _, detail := range rst.Details {
			crpDemandIDs = append(crpDemandIDs, detail.CrpDemandID)
		}

		if len(rst.Details) < int(listOpt.Page.Limit) {
			break
		}

		listOpt.Page.Start += uint32(core.DefaultMaxPageLimit)
	}

	if len(crpDemandIDs) == 0 {
		return nil, nil
	}

	// query crp demand ids corresponding demand details.
	demands, err := c.QueryIEGDemands(kt, &QueryIEGDemandsReq{CrpDemandIDs: crpDemandIDs})
	if err != nil {
		logs.Errorf("failed to query ieg demands, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return demands, nil
}

// GetProdResConsumePool get op product resource consume pool.
func (c *Controller) GetProdResConsumePool(kt *kit.Kit, prodID, planProdID int64) (ResPlanPool, error) {
	// get plan product all crp demand details.
	demands, err := c.listCrpDemandDetails(kt, tools.EqualExpression("plan_product_id", planProdID))
	if err != nil {
		logs.Errorf("failed to list all crp demand details, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// get plan product all apply order consume pool map.
	orderConsumePoolMap, err := c.getApplyOrderConsumePoolMap(kt, demands)
	if err != nil {
		logs.Errorf("failed to get apply order consume pool map, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// get order ids of op product in order ids.
	planProdOrderIDs := cvt.MapKeyToSlice(orderConsumePoolMap)
	prodOrderIDs, err := c.getProdOrders(kt, prodID, planProdOrderIDs)
	if err != nil {
		logs.Errorf("failed to get op product orders, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	prodConsumePool := make(ResPlanPool)
	strUnionFind := NewStrUnionFind()
	for _, consumePool := range orderConsumePoolMap {
		for consumePoolKey := range consumePool {
			strUnionFind.Add(consumePoolKey.DeviceType)
		}
	}

	for _, prodOrderID := range prodOrderIDs {
		consumePool := orderConsumePoolMap[prodOrderID]
		for consumePoolKey, consumeCpuCore := range consumePool {
			deviceType := consumePoolKey.DeviceType
			for _, ele := range strUnionFind.Elements() {
				matched, err := c.IsDeviceMatched(kt, []string{ele}, consumePoolKey.DeviceType)
				if err != nil {
					logs.Errorf("failed to check device matched, err: %v, rid: %s", err, kt.Rid)
					return nil, err
				}

				if matched[0] {
					strUnionFind.Union(ele, consumePoolKey.DeviceType)
					deviceType = strUnionFind.Find(ele)
					break
				}
			}

			consumePoolKey.DeviceType = deviceType
			prodConsumePool[consumePoolKey] += consumeCpuCore
		}
	}

	return prodConsumePool, nil
}

// getApplyOrderConsumePoolMap get crp demand ids corresponding apply order consume resource plan pool map.
func (c *Controller) getApplyOrderConsumePoolMap(kt *kit.Kit, demands []*cvmapi.CvmCbsPlanQueryItem) (
	map[string]ResPlanPool, error) {

	orderConsumePoolMap := make(map[string]ResPlanPool)
	mutex := sync.Mutex{}
	limit := constant.SyncConcurrencyDefaultMaxLimit

	err := concurrence.BaseExec(limit, demands, func(demand *cvmapi.CvmCbsPlanQueryItem) error {
		crpDemandID, err := strconv.ParseInt(demand.DemandId, 10, 64)
		if err != nil {
			logs.Errorf("failed to parse crp demand id, err: %v, rid: %s", err, kt.Rid)
			return err
		}

		changelogs, err := c.getDemandAllChangelogs(kt, crpDemandID)
		if err != nil {
			logs.Errorf("failed to get crp demand all changelogs, err: %v, rid: %s", err, kt.Rid)
			return err
		}

		for _, changelog := range changelogs {
			// skip changelog which does not consume resource plan.
			if !slices.Contains(enumor.GetCrpConsumeResPlanSourceTypes(), changelog.SourceType) {
				continue
			}

			consumePoolKey := ResPlanPoolKey{
				PlanType:      enumor.PlanType(demand.InPlan).ToAnotherPlanType(),
				AvailableTime: NewAvailableTime(demand.Year, time.Month(demand.Month)),
				DeviceType:    demand.InstanceModel,
				ObsProject:    demand.ProjectName,
				RegionName:    demand.CityName,
				ZoneName:      demand.ZoneName,
			}

			// 消耗预测CRP changelog中ChangeCoreAmount对应负值，因此需要乘-1取反
			consumeCpuCore := -int64(changelog.ChangeCoreAmount)

			mutex.Lock()
			if _, ok := orderConsumePoolMap[changelog.OrderId]; !ok {
				orderConsumePoolMap[changelog.OrderId] = make(ResPlanPool)
			}
			orderConsumePoolMap[changelog.OrderId][consumePoolKey] += consumeCpuCore
			mutex.Unlock()
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return orderConsumePoolMap, nil
}

// getDemandAllChangelogs get crp demand id corresponding all changelogs.
func (c *Controller) getDemandAllChangelogs(kt *kit.Kit, crpDemandID int64) (
	[]*cvmapi.DemandChangeLogQueryLogItem, error) {

	req := &cvmapi.DemandChangeLogQueryReq{
		ReqMeta: cvmapi.ReqMeta{
			Id:      cvmapi.CvmId,
			JsonRpc: cvmapi.CvmJsonRpc,
			Method:  cvmapi.CvmCbsDemandChangeLogQueryMethod,
		},
		Params: &cvmapi.DemandChangeLogQueryParam{
			DemandIdList: []int64{crpDemandID},
			Page: &cvmapi.Page{
				Start: 0,
				Size:  int(core.DefaultMaxPageLimit),
			},
		},
	}

	result := make([]*cvmapi.DemandChangeLogQueryLogItem, 0)
	for start := 0; ; start += int(core.DefaultMaxPageLimit) {
		req.Params.Page.Start = start
		resp, err := c.crpCli.QueryDemandChangeLog(kt.Ctx, kt.Header(), req)
		if err != nil {
			logs.Errorf("failed to query crp demand change log, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		if resp.Error.Code != 0 {
			logs.Errorf("failed to list crp demand change log, code: %d, msg: %s, crp_trace: %s, rid: %s",
				resp.Error.Code, resp.Error.Message, resp.TraceId, kt.Rid)
			return nil, fmt.Errorf("failed to list crp demand change log, code: %d, msg: %s", resp.Error.Code,
				resp.Error.Message)
		}

		if resp.Result == nil {
			logs.Errorf("failed to list crp demand change log, for result is empty, crp_trace: %s, rid: %s",
				resp.TraceId, kt.Rid)
			return nil, errors.New("failed to list crp demand change log, for result is empty")
		}

		if len(resp.Result.Data) == 0 {
			logs.Errorf("failed to list crp demand change log, for result data is empty, crp_trace: %s, rid: %s",
				resp.TraceId, kt.Rid)
			return nil, errors.New("failed to list crp demand change log, for result data is empty")
		}

		result = append(result, resp.Result.Data[0].Info...)

		if len(resp.Result.Data) < int(core.DefaultMaxPageLimit) {
			break
		}
	}
	return result, nil
}

// getProdOrders get order ids of op product in order ids.
func (c *Controller) getProdOrders(kt *kit.Kit, prodID int64, orderIDs []string) ([]string, error) {
	req := cvmapi.NewOrderQueryReq(&cvmapi.OrderQueryParam{OrderId: orderIDs})
	resp, err := c.crpCli.QueryCvmOrders(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("failed to query cvm orders, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if resp.Result == nil {
		logs.Errorf("query cvm orders, but result is nil, trace id: %s, rid: %s", resp.TraceId, kt.Rid)
		return nil, errors.New("query cvm orders, but result is nil")
	}

	prodOrderIDs := make([]string, 0)
	for _, order := range resp.Result.Data {
		if order.ProductId == prodID {
			prodOrderIDs = append(prodOrderIDs, order.OrderId)
		}
	}

	return prodOrderIDs, nil
}

// GetProdResRemainPool get op product resource remain pool.
// @param prodID is the op product id.
// @param planProdID is the corresponding plan product id of the op product id.
// @return prodRemainedPool is the op product in plan and out plan remained resource plan pool.
// @return prodMaxAvailablePool is the op product in plan and out plan remained max available resource plan pool.
// NOTE: maxAvailableInPlanPool = totalInPlan * 120% - consumeInPlan, because the special rules of the crp system.
func (c *Controller) GetProdResRemainPool(kt *kit.Kit, prodID, planProdID int64) (ResPlanPool, ResPlanPool, error) {
	// get op product resource plan pool.
	prodPlanPool, err := c.GetProdResPlanPool(kt, prodID)
	if err != nil {
		logs.Errorf("failed to get op product resource plan pool, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}

	// get op product resource consume pool.
	prodConsumePool, err := c.GetProdResConsumePool(kt, prodID, planProdID)
	if err != nil {
		logs.Errorf("failed to get op product resource consume pool, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}

	// compact keys of prodPlanPool and prodConsumePool, set their matched device type to the same.
	if err = c.compactResPlanPool(kt, prodPlanPool, prodConsumePool); err != nil {
		logs.Errorf("failed to compact resource plan pool, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}

	// construct product max available resource plan pool.
	prodMaxAvailablePool := make(ResPlanPool)
	for k, v := range prodPlanPool {
		if k.PlanType == enumor.PlanTypeHcmInPlan {
			// TODO: 预测内的总预测需要 * 120%，目前没整清楚120%的逻辑，先按100%计算
			prodMaxAvailablePool[k] = v
		} else {
			prodMaxAvailablePool[k] = v
		}
	}

	// matching.
	for prodResPlanKey, consumeCpuCore := range prodConsumePool {
		// zone name should not be empty, it may use resource plan which zone name is equal or zone name is empty.
		plan, ok := prodPlanPool[prodResPlanKey]
		if ok {
			canConsume := max(min(plan, consumeCpuCore), 0)
			prodPlanPool[prodResPlanKey] -= canConsume
			prodMaxAvailablePool[prodResPlanKey] -= canConsume
			consumeCpuCore -= canConsume
		}

		keyWithoutZone := ResPlanPoolKey{
			PlanType:      prodResPlanKey.PlanType,
			AvailableTime: prodResPlanKey.AvailableTime,
			DeviceType:    prodResPlanKey.DeviceType,
			ObsProject:    prodResPlanKey.ObsProject,
			RegionName:    prodResPlanKey.RegionName,
		}

		plan, ok = prodPlanPool[keyWithoutZone]
		if ok {
			canConsume := max(min(plan, consumeCpuCore), 0)
			prodPlanPool[keyWithoutZone] -= canConsume
			prodMaxAvailablePool[keyWithoutZone] -= canConsume
			consumeCpuCore -= canConsume
		}

		if consumeCpuCore > 0 {
			logs.Errorf("record :%v is not enough in op product resource plan pool, rid: %s", prodResPlanKey, kt.Rid)
			return nil, nil, fmt.Errorf("record :%v is not enough in op product resource plan pool", prodResPlanKey)
		}
	}

	return prodPlanPool, prodMaxAvailablePool, nil
}

// compactResPlanPool 紧凑资源预测池，使pool1和pool2的key中，可以通配的机型设置为同一机型。
// 例如：pool1中存在device_type1, device_type2，pool2中存在device_type3, device_type4，
// 假设device_type1和device_type3通配，device_type2和device_type4通配，
// 那么pool2中的device_type3会被修改为device_type1，device_type4会被修改为device_type2。
func (c *Controller) compactResPlanPool(kt *kit.Kit, pool1, pool2 ResPlanPool) error {
	for k1 := range pool1 {
		for k2, v2 := range pool2 {
			if k1.DeviceType == k2.DeviceType {
				continue
			}

			matched, err := c.IsDeviceMatched(kt, []string{k1.DeviceType}, k2.DeviceType)
			if err != nil {
				logs.Errorf("failed to check device matched, err: %v, rid: %s", err, kt.Rid)
				return err
			}

			if matched[0] {
				newK2 := ResPlanPoolKey{
					PlanType:      k2.PlanType,
					AvailableTime: k2.AvailableTime,
					DeviceType:    k1.DeviceType,
					ObsProject:    k2.ObsProject,
					RegionName:    k2.RegionName,
					ZoneName:      k2.ZoneName,
				}

				pool2[newK2] = v2
				delete(pool2, k2)
			}
		}
	}

	return nil
}

// VerifyProdDemands verify whether the needs of op product can be satisfied.
func (c *Controller) VerifyProdDemands(kt *kit.Kit, prodID, planProdID int64, needs []VerifyResPlanElem) (
	[]VerifyResPlanResElem, error) {

	prodRemain, prodMaxAvailable, err := c.GetProdResRemainPool(kt, prodID, planProdID)
	if err != nil {
		logs.Errorf("failed to get product resource remain pool, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	result := make([]VerifyResPlanResElem, len(needs))

	// match each need.
	for i, need := range needs {
		if need.IsPrePaid {
			// verify pre paid.
			result[i], err = c.verifyPrePaid(kt, prodRemain, prodMaxAvailable, need)
			if err != nil {
				logs.Errorf("failed to verify pre paid, err: %v, rid: %s", err, kt.Rid)
				return nil, err
			}
		} else {
			// verify post paid by hour.
			result[i], err = c.verifyPostPaidByHour(kt, prodRemain, need)
			if err != nil {
				logs.Errorf("failed to verify post paid by hour, err: %v, rid: %s", err, kt.Rid)
				return nil, err
			}
		}
	}

	return result, nil
}

// verifyPrePaid need to be satisfied require two conditions:
// 1. InPlan + OutPlan >= applied.
// 2. InPlan * 120% - consumed >= applied.
func (c *Controller) verifyPrePaid(kt *kit.Kit, prodRemain, prodMaxAvailable ResPlanPool, need VerifyResPlanElem) (
	VerifyResPlanResElem, error) {

	rst, err := c.verifyPostPaidByHour(kt, prodRemain, need)
	if err != nil {
		logs.Errorf("failed to verify post paid by hour, err: %v, rid: %s", err, kt.Rid)
		return VerifyResPlanResElem{}, err
	}

	if rst.VerifyResult != enumor.VerifyResPlanRstPass {
		return rst, nil
	}

	needCpuCore := need.CpuCore
	for key, availableCpuCore := range prodMaxAvailable {
		if key.PlanType != enumor.PlanTypeHcmInPlan ||
			key.AvailableTime != need.AvailableTime ||
			key.ObsProject != need.ObsProject ||
			key.RegionName != need.RegionName {
			continue
		}

		matched, err := c.IsDeviceMatched(kt, []string{key.DeviceType}, need.DeviceType)
		if err != nil {
			logs.Errorf("failed to check device matched, err: %v, rid: %s", err, kt.Rid)
			return VerifyResPlanResElem{}, err
		}

		if !matched[0] {
			continue
		}

		canConsume := max(min(availableCpuCore, needCpuCore), 0)
		prodMaxAvailable[key] -= canConsume
		needCpuCore -= canConsume
	}

	if needCpuCore != 0 {
		return VerifyResPlanResElem{
			VerifyResult: enumor.VerifyResPlanRstFailed,
			Reason:       "in plan resource is not enough",
		}, nil
	}

	return VerifyResPlanResElem{VerifyResult: enumor.VerifyResPlanRstPass}, nil
}

// verifyPostPaidByHour verify whether the post paid by hour need can be satisfied.
// parameter isPriorityInPlan is used to control whether priority to consume InPlan resource plan.
func (c *Controller) verifyPostPaidByHour(kt *kit.Kit, prodRemain ResPlanPool, need VerifyResPlanElem) (
	VerifyResPlanResElem, error) {

	needCpuCore := need.CpuCore
	for _, planType := range enumor.GetPlanTypeHcmMembers() {
		for key, remainCpuCore := range prodRemain {
			if key.PlanType != planType ||
				key.AvailableTime != need.AvailableTime ||
				key.ObsProject != need.ObsProject ||
				key.RegionName != need.RegionName {
				continue
			}

			matched, err := c.IsDeviceMatched(kt, []string{key.DeviceType}, need.DeviceType)
			if err != nil {
				logs.Errorf("failed to check device matched, err: %v, rid: %s", err, kt.Rid)
				return VerifyResPlanResElem{}, err
			}

			if !matched[0] {
				continue
			}

			canConsume := max(min(remainCpuCore, needCpuCore), 0)
			prodRemain[key] -= canConsume
			needCpuCore -= canConsume
		}
	}

	if needCpuCore != 0 {
		return VerifyResPlanResElem{
			VerifyResult: enumor.VerifyResPlanRstFailed,
			Reason:       "in plan or out plan resource is not enough",
		}, nil
	}

	return VerifyResPlanResElem{VerifyResult: enumor.VerifyResPlanRstPass}, nil
}

// listAllPlanDemandsByBkBizID list all plan demand by bk biz id.
func (c *Controller) listAllPlanDemandsByBkBizID(kt *kit.Kit, bkBizID int64, startDay, endDay time.Time) (
	[]rpd.ResPlanDemandTable, error) {

	startDate, err := strconv.Atoi(startDay.Format(bkbase.DateLayout))
	if err != nil {
		logs.Errorf("failed to converting stary day to integer: %v", err)
		return nil, err
	}
	endDate, err := strconv.Atoi(endDay.Format(bkbase.DateLayout))
	if err != nil {
		logs.Errorf("failed to converting end day to integer: %v", err)
		return nil, err
	}

	planDemandDetails := make([]rpd.ResPlanDemandTable, 0)
	listOpt := &rpproto.ResPlanDemandListReq{
		ListReq: core.ListReq{
			Filter: tools.ExpressionAnd(
				tools.RuleEqual("bk_biz_id", bkBizID),
				tools.RuleGreaterThanEqual("expect_time", startDate),
				tools.RuleLessThanEqual("expect_time", endDate),
			),
			Page: core.NewDefaultBasePage(),
		},
	}
	for {
		rst, err := c.client.DataService().Global.ResourcePlan.ListResPlanDemand(kt, listOpt)
		if err != nil {
			logs.Errorf("failed to list resource plan demand, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		for _, detail := range rst.Details {
			planDemandDetails = append(planDemandDetails, detail)
		}

		if len(rst.Details) < int(listOpt.Page.Limit) {
			break
		}

		listOpt.Page.Start += uint32(core.DefaultMaxPageLimit)
	}

	return planDemandDetails, nil
}

// GetProdResConsumePoolV2 get biz resource consume pool v2.
func (c *Controller) GetProdResConsumePoolV2(kt *kit.Kit, bkBizIDs []int64, startDay, endDay time.Time) (
	ResPlanConsumePool, error) {

	return c.getProdResConsumePoolV2(kt, bkBizIDs, startDay, endDay, nil)
}

func (c *Controller) getProdResConsumePoolV2(kt *kit.Kit, bkBizIDs []int64, startDay, endDay time.Time,
	excludeSuborderIDs []string) (ResPlanConsumePool, error) {

	// list apply order from db by bk biz id.
	subOrders, err := c.listApplyOrder(kt, bkBizIDs, startDay, endDay, excludeSuborderIDs)
	if err != nil {
		logs.Errorf("failed to list apply order details, err: %v, bkBizIDs: %v, startDay: %s, endDay: %s, "+
			"excludeSuborderIDs: %v, rid: %s", err, bkBizIDs, startDay.Format(constant.TimeStdFormat),
			endDay.Format(constant.TimeStdFormat), excludeSuborderIDs, kt.Rid)
		return nil, err
	}

	// get plan product all apply order consume pool map.
	orderConsumePoolMap, err := c.getApplyOrderConsumePoolMapV2(kt, subOrders)
	if err != nil {
		logs.Errorf("failed to get apply order consume pool map v2, err: %v, subOrders: %+v, rid: %s",
			err, cvt.PtrToSlice(subOrders), kt.Rid)
		return nil, err
	}

	prodConsumePool := make(ResPlanConsumePool)
	strUnionFind := NewStrUnionFind()
	for consumePoolKey := range orderConsumePoolMap {
		strUnionFind.Add(consumePoolKey.DeviceType)
	}

	for consumePoolKey, consumeCpuCore := range orderConsumePoolMap {
		deviceType := consumePoolKey.DeviceType
		for _, ele := range strUnionFind.Elements() {
			matched, err := c.IsDeviceMatched(kt, []string{ele}, consumePoolKey.DeviceType)
			if err != nil {
				logs.Errorf("failed to check device is matched v2, err: %v, consumePoolKey: %+v, rid: %s",
					err, consumePoolKey, kt.Rid)
				return nil, err
			}

			if matched[0] {
				strUnionFind.Union(ele, consumePoolKey.DeviceType)
				deviceType = strUnionFind.Find(ele)
				break
			}
		}

		consumePoolKey.DeviceType = deviceType
		prodConsumePool[consumePoolKey] += consumeCpuCore
	}
	logs.Infof("get biz resource consume pool v2, bkBizIDs: %v, startDay: %s, endDay: %s, pool: %+v, "+
		"strUnionFind: %+v, orderConsumePoolMap: %+v, rid: %s", bkBizIDs, startDay.Format(constant.TimeStdFormat),
		endDay.Format(constant.TimeStdFormat), prodConsumePool, cvt.PtrToVal(strUnionFind), orderConsumePoolMap, kt.Rid)

	return prodConsumePool, nil
}

// listApplyOrder list apply order from db by bk biz ids.
func (c *Controller) listApplyOrder(kt *kit.Kit, bkBizIDs []int64, startDay, endDay time.Time,
	excludeSuborderIDs []string) ([]*tasktypes.ApplyOrder, error) {

	result := make([]*tasktypes.ApplyOrder, 0)
	batches := slice.Split(bkBizIDs, int(core.DefaultMaxPageLimit))
	for _, batch := range batches {
		filterRules := []*filter.AtomRule{
			tools.RuleIn("bk_biz_id", batch),
			tools.RuleGreaterThanEqual("created_at", startDay.Format(constant.TimeStdFormat)),
			tools.RuleLessThanEqual("created_at", endDay.Format(constant.TimeStdFormat)),
		}
		for _, excludeBatch := range slice.Split(excludeSuborderIDs, int(core.DefaultMaxPageLimit)) {
			filterRules = append(filterRules, tools.RuleNotIn("suborder_id", excludeBatch))
		}

		listReq := &cvmapplyproto.ZiyanCvmApplySuborderListReq{
			Filter: tools.ExpressionAnd(filterRules...),
			Page:   core.NewDefaultBasePage(),
		}

		for {
			resp, err := c.client.DataService().TCloudZiyan.ZiyanCvmApplySuborder.List(kt.Ctx, kt.Header(), listReq)
			if err != nil {
				logs.Errorf("failed to list cvm apply suborder, req: %+v, err: %v, rid: %s", listReq, err, kt.Rid)
				return nil, err
			}

			for _, sub := range resp.Details {
				applyOrder, err := convertSuborderToApplyOrder(kt, sub)
				if err != nil {
					logs.Errorf("convert suborder to apply order failed, suborder_id: %s, err: %v, rid: %s",
						sub.SuborderID, err, kt.Rid)
					return nil, err
				}
				result = append(result, applyOrder)
			}

			if len(resp.Details) < int(listReq.Page.Limit) {
				break
			}
			listReq.Page.Start += uint32(listReq.Page.Limit)
		}
	}

	return result, nil
}

// convertSuborderToApplyOrder convert ziyan suborder model to scheduler ApplyOrder.
func convertSuborderToApplyOrder(kt *kit.Kit, sub *cvmapplytable.ZiyanCvmApplySuborder) (*tasktypes.ApplyOrder, error) {
	apply := initApplyOrder(sub)

	if err := parseApplyOrderFields(kt, sub, apply); err != nil {
		logs.Errorf("parse apply order fields failed, suborder_id: %s, err: %v, rid: %s",
			sub.SuborderID, err, kt.Rid)
		return nil, err
	}

	spec := initResourceSpec(sub)
	if err := parseResourceSpecFields(kt, sub, spec); err != nil {
		logs.Errorf("parse resource spec fields failed, suborder_id: %s, err: %v, rid: %s",
			sub.SuborderID, err, kt.Rid)
		return nil, err
	}
	// DeviceType 为空时 Spec 无意义，置为 nil，由调用方通过 Spec == nil 判断走 PlanExpendGroup 分支
	if spec.DeviceType == "" {
		spec = nil
	}
	apply.Spec = spec

	return apply, nil
}

// initApplyOrder initialize ApplyOrder with basic fields
func initApplyOrder(sub *cvmapplytable.ZiyanCvmApplySuborder) *tasktypes.ApplyOrder {
	return &tasktypes.ApplyOrder{
		OrderId:           sub.OrderID,
		SubOrderId:        sub.SuborderID,
		BkBizId:           sub.BkBizID,
		User:              sub.BkUsername,
		Auditor:           sub.Auditor,
		RequireType:       sub.RequireType,
		ExpectTime:        sub.ExpectTime,
		ResourceType:      sub.ResourceType,
		Source:            sub.Source,
		AntiAffinityLevel: sub.AntiAffinityLevel,
		EnableDiskCheck:   cvt.PtrToVal(sub.EnableDiskCheck),
		Description:       sub.Description,
		Remark:            sub.Remark,
		Stage:             sub.Stage,
		Status:            sub.Status,
		OriginNum:         cvt.PtrToVal(sub.OriginNum),
		TotalNum:          cvt.PtrToVal(sub.TotalNum),
		SuccessNum:        cvt.PtrToVal(sub.SuccessNum),
		PendingNum:        cvt.PtrToVal(sub.PendingNum),
		AppliedCore:       cvt.PtrToVal(sub.AppliedCore),
		DeliveredCore:     cvt.PtrToVal(sub.DeliveredCore),
		ObsProject:        sub.ObsProject,
		RetryTime:         cvt.PtrToVal(sub.RetryTime),
		ModifyTime:        cvt.PtrToVal(sub.ModifyTime),
	}
}

// parseApplyOrderFields parse fields for ApplyOrder
func parseApplyOrderFields(kt *kit.Kit, sub *cvmapplytable.ZiyanCvmApplySuborder, apply *tasktypes.ApplyOrder) error {
	if err := parseJSONField(sub.Follower, &apply.Follower); err != nil {
		logs.Errorf("failed to parse follower, err: %v, suborder_id: %s, rid: %s", err, sub.SuborderID, kt.Rid)
		return err
	}
	if err := parseJSONField(sub.PlanExpendGroup, &apply.PlanExpendGroup); err != nil {
		logs.Errorf("failed to parse plan expend group, err: %v, suborder_id: %s, rid: %s", err, sub.SuborderID,
			kt.Rid)
		return err
	}
	if err := parseJSONField(sub.UpgradeCvmList, &apply.UpgradeCVMList); err != nil {
		logs.Errorf("failed to parse upgrade cvm list, err: %v, suborder_id: %s, rid: %s", err, sub.SuborderID,
			kt.Rid)
		return err
	}
	var err error
	apply.CreateAt, err = parseTime(kt, sub.CreatedAt)
	if err != nil {
		logs.Errorf("failed to parse created at, err: %v, suborder_id: %s, rid: %s", err, sub.SuborderID, kt.Rid)
		return err
	}
	apply.UpdateAt, err = parseTime(kt, sub.UpdatedAt)
	if err != nil {
		logs.Errorf("failed to parse updated at, err: %v, suborder_id: %s, rid: %s", err, sub.SuborderID, kt.Rid)
		return err
	}
	return nil
}

// initResourceSpec initialize ResourceSpec with basic fields
func initResourceSpec(sub *cvmapplytable.ZiyanCvmApplySuborder) *tasktypes.ResourceSpec {
	return &tasktypes.ResourceSpec{
		Region:            sub.Region,
		Zone:              sub.Zone,
		DeviceGroup:       sub.DeviceGroup,
		DeviceSize:        sub.DeviceSize,
		DeviceType:        sub.DeviceType,
		ImageId:           sub.ImageID,
		Image:             sub.Image,
		DiskSize:          sub.DiskSize,
		DiskType:          sub.DiskType,
		NetworkType:       sub.NetworkType,
		Vpc:               sub.Vpc,
		Subnet:            sub.Subnet,
		OsType:            sub.OsType,
		RaidType:          sub.RaidType,
		Isp:               sub.Isp,
		ChargeType:        sub.ChargeType,
		ChargeMonths:      sub.ChargeMonths,
		InheritInstanceId: sub.InheritInstanceID,
		BkAssetID:         sub.BkAssetID,
		ResAssign:         sub.ResAssign,
		CPUThreadSwitch:   sub.CPUThreadSwitch,
	}
}

// parseResourceSpecFields parse fields for ResourceSpec
func parseResourceSpecFields(kt *kit.Kit, sub *cvmapplytable.ZiyanCvmApplySuborder,
	spec *tasktypes.ResourceSpec) error {
	if err := parseJSONField(sub.SystemDisk, &spec.SystemDisk); err != nil {
		logs.Errorf("failed to parse system disk, err: %v, suborder_id: %s, rid: %s", err, sub.SuborderID, kt.Rid)
		return err
	}
	if err := parseJSONField(sub.DataDisk, &spec.DataDisk); err != nil {
		logs.Errorf("failed to parse data disk, err: %v, suborder_id: %s, rid: %s", err, sub.SuborderID, kt.Rid)
		return err
	}
	if err := parseJSONField(sub.FailedZoneIds, &spec.FailedZoneIDs); err != nil {
		logs.Errorf("failed to parse failed zone ids, err: %v, suborder_id: %s, rid: %s", err, sub.SuborderID,
			kt.Rid)
		return err
	}
	if err := parseJSONField(sub.Zones, &spec.Zones); err != nil {
		logs.Errorf("failed to parse zones, err: %v, suborder_id: %s, rid: %s", err, sub.SuborderID, kt.Rid)
		return err
	}
	return nil
}

// parseJSONField 解析JSON字段并将其转换为指定的接口类型
func parseJSONField(field tabletypes.JsonField, out any) error {
	if field.IsEmpty() {
		return nil
	}
	return json.Unmarshal([]byte(field), out)
}

// parseTime 将 tabletypes.Time 类型转换为 time.Time 类型
func parseTime(kt *kit.Kit, t tabletypes.Time) (time.Time, error) {
	parsed, err := time.Parse(constant.TimeStdFormat, t.String())
	if err != nil {
		logs.Errorf("failed to parse time, err: %v, time: %s, rid: %s", err, t.String(), kt.Rid)
		return time.Time{}, err
	}
	return parsed, nil
}

// getApplyOrderConsumePoolMapV2 get apply order consume resource plan pool map.
func (c *Controller) getApplyOrderConsumePoolMapV2(kt *kit.Kit, subOrders []*tasktypes.ApplyOrder) (
	ResPlanConsumePool, error) {

	// 查询子单"已生产"核心数，覆盖未交付但已生产成功的主机，避免因仅按 DeliveredCore 计算导致用户卡在交付前重复提单造成超额申领
	productedCoreMap, err := c.batchCalcSuborderProductedCore(kt, subOrders)
	if err != nil {
		logs.Errorf("batch calc producted core failed, err: %v, suborder count: %d, rid: %s",
			err, len(subOrders), kt.Rid)
		return nil, err
	}

	orderConsumePoolMap := make(ResPlanConsumePool)
	for _, subOrderInfo := range subOrders {
		if !shouldCountApplyOrderConsumePool(subOrderInfo) {
			continue
		}

		demandYear, demandMonth, err := c.demandTime.GetDemandYearMonth(kt, subOrderInfo.CreateAt)
		if err != nil {
			logs.Errorf("failed to get demand year month, err: %v, subOrder: %+v, rid: %s", err, *subOrderInfo,
				kt.Rid)
			return nil, err
		}

		planType, err := c.getApplyOrderPlanType(subOrderInfo)
		if err != nil {
			logs.Errorf("failed to get plan type by charge type, err: %v, subOrder: %+v, rid: %s", err,
				*subOrderInfo, kt.Rid)
			return nil, err
		}

		if subOrderInfo.Spec != nil {
			addSpecApplyOrderConsumePool(kt, orderConsumePoolMap, subOrderInfo, planType, demandYear, demandMonth,
				productedCoreMap[subOrderInfo.SubOrderId])
			continue
		}

		addPlanExpendApplyOrderConsumePool(orderConsumePoolMap, subOrderInfo, planType, demandYear, demandMonth)
	}

	return orderConsumePoolMap, nil
}

func shouldCountApplyOrderConsumePool(sub *tasktypes.ApplyOrder) bool {
	// TODO 目前预测只关注CVM类型的主机 + 升降配主机
	if sub.ResourceType != tasktypes.ResourceTypeCvm && sub.ResourceType != tasktypes.ResourceTypeUpgradeCvm {
		return false
	}
	// 如果项目类型是常规，则RequireType也需要是对应的常规项目
	return sub.ObsProject != enumor.ObsProjectNormal || sub.RequireType == enumor.RequireTypeRegular
}

func (c *Controller) getApplyOrderPlanType(sub *tasktypes.ApplyOrder) (enumor.PlanTypeCode, error) {
	if sub.ResourceType == tasktypes.ResourceTypeUpgradeCvm {
		// 升降配需要忽略预测内外
		return "", nil
	}
	return c.GetPlanTypeByChargeType(sub.Spec.ChargeType)
}

func addSpecApplyOrderConsumePool(kt *kit.Kit, poolMap ResPlanConsumePool, sub *tasktypes.ApplyOrder,
	planType enumor.PlanTypeCode, demandYear int, demandMonth time.Month, productedCore int64) {

	consumePoolKey := ResPlanPoolKeyV2{
		PlanType:      planType,
		AvailableTime: NewAvailableTime(demandYear, demandMonth),
		DeviceType:    sub.Spec.DeviceType,
		ObsProject:    sub.ObsProject,
		BkBizID:       sub.BkBizId,
		DemandClass:   enumor.DemandClassCVM,
		RegionID:      sub.Spec.Region,
	}
	// 机房裁撤需要忽略预测内、预测外 --story=121848852
	if sub.RequireType == enumor.RequireTypeDissolve {
		consumePoolKey.PlanType = ""
	}
	// 占用预测核心数 = max(AppliedCore, 已生产核心数)，终止单据按已生产核心数
	consumeCpuCore := calcSuborderConsumeCore(sub, productedCore)
	logs.V(2).Infof("calc cvm suborder consume core, suborderID: %s, stage: %s, status: %s, applied: %d, "+
		"delivered: %d, producteCore: %d, consumeCore: %d, rid: %s",
		sub.SubOrderId, sub.Stage, sub.Status, sub.AppliedCore, sub.DeliveredCore, productedCore, consumeCpuCore, kt.Rid)
	if consumeCpuCore <= 0 {
		return
	}
	poolMap[consumePoolKey] += consumeCpuCore
}

func addPlanExpendApplyOrderConsumePool(poolMap ResPlanConsumePool, sub *tasktypes.ApplyOrder,
	planType enumor.PlanTypeCode, demandYear int, demandMonth time.Month) {

	for _, expendPlan := range sub.PlanExpendGroup {
		consumePoolKey := ResPlanPoolKeyV2{
			PlanType:      planType,
			AvailableTime: NewAvailableTime(demandYear, demandMonth),
			DeviceType:    expendPlan.DeviceType,
			ObsProject:    sub.ObsProject,
			BkBizID:       sub.BkBizId,
			DemandClass:   enumor.DemandClassCVM,
			RegionID:      expendPlan.Region,
		}
		// 机房裁撤需要忽略预测内、预测外 --story=121848852
		if sub.RequireType == enumor.RequireTypeDissolve {
			consumePoolKey.PlanType = ""
		}
		poolMap[consumePoolKey] += expendPlan.CPUCore
	}
}

// calcSuborderConsumeCore 计算单个 CVM 子单对预测额度的实际占用核心数。
//
// 规则：
//   - 终止单据：占用 = 已生产核心数（剩余不会再生产，但已生产的不会自动退还）
//   - 活跃单据：占用 = max(AppliedCore, 已生产核心数)，AppliedCore 覆盖"卡在交付前重复提单"场景，
//     已生产核心数兜底极少数手工补录超 AppliedCore 情形
//
// productedCore 由 batchCalcSuborderProductedCore 一次性预算好后传入，避免逐单查询。
func calcSuborderConsumeCore(sub *tasktypes.ApplyOrder, productedCore int64) int64 {
	// 若单据已终止，返回"已生产核心数"
	if sub.IsSuborderTerminated() {
		return productedCore
	}

	return max(int64(sub.AppliedCore), productedCore)
}

// batchCalcSuborderProductedCore 批量计算子单"已生产成功"主机的总 CPU 核心数。
//
// "已生产" 的判定：ziyan_cvm_device_info 表中存在该子单关联的设备记录（不区分是否已交付/初始化）。
// 这与 scheduler.calProductDeviceTypeCountMap(devices, false) 的语义保持一致。
//
// 性能：
//   - 仅对普通 CVM 的 Spec 分支、升降配子单查询（PlanExpendGroup 分支不需要）
//   - subOrderIDs IN(...) 单次查询设备表，按 DefaultMaxPageLimit 分批
//   - 机型 CPU 核数从 Controller 已加载的 deviceTypesMap 缓存中读取，无额外查询
func (c *Controller) batchCalcSuborderProductedCore(kt *kit.Kit, subOrders []*tasktypes.ApplyOrder) (
	map[string]int64, error) {

	subOrderIDs := make([]string, 0, len(subOrders))
	for _, sub := range subOrders {
		// 普通 CVM 仅 Spec 分支需要补齐已生产核心数；升降配不依赖 Spec，按子单查询 device_info。
		if (sub.ResourceType == tasktypes.ResourceTypeCvm && sub.Spec != nil) ||
			sub.ResourceType == tasktypes.ResourceTypeUpgradeCvm {
			subOrderIDs = append(subOrderIDs, sub.SubOrderId)
		}
	}
	result := make(map[string]int64, len(subOrderIDs))
	if len(subOrderIDs) == 0 {
		return result, nil
	}

	deviceTypeMap, err := c.deviceTypesMap.GetDeviceTypes(kt)
	if err != nil {
		logs.Errorf("batch calc suborder get device types failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	for _, batch := range slice.Split(subOrderIDs, int(core.DefaultMaxPageLimit)) {
		listReq := &cvmapplyproto.ZiyanCvmDeviceInfoListReq{
			Filter: tools.ExpressionAnd(tools.RuleIn("suborder_id", batch)),
			Page:   core.NewDefaultBasePage(),
			Fields: []string{"suborder_id", "device_type"},
		}
		for {
			resp, err := c.client.DataService().TCloudZiyan.ZiyanCvmDeviceInfo.List(kt.Ctx, kt.Header(), listReq)
			if err != nil {
				logs.Errorf("list tcloud-ziyan cvm device info failed, err: %v, suborder_count: %d, rid: %s",
					err, len(batch), kt.Rid)
				return nil, err
			}
			for _, dev := range resp.Details {
				deviceInfo, ok := deviceTypeMap[dev.DeviceType]
				if !ok {
					logs.Warnf("batch calc suborder device type %s not found in cache, suborderID: %s, rid: %s",
						dev.DeviceType, dev.SuborderID, kt.Rid)
					continue
				}
				result[dev.SuborderID] += deviceInfo.CpuCore
			}
			if len(resp.Details) < int(listReq.Page.Limit) {
				break
			}
			listReq.Page.Start += uint32(listReq.Page.Limit)
		}
	}

	return result, nil
}

// VerifyProdDemandsV2 verify whether the needs of biz can be satisfied.
func (c *Controller) VerifyProdDemandsV2(kt *kit.Kit, bkBizID int64, requireType enumor.RequireType,
	needs []VerifyResPlanElemV2) ([]VerifyResPlanResElem, error) {

	return c.verifyProdDemandsV2(kt, bkBizID, requireType, needs, nil)
}

func (c *Controller) verifyProdDemandsV2(kt *kit.Kit, bkBizID int64, requireType enumor.RequireType,
	needs []VerifyResPlanElemV2, excludeSuborderIDs []string) ([]VerifyResPlanResElem, error) {

	prodRemain, prodMaxAvailable, err := c.getProdResRemainPoolMatch(kt, bkBizID, requireType, excludeSuborderIDs)
	if err != nil {
		logs.Errorf("failed to get product resource remain pool match, bkBizID: %d, excludeSuborderIDs: %v, "+
			"err: %v, rid: %s", bkBizID, excludeSuborderIDs, err, kt.Rid)
		return nil, err
	}

	result := make([]VerifyResPlanResElem, len(needs))
	// match each need.
	for i, need := range needs {
		if need.IsPrePaid {
			// verify pre paid.
			result[i], err = c.getDemandMatchResult(kt, requireType, prodMaxAvailable, need,
				[]enumor.PlanTypeCode{enumor.PlanTypeCodeInPlan})
			if err != nil {
				logs.Errorf("failed to loop verify pre paid match, err: %v, need: %+v, rid: %s", err, need, kt.Rid)
				return nil, err
			}
		} else {
			// verify post paid by hour.
			result[i], err = c.getDemandMatchResult(kt, requireType, prodRemain, need,
				enumor.GetPlanTypeCodeHcmMembers())
			if err != nil {
				logs.Errorf("failed to loop verify post paid by hour, err: %v, need: %+v, rid: %s", err, need, kt.Rid)
				return nil, err
			}
		}
	}
	logs.Infof("verify prod demands v2 end, bkBizID: %d, excludeSuborderIDs: %v, needs: %+v, prodRemain: %+v, "+
		"prodMaxAvailable: %+v, result: %+v, rid: %s", bkBizID, excludeSuborderIDs, needs, prodRemain,
		prodMaxAvailable, result, kt.Rid)

	return result, nil
}

// GetProdResRemainPoolMatch get biz resource remain pool match.
// @param bkBizID is the bk biz id.
// @return prodRemainedPool is the biz in plan and out plan remained resource plan pool.
// @return prodMaxAvailablePool is the biz in plan and out plan remained max available resource plan pool.
// NOTE: maxAvailableInPlanPool = totalInPlan * 120% - consumeInPlan, because the special rules of the crp system.
func (c *Controller) GetProdResRemainPoolMatch(kt *kit.Kit, bkBizID int64, requireType enumor.RequireType) (
	ResPlanPoolMatch, ResPlanPoolMatch, error) {

	return c.getProdResRemainPoolMatch(kt, bkBizID, requireType, nil)
}

func (c *Controller) getProdResRemainPoolMatch(kt *kit.Kit, bkBizID int64, requireType enumor.RequireType,
	excludeSuborderIDs []string) (ResPlanPoolMatch, ResPlanPoolMatch, error) {

	prodPlanPool, prodConsumePool, err := c.getCurrMonthPlanConsumePool(kt, bkBizID, requireType, excludeSuborderIDs)
	if err != nil {
		return nil, nil, err
	}

	// construct product max available resource plan pool.
	prodMaxAvailablePool := deepCopyPlanPool(prodPlanPool)

	// matching.
	for prodResPlanKey, consumeCpuCore := range prodConsumePool {
		// 当未显示指定预测内外时，需按 预测外 -> 预测内 的顺序依次尝试匹配
		matchPlanType := []enumor.PlanTypeCode{prodResPlanKey.PlanType}
		if prodResPlanKey.PlanType == "" {
			matchPlanType = enumor.GetPlanTypeCodeHcmMembers()
		}

		for _, planType := range matchPlanType {
			keyLoop := ResPlanPoolKeyV2{
				PlanType:      planType,
				AvailableTime: prodResPlanKey.AvailableTime,
				DeviceType:    prodResPlanKey.DeviceType,
				ObsProject:    prodResPlanKey.ObsProject,
				BkBizID:       prodResPlanKey.BkBizID,
				DemandClass:   prodResPlanKey.DemandClass,
				RegionID:      prodResPlanKey.RegionID,
			}

			planMap, ok := prodPlanPool[keyLoop]
			if ok {
				for demandID, planCore := range planMap {
					canConsume := max(min(planCore, consumeCpuCore), 0)
					prodPlanPool[keyLoop][demandID] -= canConsume
					prodMaxAvailablePool[keyLoop][demandID] -= canConsume
					consumeCpuCore -= canConsume
				}
			}

			logs.Infof("biz resource plan pool is loop matched, bkBizID: %d, record: %+v, ok: %v, plan: %+v, "+
				"prodPlanPool: %+v, maxAvailablePool: %+v, consumeCpuCore: %d, rid: %s", bkBizID, prodResPlanKey, ok,
				planMap, prodPlanPool, prodMaxAvailablePool, consumeCpuCore, kt.Rid)
		}
	}

	return prodPlanPool, prodMaxAvailablePool, nil
}

func deepCopyPlanPool(src ResPlanPoolMatch) ResPlanPoolMatch {
	dst := make(ResPlanPoolMatch)
	for k, v := range src {
		newMap := make(map[string]int64)
		for demandID, planCore := range v {
			newMap[demandID] = planCore
		}
		dst[k] = newMap
	}
	return dst
}

func (c *Controller) getCurrMonthPlanConsumePool(kt *kit.Kit, bkBizID int64, requireType enumor.RequireType,
	excludeSuborderIDs []string) (ResPlanPoolMatch, ResPlanConsumePool, error) {

	nowDemandYear, nowDemandMonth, err := c.demandTime.GetDemandYearMonth(kt, time.Now())
	if err != nil {
		logs.Errorf("failed to get demand year month, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}
	startDay := time.Date(nowDemandYear, nowDemandMonth, 1, 0, 0, 0, 0, time.UTC)
	endDay := time.Date(nowDemandYear, nowDemandMonth+1, 1, 0, 0, 0, 0, time.UTC)

	// get biz resource plan pool.
	prodPlanPool, err := c.GetProdResPlanPoolMatch(kt, bkBizID, startDay, endDay, requireType)
	if err != nil {
		logs.Errorf("failed to get biz resource plan pool match, bkBizID: %d, err: %v, rid: %s", bkBizID, err, kt.Rid)
		return nil, nil, err
	}

	// get biz resource consume pool.
	prodConsumePool, err := c.getProdResConsumePoolV2(kt, []int64{bkBizID}, startDay, endDay, excludeSuborderIDs)
	if err != nil {
		logs.Errorf("failed to get biz resource consume pool v2, bkBizID: %d, excludeSuborderIDs: %v, "+
			"err: %v, rid: %s", bkBizID, excludeSuborderIDs, err, kt.Rid)
		return nil, nil, err
	}

	// compact keys of prodPlanPool and prodConsumePool, set their matched device type to the same.
	if err = c.compactResPlanPoolMatch(kt, prodPlanPool, prodConsumePool); err != nil {
		logs.Errorf("failed to compact resource plan pool match, bkBizID: %d, err: %v, rid: %s", bkBizID, err, kt.Rid)
		return nil, nil, err
	}
	return prodPlanPool, prodConsumePool, nil
}

// GetProdResPlanPoolMatch get prod resource plan pool match.
func (c *Controller) GetProdResPlanPoolMatch(kt *kit.Kit, bkBizID int64, startDay, endDay time.Time,
	requireType enumor.RequireType) (ResPlanPoolMatch, error) {

	// get biz all unlocked demand details.
	demands, err := c.listAllPlanDemandsByBkBizID(kt, bkBizID, startDay, endDay)
	if err != nil {
		logs.Errorf("failed to list all demand details, bkBizID: %d, err: %v, rid: %s", bkBizID, err, kt.Rid)
		return nil, err
	}

	// construct resource plan pool.
	pool := make(ResPlanPoolMatch)
	strUnionFind := NewStrUnionFind()
	for _, demand := range demands {
		strUnionFind.Add(demand.DeviceType)
	}

	for _, demand := range demands {
		// merge match device type.
		deviceType := demand.DeviceType
		wildcardSource := strUnionFind.Elements()
		matches, err := c.IsDeviceMatched(kt, wildcardSource, demand.DeviceType)
		if err != nil {
			logs.Errorf("failed to check device is matched, err: %v, demand: %+v, rid: %s", err, demand, kt.Rid)
			return nil, err
		}

		for idx, match := range matches {
			if match {
				strUnionFind.Union(wildcardSource[idx], demand.DeviceType)
				deviceType = strUnionFind.Find(demand.DeviceType)
				break
			}
		}

		expectTime, err := time.Parse(bkbase.DateLayout, strconv.Itoa(demand.ExpectTime))
		if err != nil {
			return nil, fmt.Errorf("conv expect time failed, expectTime: %d, err: %v", demand.ExpectTime, err)
		}

		key := ResPlanPoolKeyV2{
			PlanType:      demand.PlanType,
			AvailableTime: NewAvailableTime(expectTime.Year(), expectTime.Month()),
			DeviceType:    deviceType,
			ObsProject:    demand.ObsProject,
			BkBizID:       demand.BkBizID,
			DemandClass:   demand.DemandClass,
			RegionID:      demand.RegionID,
		}
		// 机房裁撤需要忽略预测内、预测外 --story=121848852
		if requireType == enumor.RequireTypeDissolve {
			key.PlanType = ""
		}
		if _, ok := pool[key]; !ok {
			pool[key] = make(map[string]int64, 0)
		}

		// 变更中的预测只记录未上锁的部分
		remainedCore := cvt.PtrToVal(demand.CpuCore)
		if cvt.PtrToVal(demand.Locked) == enumor.CrpDemandLocked {
			remainedCore -= cvt.PtrToVal(demand.LockedCPUCore)
		}

		pool[key][demand.ID] += remainedCore
	}
	// 记录日志方便排查问题
	logs.Infof("get res plan demand pool match success, bkBizID: %d, requireType: %d, startDay: %+v, endDay: %+v, "+
		"pool: %+v, rid: %s", bkBizID, requireType, startDay, endDay, pool, kt.Rid)
	return pool, nil
}

// compactResPlanPoolMatch 紧凑资源预测池，使pool1和pool2的key中，可以通配的机型设置为同一机型。
// 例如：pool1中存在device_type1, device_type2，pool2中存在device_type3, device_type4，
// 假设device_type1和device_type3通配，device_type2和device_type4通配，
// 那么pool2中的device_type3会被修改为device_type1，device_type4会被修改为device_type2。
func (c *Controller) compactResPlanPoolMatch(kt *kit.Kit, pool1 ResPlanPoolMatch, pool2 ResPlanConsumePool) error {
	for k1 := range pool1 {
		for k2, v2 := range pool2 {
			if k1.DeviceType == k2.DeviceType {
				continue
			}

			matched, err := c.IsDeviceMatched(kt, []string{k1.DeviceType}, k2.DeviceType)
			if err != nil {
				logs.Errorf("failed to check device matched v2, err: %v, rid: %s", err, kt.Rid)
				return err
			}

			if matched[0] {
				// 这里只需要使用k1的设备类型字段，不用重新挨个赋值，容易遗漏
				newK2 := k2
				newK2.DeviceType = k1.DeviceType
				if _, ok := pool2[newK2]; !ok {
					pool2[newK2] = v2
					delete(pool2, k2)
				}
			}
		}
	}
	return nil
}

// getDemandMatchResult get demand match result.
func (c *Controller) getDemandMatchResult(kt *kit.Kit, requireType enumor.RequireType,
	prodRemain ResPlanPoolMatch, need VerifyResPlanElemV2, matchPlanType []enumor.PlanTypeCode) (
	VerifyResPlanResElem, error) {

	matchDemandIDs := make([]string, 0)
	allResPlanCore := int64(0)
	needCpuCore := need.CpuCore
	skipReasonCoreMap := make(map[ResPlanPoolKeyV2]map[string]int64)
	for _, planType := range matchPlanType {
		for key, remainCpuCoreMap := range prodRemain {
			// 已经匹配完需要的核心数
			if needCpuCore == 0 {
				break
			}

			matched, err := c.IsDeviceMatched(kt, []string{key.DeviceType}, need.DeviceType)
			if err != nil {
				logs.Errorf("failed to check device matched match, err: %v, key: %+v, remainCpuCoreMap: %+v, "+
					"need: %+v, rid: %s", err, key, remainCpuCoreMap, need, kt.Rid)
				return VerifyResPlanResElem{}, err
			}

			if !matched[0] {
				continue
			}

			isSkip, skipReason := isSkipDemandMatch(requireType, key, need, planType)
			// 当预测的关键属性（例如业务、项目类型、预测内外）不匹配时，可以硬性跳过，不提醒用户预测不匹配
			if isSkip && skipReason == "" {
				continue
			}
			skipReasonCoreMap[key] = make(map[string]int64)

			for demandID, remainCpuCore := range remainCpuCoreMap {
				if remainCpuCore <= 0 || needCpuCore <= 0 {
					continue
				}
				if isSkip {
					skipReasonCoreMap[key][skipReason] += remainCpuCore
					continue
				}
				allResPlanCore += remainCpuCore
				canConsume := max(min(remainCpuCore, needCpuCore), 0)
				needCpuCore -= canConsume
				prodRemain[key][demandID] -= canConsume
				matchDemandIDs = append(matchDemandIDs, demandID)
			}
			logs.Infof("get res plan demand match loop, requireType: %d, allResPlanCore: %d, key: %+v, "+
				"remainCpuCoreMap: %+v, prodRemain[key]: %+v, needCpuCore: %d, need: %+v, rid: %s", requireType,
				allResPlanCore, key, remainCpuCoreMap, prodRemain[key], needCpuCore, need, kt.Rid)
		}
	}
	logs.Infof("get res plan demand match, not match skip core: %+v, rid: %s", skipReasonCoreMap, kt.Rid)

	if needCpuCore != 0 {
		verifyRes := VerifyResPlanResElem{
			VerifyResult: enumor.VerifyResPlanRstFailed,
		}

		// 如果用没匹配上的预测尝试够用，推荐用户用该方案
		for _, skipReason := range skipReasonCoreMap {
			for reason, skipCpu := range skipReason {
				if need.CpuCore <= skipCpu {
					verifyRes.Reason = reason
					return verifyRes, nil
				}
			}
		}

		// 没匹配上的核心数也不够，返回差多少核心
		verifyRes.NeedCPUCore = need.CpuCore
		verifyRes.ResPlanCore = allResPlanCore
		return verifyRes, nil
	}

	return VerifyResPlanResElem{VerifyResult: enumor.VerifyResPlanRstPass, MatchDemandIDs: matchDemandIDs}, nil
}

// isSkipDemandMatch 是否跳过不进行预测匹配
func isSkipDemandMatch(requireType enumor.RequireType, key ResPlanPoolKeyV2, need VerifyResPlanElemV2,
	planType enumor.PlanTypeCode) (bool, string) {

	// 机房裁撤支持预测内、预测外都可以选择包年包月，可以忽略预测内、预测外 --story=121848852
	if requireType == enumor.RequireTypeDissolve {
		return isDiffDemandMatch(key, need)
	} else {
		if key.PlanType != planType {
			return true, ""
		}
		return isDiffDemandMatch(key, need)
	}
}

// isDiffDemandMatch 比较预测单是否有不相同的参数，并返回不匹配原因
func isDiffDemandMatch(key ResPlanPoolKeyV2, need VerifyResPlanElemV2) (bool, string) {
	// 这些不匹配原因是显而易见的，不需要再显式返回原因
	if key.AvailableTime != need.AvailableTime || key.BkBizID != need.BkBizID || key.RegionID != need.RegionID ||
		key.ObsProject != need.ObsProject {
		return true, ""
	}

	if key.DemandClass != need.DemandClass {
		return true, enumor.DemandClassIsNotMatch.GenerateMsg(string(need.DemandClass), string(key.DemandClass))
	}

	return false, ""
}

// GetAllDeviceTypeMap get all device type map.
func (c *Controller) GetAllDeviceTypeMap(kt *kit.Kit) (map[string]dt.DistinctDeviceType, error) {
	// get all device type maps.
	deviceTypeMap, err := c.deviceTypesMap.GetDeviceTypes(kt)
	if err != nil {
		logs.Errorf("get all device type map failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	return deviceTypeMap, nil
}

// SyncBudgetOperatorByTime 同步 admin 单据的预算提报人。
func (c *Controller) SyncBudgetOperatorByTime(kt *kit.Kit, start, end time.Time) (
	*ptypes.BudgetOperatorSyncResp, error) {
	resp := &ptypes.BudgetOperatorSyncResp{}

	demands, err := c.collectBudgetOperatorDemands(kt, start, end)
	if err != nil {
		logs.Errorf("collect budget operator demands failed, err: %v, start: %v, end: %v, rid: %s", err, start, end,
			kt.Rid)
		return resp, err
	}

	resp.TotalCount = len(demands)
	if resp.TotalCount == 0 {
		return resp, nil
	}

	groups := groupBudgetDemands(demands)

	// Batch fetch all budget operator candidates from FinOps
	candidateCache, err := c.batchFetchBudgetOperatorCandidates(kt, groups)
	if err != nil {
		logs.Errorf("batch fetch budget operator candidates failed, err: %v, rid: %s", err, kt.Rid)
		resp.ProcessedCount = resp.SuccessCount + resp.FailedCount + resp.SkippedCount
		return resp, err
	}

	for _, group := range groups {
		key := fmt.Sprintf("%d-%d", group.OpProductID, group.Year)
		candidate, exists := candidateCache[key]
		if !exists || candidate == "" {
			for _, demand := range group.Items {
				resp.SkippedCount++
				resp.SkippedDemandIDs = append(resp.SkippedDemandIDs, demand.ID)
			}
			continue
		}

		c.processBudgetDemands(kt, group.Items, candidate, resp)
	}

	resp.ProcessedCount = resp.SuccessCount + resp.FailedCount + resp.SkippedCount
	return resp, nil
}

func groupBudgetDemands(demands []budgetDemand) map[string]*demandGroup {
	groups := make(map[string]*demandGroup)
	for i := range demands {
		demand := &demands[i]
		key := fmt.Sprintf("%d-%04d-%02d", demand.OpProductID,
			demand.ExpectTime.Year(), demand.ExpectTime.Month())
		group, exists := groups[key]
		if !exists {
			group = &demandGroup{
				OpProductID: demand.OpProductID,
				Year:        demand.ExpectTime.Year(),
			}
			groups[key] = group
		}
		group.Items = append(group.Items, demand)
	}

	return groups
}

// batchFetchBudgetOperatorCandidates 批量获取所有分组的预算提报人候选
func (c *Controller) batchFetchBudgetOperatorCandidates(kt *kit.Kit,
	groups map[string]*demandGroup) (map[string]string, error) {
	years, opProductIDs := collectDistinctYearsAndOpIDs(groups)
	if len(years) == 0 {
		return nil, nil
	}

	candidateCache := make(map[string]string)
	const maxYearsPerBatch = 20

	for i := 0; i < len(years); i += maxYearsPerBatch {
		end := i + maxYearsPerBatch
		if end > len(years) {
			end = len(years)
		}
		batchYears := years[i:end]

		resp, err := c.callFinOpsAPIWithRetry(kt, batchYears, opProductIDs)
		if err != nil {
			logs.Errorf("batch get budget declaration operator from finops failed, "+
				"err: %v, years: %v, op_product_ids: %v, rid: %s", err, batchYears, opProductIDs, kt.Rid)
			return nil, err
		}

		buildCandidateCache(kt, resp, candidateCache)
	}

	return candidateCache, nil
}

// collectDistinctYearsAndOpIDs 收集并去重年份和运营产品ID
func collectDistinctYearsAndOpIDs(groups map[string]*demandGroup) ([]int, []int64) {
	opProductYearSet := make(map[int64]map[int]bool)
	for _, group := range groups {
		if _, exists := opProductYearSet[group.OpProductID]; !exists {
			opProductYearSet[group.OpProductID] = make(map[int]bool)
		}
		opProductYearSet[group.OpProductID][group.Year] = true
	}

	yearSet := make(map[int]bool)
	opIDSet := make(map[int64]bool)
	for opID, yearMap := range opProductYearSet {
		opIDSet[opID] = true
		for year := range yearMap {
			yearSet[year] = true
		}
	}

	return maps.Keys(yearSet), maps.Keys(opIDSet)
}

// callFinOpsAPIWithRetry 带重试地调用 FinOps API
func (c *Controller) callFinOpsAPIWithRetry(kt *kit.Kit, years []int, opProductIDs []int64) (
	*finops.GetBudgetDeclarationOperatorResult, error) {
	param := &finops.GetBudgetDeclarationOperatorParam{
		Years:        years,
		OpProductIDs: opProductIDs,
	}

	var resp *finops.GetBudgetDeclarationOperatorResult
	var err error
	for retry := 0; retry < 3; retry++ {
		resp, err = c.finOpsCli.GetBudgetDeclarationOperator(kt, param)
		if err == nil {
			break
		}
		logs.Warnf("get budget declaration operator from finops failed, retry: %d, err: %v, rid: %s",
			retry, err, kt.Rid)
		time.Sleep(time.Second)
	}

	return resp, err
}

// buildCandidateCache 从 FinOps 响应构建候选人缓存
func buildCandidateCache(kt *kit.Kit, resp *finops.GetBudgetDeclarationOperatorResult,
	candidateCache map[string]string) {

	for _, item := range resp.Items {
		for _, comp := range item.Composition {
			key := fmt.Sprintf("%d-%d", comp.OpProductID, item.Year)
			logs.Infof("processing budget operator, op_product_id: %d, year: %d, creators: %v, committers: %v, rid: %s",
				comp.OpProductID, item.Year, comp.Creators, comp.Committers, kt.Rid)

			candidates := deduplicateBudgetOperatorCandidates(comp.Creators, comp.Committers)
			for _, operator := range candidates {
				candidateCache[key] = operator
				logs.Infof("budget operator candidate cached, key: %s, candidate: %s, rid: %s", key, operator, kt.Rid)
				break // 只取第一个
			}
		}
	}
}

func (c *Controller) collectBudgetOperatorDemands(kt *kit.Kit, start, end time.Time) ([]budgetDemand, error) {
	results := make([]budgetDemand, 0)

	page := core.NewDefaultBasePage()
	page.Limit = constant.BudgetOperatorSyncPageLimit
	page.Sort = "expect_time"
	page.Order = core.Ascending
	listFilter := tools.ExpressionAnd(
		tools.RuleEqual("creator", constant.BackendOperationUserKey),
		tools.RuleGreaterThanEqual("expect_time", times.ConvTimeToCompactInt(start)),
		tools.RuleLessThan("expect_time", times.ConvTimeToCompactInt(end.AddDate(0, 0, 1))),
	)

	for {
		req := &rpproto.ResPlanDemandListReq{
			ListReq: core.ListReq{
				Filter: listFilter,
				Fields: []string{"id", "creator", "reviser", "expect_time", "op_product_id"},
				Page:   page,
			},
		}
		resp, err := c.client.DataService().Global.ResourcePlan.ListResPlanDemand(kt, req)
		if err != nil {
			logs.Errorf("list res plan demand for budget operator sync failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		for _, item := range resp.Details {
			results = append(results, budgetDemand{
				ID:          item.ID,
				Creator:     item.Creator,
				Reviser:     item.Reviser,
				ExpectTime:  times.ConvCompactIntToTime(item.ExpectTime),
				OpProductID: item.OpProductID,
			})
		}

		if len(resp.Details) < int(page.Limit) {
			break
		}
		page.Start += uint32(page.Limit)
	}

	return results, nil
}

func deduplicateBudgetOperatorCandidates(operatorLists ...[]string) []string {
	seen := make(map[string]struct{})
	operators := make([]string, 0)
	for _, operatorList := range operatorLists {
		for _, operator := range operatorList {
			operator = strings.TrimSpace(operator)
			if operator == "" || operator == constant.BackendOperationUserKey {
				continue
			}
			if _, ok := seen[operator]; ok {
				continue
			}
			seen[operator] = struct{}{}
			operators = append(operators, operator)
		}
	}
	return operators
}

func (c *Controller) processBudgetDemands(kt *kit.Kit, demands []*budgetDemand, candidate string,
	resp *ptypes.BudgetOperatorSyncResp) {
	reviser := kt.User
	if reviser == "" {
		reviser = constant.BackendOperationUserKey
	}

	// 收集需要更新的 demand IDs
	toUpdateIDs := make([]string, 0, len(demands))
	for _, demand := range demands {
		current := strings.TrimSpace(demand.Creator)
		if current != "" && current != constant.BackendOperationUserKey {
			resp.SkippedCount++
			resp.SkippedDemandIDs = append(resp.SkippedDemandIDs, demand.ID)
			continue
		}

		if current == candidate {
			resp.SkippedCount++
			resp.SkippedDemandIDs = append(resp.SkippedDemandIDs, demand.ID)
			continue
		}

		toUpdateIDs = append(toUpdateIDs, demand.ID)
	}

	if len(toUpdateIDs) == 0 {
		return
	}

	// 批量更新，按 BudgetOperatorSyncBatchSize 分批
	for i := 0; i < len(toUpdateIDs); i += constant.BudgetOperatorSyncBatchSize {
		if i > 0 {
			time.Sleep(200 * time.Millisecond)
		}

		end := i + constant.BudgetOperatorSyncBatchSize
		if end > len(toUpdateIDs) {
			end = len(toUpdateIDs)
		}
		batchIDs := toUpdateIDs[i:end]

		updateReq := &rpproto.ResPlanDemandBatchUpdateCreatorReq{
			IDs:     batchIDs,
			Creator: candidate,
			Reviser: reviser,
		}
		updateResp, err := c.client.DataService().Global.ResourcePlan.ResPlanDemandBatchUpdateCreator(kt, updateReq)
		if err != nil {
			resp.FailedCount += len(batchIDs)
			resp.FailedDemandIDs = append(resp.FailedDemandIDs, batchIDs...)
			logs.Errorf("batch update budget operator creator failed, err: %v, demand_ids: %v, rid: %s",
				err, batchIDs, kt.Rid)
			continue
		}

		// 更新成功数和跳过数
		resp.SuccessCount += int(updateResp.UpdatedCount)
		skippedInBatch := len(batchIDs) - int(updateResp.UpdatedCount)
		if skippedInBatch > 0 {
			resp.SkippedCount += skippedInBatch
			// 注：这里无法精确知道哪些是 skipped，只能记录批次信息
			logs.Infof("batch update partial skip, batch_size: %d, updated: %d, skipped: %d, rid: %s",
				len(batchIDs), updateResp.UpdatedCount, skippedInBatch, kt.Rid)
		}
	}
}
