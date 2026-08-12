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
	"fmt"

	mtypes "hcm/cmd/woa-server/types/meta"
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/api/core"
	dt "hcm/pkg/api/core/cloud/device-type"
	rpproto "hcm/pkg/api/data-service/resource-plan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	rpts "hcm/pkg/dal/table/resource-plan/res-plan-ticket-status"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// overwriteAppendPrepared holds cancel/add demands and create request for overwrite-append.
type overwriteAppendPrepared struct {
	cancelDemands rpt.ResPlanDemands
	addDemands    rpt.ResPlanDemands
	lockedItems   []rpproto.ResPlanDemandLockOpItem
	createReq     *CreateResPlanTicketReq
}

// OverwriteAppendResPlanTicket 覆盖追加资源预测主流程：
// 1. overwrite=true 时按筛选条件命中本地 res_plan_demand，构造 cancel 条目
//    （命中流转中明细则直接失败）；
// 2. 追加 demands 构造 add 条目；
// 3. 合并为同一主单，主单 type 使用请求必填字段 req.Type，并校验与 cancel/add 组成一致；
// 4. 按 skip_itsm 分支进入调度（跳过 ITSM 直接置 auditing+skip，否则走 CreateAuditFlow）；
// 返回主单 ID。
func (c *Controller) OverwriteAppendResPlanTicket(kt *kit.Kit, bizOrgRel *mtypes.BizOrgRel,
	req *ptypes.OverwriteAppendResPlanTicketReq) (ticketID string, retErr error) {

	prepared, err := c.prepareOverwriteAppendTicket(kt, bizOrgRel, req)
	if err != nil {
		return "", err
	}

	// 创建主单（status=init），跳过严格分类型校验；不校验非本年度提报截止时间。
	ticketID, err = c.persistResPlanTicket(kt, prepared.createReq, false)
	if err != nil {
		logs.Errorf("failed to persist overwrite-append resource plan ticket, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	unlock, err := c.lockOverwriteAppendDemands(kt, ticketID, prepared.lockedItems)
	if err != nil {
		return "", err
	}
	if unlock != nil {
		defer func() {
			if retErr != nil {
				unlock()
			}
		}()
	}

	if err = c.dispatchOverwriteAppendTicket(kt, ticketID, req.SkipItsm); err != nil {
		return "", err
	}

	logs.Infof("overwrite-append resource plan ticket success, ticket_id: %s, type: %s, cancel: %d, add: %d, "+
		"skip_itsm: %v, rid: %s", ticketID, req.Type, len(prepared.cancelDemands), len(prepared.addDemands),
		req.SkipItsm, kt.Rid)
	return ticketID, nil
}

// prepareOverwriteAppendTicket builds cancel/add demands, validates device types and create request.
func (c *Controller) prepareOverwriteAppendTicket(kt *kit.Kit, bizOrgRel *mtypes.BizOrgRel,
	req *ptypes.OverwriteAppendResPlanTicketReq) (*overwriteAppendPrepared, error) {

	var cancelDemands rpt.ResPlanDemands
	var lockedItems []rpproto.ResPlanDemandLockOpItem
	if req.Overwrite {
		var err error
		cancelDemands, lockedItems, err = c.buildOverwriteCancelDemands(kt, bizOrgRel.BkBizID, req)
		if err != nil {
			logs.Errorf("failed to build overwrite cancel demands, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
	}

	var addDemands rpt.ResPlanDemands
	if len(req.Demands) > 0 {
		var err error
		addDemands, err = c.buildDemandsFromCreateReq(kt, req.DemandClass, req.Demands)
		if err != nil {
			logs.Errorf("failed to build add demands from create req, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
	}

	allDemands := make(rpt.ResPlanDemands, 0, len(cancelDemands)+len(addDemands))
	allDemands = append(allDemands, cancelDemands...)
	allDemands = append(allDemands, addDemands...)
	if len(allDemands) == 0 {
		logs.Errorf("no demands matched for overwrite and no demands to append, biz_id: %d, rid: %s",
			bizOrgRel.BkBizID, kt.Rid)
		return nil, errf.New(errf.InvalidParameter,
			"no demands matched for overwrite and no demands to append")
	}

	// 校验主单 type 与 cancel/add 明细组成一致，避免拆单阶段失败导致需求长期锁定。
	if err := validateOverwriteAppendTicketType(req.Type, len(cancelDemands) > 0, len(addDemands) > 0); err != nil {
		logs.Errorf("validate overwrite-append ticket type failed, err: %v, type: %s, cancel: %d, add: %d, rid: %s",
			err, req.Type, len(cancelDemands), len(addDemands), kt.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// 前置校验 CVM 机型是否存在
	if err := c.validateDemandsDeviceTypesExist(kt, allDemands); err != nil {
		logs.Errorf("validate demands device types failed, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	return &overwriteAppendPrepared{
		cancelDemands: cancelDemands,
		addDemands:    addDemands,
		lockedItems:   lockedItems,
		createReq: &CreateResPlanTicketReq{
			TicketType:  req.Type,
			DemandClass: req.DemandClass,
			BizOrgRel:   *bizOrgRel,
			Demands:     allDemands,
			Remark:      req.Remark,
			Applicant:   req.Applicant,
		},
	}, nil
}

// lockOverwriteAppendDemands locks overwrite-matched demands; returns unlock callback (nil if nothing locked).
func (c *Controller) lockOverwriteAppendDemands(kt *kit.Kit, ticketID string,
	lockedItems []rpproto.ResPlanDemandLockOpItem) (func(), error) {

	if len(lockedItems) == 0 {
		return nil, nil
	}
	for i := range lockedItems {
		lockedItems[i].TicketID = ticketID
	}
	lockReq := &rpproto.ResPlanDemandLockOpReq{LockedItems: lockedItems}
	if err := c.client.DataService().Global.ResourcePlan.LockResPlanDemand(kt, lockReq); err != nil {
		logs.Errorf("failed to lock overwrite matched demands, err: %v, ticket_id: %s, rid: %s",
			err, ticketID, kt.Rid)
		return nil, err
	}
	return func() {
		if tmpErr := c.client.DataService().Global.ResourcePlan.UnlockResPlanDemand(kt, lockReq); tmpErr != nil {
			logs.Errorf("failed to unlock overwrite matched demands, err: %v, ticket_id: %s, rid: %s",
				tmpErr, ticketID, kt.Rid)
		}
	}, nil
}

// dispatchOverwriteAppendTicket starts skip_itsm dispatch or ITSM audit flow.
func (c *Controller) dispatchOverwriteAppendTicket(kt *kit.Kit, ticketID string, skipItsm bool) error {
	if skipItsm {
		update := &rpts.ResPlanTicketStatusTable{
			TicketID: ticketID,
			ItsmSN:   constant.ResPlanItsmAuditSkip,
			Status:   enumor.RPTicketStatusAuditing,
		}
		if err := c.updateTicketStatus(kt, update); err != nil {
			logs.Errorf("failed to update ticket status for skip itsm, err: %v, ticket_id: %s, rid: %s",
				err, ticketID, kt.Rid)
			return err
		}
		return nil
	}
	if err := c.CreateAuditFlow(kt, ticketID); err != nil {
		logs.Errorf("failed to create resource plan ticket audit flow, err: %v, ticket_id: %s, rid: %s",
			err, ticketID, kt.Rid)
		return err
	}
	return nil
}

// buildOverwriteCancelDemands 按覆盖筛选条件命中本地 res_plan_demand，构造 cancel 条目（仅 Original）。
// 命中流转中（非 can_apply/not_ready）明细时直接失败；明细允许混合多种 DemandClass。
func (c *Controller) buildOverwriteCancelDemands(kt *kit.Kit, bkBizID int64,
	req *ptypes.OverwriteAppendResPlanTicketReq) (rpt.ResPlanDemands, []rpproto.ResPlanDemandLockOpItem, error) {

	listReq := &ptypes.ListResPlanDemandReq{
		BkBizIDs:         []int64{bkBizID},
		ObsProjects:      req.OverwriteFilter.ObsProjects,
		TechnicalClasses: req.OverwriteFilter.TechnicalClasses,
		ExpectTimeRange:  req.OverwriteFilter.ExpectTimeRange,
		Page:             core.NewDefaultBasePage(),
	}

	matched := make([]*ptypes.ListResPlanDemandItem, 0)
	for {
		rst, err := c.ListResPlanDemandAndOverview(kt, listReq)
		if err != nil {
			logs.Errorf("failed to list res plan demand and overview for overwrite, err: %v, rid: %s", err, kt.Rid)
			return nil, nil, err
		}

		for _, item := range rst.Details {
			// 流转中明细不可覆盖，直接失败，避免静默漏删。
			if item.Status != enumor.DemandStatusCanApply && item.Status != enumor.DemandStatusNotReady {
				logs.Errorf("overwrite matched demand in transit, demand_id: %s, status: %s, rid: %s",
					item.DemandID, item.Status, kt.Rid)
				return nil, nil, errf.Newf(errf.InvalidParameter,
					"overwrite matched demand in transit, demand_id: %s, status: %s",
					item.DemandID, item.Status)
			}
			matched = append(matched, item)
		}

		if len(rst.Details) < int(listReq.Page.Limit) {
			break
		}
		listReq.Page.Start += uint32(listReq.Page.Limit)
	}

	if len(matched) == 0 {
		return nil, nil, nil
	}

	cancelDemands, lockedItems, _ := c.constructAutoTransferDemands(matched)
	return cancelDemands, lockedItems, nil
}

// validateDemandsDeviceTypesExist 校验明细中 CVM 机型在机型库中存在且 CpuCore>0。
// 与拆单 splitter 中「cannot found device type」判定对齐，提前到提单失败。
func (c *Controller) validateDemandsDeviceTypesExist(kt *kit.Kit, demands rpt.ResPlanDemands) error {
	deviceTypeMap, err := c.deviceTypesMap.GetDeviceTypes(kt)
	if err != nil {
		logs.Errorf("get device type map failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}
	return checkDemandsDeviceTypesExist(demands, deviceTypeMap)
}

// checkDemandsDeviceTypesExist 纯函数：校验 Original/Updated 中非空 DeviceType 均存在且 CpuCore>0。
func checkDemandsDeviceTypesExist(demands rpt.ResPlanDemands,
	deviceTypeMap map[string]dt.DistinctDeviceType) error {

	for _, demand := range demands {
		if demand.Original != nil {
			if err := checkCvmDeviceTypeExist(demand.Original.Cvm.DeviceType, deviceTypeMap); err != nil {
				return err
			}
		}
		if demand.Updated != nil {
			if err := checkCvmDeviceTypeExist(demand.Updated.Cvm.DeviceType, deviceTypeMap); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkCvmDeviceTypeExist(deviceType string, deviceTypeMap map[string]dt.DistinctDeviceType) error {
	if deviceType == "" {
		return nil
	}
	info, ok := deviceTypeMap[deviceType]
	if !ok || info.CpuCore == 0 {
		return fmt.Errorf("cannot found device type: %s", deviceType)
	}
	return nil
}

// validateOverwriteAppendTicketType 校验主单 type 与 cancel/add 明细组成一致：
// 仅 cancel → delete/budget_declare；仅 add → add/budget_declare；混合 → adjust/budget_declare。
func validateOverwriteAppendTicketType(ticketType enumor.RPTicketType, hasCancel, hasAdd bool) error {
	// budget_declare 可承载任意 cancel/add 组合，拆单统一走 adjust。
	if ticketType == enumor.RPTicketTypeBudgetDeclare {
		return nil
	}

	switch {
	case hasCancel && hasAdd:
		if ticketType != enumor.RPTicketTypeAdjust {
			return fmt.Errorf("ticket type %s is inconsistent with cancel+add demands, expect adjust or budget_declare",
				ticketType)
		}
	case hasAdd:
		if ticketType != enumor.RPTicketTypeAdd {
			return fmt.Errorf("ticket type %s is inconsistent with add-only demands, expect add or budget_declare",
				ticketType)
		}
	case hasCancel:
		if ticketType != enumor.RPTicketTypeDelete {
			return fmt.Errorf("ticket type %s is inconsistent with cancel-only demands, expect delete or budget_declare",
				ticketType)
		}
	default:
		return fmt.Errorf("ticket type %s is inconsistent with empty demands", ticketType)
	}
	return nil
}
