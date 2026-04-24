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
	"errors"
	"sync"
	"time"

	tasktype "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	ziyan "hcm/pkg/client/data-service/tcloud-ziyan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
	"k8s.io/client-go/util/workqueue"
)

const (
	// defaultPollInterval default polling interval
	defaultPollInterval = 5 * time.Second
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
	client       *ziyan.Client
	queue        workqueue.RateLimitingInterface
	pollInterval time.Duration
	lastPollTime time.Time
	wg           sync.WaitGroup
	mu           sync.Mutex
	stopCh       chan struct{}
	stopOnce     sync.Once
}

// New creates an apply informer
func New(client *ziyan.Client) (*applyInformer, error) {
	if client == nil {
		return nil, errors.New("ziyan data-service client is nil")
	}

	applyInformer := &applyInformer{
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "apply"),
		stopCh:       make(chan struct{}),
		client:       client,
		pollInterval: defaultPollInterval,
		lastPollTime: time.Now(),
	}

	if err := applyInformer.Run(); err != nil {
		logs.Errorf("failed to start apply informer, err: %v", err)
		return nil, err
	}

	return applyInformer, nil
}

// Run starts apply informer
func (a *applyInformer) Run() error {
	// list and watch apply subOrders on startup
	if err := a.listAndWatchApplySubOrder(); err != nil {
		return err
	}

	// start polling goroutine
	a.wg.Add(1)
	go a.pollLoop()

	return nil
}

// Stop stops the apply informer
func (a *applyInformer) Stop() {
	a.stopOnce.Do(func() {
		close(a.stopCh)
		a.queue.ShutDown()
		a.wg.Wait()
	})
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
		logs.Warnf("expected string in queue but got %#v", obj)
		return "", errors.New("got non-string from queue")
	}

	a.queue.Forget(obj)

	return id, nil
}

// pollLoop continuously polls for apply suborder changes
func (a *applyInformer) pollLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCh:
			logs.Infof("apply suborder informer polling stopped")
			return
		case <-ticker.C:
			if err := a.pollApplySubOrders(); err != nil {
				logs.Errorf("failed to poll apply suborders, err: %v", err)
			}
		}
	}
}

// listAndWatchApplySubOrder lists and watch apply subOrders and adds them to queue
func (a *applyInformer) listAndWatchApplySubOrder() error {
	subOrderIDs, err := a.listApplySubOrders()
	if err != nil {
		return err
	}

	for _, id := range subOrderIDs {
		a.queue.Add(id)
	}

	logs.Infof("apply informer initialized with %d subOrders", len(subOrderIDs))
	return nil
}

// listApplySubOrders gets apply order list from MySQL
func (a *applyInformer) listApplySubOrders() ([]string, error) {
	kt := core.NewBackendKit()

	// build filter: status in (wait_for_match, matched_some) and created_at within expiration window
	now := time.Now()
	expireTime := now.AddDate(0, 0, tasktype.ExpireDays)

	filterExpr, err := tools.And(
		tools.RuleIn("status", []string{
			string(tasktype.ApplyStatusWaitForMatch),
			string(tasktype.ApplyStatusMatchedSome),
		}),
		tools.RuleGreaterThanEqual("created_at", expireTime.Format(constant.TimeStdFormat)),
		tools.RuleLessThan("created_at", now.Format(constant.TimeStdFormat)),
	)
	if err != nil {
		logs.Errorf("failed to build filter expression, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return a.queryApplySubOrders(kt, filterExpr)
}

// pollApplySubOrders polls for apply suborder changes since last poll time
func (a *applyInformer) pollApplySubOrders() error {
	kt := core.NewBackendKit()

	a.mu.Lock()
	lastPoll := a.lastPollTime
	a.mu.Unlock()

	// capture current poll upper bound and query with half-open time window: [lastPoll, pollStart)
	pollStart := time.Now()
	filterExpr, err := tools.And(
		tools.RuleIn("status", []string{
			string(tasktype.ApplyStatusWaitForMatch),
			string(tasktype.ApplyStatusMatchedSome),
		}),
		tools.RuleGreaterThanEqual("updated_at", lastPoll.Format(constant.TimeStdFormat)),
		tools.RuleLessThan("updated_at", pollStart.Format(constant.TimeStdFormat)),
	)
	if err != nil {
		logs.Errorf("failed to build poll filter expression, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	subOrderIDs, err := a.queryApplySubOrders(kt, filterExpr)
	if err != nil {
		return err
	}

	for _, id := range subOrderIDs {
		a.queue.Add(id)
	}

	// advance cursor only after successful processing to avoid skipping updates when query fails.
	a.mu.Lock()
	a.lastPollTime = pollStart
	a.mu.Unlock()

	if len(subOrderIDs) > 0 {
		logs.V(5).Infof("apply informer polled %d orders, rid: %s", len(subOrderIDs), kt.Rid)
	}

	return nil
}

// queryApplySubOrders queries apply suborders from data-service
func (a *applyInformer) queryApplySubOrders(kt *kit.Kit, filterExpr *filter.Expression) ([]string, error) {
	subOrderIDs := make([]string, 0)

	req := &cvmapplyproto.ZiyanCvmApplySuborderListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
		Fields: []string{"suborder_id"},
	}

	for {
		resp, err := a.client.ZiyanCvmApplySuborder.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			logs.Errorf("failed to list apply suborders, err: %v, req: %+v, rid: %s", err, cvt.PtrToVal(req), kt.Rid)
			return nil, err
		}

		for _, order := range resp.Details {
			if order.SuborderID != "" {
				subOrderIDs = append(subOrderIDs, order.SuborderID)
			}
		}

		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	return subOrderIDs, nil
}
