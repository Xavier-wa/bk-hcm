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
	"sync/atomic"
	"testing"
	"time"

	"hcm/pkg/kit"
	"hcm/pkg/metrics"
)

func TestDispatcherTimeoutDrops(t *testing.T) {
	metrics.EnsureAiagentMetric()
	started := make(chan struct{})
	block := make(chan struct{})
	d := NewDispatcher(1, 50*time.Millisecond, func(_ *kit.Kit, _ string, _ bool) {
		close(started)
		<-block
	})
	kt := kit.New()
	d.Submit(kt, "r1")
	<-started
	d.Submit(kt, "r2")
	close(block)
	d.Shutdown(time.Second)
}

func TestDispatcherNegativeWait(t *testing.T) {
	var n atomic.Int32
	d := NewDispatcher(1, -time.Second, func(_ *kit.Kit, _ string, _ bool) {
		n.Add(1)
		time.Sleep(20 * time.Millisecond)
	})
	kt := kit.New()
	done := make(chan struct{})
	go func() {
		d.Submit(kt, "a")
		d.Submit(kt, "b")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("-1s wait must eventually acquire")
	}
	d.Shutdown(time.Second)
	if n.Load() != 2 {
		t.Fatalf("jobs = %d, want 2", n.Load())
	}
}

func TestDispatcherSubmitOverwrite(t *testing.T) {
	got := make(chan bool, 1)
	d := NewDispatcher(1, time.Second, func(_ *kit.Kit, runID string, overwrite bool) {
		if runID != "r1" {
			t.Errorf("runID = %s, want r1", runID)
		}
		got <- overwrite
	})
	if !d.SubmitOverwrite(kit.New(), "r1", true) {
		t.Fatal("SubmitOverwrite must accept the job")
	}
	d.Shutdown(time.Second)
	select {
	case overwrite := <-got:
		if !overwrite {
			t.Fatal("job must receive overwrite=true")
		}
	default:
		t.Fatal("job did not run")
	}
}
