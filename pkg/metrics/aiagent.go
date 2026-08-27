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
	"strconv"
	"sync"
	"time"

	"hcm/pkg/criteria/enumor"

	"github.com/prometheus/client_golang/prometheus"
)

type aiagentMetric struct {
	runTotal    *prometheus.CounterVec
	runDuration *prometheus.HistogramVec
	runInflight *prometheus.GaugeVec
	runCancel   *prometheus.CounterVec
}

var (
	aiagentOnce sync.Once
	aiagent     *aiagentMetric
)

func initAiagentMetric() {
	aiagentOnce.Do(func() {
		m := &aiagentMetric{}
		m.runTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: AiagentSubSys,
			Name:      "run_total",
			Help:      "the total count of terminal aiagent runs by state / bkcc_biz_id / scene.",
		}, []string{LabelState, LabelBKCCBizID, LabelScene})
		Register().MustRegister(m.runTotal)

		m.runDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: AiagentSubSys,
			Name:      "run_duration_seconds",
			Help: "the duration seconds of an aiagent run that finished or errored " +
				"by state / bkcc_biz_id / scene.",
			Buckets: []float64{2, 5, 10, 15, 20, 30, 45, 60, 90, 120, 180, 300},
		}, []string{LabelState, LabelBKCCBizID, LabelScene})
		Register().MustRegister(m.runDuration)

		m.runInflight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: AiagentSubSys,
			Name:      "run_inflight",
			Help:      "the number of in-flight aiagent runs observed from AG-UI events by bkcc_biz_id.",
		}, []string{LabelBKCCBizID})
		Register().MustRegister(m.runInflight)

		m.runCancel = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: AiagentSubSys,
			Name:      "run_cancel",
			Help:      "the total count of aiagent /cancel requests by bkcc_biz_id / scene.",
		}, []string{LabelBKCCBizID, LabelScene})
		Register().MustRegister(m.runCancel)

		aiagent = m
	})
}

// EnsureAiagentMetric is idempotent and registers the hcm_aiagent_* metrics.
func EnsureAiagentMetric() {
	initAiagentMetric()
}

func isAiagentRunTotalState(state enumor.AiagentRunState) bool {
	switch state {
	case enumor.AiagentRunStateFinished, enumor.AiagentRunStateError, enumor.AiagentRunStateUnknown:
		return true
	default:
		return false
	}
}

func isAiagentDurationState(state enumor.AiagentRunState) bool {
	switch state {
	case enumor.AiagentRunStateFinished, enumor.AiagentRunStateError:
		return true
	default:
		return false
	}
}

// IncAiagentRunTotal increments run_total for terminal states
// (finished / error / unknown). running and cancel are ignored.
func IncAiagentRunTotal(bkBizID int64, state enumor.AiagentRunState, scene string) {
	if aiagent == nil {
		initAiagentMetric()
	}
	if !isAiagentRunTotalState(state) {
		return
	}
	aiagent.runTotal.With(prometheus.Labels{
		LabelState:     string(state),
		LabelBKCCBizID: strconv.FormatInt(bkBizID, 10),
		LabelScene:     scene,
	}).Inc()
}

// ObserveAiagentRunDuration records one run duration sample for finished or error.
// The bkBizID parameter is written as bkcc_biz_id.
func ObserveAiagentRunDuration(bkBizID int64, state enumor.AiagentRunState, scene string, cost time.Duration) {
	if aiagent == nil {
		initAiagentMetric()
	}
	if !isAiagentDurationState(state) {
		return
	}
	aiagent.runDuration.With(prometheus.Labels{
		LabelState:     string(state),
		LabelBKCCBizID: strconv.FormatInt(bkBizID, 10),
		LabelScene:     scene,
	}).Observe(cost.Seconds())
}

// AddAiagentRunInflight adjusts the in-flight gauge for one bkcc_biz_id.
// Callers must pair +1/-1 with the same bkBizID; unpaired Add(-1) will go negative.
// scene is intentionally not a label: STARTED often still has an empty scene.
func AddAiagentRunInflight(bkBizID int64, delta float64) {
	if aiagent == nil {
		initAiagentMetric()
	}
	aiagent.runInflight.With(prometheus.Labels{
		LabelBKCCBizID: strconv.FormatInt(bkBizID, 10),
	}).Add(delta)
}

// IncAiagentRunCancel increments run_cancel once per authorized /cancel request.
func IncAiagentRunCancel(bkBizID int64, scene string) {
	if aiagent == nil {
		initAiagentMetric()
	}
	aiagent.runCancel.With(prometheus.Labels{
		LabelBKCCBizID: strconv.FormatInt(bkBizID, 10),
		LabelScene:     scene,
	}).Inc()
}
