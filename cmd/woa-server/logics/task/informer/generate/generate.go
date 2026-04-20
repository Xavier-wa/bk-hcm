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

// Package generate implements generate informer
package generate

import (
	"errors"
	"sync"
	"time"

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

// Interface generate informer interface
type Interface interface {
	// Pop gets head of generate record info queue
	Pop() (string, error)
	// Stop stops generate informer watch loop.
	Stop()
}

// generateInformer generate informer which list and watch database and cache generate record info
type generateInformer struct {
	client       *ziyan.Client
	queue        workqueue.RateLimitingInterface
	pollInterval time.Duration
	lastPollTime time.Time
	wg           sync.WaitGroup
	mu           sync.Mutex
	stopCh       chan struct{}
	stopOnce     sync.Once
}

// New creates a generate informer
func New(client *ziyan.Client) (*generateInformer, error) {
	if client == nil {
		return nil, errors.New("ziyan data-service client is nil")
	}

	generateInformer := &generateInformer{
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "generate"),
		stopCh:       make(chan struct{}),
		client:       client,
		pollInterval: defaultPollInterval,
		lastPollTime: time.Now(),
	}

	if err := generateInformer.Run(); err != nil {
		logs.Errorf("failed to start generate informer, err: %v", err)
		return nil, err
	}

	return generateInformer, nil
}

// Run starts generate informer
func (g *generateInformer) Run() error {
	// start polling goroutine
	g.wg.Add(1)
	go g.pollLoop()

	logs.Infof("generate informer started")
	return nil
}

// Stop stops the generate informer
func (g *generateInformer) Stop() {
	g.stopOnce.Do(func() {
		close(g.stopCh)
		g.queue.ShutDown()
		g.wg.Wait()
	})
}

// Pop gets head of generate record info queue
func (g *generateInformer) Pop() (string, error) {
	obj, shutdown := g.queue.Get()
	if shutdown {
		return "", nil
	}

	defer g.queue.Done(obj)

	id, ok := obj.(string)
	if !ok {
		g.queue.Forget(obj)
		logs.Warnf("expected string in queue but got %#v", obj)
		return "", errors.New("got non-string from queue")
	}

	g.queue.Forget(obj)

	return id, nil
}

// pollLoop continuously polls for generate record changes
func (g *generateInformer) pollLoop() {
	defer g.wg.Done()

	ticker := time.NewTicker(g.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCh:
			logs.Infof("generate informer polling stopped")
			return
		case <-ticker.C:
			if err := g.pollGenerateRecords(); err != nil {
				logs.Errorf("failed to poll generate records, err: %v", err)
			}
		}
	}
}

// pollGenerateRecords polls for generate record changes since last poll time
func (g *generateInformer) pollGenerateRecords() error {
	kt := core.NewBackendKit()

	g.mu.Lock()
	lastPoll := g.lastPollTime
	g.lastPollTime = time.Now()
	g.mu.Unlock()

	// build filter: updated_at > lastPollTime
	filterExpr := &filter.Expression{
		Op: filter.And,
		Rules: []filter.RuleFactory{
			tools.RuleGreaterThan("updated_at", lastPoll.Format(constant.TimeStdFormat)),
		},
	}

	recordIDs, err := g.queryGenerateRecords(kt, filterExpr)
	if err != nil {
		return err
	}

	for _, id := range recordIDs {
		g.queue.Add(id)
	}

	if len(recordIDs) > 0 {
		logs.V(5).Infof("generate informer polled %d records, rid: %s", len(recordIDs), kt.Rid)
	}

	return nil
}

// queryGenerateRecords queries generate records from data-service
func (g *generateInformer) queryGenerateRecords(kt *kit.Kit, filterExpr *filter.Expression) ([]string, error) {
	recordIDs := make([]string, 0)

	req := &cvmapplyproto.ZiyanCvmGenerateRecordListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
		Fields: []string{"generate_id"},
	}

	for {
		resp, err := g.client.ZiyanCvmGenerateRecord.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			logs.Errorf("failed to list generate records, err: %v, req: %+v, rid: %s", err, cvt.PtrToVal(req), kt.Rid)
			return nil, err
		}

		for _, record := range resp.Details {
			if record.GenerateID != "" {
				recordIDs = append(recordIDs, record.GenerateID)
			}
		}

		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	return recordIDs, nil
}
