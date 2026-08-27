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

package common

import (
	"context"
	"time"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/metrics"
	"hcm/pkg/tools/timing"
)

// resSyncFullCtxKey marks the kit context as a full sync.
type resSyncFullCtxKey struct{}

// MarkFullSyncSource returns a sub kit marked as full sync. 仅全量同步入口调用。
func MarkFullSyncSource(kt *kit.Kit) *kit.Kit {
	return kt.NewSubKitWithCtx(context.WithValue(kt.Ctx, resSyncFullCtxKey{}, true))
}

// resolveSource 以全量标记判定 full / incremental（RequestSource 的异步语义会误标，勿用）。
func resolveSource(kt *kit.Kit) enumor.ResSyncSource {
	if full, _ := kt.Ctx.Value(resSyncFullCtxKey{}).(bool); full {
		return enumor.ResSyncSourceFull
	}
	return enumor.ResSyncSourceIncremental
}

// ResSyncTrace is the observability base of one resource sync batch: 步骤耗时采集 +
// 指标导出（hcm_res_sync_cost_seconds）。无自有日志字段的 vendor 直接使用，
// 有外壳的（如 ziyan SyncHostTrace）组合内嵌。全部方法 nil 安全。
type ResSyncTrace struct {
	// steps 步骤耗时采集器。
	steps *timing.Collector
	// vendor 指标 vendor 标签。
	vendor string
	// resource 指标 resource 标签。
	resource string
	// source 指标 source 标签（full / incremental）。
	source enumor.ResSyncSource
	// metricSteps 需要导出为指标的 step 白名单；日志专用的细粒度 step 不在其中。
	metricSteps []enumor.ResSyncStep
	// startAt 本批开始时间，用于计算 total 耗时。
	startAt time.Time
}

// NewResSyncTrace creates the trace of one sync batch.
func NewResSyncTrace(kt *kit.Kit, vendor enumor.Vendor, resource enumor.CloudResourceType,
	metricSteps []enumor.ResSyncStep) *ResSyncTrace {

	return &ResSyncTrace{
		steps:       timing.New(),
		vendor:      string(vendor),
		resource:    string(resource),
		source:      resolveSource(kt),
		metricSteps: metricSteps,
		startAt:     time.Now(),
	}
}

// Track starts timing a step and returns the function that records it.
func (t *ResSyncTrace) Track(name string) func() {
	if t == nil {
		return func() {}
	}
	return t.steps.Track(name)
}

// AddStep appends a step whose cost is measured by the caller.
func (t *ResSyncTrace) AddStep(name string, cost time.Duration) {
	if t == nil {
		return
	}
	t.steps.AddStep(timing.Step{Name: name, Cost: cost})
}

// Summary renders the aggregated steps as a single log-friendly line.
func (t *ResSyncTrace) Summary() string {
	if t == nil {
		return ""
	}
	return t.steps.Summary()
}

// Cost returns the total cost since the trace was created.
func (t *ResSyncTrace) Cost() time.Duration {
	if t == nil {
		return 0
	}
	return time.Since(t.startAt)
}

// FlushMetrics 导出白名单内已执行环节的耗时指标，并附带 total。syncErr 非 nil 时带 result=failed。
func (t *ResSyncTrace) FlushMetrics(syncErr error) {
	if t == nil {
		return
	}

	result := enumor.ResSyncResultSuccess
	if syncErr != nil {
		result = enumor.ResSyncResultFailed
	}

	for _, step := range t.metricSteps {
		if t.steps.Has(string(step)) {
			metrics.ObserveResSyncCost(t.vendor, t.resource, step, t.source, result, t.steps.Cost(string(step)))
		}
	}
	metrics.ObserveResSyncCost(t.vendor, t.resource, enumor.ResSyncStepTotal, t.source, result, t.Cost())
}
