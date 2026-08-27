/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package metrics

import (
	"sync"
	"time"

	"hcm/pkg/criteria/enumor"

	"github.com/prometheus/client_golang/prometheus"
)

// ccWatchMetric holds the hcm_cc_watch_* collectors of the cloud-server cc
// watch consumer side.
//
// The label sets are:
//
//	hcm_cc_watch_processing_start_timestamp_seconds      : tenant, res_type
//	hcm_cc_watch_processing_last_event_timestamp_seconds : tenant, res_type
//	hcm_cc_watch_consume_cost_seconds                    : tenant, res_type
//	hcm_cc_watch_events_total                            : tenant, res_type, event_type
//	hcm_cc_watch_hosts_total                             : tenant, operation
//	hcm_cc_watch_hosts_sync_total                        : tenant, operation, result
type ccWatchMetric struct {
	// processingTs 当前正在消费的批次的开始时间（unix 秒），空闲时为 0。
	processingTs *prometheus.GaugeVec
	// processingLastTs 当前正在消费批次的最后一个事件时间（unix 秒），空闲时为 0。
	processingLastTs *prometheus.GaugeVec
	// consumeCost 单批 consume 段耗时（不含 fetch 长轮询等待）。
	consumeCost *prometheus.HistogramVec
	// eventsTotal 按事件类型统计的事件数。
	eventsTotal *prometheus.CounterVec
	// hostsTotal 去重后下发的主机数。
	hostsTotal *prometheus.CounterVec
	// syncTotal 子批（100 台一次 hc 调用）同步成功/失败的主机数。
	syncTotal *prometheus.CounterVec
}

var (
	ccWatchOnce sync.Once
	ccWatch     *ccWatchMetric
)

// initCCWatchMetric registers the cc watch metrics on the global registerer.
// 惰性注册且幂等；watcher 在 service 启动后运行，此时 InitMetrics 已完成，
// 帮助函数内也会兜底触发注册。
func initCCWatchMetric() {
	ccWatchOnce.Do(func() {
		m := &ccWatchMetric{}

		m.processingTs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: CCWatchSubSys,
			Name:      "processing_start_timestamp_seconds",
			Help:      "start time (unix seconds) of the consume batch in progress, 0 when idle.",
		}, []string{"tenant", "res_type"})

		m.processingLastTs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: CCWatchSubSys,
			Name:      "processing_last_event_timestamp_seconds",
			Help:      "last event time (unix seconds) of the consume batch in progress, 0 when idle.",
		}, []string{"tenant", "res_type"})

		m.consumeCost = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: CCWatchSubSys,
			Name:      "consume_cost_seconds",
			Help:      "cost seconds of the consume stage of one event batch (fetch long-poll excluded).",
			Buckets:   []float64{0.1, 0.5, 1, 2, 3, 4, 5, 8, 10, 15, 20, 30, 60},
		}, []string{"tenant", "res_type"})

		m.eventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: CCWatchSubSys,
			Name:      "events_total",
			Help:      "total events fetched from cc, by event type.",
		}, []string{"tenant", "res_type", "event_type"})

		m.hostsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: CCWatchSubSys,
			Name:      "hosts_total",
			Help:      "total deduplicated hosts dispatched to hc-service, by operation.",
		}, []string{"tenant", "operation"})

		m.syncTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: CCWatchSubSys,
			Name:      "hosts_sync_total",
			Help:      "total hosts of sub-batch (100 hosts per hc call) sync results, by operation and result.",
		}, []string{"tenant", "operation", "result"})

		Register().MustRegister(m.processingTs, m.processingLastTs,
			m.consumeCost, m.eventsTotal, m.hostsTotal, m.syncTotal)
		ccWatch = m
	})
}

// EnsureCCWatchMetric is idempotent and registers the cc_watch_* metrics.
func EnsureCCWatchMetric() {
	initCCWatchMetric()
}

// SetCCWatchProcessingStart marks the consume batch of (tenant, resType) as in
// progress, starting at startAt.
// 消费期间持续可见：卡在单次下游调用上时，可与"无事件"状态区分。
func SetCCWatchProcessingStart(tenant, resType string, startAt time.Time) {
	if ccWatch == nil {
		initCCWatchMetric()
	}
	ccWatch.processingTs.With(prometheus.Labels{
		"tenant":   tenant,
		"res_type": resType,
	}).Set(float64(startAt.Unix()))
}

// ClearCCWatchProcessingStart marks the consume batch of (tenant, resType) as
// finished by resetting the gauge to 0.
func ClearCCWatchProcessingStart(tenant, resType string) {
	if ccWatch == nil {
		initCCWatchMetric()
	}
	ccWatch.processingTs.With(prometheus.Labels{
		"tenant":   tenant,
		"res_type": resType,
	}).Set(0)
}

// SetCCWatchProcessingLastEvent records the last event time of the consume
// batch in progress. 与 processing_start 同生命周期：批内最新事件时间用于
// 追尾观察（最新事件滞后）。
func SetCCWatchProcessingLastEvent(tenant, resType string, lastEventAt time.Time) {
	if ccWatch == nil {
		initCCWatchMetric()
	}
	ccWatch.processingLastTs.With(prometheus.Labels{
		"tenant":   tenant,
		"res_type": resType,
	}).Set(float64(lastEventAt.Unix()))
}

// ClearCCWatchProcessingLastEvent resets the last event time of the consume
// batch to 0 when the batch is finished or event time is unavailable.
func ClearCCWatchProcessingLastEvent(tenant, resType string) {
	if ccWatch == nil {
		initCCWatchMetric()
	}
	ccWatch.processingLastTs.With(prometheus.Labels{
		"tenant":   tenant,
		"res_type": resType,
	}).Set(0)
}

// ObserveCCWatchConsumeCost records the consume stage cost of one event batch.
func ObserveCCWatchConsumeCost(tenant, resType string, cost time.Duration) {
	if ccWatch == nil {
		initCCWatchMetric()
	}
	ccWatch.consumeCost.With(prometheus.Labels{
		"tenant":   tenant,
		"res_type": resType,
	}).Observe(cost.Seconds())
}

// AddCCWatchEvents records fetched events of one batch, by event type.
func AddCCWatchEvents(tenant, resType, eventType string, count int) {
	if ccWatch == nil {
		initCCWatchMetric()
	}
	ccWatch.eventsTotal.With(prometheus.Labels{
		"tenant":     tenant,
		"res_type":   resType,
		"event_type": eventType,
	}).Add(float64(count))
}

// AddCCWatchHosts records deduplicated hosts dispatched to hc-service.
func AddCCWatchHosts(tenant string, operation enumor.CCWatchOp, count int) {
	if ccWatch == nil {
		initCCWatchMetric()
	}
	ccWatch.hostsTotal.With(prometheus.Labels{
		"tenant":    tenant,
		"operation": string(operation),
	}).Add(float64(count))
}

// AddCCWatchSyncResult records the sync result of one host sub-batch.
func AddCCWatchSyncResult(tenant string, operation enumor.CCWatchOp, result enumor.CCWatchResult,
	hostCount int) {

	if ccWatch == nil {
		initCCWatchMetric()
	}
	ccWatch.syncTotal.With(prometheus.Labels{
		"tenant":    tenant,
		"operation": string(operation),
		"result":    string(result),
	}).Add(float64(hostCount))
}
