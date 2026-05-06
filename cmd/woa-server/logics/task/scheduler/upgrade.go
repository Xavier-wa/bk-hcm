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

package scheduler

import (
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"hcm/cmd/woa-server/logics/task/scheduler/record"
	"hcm/cmd/woa-server/model/task"
	types "hcm/cmd/woa-server/types/task"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/maps"
)

// CreateUpgradeTicketANDOrder creates upgrade cvm ticket and suborder
func (s *scheduler) CreateUpgradeTicketANDOrder(kt *kit.Kit, param *types.ApplyReq) (
	*types.CreateUpgradeCrpOrderResult, error) {

	rst := new(types.CreateUpgradeCrpOrderResult)
	suborders := make([]*types.ApplyOrder, 0)
	var err error = nil

	txnErr := dal.RunTransaction(kt, func(sc mongo.SessionContext) error {
		sessionKit := kt.NewSubKitWithCtx(sc)

		applyOrderRst := new(types.CreateApplyOrderResult)
		// create apply ticket
		if param.OrderId <= 0 {
			applyOrderRst, err = s.createApplyTicket(sessionKit, param, types.TicketStageAudit)
		} else {
			applyOrderRst, err = s.updateApplyTicket(sessionKit, param, types.TicketStageAudit)
		}
		if err != nil {
			logs.Errorf("failed to create apply order, orderId: %d, err: %v, rid: %s", param.OrderId, err, kt.Rid)
			return err
		}

		param.OrderId = applyOrderRst.OrderId
		// update apply ticket to running
		if err = s.UpdateTicketState(sessionKit, param.OrderId, types.TicketStageRunning); err != nil {
			logs.Errorf("failed to update apply ticket, orderId: %d, err: %v, rid: %s", param.OrderId, err, kt.Rid)
			return err
		}

		// 补充核心数
		if param, err = s.fillUpgradeCVMAppliedCore(sessionKit, param); err != nil {
			logs.Errorf("failed to fill upgrade cvm applied core, orderId: %d, err: %v, rid: %s", param.OrderId,
				err, kt.Rid)
			return err
		}

		// create apply order
		if suborders, err = s.createSubOrdersToMatching(sessionKit, param.OrderId, param); err != nil {
			logs.Errorf("failed to create upgrade suborder, orderId: %d, err: %v, rid: %s", param.OrderId, err,
				kt.Rid)
			return err
		}

		if len(suborders) < 1 {
			logs.Errorf("failed to create suborder, orderId: %d, err: %v, rid: %s", param.OrderId, err, kt.Rid)
			return errors.New("failed to create suborder, suborder is empty")
		}

		return nil
	})
	if txnErr != nil {
		return nil, txnErr
	}

	// TODO 目前同步提单模式仅支持单个suborder
	crpOrderID, err := s.generator.UpgradeCVMSync(kt, suborders[0])
	if err != nil {
		logs.Errorf("failed to upgrade cvm, orderId: %d, err: %v, rid: %s", param.OrderId, err, kt.Rid)

		// update order status to TERMINATE
		errUpdate := s.updateApplyOrderStatus(kt, suborders[0], types.TicketStageSuspend, types.ApplyStatusTerminate)
		if errUpdate != nil {
			logs.Warnf("failed to update apply order %s status, err: %v, rid: %s", suborders[0].SubOrderId,
				errUpdate, kt.Rid)
		}

		return nil, err
	}

	rst.CRPOrderID = crpOrderID
	return rst, nil
}

func (s *scheduler) UpdateTicketState(kt *kit.Kit, orderId uint64, stage types.TicketStage) error {
	filter := tools.ExpressionAnd(tools.RuleEqual("order_id", orderId))
	update := &cvmapplyproto.ZiyanCvmApplyOrderUpdateReq{
		Stage: stage,
	}

	if err := model.Operation().ApplyTicket().UpdateApplyTicket(kt, filter, update); err != nil {
		logs.Errorf("failed to update apply ticket, err: %v, orderID: %d, rid: %s", err, orderId, kt.Rid)
		return err
	}

	return nil
}

// GetSuborders 根据order获得子单
func (s *scheduler) GetSuborders(kt *kit.Kit, orderID uint64) ([]*types.ApplyOrder, error) {
	filter := tools.ExpressionAnd(tools.RuleEqual("order_id", orderID))
	orders, err := model.Operation().ApplyOrder().FindManyApplyOrder(kt, filter, nil)
	if err != nil {
		logs.Errorf("failed to list apply order by orderId, err: %v, orderID: %d, rid: %s", err, orderID, kt.Rid)
		return nil, err
	}
	return orders, nil
}

func (s *scheduler) fillUpgradeCVMAppliedCore(kt *kit.Kit, param *types.ApplyReq) (*types.ApplyReq, error) {
	if param == nil {
		logs.Errorf("failed to fill applied core, param is nil, rid: %s", kt.Rid)
		return nil, errf.New(errf.InvalidParameter, "param is nil")
	}

	deviceTypesMap := make(map[string]interface{})
	for _, suborder := range param.Suborders {
		for _, item := range suborder.UpgradeCVMList {
			deviceTypesMap[item.TargetInstanceType] = nil
		}
	}
	deviceTypes := maps.Keys(deviceTypesMap)

	deviceTypeInfoMap, err := s.configLogics.Device().ListCvmInstanceInfoByDeviceTypes(kt, deviceTypes)
	if err != nil {
		logs.Errorf("get cvm instance info by device type failed, err: %v, device_types: %v, rid: %s",
			err, deviceTypes, kt.Rid)
		return nil, err
	}

	for i, suborder := range param.Suborders {
		var totalCPUCore uint
		for _, item := range suborder.UpgradeCVMList {
			deviceTypeInfo, ok := deviceTypeInfoMap[item.TargetInstanceType]
			if !ok {
				logs.Errorf("can not find device_type, type: %s, rid: %s", item.TargetInstanceType, kt.Rid)
				return nil, fmt.Errorf("can not find device_type, type: %s", item.TargetInstanceType)
			}

			totalCPUCore += uint(deviceTypeInfo.CPUAmount)
		}
		param.Suborders[i].AppliedCore = totalCPUCore
	}

	return param, nil
}

// createSubOrdersToMatching create suborders and set them status to matching
func (s *scheduler) createSubOrdersToMatching(kt *kit.Kit, orderID uint64, param *types.ApplyReq) (
	[]*types.ApplyOrder, error) {

	now := time.Now()
	suborders := make([]*types.ApplyOrder, len(param.Suborders))
	for index, suborder := range param.Suborders {
		subOrder := &types.ApplyOrder{
			OrderId:           orderID,
			SubOrderId:        fmt.Sprintf("%d-%d", orderID, index+1),
			BkBizId:           param.BkBizId,
			User:              param.User,
			Follower:          param.Follower,
			Auditor:           "",
			RequireType:       param.RequireType,
			ExpectTime:        param.ExpectTime,
			ResourceType:      suborder.ResourceType,
			ProductType:       enumor.ProductTypeBusiness,
			Spec:              suborder.Spec,
			UpgradeCVMList:    suborder.UpgradeCVMList,
			AntiAffinityLevel: suborder.AntiAffinityLevel,
			EnableDiskCheck:   suborder.EnableDiskCheck,
			Description:       param.Remark,
			Remark:            suborder.Remark,
			Stage:             types.TicketStageRunning,
			Status:            types.ApplyStatusMatching,
			OriginNum:         suborder.Replicas,
			TotalNum:          suborder.Replicas,
			PendingNum:        suborder.Replicas,
			SuccessNum:        0,
			AppliedCore:       suborder.AppliedCore,
			ObsProject:        param.RequireType.ToObsProject(),
			RetryTime:         0,
			ModifyTime:        0,
			CreateAt:          now,
			UpdateAt:          now,
		}
		logs.V(4).Infof("create suborder to matching data, bkBizID: %d, subOrder: %+v, rid: %s",
			param.BkBizId, subOrder, kt.Rid)

		if err := model.Operation().ApplyOrder().CreateApplyOrder(kt, subOrder); err != nil {
			logs.Errorf("failed to create upgrade order, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		// init all step record
		if err := s.InitUpgradeCVMSteps(kt, subOrder.SubOrderId, subOrder.TotalNum); err != nil {
			logs.Errorf("failed to init upgrade step record, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		suborders[index] = subOrder
	}

	return suborders, nil
}

// InitUpgradeCVMSteps init upgrade cvm order all steps
func (s *scheduler) InitUpgradeCVMSteps(kt *kit.Kit, suborderId string, total uint) error {
	// init commit step
	stepID := 1
	if err := record.CreateCommitStep(kt, suborderId, total, stepID); err != nil {
		logs.Errorf("order %s failed to create commit step, err: %v, rid: %s", suborderId, err, kt.Rid)
		return err
	}

	// init generate step
	stepID++
	if err := record.CreateGenerateStep(kt, suborderId, total, stepID); err != nil {
		logs.Errorf("order %s failed to create generate step, err: %v, rid: %s", suborderId, err, kt.Rid)
		return err
	}

	return nil
}

// updateApplyOrderStatus update apply order status
func (s *scheduler) updateApplyOrderStatus(kt *kit.Kit, order *types.ApplyOrder, stage types.TicketStage,
	status types.ApplyStatus) error {

	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", order.SubOrderId))

	update := &cvmapplyproto.ZiyanCvmApplySuborderUpdateReq{
		Stage:  stage,
		Status: status,
	}

	if err := model.Operation().ApplyOrder().UpdateApplyOrder(kt, filter, update); err != nil {
		logs.Errorf("failed to update apply order status, id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return err
	}

	return nil
}
