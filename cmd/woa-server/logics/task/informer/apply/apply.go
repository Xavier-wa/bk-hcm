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

// Package apply apply informer
package apply

import (
	"context"
	"errors"
	"sync"
	"time"

	"hcm/cmd/woa-server/model/task"
	"hcm/cmd/woa-server/storage/dal"
	"hcm/cmd/woa-server/storage/stream"
	"hcm/cmd/woa-server/storage/stream/types"
	recovertask "hcm/cmd/woa-server/types/task"
	tasktype "hcm/cmd/woa-server/types/task"
	"hcm/pkg"
	"hcm/pkg/criteria/mapstr"
	"hcm/pkg/logs"
	"hcm/pkg/tools/metadata"

	"github.com/tidwall/gjson"
	"k8s.io/client-go/util/workqueue"
)

// Interface apply informer interface
type Interface interface {
	// Pop gets head of apply info queue
	Pop() (string, error)
	// Stop stops apply informer watch loop.
	Stop()
}

// applyInformer apply informer which list and watch database and cache apply order info
type applyInformer struct {
	key     Key
	watchDB dal.DB
	event   stream.LoopInterface
	queue   workqueue.RateLimitingInterface

	stopCh   chan struct{}
	stopOnce sync.Once
}

// New creates an apply informer
func New(loopWatch stream.LoopInterface, watchDB dal.DB) (*applyInformer, error) {
	applyInformer := &applyInformer{
		key:     KeyApply,
		watchDB: watchDB,
		event:   loopWatch,
		queue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "apply"),
		stopCh:  make(chan struct{}),
	}

	if err := applyInformer.Run(); err != nil {
		logs.Errorf("failed to start apply informer, err: %v", err)
		return nil, err
	}

	return applyInformer, nil
}

// Run starts apply informer
func (a *applyInformer) Run() error {
	return a.listAndWatchApplyOrder()
}

// Pop gets head of apply info queue
func (a *applyInformer) Pop() (string, error) {
	obj, shutdown := a.queue.Get()
	if shutdown {
		return "", nil
	}

	defer a.queue.Done(obj)

	id, ok := obj.(string)
	if !ok {
		a.queue.Forget(obj)
		logs.Warnf("Expected string in queue but got %#v", obj)
		return "", errors.New("got non-string from queue")
	}

	a.queue.Forget(obj)

	return id, nil
}

// Stop stops apply informer watch loop.
func (a *applyInformer) Stop() {
	a.stopOnce.Do(func() {
		close(a.stopCh)
		a.queue.ShutDown()
	})
}

// listAndWatchApplyOrder list and watch database and cache apply order into queue
func (a *applyInformer) listAndWatchApplyOrder() error {
	// list apply order
	applyOrders, err := a.listApplyOrder()
	if err != nil {
		return err
	}

	for _, order := range applyOrders {
		a.queue.Add(order)
	}
	// watch apply order
	handler := newApplyTokenHandler(a.key, a.watchDB)
	startTime := &types.TimeStamp{Sec: uint32(time.Now().Unix())}

	loopOpts := &types.LoopOneOptions{
		LoopOptions: types.LoopOptions{
			Name: "apply_info",
			WatchOpt: &types.WatchOptions{
				Options: types.Options{
					EventStruct:     new(map[string]interface{}),
					Collection:      pkg.BKTableNameApplyOrder,
					StartAfterToken: nil,
					StartAtTime:     startTime,
					// TODO: add failure callback
					WatchFatalErrorCallback: nil,
				},
			},
			TokenHandler: handler,
			RetryOptions: &types.RetryOptions{
				MaxRetryCount: 4,
				RetryDuration: 500 * time.Millisecond,
			},
			StopNotifier: a.stopCh,
		},
		EventHandler: &types.OneHandler{
			DoAdd:    a.onUpsert,
			DoUpdate: a.onUpsert,
			DoDelete: a.onDelete,
		},
	}

	return a.event.WithOne(loopOpts)
}

// listApplyOrder gets apply order list from database
func (a *applyInformer) listApplyOrder() ([]string, error) {
	restartTime := time.Now()
	expireTime := restartTime.AddDate(0, 0, recovertask.ExpireDays)
	filter := map[string]interface{}{
		"status": &mapstr.MapStr{
			pkg.BKDBIN: []string{string(tasktype.ApplyStatusWaitForMatch), string(tasktype.ApplyStatusMatchedSome)},
		},
		"create_at": mapstr.MapStr{
			"$gte": expireTime,
			"$lt":  restartTime,
		},
	}

	page := metadata.BasePage{
		Limit: pkg.BKNoLimit,
	}

	orders, err := model.Operation().ApplyOrder().FindManyApplyOrder(context.Background(), page, filter)
	if err != nil {
		logs.Errorf("failed to list apply order by filter: %+v, err: %v", filter, err)
		return nil, err
	}

	orderIds := make([]string, 0)
	for _, order := range orders {
		orderIds = append(orderIds, order.SubOrderId)
	}

	return orderIds, nil
}

// onUpsert set or update apply order cache
func (a *applyInformer) onUpsert(e *types.Event) bool {
	logs.V(5).Infof("received apply order event, op: %s, doc: %s, rid: %s", e.OperationType, e.DocBytes, e.ID())

	// TODO: suborder_id as const
	id := gjson.GetBytes(e.DocBytes, "suborder_id").String()
	if len(id) <= 0 {
		logs.Errorf("received invalid apply event, skip, op: %s, doc: %s, rid: %s", e.OperationType, e.DocBytes, e.ID())
		return false
	}

	a.queue.Add(id)

	return false
}

// onDelete delete apply order cache
func (a *applyInformer) onDelete(e *types.Event) bool {
	// TODO: add exception log
	return false
}
