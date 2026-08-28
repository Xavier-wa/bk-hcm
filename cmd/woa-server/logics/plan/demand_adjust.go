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
	"errors"
	"fmt"
	"slices"
	"strconv"

	mtypes "hcm/cmd/woa-server/types/meta"
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/api/core"
	dt "hcm/pkg/api/core/cloud/device-type"
	rpproto "hcm/pkg/api/data-service/resource-plan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	rpdtablers "hcm/pkg/dal/table/resource-plan/res-plan-demand"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/maps"
	"hcm/pkg/tools/slice"
	"hcm/pkg/tools/times"

	"github.com/shopspring/decimal"
)

// AdjustBizResPlanDemand adjust biz res plan demand.
func (c *Controller) AdjustBizResPlanDemand(kt *kit.Kit, req *ptypes.AdjustRPDemandReq, bkBizID int64,
	bizOrgRel *mtypes.BizOrgRel) (ticketID string, retErr error) {

	demandIDs := collectNonEmptyAdjustDemandIDs(req.Adjusts)

	if len(demandIDs) > 0 {
		// check whether all crp demand belong to the biz.
		allBelong, err := c.AreAllDemandBelongToBiz(kt, demandIDs, bkBizID)
		if err != nil {
			logs.Errorf("failed to check whether all demand belong to biz, err: %v, rid: %s", err, kt.Rid)
			return "", err
		}

		if !allBelong {
			logs.Errorf("not all adjust demand belong to biz: %d, rid: %s", bkBizID, kt.Rid)
			return "", fmt.Errorf("not all adjust crp demand belong to biz: %d", bkBizID)
		}

		if err = c.validateAdjustDemandsAdjustable(kt, bkBizID, demandIDs); err != nil {
			logs.Errorf("failed to validate adjust demands adjustable, err: %v, rid: %s", err, kt.Rid)
			return "", err
		}
	}

	// examine whether all resource plan demand classes are the same, and get the demand class.
	demandClass, err := c.resolveAdjustDemandClass(kt, req)
	if err != nil {
		logs.Errorf("failed to resolve demand class, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	// construct adjust biz resource plan demand request.
	adjustReq, lockedItems, err := c.constructAdjustReq(kt, bizOrgRel, demandClass, req)
	if err != nil {
		logs.Errorf("failed to construct adjust resource plan ticket request, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	// create cancel resource plan ticket.
	ticketID, err = c.CreateResPlanTicket(kt, adjustReq)
	if err != nil {
		logs.Errorf("failed to create resource plan ticket, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	// lock all resource plan demand.
	for i := range lockedItems {
		lockedItems[i].TicketID = ticketID
	}
	lockReq := &rpproto.ResPlanDemandLockOpReq{
		LockedItems: lockedItems,
	}
	if len(lockedItems) > 0 {
		if err = c.client.DataService().Global.ResourcePlan.LockResPlanDemand(kt, lockReq); err != nil {
			logs.Errorf("failed to lock all resource plan demand, err: %v, demandIDs: %v, rid: %s", err, demandIDs,
				kt.Rid)
			return "", err
		}
	}

	// defer is used to unlock all resource plan demand when some errors occur.
	defer func() {
		if retErr != nil && len(lockedItems) > 0 {
			if tmpErr := c.client.DataService().Global.ResourcePlan.UnlockResPlanDemand(kt, lockReq); tmpErr != nil {
				logs.Errorf("failed to unlock all resource plan demand, err: %v, rid: %s", tmpErr, kt.Rid)
			}
		}
	}()

	// create adjust resource plan ticket itsm audit flow.
	if err = c.CreateAuditFlow(kt, ticketID); err != nil {
		logs.Errorf("failed to create resource plan ticket audit flow, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	return ticketID, nil
}

func collectNonEmptyAdjustDemandIDs(adjusts []ptypes.AdjustRPDemandReqElem) []string {
	demandIDs := make([]string, 0, len(adjusts))
	for _, adjust := range adjusts {
		if adjust.DemandID != "" {
			demandIDs = append(demandIDs, adjust.DemandID)
		}
	}
	return demandIDs
}

// validateAdjustDemandsAdjustable rejects adjust when any referenced demand is not adjustable.
func (c *Controller) validateAdjustDemandsAdjustable(kt *kit.Kit, bkBizID int64, demandIDs []string) error {
	if len(demandIDs) == 0 {
		return nil
	}

	// list by demand ids once to derive expect time range for overview query.
	demands, err := c.listResPlanDemandTablesByIDs(kt, demandIDs)
	if err != nil {
		logs.Errorf("failed to list res plan demand for adjust validation, err: %v, demand_ids: %v, rid: %s",
			err, demandIDs, kt.Rid)
		return err
	}

	expectTimeRange, err := buildExpectTimeRangeFromDemandTables(kt, demands)
	if err != nil {
		return err
	}

	listReq := &ptypes.ListResPlanDemandReq{
		BkBizIDs:        []int64{bkBizID},
		DemandIDs:       demandIDs,
		ExpectTimeRange: expectTimeRange,
		Page:            core.NewDefaultBasePage(),
	}
	rst, err := c.ListResPlanDemandAndOverview(kt, listReq)
	if err != nil {
		logs.Errorf("failed to list res plan demand overview for adjust validation, err: %v, demand_ids: %v, rid: %s",
			err, demandIDs, kt.Rid)
		return err
	}

	statusMap := slice.FuncToMap(rst.Details, func(item *ptypes.ListResPlanDemandItem) (string, enumor.DemandStatus) {
		return item.DemandID, item.Status
	})
	return validateAdjustDemandStatuses(demandIDs, statusMap)
}

func validateAdjustDemandStatuses(demandIDs []string, statusMap map[string]enumor.DemandStatus) error {
	for _, demandID := range demandIDs {
		status, ok := statusMap[demandID]
		if !ok {
			return fmt.Errorf("demand id: %s is not found", demandID)
		}
		switch status {
		case enumor.DemandStatusSpentAll:
			return fmt.Errorf("demand %s is spent all, cannot adjust", demandID)
		case enumor.DemandStatusLocked:
			return fmt.Errorf("demand %s is locked, cannot adjust", demandID)
		}
	}
	return nil
}

// listResPlanDemandTablesByIDs lists res plan demand table rows by demand ids with batching.
func (c *Controller) listResPlanDemandTablesByIDs(kt *kit.Kit, demandIDs []string) (
	[]rpdtablers.ResPlanDemandTable, error) {

	demands := make([]rpdtablers.ResPlanDemandTable, 0, len(demandIDs))
	for _, batchIDs := range slice.Split(demandIDs, int(filter.DefaultMaxInLimit)) {
		listReq := &ptypes.ListResPlanDemandReq{
			DemandIDs: batchIDs,
			Page:      core.NewDefaultBasePage(),
		}
		batchDemands, _, err := c.listAllResPlanDemand(kt, listReq)
		if err != nil {
			logs.Errorf("failed to list res plan demand, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		demands = append(demands, batchDemands...)
	}
	if len(demands) == 0 {
		return nil, fmt.Errorf("demands %v not found", demandIDs)
	}

	return demands, nil
}

// buildExpectTimeRangeFromDemandTables builds expect time range covering all given demand rows.
func buildExpectTimeRangeFromDemandTables(kt *kit.Kit, demands []rpdtablers.ResPlanDemandTable) (
	*times.DateRange, error) {

	if len(demands) == 0 {
		return nil, errors.New("demands is empty")
	}

	minExpectTime := demands[0].ExpectTime
	maxExpectTime := demands[0].ExpectTime
	for _, demand := range demands[1:] {
		if demand.ExpectTime < minExpectTime {
			minExpectTime = demand.ExpectTime
		}
		if demand.ExpectTime > maxExpectTime {
			maxExpectTime = demand.ExpectTime
		}
	}

	start, err := times.TransTimeStrWithLayout(strconv.Itoa(minExpectTime), constant.DateLayoutCompact,
		constant.DateLayout)
	if err != nil {
		logs.Errorf("failed to convert min expect time, err: %v, expect_time: %d, rid: %s", err, minExpectTime, kt.Rid)
		return nil, err
	}
	end, err := times.TransTimeStrWithLayout(strconv.Itoa(maxExpectTime), constant.DateLayoutCompact,
		constant.DateLayout)
	if err != nil {
		logs.Errorf("failed to convert max expect time, err: %v, expect_time: %d, rid: %s", err, maxExpectTime, kt.Rid)
		return nil, err
	}

	return &times.DateRange{Start: start, End: end}, nil
}

// resolveAdjustDemandClass resolves demand class for adjust request.
// When all adjusts are add type without demand_id, request demand_class is required.
// For mixed batch, demand class is derived from existing demands in DB.
func (c *Controller) resolveAdjustDemandClass(kt *kit.Kit, req *ptypes.AdjustRPDemandReq) (enumor.DemandClass,
	error) {

	demandIDs := collectNonEmptyAdjustDemandIDs(req.Adjusts)
	if len(demandIDs) == 0 {
		if req.DemandClass == "" {
			return "", errors.New("demand_class is required when all adjusts are add")
		}
		if err := req.DemandClass.Validate(); err != nil {
			return "", err
		}
		return req.DemandClass, nil
	}

	return c.ExamineDemandClass(kt, demandIDs)
}

func buildAdjustLockItems(demands rpt.ResPlanDemands) []rpproto.ResPlanDemandLockOpItem {
	lockedItems := make([]rpproto.ResPlanDemandLockOpItem, 0, len(demands))
	for _, demand := range demands {
		if demand.Original == nil {
			continue
		}
		lockedItems = append(lockedItems, rpproto.ResPlanDemandLockOpItem{
			ID:            demand.Original.DemandID,
			LockedCPUCore: demand.Original.Cvm.CpuCore,
		})
	}
	return lockedItems
}

func getDemandIDsAndLockedCoreFromCancelReq(kt *kit.Kit, cancelElems []ptypes.CancelRPDemandReqElem) (
	[]string, []rpproto.ResPlanDemandLockOpItem, error) {

	demandIDs := make([]string, len(cancelElems))
	lockedItems := make([]rpproto.ResPlanDemandLockOpItem, len(cancelElems))
	for i, cancel := range cancelElems {
		demandIDs[i] = cancel.DemandID

		lockedItems[i] = rpproto.ResPlanDemandLockOpItem{
			ID:            cancel.DemandID,
			LockedCPUCore: cancel.RemainedCpuCore,
		}
	}

	return demandIDs, lockedItems, nil
}

// CancelBizResPlanDemand cancel biz res plan demand.
func (c *Controller) CancelBizResPlanDemand(kt *kit.Kit, req *ptypes.CancelRPDemandReq, bkBizID int64,
	bizOrgRel *mtypes.BizOrgRel) (string, error) {

	// 从请求中提取预测需求ID即预期变更的核心数
	demandIDs, lockedItems, err := getDemandIDsAndLockedCoreFromCancelReq(kt, req.CancelDemands)
	if err != nil {
		logs.Errorf("failed to get demand ids and locked items from cancel req, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	// check whether all crp demand belong to the biz.
	allBelong, err := c.AreAllDemandBelongToBiz(kt, demandIDs, bkBizID)
	if err != nil {
		logs.Errorf("failed to check whether all demand belong to biz, err: %v, demand_ids: %v, bk_biz_id: %d, rid: %s",
			err, demandIDs, bkBizID, kt.Rid)
		return "", err
	}

	if !allBelong {
		logs.Errorf("not all adjust demand belong to biz: %d, rid: %s", bkBizID, kt.Rid)
		return "", fmt.Errorf("not all adjust crp demand belong to biz: %d", bkBizID)
	}

	// examine whether all resource plan demand classes are the same, and get the demand class.
	demandClass, err := c.ExamineDemandClass(kt, demandIDs)
	if err != nil {
		logs.Errorf("failed to examine demand class, err: %v, demand_ids: %v, rid: %s", err, demandIDs, kt.Rid)
		return "", err
	}

	// construct cancel biz resource plan demand request.
	cancelReq, err := c.constructCancelReq(kt, bizOrgRel, demandClass, req.CancelDemands)
	if err != nil {
		logs.Errorf("failed to construct adjust resource plan ticket request, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	// create cancel resource plan ticket.
	ticketID, err := c.CreateResPlanTicket(kt, cancelReq)
	if err != nil {
		logs.Errorf("failed to create resource plan ticket, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	// lock all resource plan demand.
	for i := range lockedItems {
		lockedItems[i].TicketID = ticketID
	}
	lockReq := &rpproto.ResPlanDemandLockOpReq{
		LockedItems: lockedItems,
	}
	if err = c.client.DataService().Global.ResourcePlan.LockResPlanDemand(kt, lockReq); err != nil {
		logs.Errorf("failed to lock all resource plan demand, err: %v, demandIDs: %v, rid: %s", err,
			demandIDs, kt.Rid)
		return "", err
	}

	// defer is used to unlock all resource plan demand when some errors occur.
	defer func() {
		if err != nil {
			if tmpErr := c.client.DataService().Global.ResourcePlan.UnlockResPlanDemand(kt, lockReq); tmpErr != nil {
				logs.Errorf("failed to unlock all resource plan demand, err: %v, rid: %s", tmpErr, kt.Rid)
			}
		}
	}()

	// create adjust resource plan ticket itsm audit flow.
	if err = c.CreateAuditFlow(kt, ticketID); err != nil {
		logs.Errorf("failed to create resource plan ticket audit flow, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	return ticketID, nil
}

// AreAllDemandBelongToBiz return whether all input demand ids belong to input biz.
func (c *Controller) AreAllDemandBelongToBiz(kt *kit.Kit, demandIDs []string, bkBizID int64) (bool, error) {
	if len(demandIDs) == 0 {
		return false, errors.New("demand ids is empty")
	}

	listReq := &rpproto.ResPlanDemandListReq{
		ListReq: core.ListReq{
			Filter: tools.ExpressionAnd(
				tools.RuleIn("id", demandIDs),
				tools.RuleEqual("bk_biz_id", bkBizID),
			),
			Page: core.NewCountPage(),
		},
	}

	rst, err := c.client.DataService().Global.ResourcePlan.ListResPlanDemand(kt, listReq)
	if err != nil {
		logs.Errorf("failed to list resource plan demand, err: %v, rid: %s", err, kt.Rid)
		return false, err
	}

	return len(demandIDs) == int(rst.Count), nil
}

// ExamineDemandClass examine whether all demands are the same demand class, and return the demand class.
func (c *Controller) ExamineDemandClass(kt *kit.Kit, demandIDs []string) (enumor.DemandClass, error) {
	listReq := &rpproto.ResPlanDemandListReq{
		ListReq: core.ListReq{
			Fields: []string{"demand_class"},
			Filter: tools.ContainersExpression("id", demandIDs),
			Page:   core.NewDefaultBasePage(),
		},
	}

	rstDetails := make([]rpdtablers.ResPlanDemandTable, 0)
	for {
		rst, err := c.client.DataService().Global.ResourcePlan.ListResPlanDemand(kt, listReq)
		if err != nil {
			logs.Errorf("failed to list resource plan demand, err: %v, rid: %s", err, kt.Rid)
			return "", err
		}

		rstDetails = append(rstDetails, rst.Details...)

		if len(rst.Details) < int(listReq.Page.Limit) {
			break
		}
		listReq.Page.Start += uint32(listReq.Page.Limit)
	}

	if len(rstDetails) == 0 {
		logs.Errorf("list resource plan demand, but len detail is 0, rid: %s", kt.Rid)
		return "", errors.New("list resource plan demand, but len detail is 0")
	}

	demandClass := rstDetails[0].DemandClass
	for _, detail := range rstDetails {
		if detail.DemandClass != demandClass {
			logs.Errorf("not all demand classes are the same, rid: %s", kt.Rid)
			return "", errors.New("not all demand classes are the same")
		}
	}

	return demandClass, nil
}

// constructAdjustReq construct create resource plan ticket request of adjust.
func (c *Controller) constructAdjustReq(kt *kit.Kit, bizOrgRel *mtypes.BizOrgRel, demandClass enumor.DemandClass,
	req *ptypes.AdjustRPDemandReq) (*CreateResPlanTicketReq, []rpproto.ResPlanDemandLockOpItem, error) {

	addDemands := make([]ptypes.AdjustRPDemandReqElem, 0)
	updateDemands := make([]ptypes.AdjustRPDemandReqElem, 0)
	delayDemands := make([]ptypes.AdjustRPDemandReqElem, 0)
	for _, adjust := range req.Adjusts {
		switch adjust.AdjustType {
		case enumor.RPDemandAdjustTypeAdd:
			addDemands = append(addDemands, adjust)
		case enumor.RPDemandAdjustTypeUpdate:
			updateDemands = append(updateDemands, adjust)
		case enumor.RPDemandAdjustTypeDelay:
			delayDemands = append(delayDemands, adjust)
		default:
			return nil, nil, fmt.Errorf("unsupported resource plan demand adjust type: %s", adjust.AdjustType)
		}
	}

	addReqs := make([]ptypes.CreateResPlanDemandReq, 0, len(addDemands))
	for _, adjust := range addDemands {
		if adjust.UpdatedInfo == nil {
			return nil, nil, errors.New("updated_info is required for add adjust")
		}
		updated := cvt.PtrToVal(adjust.UpdatedInfo)
		if updated.DemandSource == "" {
			updated.DemandSource = adjust.DemandSource
		}
		addReqs = append(addReqs, updated)
	}

	addBuilt, err := c.buildDemandsFromCreateReq(kt, demandClass, addReqs)
	if err != nil {
		logs.Errorf("failed to build add demands from create req, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}

	// construct update demands.
	updates, err := c.constructUpdateDemands(kt, updateDemands, demandClass)
	if err != nil {
		logs.Errorf("failed to construct update demands, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}

	// construct delay demands.
	delays, err := c.constructDelayDemands(kt, delayDemands, demandClass)
	if err != nil {
		logs.Errorf("failed to construct delay demands, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}

	demands := append(addBuilt, updates...)
	demands = append(demands, delays...)
	adjustReq := &CreateResPlanTicketReq{
		TicketType:  enumor.RPTicketTypeAdjust,
		DemandClass: demandClass,
		BizOrgRel:   *bizOrgRel,
		Demands:     demands,
	}

	lockedItems := buildAdjustLockItems(demands)

	return adjustReq, lockedItems, nil
}

// constructCancelReq construct create resource plan ticket request of cancel.
func (c *Controller) constructCancelReq(kt *kit.Kit, bizOrgRel *mtypes.BizOrgRel, demandClass enumor.DemandClass,
	cancelDemands []ptypes.CancelRPDemandReqElem) (*CreateResPlanTicketReq, error) {

	originDemandMap := make(map[string]ptypes.CreateResPlanDemandResource)
	for _, cancelD := range cancelDemands {
		originDemandMap[cancelD.DemandID] = ptypes.CreateResPlanDemandResource{
			CpuCore: cancelD.RemainedCpuCore,
		}
	}

	// construct crp demand id and origin demand map, crp demand id and remain cpu core map.
	demandOriginMap, err := c.constructOriginalDemandMap(kt, originDemandMap)
	if err != nil {
		logs.Errorf("failed to construct original demand map, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// construct demands.
	demands := make(rpt.ResPlanDemands, 0, len(demandOriginMap))
	for _, origin := range demandOriginMap {
		demands = append(demands, rpt.ResPlanDemand{
			DemandClass: demandClass,
			Original:    origin,
		})
	}

	req := &CreateResPlanTicketReq{
		TicketType:  enumor.RPTicketTypeDelete,
		DemandClass: demandClass,
		BizOrgRel:   *bizOrgRel,
		Demands:     demands,
	}

	return req, nil
}

// constructUpdateDemands construct update demand.
func (c *Controller) constructUpdateDemands(kt *kit.Kit, updates []ptypes.AdjustRPDemandReqElem,
	demandClass enumor.DemandClass) ([]rpt.ResPlanDemand, error) {

	if len(updates) == 0 {
		return nil, nil
	}

	// get create resource plan ticket needed zoneMap, regionAreaMap and deviceTypeMap.
	zoneMap, regionAreaMap, deviceTypeMap, err := c.resFetcher.GetMetaMaps(kt)
	if err != nil {
		logs.Errorf("failed to get meta maps, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// construct crp demand id and origin demand map, crp demand id and remain cpu core map.
	originDemandMap := slice.FuncToMap(updates,
		func(update ptypes.AdjustRPDemandReqElem) (string, ptypes.CreateResPlanDemandResource) {
			return update.DemandID, update.OriginalInfo.GetResource()
		})
	demandOriginMap, err := c.constructOriginalDemandMap(kt, originDemandMap)
	if err != nil {
		logs.Errorf("failed to construct original demand map, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	result := make([]rpt.ResPlanDemand, len(updates))
	for idx, update := range updates {
		// TODO 目前CRP接口不支持修改CBS类型的预测，且CBS类型的预测不会产生罚金，因此暂时不允许单独修改CBS类型的预测
		if len(update.UpdatedInfo.DemandResTypes) == 1 &&
			slices.Contains(update.UpdatedInfo.DemandResTypes, enumor.DemandResTypeCBS) {
			return nil, errors.New("cannot adjust cbs plan demand")
		}

		original, ok := demandOriginMap[update.DemandID]
		if !ok || original == nil {
			logs.Errorf("failed to get original demand for update adjust, demand_id: %s, rid: %s",
				update.DemandID, kt.Rid)
			return nil, fmt.Errorf("demand id: %s is not found", update.DemandID)
		}

		demandSource := update.DemandSource
		if update.UpdatedInfo.DemandSource != "" {
			demandSource = update.UpdatedInfo.DemandSource
		}

		result[idx] = rpt.ResPlanDemand{
			DemandClass: demandClass,
			Original:    original,
			Updated: &rpt.UpdatedRPDemandItem{
				ObsProject:     update.UpdatedInfo.ObsProject,
				ExpectTime:     update.UpdatedInfo.ExpectTime,
				ReturnPlanTime: update.UpdatedInfo.ReturnPlanTime,
				ZoneID:         update.UpdatedInfo.ZoneID,
				ZoneName:       zoneMap[update.UpdatedInfo.ZoneID],
				RegionID:       update.UpdatedInfo.RegionID,
				RegionName:     regionAreaMap[update.UpdatedInfo.RegionID].RegionName,
				AreaName:       regionAreaMap[update.UpdatedInfo.RegionID].AreaName,
				DemandSource:   demandSource,
				Cvm: rpt.Cvm{
					ResMode:        update.UpdatedInfo.Cvm.ResMode,
					DeviceType:     update.UpdatedInfo.Cvm.DeviceType,
					DeviceClass:    deviceTypeMap[update.UpdatedInfo.Cvm.DeviceType].DeviceClass,
					DeviceFamily:   deviceTypeMap[update.UpdatedInfo.Cvm.DeviceType].DeviceFamily,
					TechnicalClass: deviceTypeMap[update.UpdatedInfo.Cvm.DeviceType].TechnicalClass,
					CoreType:       string(deviceTypeMap[update.UpdatedInfo.Cvm.DeviceType].CoreType),
					Os:             types.Decimal{Decimal: cvt.PtrToVal(update.UpdatedInfo.Cvm.Os)},
					CpuCore:        cvt.PtrToVal(update.UpdatedInfo.Cvm.CpuCore),
					Memory:         cvt.PtrToVal(update.UpdatedInfo.Cvm.Memory),
				},
			},
		}

		if slices.Contains(update.UpdatedInfo.DemandResTypes, enumor.DemandResTypeCBS) {
			result[idx].Updated.Cbs = rpt.Cbs{
				DiskType:     update.UpdatedInfo.Cbs.DiskType,
				DiskTypeName: update.UpdatedInfo.Cbs.DiskType.Name(),
				DiskIo:       cvt.PtrToVal(update.UpdatedInfo.Cbs.DiskIo),
				DiskSize:     cvt.PtrToVal(update.UpdatedInfo.Cbs.DiskSize),
			}
		}
	}

	return result, nil
}

// constructOriginalDemandMap construct original demand map.
// return demand id and demand class map, demand id and remain cpu core map.
func (c *Controller) constructOriginalDemandMap(kt *kit.Kit,
	originDemandMap map[string]ptypes.CreateResPlanDemandResource) (map[string]*rpt.OriginalRPDemandItem, error) {

	if len(originDemandMap) == 0 {
		return make(map[string]*rpt.OriginalRPDemandItem), nil
	}

	demandIDs := maps.Keys(originDemandMap)

	// get demand details, batch by MaxInLimit to avoid RuleIn size limit.
	demands := make([]rpdtablers.ResPlanDemandTable, 0, len(demandIDs))
	for _, batchIDs := range slice.Split(demandIDs, int(filter.DefaultMaxInLimit)) {
		listReq := &ptypes.ListResPlanDemandReq{
			DemandIDs: batchIDs,
			Page:      core.NewDefaultBasePage(),
		}
		batchDemands, _, err := c.listAllResPlanDemand(kt, listReq)
		if err != nil {
			logs.Errorf("failed to list res plan demand, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		demands = append(demands, batchDemands...)
	}

	deviceTypeMap, err := c.GetAllDeviceTypeMap(kt)
	if err != nil {
		logs.Errorf("failed to get all device type map, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	demandOriginMap := make(map[string]*rpt.OriginalRPDemandItem)
	for _, demand := range demands {
		deviceType, ok := deviceTypeMap[demand.DeviceType]
		if !ok {
			logs.Errorf("failed to get device type, device type: %s, rid: %s", demand.DeviceType, kt.Rid)
			return nil, fmt.Errorf("device type: %s is not found", demand.DeviceType)
		}

		originDemandRemainRes, ok := originDemandMap[demand.ID]
		if !ok {
			logs.Errorf("failed to list demand, demand id: %s, rid: %s", demand.ID, kt.Rid)
			return nil, fmt.Errorf("demand id: %s is not found", demand.ID)
		}

		demandItem, err := c.constructOriginalDemandWithCPUCore(kt, demand, originDemandRemainRes, deviceType)
		if err != nil {
			logs.Errorf("failed to construct original demand with cpu core, err: %v, demand_id: %s, "+
				"remain res: %+v, rid: %s", err, demand.ID, originDemandRemainRes, kt.Rid)
			return nil, err
		}

		demandOriginMap[demand.ID] = demandItem
	}

	return demandOriginMap, nil
}

// constructOriginalDemandWithCPUCore construct original demand according to db demand,
// with cpu core specified via parameters.
func (c *Controller) constructOriginalDemandWithCPUCore(kt *kit.Kit, demand rpdtablers.ResPlanDemandTable,
	originDemandRemainRes ptypes.CreateResPlanDemandResource, deviceType dt.DistinctDeviceType) (
	*rpt.OriginalRPDemandItem, error) {

	// 变更前资源量以请求中的变更前数据为准
	originCPUCore := originDemandRemainRes.CpuCore
	originOS := decimal.NewFromInt(originCPUCore).Div(decimal.NewFromInt(deviceType.CpuCore))
	// 请求中可能以os为基准变更，此时请求中的cpu core应该为 CreateResPlanDemandUseOsField
	if originCPUCore == ptypes.CreateResPlanDemandUseOsField {
		originOS = originDemandRemainRes.Os
		originCPUCore = originOS.Mul(decimal.NewFromInt(deviceType.CpuCore)).Round(0).IntPart()
	}
	originMem := originOS.Mul(decimal.NewFromInt(deviceType.Memory)).Round(0).IntPart()

	expectTimeStr, err := times.TransTimeStrWithLayout(strconv.Itoa(demand.ExpectTime),
		constant.DateLayoutCompact, constant.DateLayout)
	if err != nil {
		logs.Errorf("failed to convert expect time to string, err: %v, demand_id: %s, expect time: %d, rid: %s",
			err, demand.ID, demand.ExpectTime, kt.Rid)
		return nil, err
	}
	var returnTimeStr string
	if demand.ObsProject == enumor.ObsProjectShortLease {
		returnTimeStr, err = times.TransTimeStrWithLayout(strconv.Itoa(demand.ReturnPlanTime),
			constant.DateLayoutCompact, constant.DateLayout)
		if err != nil {
			logs.Warnf("failed to convert return plan time to string, err: %v, demand_id: %s, return_plan_time: %d, "+
				"rid: %s", err, demand.ID, demand.ReturnPlanTime, kt.Rid)
		}
	}

	return &rpt.OriginalRPDemandItem{
		DemandID:       demand.ID,
		ObsProject:     demand.ObsProject,
		ExpectTime:     expectTimeStr,
		ReturnPlanTime: returnTimeStr,
		ZoneID:         demand.ZoneID,
		ZoneName:       demand.ZoneName,
		RegionID:       demand.RegionID,
		RegionName:     demand.RegionName,
		AreaName:       demand.AreaName,
		Cvm: rpt.Cvm{
			ResMode:        demand.ResMode.Name(),
			DeviceType:     demand.DeviceType,
			DeviceClass:    demand.DeviceClass,
			DeviceFamily:   demand.DeviceFamily,
			TechnicalClass: demand.TechnicalClass,
			CoreType:       string(demand.CoreType),
			Os:             types.Decimal{Decimal: originOS},
			CpuCore:        originCPUCore,
			Memory:         originMem,
		},
		Cbs: rpt.Cbs{
			DiskType:     demand.DiskType,
			DiskTypeName: demand.DiskTypeName,
			DiskIo:       demand.DiskIO,
			DiskSize:     originDemandRemainRes.DiskSize,
		},
	}, nil
}

// constructDelayDemands construct delay demand.
func (c *Controller) constructDelayDemands(kt *kit.Kit, delays []ptypes.AdjustRPDemandReqElem,
	demandClass enumor.DemandClass) ([]rpt.ResPlanDemand, error) {

	if len(delays) == 0 {
		return nil, nil
	}

	demandIDs := slice.Map(delays, func(d ptypes.AdjustRPDemandReqElem) string { return d.DemandID })
	originDemandMap := make(map[string]ptypes.CreateResPlanDemandResource, len(demandIDs))
	for _, batchIDs := range slice.Split(demandIDs, int(filter.DefaultMaxInLimit)) {
		listReq := &ptypes.ListResPlanDemandReq{DemandIDs: batchIDs, Page: core.NewDefaultBasePage()}
		batchDemands, _, err := c.listAllResPlanDemand(kt, listReq)
		if err != nil {
			logs.Errorf("failed to list res plan demand for delay, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		for _, dbDemand := range batchDemands {
			originDemandMap[dbDemand.ID] = ptypes.CreateResPlanDemandResource{
				Os: cvt.PtrToVal(dbDemand.OS).Decimal, CpuCore: cvt.PtrToVal(dbDemand.CpuCore),
				Memory: cvt.PtrToVal(dbDemand.Memory), DiskSize: cvt.PtrToVal(dbDemand.DiskSize),
			}
		}
	}
	for _, delayD := range delays {
		if _, ok := originDemandMap[delayD.DemandID]; !ok {
			logs.Errorf("failed to list demand, demand id: %s, rid: %s", delayD.DemandID, kt.Rid)
			return nil, fmt.Errorf("demand id: %s is not found", delayD.DemandID)
		}
	}

	demandOriginMap, err := c.constructOriginalDemandMap(kt, originDemandMap)
	if err != nil {
		logs.Errorf("failed to construct original demand map, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return assembleDelayResPlanDemands(kt, delays, demandOriginMap, demandClass)
}

// assembleDelayResPlanDemands assembles delay ResPlanDemand slice from origin map.
func assembleDelayResPlanDemands(kt *kit.Kit, delays []ptypes.AdjustRPDemandReqElem,
	demandOriginMap map[string]*rpt.OriginalRPDemandItem, demandClass enumor.DemandClass) (
	[]rpt.ResPlanDemand, error) {

	result := make([]rpt.ResPlanDemand, len(delays))
	for idx, delay := range delays {
		original, ok := demandOriginMap[delay.DemandID]
		if !ok || original == nil {
			logs.Errorf("failed to get original demand for delay adjust, demand_id: %s, rid: %s",
				delay.DemandID, kt.Rid)
			return nil, fmt.Errorf("demand id: %s is not found", delay.DemandID)
		}
		if original.ExpectTime == delay.ExpectTime {
			logs.Errorf("expect time unchanged for delay adjust, demand_id: %s, expect_time: %s, rid: %s",
				delay.DemandID, delay.ExpectTime, kt.Rid)
			return nil, errors.New("expect time unchanged for delay adjust")
		}

		result[idx] = rpt.ResPlanDemand{
			DemandClass: demandClass,
			Original:    original,
			Updated:     buildDelayUpdatedItem(original, delay.ExpectTime),
		}
	}

	return result, nil
}

// buildDelayUpdatedItem copies original demand fields with new expect time for delay adjust.
func buildDelayUpdatedItem(original *rpt.OriginalRPDemandItem, expectTime string) *rpt.UpdatedRPDemandItem {
	return &rpt.UpdatedRPDemandItem{
		ObsProject:     original.ObsProject,
		ExpectTime:     expectTime,
		ReturnPlanTime: original.ReturnPlanTime,
		ZoneID:         original.ZoneID,
		ZoneName:       original.ZoneName,
		RegionID:       original.RegionID,
		RegionName:     original.RegionName,
		AreaName:       original.AreaName,
		Cvm: rpt.Cvm{
			ResMode:        original.Cvm.ResMode,
			DeviceType:     original.Cvm.DeviceType,
			DeviceClass:    original.Cvm.DeviceClass,
			DeviceFamily:   original.Cvm.DeviceFamily,
			TechnicalClass: original.Cvm.TechnicalClass,
			CoreType:       original.Cvm.CoreType,
			Os:             original.Cvm.Os,
			CpuCore:        original.Cvm.CpuCore,
			Memory:         original.Cvm.Memory,
		},
		Cbs: rpt.Cbs{
			DiskType:     original.Cbs.DiskType,
			DiskTypeName: original.Cbs.DiskTypeName,
			DiskIo:       original.Cbs.DiskIo,
			DiskSize:     original.Cbs.DiskSize,
		},
	}
}
