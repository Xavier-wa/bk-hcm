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
	"hcm/pkg"
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/mapstr"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
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
	applyOrder, err := d.getApplyOrder(key)
	if err != nil {
		logs.Errorf("get apply order by key %s failed, err: %v", key, err)
		return err
	}

	// check order stage
	if applyOrder.Stage != types.TicketStageRunning {
		logs.Infof("apply order %s need not dispatch, stage: %s", key, applyOrder.Stage)
		return nil
	}

	// check order status
	if !shouldDispatch(applyOrder.Status) {
		logs.Infof("apply order %s need not dispatch, status: %s", key, applyOrder.Status)
		return nil
	}

	// check retry time
	retryLimit := uint(3)
	if applyOrder.RetryTime > retryLimit {
		logs.Infof("apply order %s need not dispatch, for retry time %d exceeds limit %d", key, applyOrder.RetryTime,
			retryLimit)
		// update order status to TERMINATE
		if err := d.updateApplyOrderStatus(applyOrder, types.TicketStageSuspend,
			types.ApplyStatusTerminate); err != nil {
			logs.Errorf("failed to update apply order %s status, err: %v, rid: %s", key, err, kt.Rid)
		}
		return nil
	}

	// lock apply order
	if err := d.lockApplyOrder(applyOrder); err != nil {
		logs.Errorf("failed to lock apply order %s, err: %v", key, err)
		return err
	}
	// start generate step
	if err := record.StartStep(applyOrder.SubOrderId, types.StepNameGenerate); err != nil {
		logs.Errorf("failed to start generate step, order id: %s, err: %v", key, err)
		return err
	}

	// generate devices according to apply order
	if err := d.generateDevices(kt, applyOrder); err != nil {
		logs.Errorf("failed to generate device, order id: %s, err: %v", key, err)
		// update generate step record
		if errStep := record.UpdateGenerateStep(applyOrder.SubOrderId, applyOrder.TotalNum, err); errStep != nil {
			logs.Errorf("failed to generate device, order id: %s, err: %v", key, errStep)
			return errStep
		}

		// update order status to TERMINATE
		errUpdate := d.updateApplyOrderStatus(applyOrder, types.TicketStageSuspend, types.ApplyStatusTerminate)
		if errUpdate != nil {
			logs.Warnf("failed to update apply order %s status, err: %v", key, errUpdate)
		}

		return err
	}

	// update generate step record
	if err := record.UpdateGenerateStep(applyOrder.SubOrderId, applyOrder.TotalNum, nil); err != nil {
		logs.Errorf("failed to generate device, order id: %s, err: %v", key, err)
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
func (d *Dispatcher) getApplyOrder(key string) (*types.ApplyOrder, error) {
	filter := &mapstr.MapStr{
		"suborder_id": key,
	}
	order, err := model.Operation().ApplyOrder().GetApplyOrder(context.Background(), filter)
	if err != nil {
		logs.Errorf("failed to get apply order by id: %s", key)
		return nil, err
	}

	return order, nil
}

// lockApplyOrder locks apply order to avoid order repeat dispatch
func (d *Dispatcher) lockApplyOrder(order *types.ApplyOrder) error {
	filter := &mapstr.MapStr{
		"suborder_id": order.SubOrderId,
		"status": &mapstr.MapStr{
			pkg.BKDBNE: types.ApplyStatusMatching,
		},
	}

	doc := &mapstr.MapStr{
		"status":     types.ApplyStatusMatching,
		"retry_time": order.RetryTime + 1,
		"update_at":  time.Now(),
	}

	if err := model.Operation().ApplyOrder().UpdateApplyOrder(context.Background(), filter, doc); err != nil {
		logs.Errorf("failed to lock apply order, id: %s, err: %v", order.SubOrderId, err)
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
func (d *Dispatcher) updateApplyOrderStatus(order *types.ApplyOrder, stage types.TicketStage,
	status types.ApplyStatus) error {

	filter := &mapstr.MapStr{
		"suborder_id": order.SubOrderId,
	}

	doc := &mapstr.MapStr{
		"stage":     stage,
		"status":    status,
		"update_at": time.Now(),
	}

	if err := model.Operation().ApplyOrder().UpdateApplyOrder(context.Background(), filter, doc); err != nil {
		logs.Errorf("failed to update apply order status, id: %s, err: %v", order.SubOrderId, err)
		return err
	}

	return nil
}
