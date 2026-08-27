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

package bkcc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/metrics"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/tools/timing"
)

// WatchBatchTrace is the observability record of one cc watch batch: fetching
// events from cc, consuming them and dispatching hosts to hc-service.
// 通过 kt.Ctx 传递，全部方法 nil 安全，未挂载时调用不产生任何影响。
type WatchBatchTrace struct {
	// kt 本批上下文，用于内部日志带 rid。
	kt *kit.Kit
	// steps 复用通用机制包采集步骤耗时。
	steps *timing.Collector

	// tenant 本批所属租户，作为全部指标的 tenant label。
	tenant string
	// resType 本批监听的资源类型，作为指标的 res_type label。
	resType cmdb.CursorType

	mu sync.RWMutex
	// startAt 本批开始时间，用于计算总耗时。
	startAt time.Time
	// eventCount 本批从 cc 拉到的事件数。
	eventCount int
	// eventTypes 按事件类型统计的事件数。
	eventTypes map[cmdb.EventType]int
	// firstCursor 本批首个事件的 cursor。
	firstCursor *cmdb.CursorDetail
	// lastCursor 本批最后一个事件的 cursor，提交后成为下批起点。
	lastCursor *cmdb.CursorDetail
	// upsertHostCount 去重后待 upsert 的主机数。
	upsertHostCount int
	// deleteHostCount 去重后待删除的主机数。
	deleteHostCount int
	// parseFailedCount 事件解析失败数。
	parseFailedCount int
	// consumeCost 本批 consume 段耗时。
	consumeCost time.Duration
	// patches 按（厂商，隔离空间）划分的下发批进度。
	patches map[string]*watchPatch
}

// watchPatch is the dispatch progress of one (vendor, space) patch.
type watchPatch struct {
	// vendor 批内主机的云厂商。
	vendor enumor.Vendor
	// space 隔离空间，公有云为账号 ID，其他（如自研云）为业务 ID。
	space string
	// hostCount 批内主机数。
	hostCount int
	// subBatchDone 已成功下发的主机小批数。
	subBatchDone int
	// subBatchTotal 按 100 台拆出的主机小批总数。
	subBatchTotal int
	// status 批状态，取值见 enumor.CCWatchPatchStatusXxx。
	status enumor.CCWatchPatchStatus
	// startAt 批开始处理时间。
	startAt time.Time
	// cost 批处理耗时，批结束时写入。
	cost time.Duration
}

type watchTraceCtxKey struct{}

// newWatchBatchTrace creates the trace of one watch batch and returns a kit
// carrying it in the context.
func newWatchBatchTrace(kt *kit.Kit, resType cmdb.CursorType) (*kit.Kit, *WatchBatchTrace) {
	tr := &WatchBatchTrace{
		kt:         kt,
		steps:      timing.New(),
		tenant:     kt.TenantID,
		resType:    resType,
		startAt:    time.Now(),
		eventTypes: make(map[cmdb.EventType]int),
		patches:    make(map[string]*watchPatch),
	}

	return kt.NewSubKitWithCtx(context.WithValue(kt.Ctx, watchTraceCtxKey{}, tr)), tr
}

// watchTraceFromCtx extracts the trace carried by the context, nil if absent.
// 供深层调用点在不改函数签名的前提下写入观测数据。
func watchTraceFromCtx(ctx context.Context) *WatchBatchTrace {
	if ctx == nil {
		return nil
	}
	tr, _ := ctx.Value(watchTraceCtxKey{}).(*WatchBatchTrace)
	return tr
}

// Track starts timing a step and returns the function that records it.
func (t *WatchBatchTrace) Track(step enumor.CCWatchStep) func() {
	if t == nil {
		return func() {}
	}
	return t.steps.Track(string(step))
}

// TrackConsume starts the consume stage: it marks the batch as in progress for
// the processing_*_timestamp metrics and starts timing the consume step.
// 返回的收尾函数记录步骤耗时并清除"处理中"标记，defer 调用即可覆盖消费
// 失败与 panic 路径。last 事件时间在 SetFetchResult 后已就绪；若 cursor
// 解码失败则保持 0，避免错误时间参与 lag 告警。
func (t *WatchBatchTrace) TrackConsume() func() {
	if t == nil {
		return func() {}
	}

	t.mu.RLock()
	lastCursor := t.lastCursor
	t.mu.RUnlock()

	tenant, resType := t.tenant, string(t.resType)
	metrics.SetCCWatchProcessingStart(tenant, resType, time.Now())
	if lastCursor != nil && lastCursor.Decoded {
		metrics.SetCCWatchProcessingLastEvent(tenant, resType, lastCursor.EventTime)
	} else {
		metrics.ClearCCWatchProcessingLastEvent(tenant, resType)
	}
	consumeStart := time.Now()

	return func() {
		cost := time.Since(consumeStart)
		//step 仅用于日志统计打印，不用于指标上报
		t.AddStep(enumor.CCWatchStepConsume, cost)
		t.mu.Lock()
		t.consumeCost = cost
		t.mu.Unlock()
		metrics.ClearCCWatchProcessingStart(tenant, resType)
		metrics.ClearCCWatchProcessingLastEvent(tenant, resType)
	}
}

// AddStep records a step whose cost is measured by the caller.
func (t *WatchBatchTrace) AddStep(step enumor.CCWatchStep, cost time.Duration) {
	if t == nil {
		return
	}
	t.steps.AddStep(timing.Step{Name: string(step), Cost: cost})
}

// SetFetchResult records the outcome of one cc fetch.
func (t *WatchBatchTrace) SetFetchResult(events []cmdb.WatchEventDetail) {
	if t == nil || len(events) == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.eventCount = len(events)
	for _, event := range events {
		t.eventTypes[event.EventType]++
	}
	t.firstCursor = cmdb.DecodeCursor(t.kt, events[0].Cursor)
	t.lastCursor = cmdb.DecodeCursor(t.kt, events[len(events)-1].Cursor)
}

// SetHostCount records the deduplicated host counts of this batch.
func (t *WatchBatchTrace) SetHostCount(upsertCount, deleteCount, parseFailed int) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.upsertHostCount = upsertCount
	t.deleteHostCount = deleteCount
	t.parseFailedCount = parseFailed
}

// BeginPatch registers a (vendor, space) dispatch patch.
// subBatchTotal 是本批按 100 台拆出的主机小批数，调用方按现有拆批规则计算。
func (t *WatchBatchTrace) BeginPatch(vendor enumor.Vendor, space any, hostCount, subBatchTotal int) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.patches[patchKey(vendor, space)] = &watchPatch{
		vendor:        vendor,
		space:         spaceString(space),
		hostCount:     hostCount,
		subBatchTotal: subBatchTotal,
		status:        enumor.CCWatchPatchStatusRunning,
		startAt:       time.Now(),
	}
}

// IncrPatchSubBatch records one successfully dispatched sub-batch of a patch.
func (t *WatchBatchTrace) IncrPatchSubBatch(vendor enumor.Vendor, space any) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if patch, exists := t.patches[patchKey(vendor, space)]; exists {
		patch.subBatchDone++
	}
}

// FinishPatch closes a patch with its final status.
func (t *WatchBatchTrace) FinishPatch(vendor enumor.Vendor, space any) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	patch, exists := t.patches[patchKey(vendor, space)]
	if !exists {
		return
	}
	patch.cost = time.Since(patch.startAt)
	patch.status = enumor.CCWatchPatchStatusDone
}

// FetchedSummary renders the fetched batch info as a log-friendly fragment,
// used by the batch start log.
func (t *WatchBatchTrace) FetchedSummary() string {
	if t == nil {
		return ""
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return fmt.Sprintf("events: %d, event types: %v, first event time: %s, last event time: %s, "+
		"first cursor: %s, last cursor: %s", t.eventCount, t.eventTypes, t.firstCursor.EventTimeString(),
		t.lastCursor.EventTimeString(), cursorRaw(t.firstCursor), cursorRaw(t.lastCursor))
}

// ConsumedSummary renders the consume outcome of this batch as a log-friendly
// one-line summary, used by the batch end log.
func (t *WatchBatchTrace) ConsumedSummary() string {
	if t == nil {
		return ""
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return fmt.Sprintf("events: %d, upsert hosts: %d, delete hosts: %d, parse failed: %d, "+
		"patches: %d, cost: %s, steps: [%s]",
		t.eventCount, t.upsertHostCount, t.deleteHostCount, t.parseFailedCount,
		len(t.patches), time.Since(t.startAt), t.steps.Summary())
}

// PatchSummary renders one patch's progress as a log-friendly fragment.
func (t *WatchBatchTrace) PatchSummary(vendor enumor.Vendor, space any) string {
	if t == nil {
		return ""
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	patch, exists := t.patches[patchKey(vendor, space)]
	if !exists {
		return ""
	}
	summary := fmt.Sprintf("vendor: %s, space: %s, hosts: %d, status: %s, cost: %s", patch.vendor, patch.space,
		patch.hostCount, patch.status, patch.cost)
	// 小批计数目前只有 ziyan 链路上报，其余厂商恒为 0，打出来会被误读成“一批都没发出去”
	if patch.subBatchTotal > 0 {
		summary += fmt.Sprintf(", done sub-batches: %d/%d", patch.subBatchDone, patch.subBatchTotal)
	}
	return summary
}

// AddSyncResult records the sync result of one host sub-batch and reports it
// to the hosts_sync_total metric immediately.
// 与批末 flush 不同步进行：成功率按 success/(success+failed) 自闭合计算，
// 分子分母同点采集，换取慢批中途的失败可见性。
func (t *WatchBatchTrace) AddSyncResult(operation enumor.CCWatchOp, result enumor.CCWatchResult,
	hostCount int) {
	if t == nil {
		return
	}

	metrics.AddCCWatchSyncResult(t.tenant, operation, result, hostCount)
}

// flushMetrics exports the batch-level metrics from the trace fields, called
// once at the end of a watched batch.
// consume_cost / events_total / hosts_total 同点导出，保证两两相除
// （单台/单事件均耗）时分子分母同属"已完成批次"集合，短窗口不错位。
func (t *WatchBatchTrace) flushMetrics() {
	if t == nil {
		return
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	// 空批（watched 但无事件）没有消费内容，跳过避免 Observe(0) 稀释耗时分布。
	if t.eventCount == 0 {
		return
	}

	metrics.ObserveCCWatchConsumeCost(t.tenant, string(t.resType), t.consumeCost)

	for eventType, count := range t.eventTypes {
		metrics.AddCCWatchEvents(t.tenant, string(t.resType), string(eventType), count)
	}

	metrics.AddCCWatchHosts(t.tenant, enumor.CCWatchOpUpsert, t.upsertHostCount)
	metrics.AddCCWatchHosts(t.tenant, enumor.CCWatchOpDelete, t.deleteHostCount)
}

func cursorRaw(cursor *cmdb.CursorDetail) string {
	if cursor == nil {
		return ""
	}
	return cursor.Raw
}

func patchKey(vendor enumor.Vendor, space any) string {
	return string(vendor) + "/" + spaceString(space)
}

func spaceString(space any) string {
	if space == nil {
		return ""
	}
	return fmt.Sprintf("%v", space)
}
