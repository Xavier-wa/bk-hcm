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

package cvm

import (
	"fmt"
	"time"

	taskModel "hcm/cmd/woa-server/model/task"
	types "hcm/cmd/woa-server/types/cvm"
	taskTypes "hcm/cmd/woa-server/types/task"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"
)

// verifyCvmGPUCharge verifies GPU special model charge duration
func (l *logics) verifyCvmGPUCharge(kt *kit.Kit, param *types.CvmCreateReq) error {
	verifySubOrderReq := []*taskTypes.Suborder{
		{
			ResourceType: taskTypes.ResourceTypeCvm,
			Spec: &taskTypes.ResourceSpec{
				ChargeType:   param.Spec.ChargeType,
				ChargeMonths: param.Spec.ChargeMonths,
				DeviceType:   param.Spec.DeviceType,
			},
		},
	}
	return l.schedulerLogic.VerifyCvmGPUChargeMonth(kt, verifySubOrderReq)
}

// buildApplyOrder builds apply order request
func (l *logics) buildApplyOrder(param *types.CvmCreateReq, now time.Time) *taskTypes.ApplyReq {
	return &taskTypes.ApplyReq{
		BkBizId:     param.BkBizId,
		User:        param.User,
		RequireType: param.RequireType,
		ExpectTime:  now.Format(constant.DateTimeLayout),
		Remark:      param.Remark,
		Suborders: []*taskTypes.Suborder{{
			ResourceType: taskTypes.ResourceTypeCvm,
			Replicas:     param.Replicas,
			Remark:       param.Remark,
			Source:       enumor.ApplyTicketSrcBusiness,
			Spec:         param.Spec,
		}},
		ProductType: enumor.ProductTypeAdmin, // 管理员生产
	}
}

// buildSubOrder builds sub order from order info
func (l *logics) buildSubOrder(param *types.CvmCreateReq, order *taskTypes.ApplyReq,
	orderInfo *taskTypes.CreateApplyOrderResult, newParam *taskTypes.ApplyReq, now time.Time) *taskTypes.ApplyOrder {
	return &taskTypes.ApplyOrder{
		OrderId:      orderInfo.OrderId,
		SubOrderId:   fmt.Sprintf("%d-1", orderInfo.OrderId),
		BkBizId:      param.BkBizId,
		User:         param.User,
		RequireType:  param.RequireType,
		ExpectTime:   order.ExpectTime,
		ResourceType: order.Suborders[0].ResourceType,
		Source:       order.Suborders[0].Source,
		ProductType:  order.ProductType,
		Spec:         param.Spec,
		Description:  param.Remark,
		Remark:       param.Remark,
		Stage:        taskTypes.TicketStageRunning,
		Status:       taskTypes.ApplyStatusWaitForMatch,
		OriginNum:    param.Replicas,
		TotalNum:     param.Replicas,
		PendingNum:   param.Replicas,
		SuccessNum:   0,
		AppliedCore:  newParam.Suborders[0].AppliedCore,
		ObsProject:   param.RequireType.ToObsProject(),
		CreateAt:     now,
		UpdateAt:     now,
	}
}

// createAndInitSubOrder creates sub order and initializes steps
func (l *logics) createAndInitSubOrder(kt *kit.Kit, subOrder *taskTypes.ApplyOrder) error {
	if err := l.processingOrderByRequireType(kt, subOrder); err != nil {
		logs.Errorf("processing cvm apply order by require type failed, err: %v, orderID: %d, subOrderID: %s, rid: %s",
			err, subOrder.OrderId, subOrder.SubOrderId, kt.Rid)
		return err
	}

	if err := taskModel.Operation(l.client).ApplyOrder().CreateApplyOrder(kt, subOrder); err != nil {
		logs.Errorf("failed to create cvm apply order, err: %v, orderID: %d, subOrderID: %s, rid: %s",
			err, subOrder.OrderId, subOrder.SubOrderId, kt.Rid)
		return err
	}

	if err := l.schedulerLogic.InitUpgradeCVMSteps(kt, subOrder.SubOrderId, subOrder.TotalNum); err != nil {
		logs.Errorf("failed to init cvm product step record, orderID: %d, subOrderID: %s, err: %v, rid: %s",
			subOrder.OrderId, subOrder.SubOrderId, err, kt.Rid)
		return err
	}

	return nil
}

// CreateCvmProductApplyOrder creates cvm product apply order(CVM生产-创建单据-新)
func (l *logics) CreateCvmProductApplyOrder(kt *kit.Kit, param *types.CvmCreateReq) (*types.CvmCreateResult, error) {
	// GPU特殊机型的计费时长校验
	if err := l.verifyCvmGPUCharge(kt, param); err != nil {
		return nil, err
	}

	// 生成主单ID
	now := time.Now()
	order := l.buildApplyOrder(param, now)

	newParam, err := l.schedulerLogic.FillCVMAppliedCore(kt, order)
	if err != nil {
		logs.Errorf("failed to fill applied core, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	orderInfo, err := l.schedulerLogic.UpdateApplyTicket(kt, newParam)
	if err != nil {
		logs.Errorf("failed to create cvm product apply order, err: %v, newParam: %+v, rid: %s",
			err, cvt.PtrToVal(newParam), kt.Rid)
		return nil, err
	}

	// update apply ticket to running
	if err = l.schedulerLogic.UpdateTicketState(kt, orderInfo.OrderId, taskTypes.TicketStageRunning); err != nil {
		logs.Errorf("failed to update apply ticket, orderId: %d, err: %v, rid: %s", orderInfo.OrderId, err, kt.Rid)
		return nil, err
	}

	subOrder := l.buildSubOrder(param, order, orderInfo, newParam, now)
	if err = l.createAndInitSubOrder(kt, subOrder); err != nil {
		return nil, err
	}

	logs.Infof("scheduler:logics:cvm:create:apply:order:init, orderID: %d, subOrderID: %s, param: %+v, rid: %s",
		orderInfo.OrderId, subOrder.SubOrderId, cvt.PtrToVal(param), kt.Rid)

	return &types.CvmCreateResult{OrderId: orderInfo.OrderId}, nil
}
