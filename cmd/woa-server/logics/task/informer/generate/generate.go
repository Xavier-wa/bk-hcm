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

	"hcm/cmd/woa-server/storage/dal"
	"hcm/cmd/woa-server/storage/stream"
	"hcm/cmd/woa-server/storage/stream/types"
	"hcm/pkg"
	"hcm/pkg/logs"

	"github.com/tidwall/gjson"
	"k8s.io/client-go/util/workqueue"
)

// Interface generate informer interface
type Interface interface {
	// Pop gets head of generate record info queue
	Pop() (uint64, error)
	// Stop stops generate informer watch loop.
	Stop()
}

// generateInformer generate informer which list and watch database and cache generate record info
type generateInformer struct {
	key     Key
	watchDB dal.DB
	event   stream.LoopInterface

	queue workqueue.RateLimitingInterface

	stopCh   chan struct{}
	stopOnce sync.Once
}

// New create a generate informer
func New(loopWatch stream.LoopInterface, watchDB dal.DB) (*generateInformer, error) {
	generateInformer := &generateInformer{
		key:     KeyGenerate,
		watchDB: watchDB,
		event:   loopWatch,
		queue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "generate"),
		stopCh:  make(chan struct{}),
	}

	if err := generateInformer.Run(); err != nil {
		logs.Errorf("failed to start generate informer, err: %v", err)
		return nil, err
	}

	return generateInformer, nil
}

// Run starts generate informer
func (i *generateInformer) Run() error {
	return i.listAndWatchGenerateRecord()
}

// Pop gets head of generate record info queue
func (i *generateInformer) Pop() (uint64, error) {
	obj, shutdown := i.queue.Get()
	if shutdown {
		return 0, nil
	}

	defer i.queue.Done(obj)

	id, ok := obj.(uint64)
	if !ok {
		i.queue.Forget(obj)
		logs.Warnf("Expected int in queue but got %#v", obj)
		return 0, errors.New("got non-int from queue")
	}

	i.queue.Forget(obj)

	return id, nil
}

// Stop stops generate informer watch loop.
func (i *generateInformer) Stop() {
	i.stopOnce.Do(func() {
		close(i.stopCh)
		i.queue.ShutDown()
	})
}

// listAndWatchGenerateRecord list and watch database and cache generate record into queue
func (i *generateInformer) listAndWatchGenerateRecord() error {
	// watch generate record
	handler := newGenerateTokenHandler(i.key, i.watchDB)
	startTime := &types.TimeStamp{Sec: uint32(time.Now().Unix())}

	loopOpts := &types.LoopOneOptions{
		LoopOptions: types.LoopOptions{
			Name: "generate_info",
			WatchOpt: &types.WatchOptions{
				Options: types.Options{
					EventStruct:     new(map[string]interface{}),
					Collection:      pkg.BKTableNameGenerateRecord,
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
			StopNotifier: i.stopCh,
		},
		EventHandler: &types.OneHandler{
			DoAdd:    i.onUpsert,
			DoUpdate: i.onUpsert,
			DoDelete: i.onDelete,
		},
	}

	return i.event.WithOne(loopOpts)
}

// onUpsert set or update generate cache
func (i *generateInformer) onUpsert(e *types.Event) bool {
	logs.V(5).Infof("received generate record event, op: %s, doc: %s, rid: %s", e.OperationType, e.DocBytes, e.ID())

	id := gjson.GetBytes(e.DocBytes, "generate_id").Uint()
	if id < 0 {
		logs.Errorf("received invalid generate record event, skip, op: %s, doc: %s, rid: %s", e.OperationType,
			e.DocBytes, e.ID())
		return false
	}

	i.queue.Add(id)

	return false
}

// onDelete delete generate cache
func (i *generateInformer) onDelete(e *types.Event) bool {
	// TODO
	return false
}
