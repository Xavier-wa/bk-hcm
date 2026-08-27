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
	"time"

	"hcm/pkg/criteria/enumor"

	"github.com/prometheus/client_golang/prometheus"
)

// ResSyncSubSys defines the hc-service resource sync related sub system.
const ResSyncSubSys = "res_sync"

// resSyncMetric holds the hcm_res_sync_* collectors of the hc-service sync side.
//
//	hcm_res_sync_cost_seconds: vendor, resource, step, source, result
type resSyncMetric struct {
	// cost 各同步环节的耗时分布。
	cost *prometheus.HistogramVec
}

var (
	resSyncOnce sync.Once
	resSync     *resSyncMetric
)

// initResSyncMetric registers the res sync metrics on the global registerer.
func initResSyncMetric() {
	resSyncOnce.Do(func() {
		m := &resSyncMetric{}

		m.cost = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: ResSyncSubSys,
			Name:      "cost_seconds",
			Help:      "cost seconds of one resource sync stage, by vendor, resource, step, source and result.",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 20, 30, 60, 120, 300, 600},
		}, []string{"vendor", "resource", "step", "source", "result"})

		Register().MustRegister(m.cost)
		resSync = m
	})
}

// EnsureResSyncMetric is idempotent and registers the res_sync_* metrics.
func EnsureResSyncMetric() {
	initResSyncMetric()
}

// ObserveResSyncCost records the cost of one sync stage. label 值 MUST 低基数。
func ObserveResSyncCost(vendor, resource string, step enumor.ResSyncStep, source enumor.ResSyncSource,
	result enumor.ResSyncResult, cost time.Duration) {

	if resSync == nil {
		initResSyncMetric()
	}
	resSync.cost.With(prometheus.Labels{
		"vendor":   vendor,
		"resource": resource,
		"step":     string(step),
		"source":   string(source),
		"result":   string(result),
	}).Observe(cost.Seconds())
}
