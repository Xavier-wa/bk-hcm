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
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/api/core"
	dt "hcm/pkg/api/core/cloud/device-type"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	rpd "hcm/pkg/dal/table/resource-plan/res-plan-demand"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/times"

	"github.com/shopspring/decimal"
)

// demandGroupKey 用于按 (region_id, zone_id) 分组查询通配机型
type demandGroupKey struct {
	RegionID string
	ZoneID   string
}

// demandGroupResult 存储每个分组的通配机型查询结果
type demandGroupResult struct {
	DeviceTypes []string
	Err         error
}

// aggregateDemands 对预测列表按聚合键进行聚合。
// 返回聚合后的列表、以首条 demand_id 为 key 的原始机型列表映射，以及以首条 demand_id 为 key 的消耗池 key 映射。
func (c *Controller) aggregateDemands(kt *kit.Kit, demands []*ptypes.ListResPlanDemandWithDeviceTypesItem) (
	[]*ptypes.ListResPlanDemandWithDeviceTypesItem, map[string][]string,
	map[string]ptypes.ResPlanDemandExpendKey, error) {

	// 按聚合键分组，保持稳定顺序
	groupOrder := make([]ptypes.ResPlanDemandExpendKey, 0)
	groups := make(map[ptypes.ResPlanDemandExpendKey][]*ptypes.ListResPlanDemandWithDeviceTypesItem)

	for _, demand := range demands {
		// 构造完整的消耗池 key（与 getDemandExpendKeyFromTable 逻辑一致）
		demandKey, err := c.buildDemandExpendKey(kt, demand)
		if err != nil {
			return nil, nil, nil, err
		}
		if _, exists := groups[demandKey]; !exists {
			groupOrder = append(groupOrder, demandKey)
		}
		groups[demandKey] = append(groups[demandKey], demand)
	}

	// aggregationDeviceTypesMap: 以首条 demand_id 为 key，存储被聚合预测的所有原始机型（去重）
	aggregationDeviceTypesMap := make(map[string][]string)
	demandKeyMap := make(map[string]ptypes.ResPlanDemandExpendKey)
	result := make([]*ptypes.ListResPlanDemandWithDeviceTypesItem, 0, len(groups))

	for _, key := range groupOrder {
		group := groups[key]
		if len(group) == 1 {
			item := group[0]
			firstDemandID := item.DemandIDs[0]
			result = append(result, item)
			demandKeyMap[firstDemandID] = key
			aggregationDeviceTypesMap[firstDemandID] = []string{item.OriginalDeviceType}
			continue
		}

		aggregated, deviceTypes := aggregateDemandGroup(group)
		result = append(result, aggregated)
		firstDemandID := aggregated.DemandIDs[0]
		aggregationDeviceTypesMap[firstDemandID] = deviceTypes
		demandKeyMap[firstDemandID] = key
	}

	return result, aggregationDeviceTypesMap, demandKeyMap, nil
}

// buildDemandExpendKey 构造消耗池 key，逻辑与 getDemandExpendKeyFromTable 一致，
// 但直接从 ListResPlanDemandItem 取值，无需再转回 rpd.ResPlanDemandTable。
func (c *Controller) buildDemandExpendKey(kt *kit.Kit, item *ptypes.ListResPlanDemandWithDeviceTypesItem) (
	ptypes.ResPlanDemandExpendKey, error) {

	t, err := time.Parse(constant.DateLayout, item.ExpectTime)
	if err != nil {
		logs.Errorf("failed to parse demand expect time, err: %v, expect_time: %s, rid: %s", err,
			item.ExpectTime, kt.Rid)
		return ptypes.ResPlanDemandExpendKey{}, err
	}

	availableYear, availableMonth, err := c.demandTime.GetDemandYearMonth(kt, t)
	if err != nil {
		logs.Errorf("failed to get demand year month, err: %v, expect_time: %s, rid: %s", err,
			item.ExpectTime, kt.Rid)
		return ptypes.ResPlanDemandExpendKey{}, err
	}

	key := ptypes.ResPlanDemandExpendKey{
		DemandClass:   item.DemandClass,
		BkBizID:       item.BkBizID,
		PlanType:      item.PlanType.GetCode(),
		AvailableTime: ptypes.NewAvailableMonth(availableYear, availableMonth),
		DeviceFamily:  item.DeviceFamily,
		CoreType:      string(item.CoreType),
		ObsProject:    item.ObsProject,
		RegionID:      item.RegionID,
	}

	// 机房裁撤的预测消耗忽略预测内、预测外 --story=121848852
	if enumor.IsDissolveObsProjectForResPlan(item.ObsProject) {
		key.PlanType = ""
	}

	return key, nil
}

// aggregateDemandGroup 对一组预测进行聚合，以第一条记录为基础，累加数值字段。
// 返回聚合后的记录以及被聚合预测的所有原始机型（去重）。
func aggregateDemandGroup(group []*ptypes.ListResPlanDemandWithDeviceTypesItem) (
	*ptypes.ListResPlanDemandWithDeviceTypesItem, []string) {

	result := *group[0]

	// 重置数值字段，然后累加所有记录
	result.TotalCpuCore = 0
	result.TotalOS = decimal.NewFromInt(0)
	result.TotalMemory = 0
	result.TotalDiskSize = 0

	deviceTypesSet := make(map[string]struct{})
	demandIDs := make([]string, 0, len(group))

	for _, demand := range group {
		result.TotalCpuCore += demand.TotalCpuCore
		result.TotalOS = result.TotalOS.Add(demand.TotalOS)
		result.TotalMemory += demand.TotalMemory
		result.TotalDiskSize += demand.TotalDiskSize

		deviceTypesSet[demand.OriginalDeviceType] = struct{}{}
		demandIDs = append(demandIDs, demand.DemandIDs...)
	}

	// 初始化消耗字段（消耗计算会在后续步骤更新）
	result.AppliedCpuCore = 0
	result.AppliedOS = decimal.NewFromInt(0)
	result.AppliedMemory = 0
	result.RemainedCpuCore = result.TotalCpuCore
	result.RemainedOS = result.TotalOS
	result.RemainedMemory = result.TotalMemory
	result.RemainedDiskSize = result.TotalDiskSize

	// demand_ids 字段
	result.DemandIDs = demandIDs

	// 去重后的原始机型列表
	allDeviceTypes := make([]string, 0, len(deviceTypesSet))
	for dType := range deviceTypesSet {
		allDeviceTypes = append(allDeviceTypes, dType)
	}

	return &result, allDeviceTypes
}

// calculateSingleDemandConsumption 计算单条预测的消耗
func calculateSingleDemandConsumption(demandItem *ptypes.ListResPlanDemandWithDeviceTypesItem,
	planAppliedCore map[ptypes.ResPlanDemandExpendKey]int64,
	demandKey ptypes.ResPlanDemandExpendKey,
	deviceTypes map[string]dt.DistinctDeviceType) {

	matchPlanType := []enumor.PlanTypeCode{demandKey.PlanType, enumor.PlanTypeCodeIgnore}
	for _, planType := range matchPlanType {
		demandKey.PlanType = planType
		if allAppliedCPUCore, ok := planAppliedCore[demandKey]; ok {
			demandAppliedCPUCore := min(allAppliedCPUCore, demandItem.RemainedCpuCore)
			demandItem.AppliedCpuCore += demandAppliedCPUCore
			planAppliedCore[demandKey] -= demandAppliedCPUCore

			deviceInfo := deviceTypes[demandItem.OriginalDeviceType]
			deviceCPUCore := decimal.NewFromInt(deviceInfo.CpuCore)
			deviceMemory := decimal.NewFromInt(deviceInfo.Memory)
			demandItem.AppliedOS = decimal.NewFromInt(demandItem.AppliedCpuCore).Div(deviceCPUCore)
			demandItem.AppliedMemory = demandItem.AppliedOS.Mul(deviceMemory).IntPart()
		}
		demandItem.RemainedOS = demandItem.TotalOS.Sub(demandItem.AppliedOS)
		demandItem.RemainedCpuCore = demandItem.TotalCpuCore - demandItem.AppliedCpuCore
		demandItem.RemainedMemory = demandItem.TotalMemory - demandItem.AppliedMemory
		if demandItem.RemainedCpuCore <= 0 {
			break
		}
	}
}

// ListResPlanDemandWithDeviceTypes list res plan demand with device types.
func (c *Controller) ListResPlanDemandWithDeviceTypes(kt *kit.Kit, req *ptypes.ListResPlanDemandWithDeviceTypesReq) (
	*ptypes.ListResPlanDemandWithDeviceTypesResp, error) {

	listReq, err := c.buildDemandWithDeviceTypesListReq(kt, req)
	if err != nil {
		return nil, err
	}

	demandList, prodConsumePool, err := c.fetchDemandAndConsumePool(kt, listReq)
	if err != nil {
		return nil, err
	}

	deviceTypeMap, err := c.deviceTypesMap.GetDeviceTypes(kt)
	if err != nil {
		logs.Errorf("failed to get device type map, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// Step 1: 数据转换（不计算消耗）
	demandItems, err := convDemandTablesToItems(kt, demandList)
	if err != nil {
		return nil, err
	}

	// Step 2: 预测数据聚合
	aggregatedItems, aggregationDeviceTypesMap, demandKeyMap, err := c.aggregateDemands(kt, demandItems)
	if err != nil {
		return nil, err
	}

	// Step 3: 消耗计算
	planAppliedCore := convResConsumePoolToExpendMap(kt, prodConsumePool, deviceTypeMap)
	c.calcDemandConsumptions(aggregatedItems, planAppliedCore, demandKeyMap, deviceTypeMap)

	// Step 4: 状态计算、过滤
	filteredItems, err := c.filterDemandItems(kt, aggregatedItems, demandList, req)
	if err != nil {
		return nil, err
	}

	// Step 5: 并发查询通配机型，展开明细
	groupResults := c.queryWildcardDeviceTypes(kt, filteredItems)
	expandedItems := make([]*ptypes.ListResPlanDemandWithDeviceTypesItem, 0)
	for _, demandItem := range filteredItems {
		expandedItems = append(expandedItems, c.expandDemandToDeviceTypes(kt, demandItem, deviceTypeMap,
			groupResults, req, aggregationDeviceTypesMap)...)
	}

	// 排序：is_original 降序（true 在前），同组内按 device_type 升序
	sort.Slice(expandedItems, func(i, j int) bool {
		if expandedItems[i].IsOriginal != expandedItems[j].IsOriginal {
			return expandedItems[i].IsOriginal
		}
		return expandedItems[i].DeviceType < expandedItems[j].DeviceType
	})

	if req.Page.Count {
		return &ptypes.ListResPlanDemandWithDeviceTypesResp{Count: uint64(len(expandedItems))}, nil
	}

	return &ptypes.ListResPlanDemandWithDeviceTypesResp{
		Count:   uint64(len(expandedItems)),
		Details: pageDeviceTypeDemands(req.Page, expandedItems),
	}, nil
}

// convDemandTablesToItems 将数据库表结构批量转换为 ListResPlanDemandWithDeviceTypesItem（不计算消耗）
func convDemandTablesToItems(kt *kit.Kit, demandList []rpd.ResPlanDemandTable) (
	[]*ptypes.ListResPlanDemandWithDeviceTypesItem, error) {

	demandItems := make([]*ptypes.ListResPlanDemandWithDeviceTypesItem, 0, len(demandList))
	for i := range demandList {
		table := demandList[i]
		expectDateStr, err := times.TransTimeStrWithLayout(strconv.Itoa(table.ExpectTime), constant.DateLayoutCompact,
			constant.DateLayout)
		if err != nil {
			logs.Errorf("failed to parse demand expect time, err: %v, expect_time: %d, rid: %s", err,
				table.ExpectTime, kt.Rid)
			return nil, err
		}
		returnTimePtr := parseReturnPlanTime(kt, table)
		demandItems = append(demandItems, convDemandTableToItem(table, expectDateStr, returnTimePtr))
	}
	return demandItems, nil
}

// convDemandTableToItem 将单条 ResPlanDemandTable 转换为 ListResPlanDemandWithDeviceTypesItem
func convDemandTableToItem(table rpd.ResPlanDemandTable, expectTime string,
	returnPlanTime *string) *ptypes.ListResPlanDemandWithDeviceTypesItem {

	return &ptypes.ListResPlanDemandWithDeviceTypesItem{
		ListResPlanDemandItemBase: ptypes.ListResPlanDemandItemBase{
			DemandClass:     table.DemandClass,
			ExpectTime:      expectTime,
			ReturnPlanTime:  returnPlanTime,
			TotalCpuCore:    cvt.PtrToVal(table.CpuCore),
			RemainedCpuCore: cvt.PtrToVal(table.CpuCore),
		},

		DemandIDs:      []string{table.ID},
		BkBizID:        table.BkBizID,
		BkBizName:      table.BkBizName,
		DemandResType:  table.DemandResType,
		RegionID:       table.RegionID,
		RegionName:     table.RegionName,
		ZoneID:         table.ZoneID,
		ZoneName:       table.ZoneName,
		PlanType:       table.PlanType.Name(),
		ObsProject:     table.ObsProject,
		TechnicalClass: table.TechnicalClass,
		DeviceFamily:   table.DeviceFamily,
		CoreType:       table.CoreType,
		DiskType:       table.DiskType,
		DiskTypeName:   table.DiskType.Name(),
		DiskIO:         table.DiskIO,

		OriginalDeviceType: table.DeviceType,
		DeviceClass:        table.DeviceClass,
		TotalOS:            table.OS.Decimal,
		RemainedOS:         table.OS.Decimal,
		TotalMemory:        cvt.PtrToVal(table.Memory),
		TotalDiskSize:      cvt.PtrToVal(table.DiskSize),
		RemainedDiskSize:   cvt.PtrToVal(table.DiskSize),
	}
}

// parseReturnPlanTime 解析短租项目的预期退回时间，非短租返回 nil
func parseReturnPlanTime(kt *kit.Kit, table rpd.ResPlanDemandTable) *string {
	if table.ObsProject != enumor.ObsProjectShortLease {
		return nil
	}
	returnTime, err := times.TransTimeStrWithLayout(strconv.Itoa(table.ReturnPlanTime),
		constant.DateLayoutCompact, constant.DateLayout)
	if err != nil {
		logs.Warnf("failed to parse demand return plan time, err: %v, return_plan_time: %d, rid: %s", err,
			table.ReturnPlanTime, kt.Rid)
		return nil
	}
	return &returnTime
}

// calcDemandConsumptions 对聚合后的预测列表执行消耗计算
func (c *Controller) calcDemandConsumptions(items []*ptypes.ListResPlanDemandWithDeviceTypesItem,
	planAppliedCore map[ptypes.ResPlanDemandExpendKey]int64, demandKeyMap map[string]ptypes.ResPlanDemandExpendKey,
	deviceTypeMap map[string]dt.DistinctDeviceType) {

	for _, item := range items {
		firstDemandID := item.DemandIDs[0]
		demandKey := demandKeyMap[firstDemandID]
		calculateSingleDemandConsumption(item, planAppliedCore, demandKey, deviceTypeMap)
	}
}

// filterDemandItems 对聚合后的预测列表进行状态计算、过滤，返回满足条件的预测列表
func (c *Controller) filterDemandItems(kt *kit.Kit, items []*ptypes.ListResPlanDemandWithDeviceTypesItem,
	demandList []rpd.ResPlanDemandTable, req *ptypes.ListResPlanDemandWithDeviceTypesReq) (
	[]*ptypes.ListResPlanDemandWithDeviceTypesItem, error) {

	filtered := make([]*ptypes.ListResPlanDemandWithDeviceTypesItem, 0, len(items))
	for _, item := range items {
		firstDemandID := item.DemandIDs[0]
		lockedStatus := findDemandLockedStatus(firstDemandID, demandList)

		belong, err := c.aggregateDemandBelongListReq(kt, item, req)
		if err != nil {
			logs.Errorf("failed to check demand belong, err: %v, demand: %s, rid: %s", err, firstDemandID, kt.Rid)
			return nil, err
		}
		if !belong {
			continue
		}

		c.setDemandStatus(kt, firstDemandID, lockedStatus, &item.ListResPlanDemandItemBase)
		if len(req.Statuses) > 0 && !slices.Contains(req.Statuses, item.Status) {
			continue
		}

		filtered = append(filtered, item)
	}
	return filtered, nil
}

// findDemandLockedStatus 在 demandList 中查找指定 demand 的 locked 状态
func findDemandLockedStatus(demandID string, demandList []rpd.ResPlanDemandTable) enumor.CrpDemandLockStatus {
	for _, table := range demandList {
		if table.ID == demandID {
			return cvt.PtrToVal(table.Locked)
		}
	}
	return enumor.CrpDemandUnLocked
}

func (c *Controller) aggregateDemandBelongListReq(kt *kit.Kit, demandItem *ptypes.ListResPlanDemandWithDeviceTypesItem,
	req *ptypes.ListResPlanDemandWithDeviceTypesReq) (bool, error) {

	if !req.CheckObsProjects(demandItem.ObsProject) {
		return false, nil
	}
	if !req.CheckDeviceClasses(demandItem.DeviceClass) {
		return false, nil
	}
	if !req.CheckDeviceTypes(demandItem.OriginalDeviceType) {
		return false, nil
	}
	if !req.CheckRegionIDs(demandItem.RegionID) {
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

// buildDemandWithDeviceTypesListReq 构造内部查询请求（硬编码 demand_class=CVM）并扩展时间范围
func (c *Controller) buildDemandWithDeviceTypesListReq(kt *kit.Kit,
	req *ptypes.ListResPlanDemandWithDeviceTypesReq) (*ptypes.ListResPlanDemandReq, error) {

	listReq := &ptypes.ListResPlanDemandReq{
		BkBizIDs:        []int64{req.BkBizID},
		ObsProjects:     req.ObsProjects,
		DemandClasses:   []enumor.DemandClass{enumor.DemandClassCVM},
		CoreTypes:       req.CoreTypes,
		DeviceFamilies:  req.DeviceFamilies,
		DeviceClasses:   req.DeviceClasses,
		DeviceTypes:     req.DeviceTypes,
		RegionIDs:       req.RegionIDs,
		PlanTypes:       req.PlanTypes,
		ExpiringOnly:    req.ExpiringOnly,
		ExpectTimeRange: req.ExpectTimeRange,
		Statuses:        req.Statuses,
		Page:            req.Page,
	}

	extendReq, err := c.extendResPlanListReq(kt, listReq)
	if err != nil {
		logs.Errorf("failed to extend res plan list req, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return extendReq, nil
}

// fetchDemandAndConsumePool Layer 1：并发查询预测需求列表和消耗池
func (c *Controller) fetchDemandAndConsumePool(kt *kit.Kit, extendReq *ptypes.ListResPlanDemandReq) (
	[]rpd.ResPlanDemandTable, ResPlanConsumePool, error) {

	var (
		demandList      []rpd.ResPlanDemandTable
		prodConsumePool ResPlanConsumePool
		demandErr       error
		consumeErr      error
		wg              sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		demandList, _, demandErr = c.listAllResPlanDemand(kt, extendReq)
	}()
	go func() {
		defer wg.Done()
		startDay, endDay, parseErr := extendReq.ExpectTimeRange.GetTimeDate()
		if parseErr != nil {
			consumeErr = parseErr
			return
		}
		prodConsumePool, consumeErr = c.GetProdResConsumePoolV2(kt, extendReq.BkBizIDs, startDay, endDay)
	}()
	wg.Wait()

	if demandErr != nil {
		logs.Errorf("failed to list all res plan demand, err: %v, rid: %s", demandErr, kt.Rid)
		return nil, nil, demandErr
	}
	if consumeErr != nil {
		logs.Errorf("failed to get prod res consume pool v2, err: %v, rid: %s", consumeErr, kt.Rid)
		return nil, nil, consumeErr
	}

	return demandList, prodConsumePool, nil
}

// queryWildcardDeviceTypes Layer 2：按 (region_id, zone_id) 分组，并发查询通配机型
func (c *Controller) queryWildcardDeviceTypes(kt *kit.Kit,
	demandItems []*ptypes.ListResPlanDemandWithDeviceTypesItem) map[demandGroupKey]*demandGroupResult {

	groupMap := make(map[demandGroupKey][]*ptypes.ListResPlanDemandWithDeviceTypesItem)
	for _, item := range demandItems {
		key := demandGroupKey{RegionID: item.RegionID, ZoneID: item.ZoneID}
		groupMap[key] = append(groupMap[key], item)
	}

	groupResults := make(map[demandGroupKey]*demandGroupResult)
	var resultMu sync.Mutex
	var queryWg sync.WaitGroup

	for key := range groupMap {
		queryWg.Add(1)
		go func(k demandGroupKey) {
			defer queryWg.Done()
			matchedTypes, err := c.getMatchedDeviceTypes(kt, k.RegionID, k.ZoneID)
			resultMu.Lock()
			groupResults[k] = &demandGroupResult{DeviceTypes: matchedTypes, Err: err}
			resultMu.Unlock()
		}(key)
	}
	queryWg.Wait()

	return groupResults
}

// expandDemandToDeviceTypes 将单条（或聚合后的）预测记录展开为多条机型明细记录。
// aggregationDeviceTypesMap 用于获取聚合预测的原始机型列表。
func (c *Controller) expandDemandToDeviceTypes(kt *kit.Kit, demandItem *ptypes.ListResPlanDemandWithDeviceTypesItem,
	deviceTypeMap map[string]dt.DistinctDeviceType, groupResults map[demandGroupKey]*demandGroupResult,
	req *ptypes.ListResPlanDemandWithDeviceTypesReq,
	aggregationDeviceTypesMap map[string][]string) []*ptypes.ListResPlanDemandWithDeviceTypesItem {

	result := make([]*ptypes.ListResPlanDemandWithDeviceTypesItem, 0)

	// 确定原始机型列表（单条和聚合记录均已存入 aggregationDeviceTypesMap）
	firstDemandID := demandItem.DemandIDs[0]
	originalDeviceTypes := aggregationDeviceTypesMap[firstDemandID]

	// 1. 处理所有原始机型（is_original=true），应用 cpu_cores/memories 筛选
	for _, originalDeviceType := range originalDeviceTypes {
		originalDeviceInfo, ok := deviceTypeMap[originalDeviceType]
		if !ok {
			logs.Warnf("original device type %s not found in device_type table, fallback to demand origin fields, rid: %s",
				originalDeviceType, kt.Rid)
			if originalDeviceType == demandItem.OriginalDeviceType {
				result = append(result, buildDeviceTypeItemFromDemand(demandItem))
			}
			continue
		}
		if !matchCpuMemFilter(originalDeviceInfo, req) {
			continue
		}
		result = append(result, buildDeviceTypeItem(demandItem, originalDeviceType, true, originalDeviceInfo))
	}

	// 2. 查询并添加通配机型（is_original=false）
	key := demandGroupKey{RegionID: demandItem.RegionID, ZoneID: demandItem.ZoneID}
	groupRes, exists := groupResults[key]
	if !exists || groupRes.Err != nil || len(groupRes.DeviceTypes) == 0 {
		return result
	}

	// 对所有原始机型收集通配机型（去重），排除原始机型自身
	originalSet := make(map[string]struct{}, len(originalDeviceTypes))
	for _, ot := range originalDeviceTypes {
		originalSet[ot] = struct{}{}
	}

	allMatchedTypes := make(map[string]struct{})
	for _, originalType := range originalDeviceTypes {
		matched, err := c.IsDeviceMatched(kt, groupRes.DeviceTypes, originalType)
		if err != nil {
			logs.Errorf("failed to check device matched, err: %v, device_type: %s, rid: %s",
				err, originalType, kt.Rid)
			continue
		}
		for idx, isMatched := range matched {
			if isMatched {
				matchedType := groupRes.DeviceTypes[idx]
				if _, isOriginal := originalSet[matchedType]; !isOriginal {
					allMatchedTypes[matchedType] = struct{}{}
				}
			}
		}
	}

	// 添加通配机型（is_original=false），应用 cpu_cores/memories 筛选
	for matchedType := range allMatchedTypes {
		matchedDeviceInfo, ok := deviceTypeMap[matchedType]
		if !ok {
			continue
		}
		if !matchCpuMemFilter(matchedDeviceInfo, req) {
			continue
		}
		result = append(result, buildDeviceTypeItem(demandItem, matchedType, false, matchedDeviceInfo))
	}

	return result
}

// buildDeviceTypeItem 构建单条机型明细记录
func buildDeviceTypeItem(demandItem *ptypes.ListResPlanDemandWithDeviceTypesItem, deviceType string, isOriginal bool,
	deviceInfo dt.DistinctDeviceType) *ptypes.ListResPlanDemandWithDeviceTypesItem {

	cpuCore := deviceInfo.CpuCore
	memory := deviceInfo.Memory

	// 计算 OS 数：os = cpu_core / cpu_core_per_device（使用 decimal 精确除法）
	var totalOS, appliedOS, remainedOS decimal.Decimal
	if cpuCore > 0 {
		deviceCPUCore := decimal.NewFromInt(cpuCore)
		totalOS = decimal.NewFromInt(demandItem.TotalCpuCore).Div(deviceCPUCore)
		appliedOS = decimal.NewFromInt(demandItem.AppliedCpuCore).Div(deviceCPUCore)
		remainedOS = decimal.NewFromInt(demandItem.RemainedCpuCore).Div(deviceCPUCore)
	}

	return &ptypes.ListResPlanDemandWithDeviceTypesItem{
		ListResPlanDemandItemBase: demandItem.ListResPlanDemandItemBase,

		DemandIDs:      demandItem.DemandIDs,
		BkBizID:        demandItem.BkBizID,
		BkBizName:      demandItem.BkBizName,
		DemandResType:  demandItem.DemandResType,
		RegionID:       demandItem.RegionID,
		RegionName:     demandItem.RegionName,
		ZoneID:         demandItem.ZoneID,
		ZoneName:       demandItem.ZoneName,
		PlanType:       demandItem.PlanType,
		ObsProject:     demandItem.ObsProject,
		TechnicalClass: demandItem.TechnicalClass,
		DeviceFamily:   demandItem.DeviceFamily,
		CoreType:       demandItem.CoreType,
		DiskType:       demandItem.DiskType,
		DiskTypeName:   demandItem.DiskTypeName,
		DiskIO:         demandItem.DiskIO,

		DeviceType:       deviceType,
		IsOriginal:       isOriginal,
		DeviceTypeClass:  string(deviceInfo.DeviceTypeClass),
		DeviceClass:      deviceInfo.DeviceClass,
		CpuCore:          cpuCore,
		Memory:           memory,
		DetailTotalOS:    totalOS,
		DetailAppliedOS:  appliedOS,
		DetailRemainedOS: remainedOS,
	}
}

// buildDeviceTypeItemFromDemand 当原始机型不在 device_type 表中时，
// 用需求表的原始字段（os/device_class）回填，cpu_core/memory 无单机值置 0。
func buildDeviceTypeItemFromDemand(demandItem *ptypes.ListResPlanDemandWithDeviceTypesItem) *ptypes.ListResPlanDemandWithDeviceTypesItem {
	return &ptypes.ListResPlanDemandWithDeviceTypesItem{
		ListResPlanDemandItemBase: demandItem.ListResPlanDemandItemBase,

		DemandIDs:      demandItem.DemandIDs,
		BkBizID:        demandItem.BkBizID,
		BkBizName:      demandItem.BkBizName,
		DemandResType:  demandItem.DemandResType,
		RegionID:       demandItem.RegionID,
		RegionName:     demandItem.RegionName,
		ZoneID:         demandItem.ZoneID,
		ZoneName:       demandItem.ZoneName,
		PlanType:       demandItem.PlanType,
		ObsProject:     demandItem.ObsProject,
		TechnicalClass: demandItem.TechnicalClass,
		DeviceFamily:   demandItem.DeviceFamily,
		CoreType:       demandItem.CoreType,
		DiskType:       demandItem.DiskType,
		DiskTypeName:   demandItem.DiskTypeName,
		DiskIO:         demandItem.DiskIO,

		DeviceType:       demandItem.OriginalDeviceType,
		IsOriginal:       true,
		DeviceClass:      demandItem.DeviceClass,
		DetailTotalOS:    demandItem.TotalOS,
		DetailAppliedOS:  demandItem.AppliedOS,
		DetailRemainedOS: demandItem.RemainedOS,
	}
}

// matchCpuMemFilter 检查机型是否满足 cpu_cores 和 memories 筛选条件
func matchCpuMemFilter(deviceInfo dt.DistinctDeviceType, req *ptypes.ListResPlanDemandWithDeviceTypesReq) bool {
	if len(req.CpuCores) > 0 && !slices.Contains(req.CpuCores, deviceInfo.CpuCore) {
		return false
	}
	if len(req.Memories) > 0 && !slices.Contains(req.Memories, deviceInfo.Memory) {
		return false
	}
	return true
}

// pageDeviceTypeDemands 对展开后的明细记录分页
func pageDeviceTypeDemands(page *core.BasePage,
	demands []*ptypes.ListResPlanDemandWithDeviceTypesItem) []*ptypes.ListResPlanDemandWithDeviceTypesItem {

	if page.Start >= uint32(len(demands)) {
		return []*ptypes.ListResPlanDemandWithDeviceTypesItem{}
	}

	offset := int(page.Start + uint32(page.Limit))
	if offset > len(demands) {
		offset = len(demands)
	}
	return demands[int(page.Start):offset]
}
