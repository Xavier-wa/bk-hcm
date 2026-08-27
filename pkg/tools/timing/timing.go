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

// Package timing provides a lightweight collector of named step costs, used to
// render one-line summaries of where a batch's time went.
//
// 该包只负责“采集机制”，不规定业务语义：各业务侧在其之上构建自己的记录类型
// （如 cloud-server 的 WatchBatchTrace、hc-service 的 SyncHostTrace）。所有方法均 nil
// 安全，未挂载 collector 的调用路径可直接调用而无需判空。与 OpenTelemetry trace
// 无关，只做耗时聚合。
package timing

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"hcm/pkg/tools/converter"
)

// Step is a timed stage inside a collector.
type Step struct {
	// Name 步骤名，如 list_biz_host、cc_watch。
	Name string
	// Cost 步骤耗时。
	Cost time.Duration
}

// Collector collects step costs. It is safe for concurrent use.
type Collector struct {
	mu    sync.RWMutex
	steps []Step
}

// New creates a collector.
func New() *Collector {
	return &Collector{}
}

// Track starts timing a step and returns the function that records it.
// Typical usage: `defer c.Track("consume")()`.
func (c *Collector) Track(name string) func() {
	if c == nil {
		return func() {}
	}

	start := time.Now()
	return func() {
		c.AddStep(Step{Name: name, Cost: time.Since(start)})
	}
}

// AddStep appends a step whose cost is measured by the caller.
func (c *Collector) AddStep(step Step) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.steps = append(c.steps, step)
}

// Cost returns the cost of the first recorded step with the given name.
func (c *Collector) Cost(name string) time.Duration {
	if c == nil {
		return 0
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, step := range c.steps {
		if step.Name == name {
			return step.Cost
		}
	}
	return 0
}

// Has reports whether at least one step with the given name was recorded.
func (c *Collector) Has(name string) bool {
	if c == nil {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, step := range c.steps {
		if step.Name == name {
			return true
		}
	}
	return false
}

// Summary renders the aggregated steps as a single log-friendly line, with the
// steps ordered by cost descending, e.g. `cc_watch=1.20s(x2) classify=30ms(x1)`.
func (c *Collector) Summary() string {
	if c == nil {
		return ""
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	type agg struct {
		name  string
		cost  time.Duration
		count int
	}
	aggMap := make(map[string]*agg)
	for _, step := range c.steps {
		item, exists := aggMap[step.Name]
		if !exists {
			item = &agg{name: step.Name}
			aggMap[step.Name] = item
		}
		item.cost += step.Cost
		item.count++
	}

	items := converter.MapValueToSlice(aggMap)
	sort.Slice(items, func(i, j int) bool { return items[i].cost > items[j].cost })

	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("%s=%s(x%d)", item.name, item.cost, item.count))
	}
	return strings.Join(parts, " ")
}
