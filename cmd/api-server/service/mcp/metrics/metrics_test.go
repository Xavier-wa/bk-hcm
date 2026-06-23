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

package metrics

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// reset 仅用于测试，重置 sync.Once 与 holder。
// 测试代码应配合 initWithRegisterer(prometheus.NewRegistry()) 使用以避免
// 与生产 registerer 冲突。
func reset() {
	once = sync.Once{}
	holder = nil
}

// resetAndInit 重置 holder 与 once，并把 metric 注册到一个独立 registry
// （避免与全局 prometheus registerer 冲突）。
// 该函数仅供测试调用；生产代码路径只调用 InitMCPMetrics 一次。
func resetAndInit(t *testing.T) {
	t.Helper()
	reset()
	initWithRegisterer(prometheus.NewRegistry())
}

func TestInitMCPMetrics_Idempotent(t *testing.T) {
	// 用独立 registry 跑一次 once-protected 路径，第二次调用应直接返回不再注册。
	reset()
	once.Do(func() { initWithRegisterer(prometheus.NewRegistry()) })
	first := holder

	InitMCPMetrics() // 走全局 registerer 但因 once 已耗尽，不会真正执行注册
	if holder != first {
		t.Fatal("holder should not be replaced on second InitMCPMetrics call")
	}
	if holder == nil {
		t.Fatal("holder should be non-nil after first init")
	}
}

func TestIncToolsCallAccumulates(t *testing.T) {
	resetAndInit(t)

	IncToolsCall("send_message", "bk-hcm-test", StatusSuccess)
	IncToolsCall("send_message", "bk-hcm-test", StatusSuccess)
	IncToolsCall("send_message", "bk-hcm-test", StatusError)

	successCount := testutil.ToFloat64(holder.toolsCallTotal.WithLabelValues(
		"send_message", "bk-hcm-test", StatusSuccess))
	if successCount != 2 {
		t.Errorf("success counter = %v, want 2", successCount)
	}

	errCount := testutil.ToFloat64(holder.toolsCallTotal.WithLabelValues(
		"send_message", "bk-hcm-test", StatusError))
	if errCount != 1 {
		t.Errorf("error counter = %v, want 1", errCount)
	}
}

func TestObserveToolsCallDuration(t *testing.T) {
	resetAndInit(t)

	ObserveToolsCallDuration("send_message", "bk-hcm-test", 0.5)
	ObserveToolsCallDuration("send_message", "bk-hcm-test", 1.5)

	// CollectAndCount 返回 metric 数量，histogram 计为单 family。
	got := testutil.CollectAndCount(holder.toolsCallDurationSeconds)
	if got == 0 {
		t.Errorf("expected non-zero histogram samples, got %d", got)
	}
}

func TestBridgeActiveTasksGauge(t *testing.T) {
	resetAndInit(t)

	IncBridgeActiveTasks(1)
	IncBridgeActiveTasks(1)
	if got := testutil.ToFloat64(holder.bridgeActiveTasks); got != 2 {
		t.Errorf("bridgeActiveTasks = %v, want 2 after two Inc", got)
	}

	IncBridgeActiveTasks(-1)
	if got := testutil.ToFloat64(holder.bridgeActiveTasks); got != 1 {
		t.Errorf("bridgeActiveTasks = %v, want 1 after Inc(-1)", got)
	}

	SetBridgeActiveTasks(0)
	if got := testutil.ToFloat64(holder.bridgeActiveTasks); got != 0 {
		t.Errorf("bridgeActiveTasks = %v, want 0 after Set(0)", got)
	}
}

func TestSetInternalSchemaStale(t *testing.T) {
	resetAndInit(t)

	SetInternalSchemaStale(false)
	if got := testutil.ToFloat64(holder.internalSchemaStale); got != 0 {
		t.Errorf("internalSchemaStale = %v, want 0 for fresh", got)
	}

	SetInternalSchemaStale(true)
	if got := testutil.ToFloat64(holder.internalSchemaStale); got != 1 {
		t.Errorf("internalSchemaStale = %v, want 1 for stale", got)
	}
}

func TestAccessorsBeforeInit_NoPanic(t *testing.T) {
	// 重置但**不**调用 InitMCPMetrics，模拟生产代码意外提前调用。
	reset()

	// 所有 accessor 应当静默 no-op，不 panic。
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("accessor panicked when holder is nil: %v", r)
		}
	}()
	IncToolsCall("send_message", "n", StatusSuccess)
	ObserveToolsCallDuration("send_message", "n", 1.0)
	IncBridgeActiveTasks(1)
	SetBridgeActiveTasks(0)
	SetInternalSchemaStale(true)
}
