/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may obtain a copy of the License at http://opensource.org/licenses/MIT
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

package eval

import (
	"context"
	"errors"
	"sync"
	"time"

	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/metrics"

	"golang.org/x/sync/semaphore"
)

// JobFunc evaluates one run_id. overwrite is true for manual reeval.
type JobFunc func(kt *kit.Kit, runID string, overwrite bool)

// Dispatcher limits in-process eval concurrency with a weighted semaphore.
type Dispatcher struct {
	sem         *semaphore.Weighted
	waitTimeout time.Duration
	job         JobFunc
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewDispatcher creates a dispatcher. waitTimeout < 0 waits forever on Acquire.
func NewDispatcher(concurrency int64, waitTimeout time.Duration, job JobFunc) *Dispatcher {
	if concurrency < 1 {
		concurrency = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Dispatcher{
		sem:         semaphore.NewWeighted(concurrency),
		waitTimeout: waitTimeout,
		job:         job,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Submit queues a realtime job. Positive waitTimeout caps Acquire; negative waits
// until a slot is free or shutdown. Timeout increments dropped_total.
func (d *Dispatcher) Submit(kt *kit.Kit, runID string) bool {
	return d.submit(kt, runID, false, false)
}

// SubmitOverwrite queues a manual reeval with the same concurrency and wait cap as Submit.
func (d *Dispatcher) SubmitOverwrite(kt *kit.Kit, runID string, overwrite bool) bool {
	return d.submit(kt, runID, overwrite, false)
}

// SubmitUncapped queues one job and waits for a slot without submitWaitTimeout.
func (d *Dispatcher) SubmitUncapped(kt *kit.Kit, runID string) {
	d.submit(kt, runID, false, true)
}

func (d *Dispatcher) submit(kt *kit.Kit, runID string, overwrite, unlimited bool) bool {
	if d == nil || d.job == nil {
		return false
	}
	if !d.acquire(kt, unlimited) {
		return false
	}
	d.startJob(kt, runID, overwrite)
	return true
}

// SubmitBatch queues many compensate jobs, each waiting uncapped for a slot.
func (d *Dispatcher) SubmitBatch(kt *kit.Kit, runIDs []string) {
	for _, runID := range runIDs {
		d.SubmitUncapped(kt, runID)
	}
}

func (d *Dispatcher) acquire(kt *kit.Kit, unlimited bool) bool {
	// d.ctx 是进程级关停信号，没有 rid。Acquire 仍以它为父 context，
	// 避免绑请求 ctx（Ledger 异步 Submit 时 HTTP 可能已结束）。
	acquireCtx := d.ctx
	rid := ""
	if kt != nil {
		rid = kt.Rid
	}
	var cancel context.CancelFunc
	if !unlimited && d.waitTimeout >= 0 {
		acquireCtx, cancel = context.WithTimeout(d.ctx, d.waitTimeout)
	}
	if cancel != nil {
		defer cancel()
	}
	if err := d.sem.Acquire(acquireCtx, 1); err != nil {
		if !unlimited && d.waitTimeout >= 0 && errors.Is(err, context.DeadlineExceeded) {
			metrics.IncAiagentEvalDropped()
			logs.Warnf("eval submit wait timeout, dropped job, rid: %s", rid)
			return false
		}
		logs.Warnf("eval submit acquire failed, err: %v, rid: %s", err, rid)
		return false
	}
	return true
}

func (d *Dispatcher) startJob(kt *kit.Kit, runID string, overwrite bool) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer d.sem.Release(1)
		defer func() {
			if rec := recover(); rec != nil {
				logs.Errorf("eval job panic, recover: %v, run_id: %s, rid: %s", rec, runID, kt.Rid)
				metrics.IncAiagentEvalTotal(metrics.AiagentEvalResultFail)
			}
		}()
		d.job(kt, runID, overwrite)
	}()
}

// Shutdown cancels in-flight Acquire waits and drains running jobs.
func (d *Dispatcher) Shutdown(timeout time.Duration) {
	if d == nil {
		return
	}
	d.cancel()
	// Drain in a goroutine so Shutdown can still respect timeout if a job hangs.
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()
	if timeout <= 0 {
		<-done
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		logs.Warnf("eval dispatcher drain timeout, leftover jobs may still run")
	}
}
