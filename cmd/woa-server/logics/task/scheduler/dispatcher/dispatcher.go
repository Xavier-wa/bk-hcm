/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package dispatcher implements the dispatcher of apply order
package dispatcher

import (
	"context"
	"fmt"
	"time"

	"hcm/cmd/woa-server/logics/task/informer"
	"hcm/cmd/woa-server/logics/task/scheduler/generator"
	"hcm/cmd/woa-server/logics/task/scheduler/record"
	"hcm/cmd/woa-server/model/task"
	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/utils/wait"
)

// Dispatcher dispatch and deal apply order
type Dispatcher struct {
	informer  informer.Interface
	generator *generator.Generator
	// ctx used to manage life cycle
	ctx context.Context
}

// New create a dispatcher
func New(ctx context.Context, informer informer.Interface) (*Dispatcher, error) {
	dispatcher := &Dispatcher{
		informer: informer,
		ctx:      ctx,
	}

	// TODO: get worker num from config
	go dispatcher.Run(20)

	return dispatcher, nil
}

// SetGenerator set dispatcher member generator
func (d *Dispatcher) SetGenerator(generator *generator.Generator) {
	d.generator = generator
}

// Run starts dispatcher
func (d *Dispatcher) Run(workers int) {
	for i := 0; i < workers; i++ {
		go wait.Until(d.runWorker, time.Second, d.ctx)
	}

	select {
	case <-d.ctx.Done():
		logs.Infof("dispatcher exits")
	}
}

// runWorker deals with apply order
func (d *Dispatcher) runWorker() error {
	// Check if informer is available (only available on master node)
	if d.informer == nil {
		logs.Warnf("task scheduler informer dispatcher is not available")
		time.Sleep(time.Second)
		return nil
	}

	applyInformer := d.informer.Apply()
	if applyInformer == nil {
		logs.Warnf("task scheduler apply informer is not available")
		time.Sleep(time.Second)
		return nil
	}

	order, err := applyInformer.Pop()
	if err != nil {
		logs.Errorf("failed to deal apply order, for get apply order from informer err: %v", err)
		return err
	}
	if order == "" {
		logs.Warnf("shutdown to deal apply order, for get apply order from informer")
		time.Sleep(time.Second)
		return nil
	}
	if err = d.dispatchHandler(core.NewBackendKit(), order); err != nil {
		logs.Errorf("failed to dispatch apply order %s, err: %v", order, err)
		return err
	}
	logs.Infof("Successfully dispatch apply order: %s", order)

	return nil
}

// dispatchHandler apply order dispatch handler
func (d *Dispatcher) dispatchHandler(kt *kit.Kit, key string) error {
	// get apply order by key
	applyOrder, err := d.getApplyOrder(kt, key)
	if err != nil {
		logs.Errorf("get apply order by key %s failed, err: %v, rid: %s", key, err, kt.Rid)
		return err
	}

	// check order stage
	if applyOrder.Stage != types.TicketStageRunning {
		logs.Infof("apply order %s need not dispatch, stage: %s, rid: %s", key, applyOrder.Stage, kt.Rid)
		return nil
	}

	// check order status
	if !shouldDispatch(applyOrder.Status) {
		logs.Infof("apply order %s need not dispatch, status: %s, rid: %s", key, applyOrder.Status, kt.Rid)
		return nil
	}

	// check retry time
	retryLimit := uint(3)
	if applyOrder.RetryTime > retryLimit {
		logs.Infof("apply order %s need not dispatch, for retry time %d exceeds limit %d, rid: %s",
			key, applyOrder.RetryTime, retryLimit, kt.Rid)
		// update order status to TERMINATE
		if err = d.updateApplyOrderStatus(
			kt, applyOrder, types.TicketStageSuspend, types.ApplyStatusTerminate); err != nil {
			logs.Errorf("failed to update apply order %s status, err: %v, rid: %s", key, err, kt.Rid)
		}
		return nil
	}

	// lock apply order
	if err = d.lockApplyOrder(kt, applyOrder); err != nil {
		logs.Errorf("failed to lock apply order %s, err: %v, rid: %s", key, err, kt.Rid)
		return err
	}

	// 锁定成功后，需要更新applyOrder的状态为：匹配中
	applyOrder.Status = types.ApplyStatusMatching

	// start generate step
	if err = record.StartStep(kt, applyOrder.SubOrderId, types.StepNameGenerate); err != nil {
		logs.Errorf("failed to start generate step, order id: %s, err: %v, rid: %s", key, err, kt.Rid)
		return err
	}

	// generate devices according to apply order
	if err = d.generateDevices(kt, applyOrder); err != nil {
		logs.Errorf("failed to generate device, order id: %s, err: %v, rid: %s", key, err, kt.Rid)
		// update generate step record
		if errStep := record.UpdateGenerateStep(
			kt, applyOrder.SubOrderId, applyOrder.TotalNum, err); errStep != nil {
			logs.Errorf("failed to generate device, order id: %s, err: %v, rid: %s", key, errStep, kt.Rid)
			return errStep
		}

		// update order status to TERMINATE
		errUpdate := d.updateApplyOrderStatus(kt, applyOrder, types.TicketStageSuspend, types.ApplyStatusTerminate)
		if errUpdate != nil {
			logs.Warnf("failed to update apply order %s status, err: %v, rid: %s", key, errUpdate, kt.Rid)
		}

		return err
	}

	// update generate step record
	if err = record.UpdateGenerateStep(
		kt, applyOrder.SubOrderId, applyOrder.TotalNum, nil); err != nil {
		logs.Errorf("failed to generate device, order id: %s, err: %v, rid: %s", key, err, kt.Rid)
		return err
	}

	logs.Infof("finished dispatch order %s", key)

	return nil
}

// shouldDispatch checks if apply order should not dispatch
func shouldDispatch(status types.ApplyStatus) bool {
	switch status {
	case types.ApplyStatusDone,
		types.ApplyStatusMatching,
		types.ApplyStatusTerminate,
		types.ApplyStatusGracefulTerminate:
		return false
	default:
		return true
	}
}

// getApplyOrder gets apply order by order id
func (d *Dispatcher) getApplyOrder(kt *kit.Kit, key string) (*types.ApplyOrder, error) {
	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", key))
	order, err := model.Operation().ApplyOrder().GetApplyOrder(kt, filter)
	if err != nil {
		logs.Errorf("failed to get apply order by id: %s, err: %v, rid: %s", key, err, kt.Rid)
		return nil, err
	}

	return order, nil
}

// lockApplyOrder locks apply order to avoid order repeat dispatch
func (d *Dispatcher) lockApplyOrder(kt *kit.Kit, order *types.ApplyOrder) error {
	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", order.SubOrderId),
		tools.RuleNotEqual("status", types.ApplyStatusMatching),
	)

	// 校验该查询条件是否存在子单数据
	applyOrder, err := model.Operation().ApplyOrder().GetApplyOrder(kt, filter)
	if err != nil {
		logs.Errorf("failed to query apply order, id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return err
	}

	if applyOrder == nil || len(applyOrder.SubOrderId) == 0 {
		logs.Warnf("failed to lock apply order, apply order not found, subOrderID: %s, stage: %s, status: %s",
			order.SubOrderId, order.Stage, order.Status)
		return errf.Newf(errf.InvalidParameter, "failed to lock apply order, apply order not found, subOrderID: %s",
			order.SubOrderId)
	}

	update := &cvmapplyproto.ZiyanCvmApplySuborderUpdateReq{
		Status:    types.ApplyStatusMatching,
		RetryTime: cvt.ValToPtr(order.RetryTime + 1),
	}
	if err = model.Operation().ApplyOrder().UpdateApplyOrder(kt, filter, update); err != nil {
		logs.Errorf("failed to lock apply order, id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return err
	}

	return nil
}

// generateDevices generates devices to meet order need
func (d *Dispatcher) generateDevices(kt *kit.Kit, order *types.ApplyOrder) error {
	if d.generator == nil {
		return fmt.Errorf("failed to generate device, for generator is nil")
	}
	if order.Spec == nil {
		return fmt.Errorf("failed to generate device, for order spec is nil")
	}

	switch order.ResourceType {
	case types.ResourceTypeCvm:
		return d.generator.GenerateCVM(kt, order)
	case types.ResourceTypeIdcDvm, types.ResourceTypeQcloudDvm:
		return d.generator.GenerateDVM(kt, order)
	case types.ResourceTypePm:
		return d.generator.MatchPM(kt, order)
	case types.ResourceTypeUpgradeCvm:
		return d.generator.UpgradeCVM(kt, order)
	default:
		logs.Errorf("unknown resource type: %s", order.ResourceType)
		return fmt.Errorf("unknown resource type: %s", order.ResourceType)
	}
}

// updateApplyOrderStatus update apply order status
func (d *Dispatcher) updateApplyOrderStatus(kt *kit.Kit, order *types.ApplyOrder,
	stage types.TicketStage, status types.ApplyStatus) error {

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
