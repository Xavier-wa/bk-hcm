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

package runobserve

import (
	"context"
	"time"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/metrics"
	"hcm/pkg/rest"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ObserveEvents inspects translated AG-UI events and reports run metrics in place.
// 无本轮元数据（如 /history）直接返回；自身 panic 不外泄，也不改写事件。
func ObserveEvents(ctx context.Context, evts []aguievents.Event) {
	rid := rest.RidFromContext(ctx)
	defer func() {
		if rec := recover(); rec != nil {
			logs.Errorf("observe run events panic, recover: %v, rid: %s", rec, rid)
		}
	}()

	meta := RunMetaFromCtx(ctx)
	if meta == nil {
		return
	}

	for _, evt := range evts {
		if evt == nil {
			continue
		}
		observeEvent(meta, evt, rid)
	}
}

// observeEvent 按单条 AG-UI 生命周期事件就地打点。
func observeEvent(meta *RunMeta, evt aguievents.Event, rid string) {
	scene := meta.SceneForMetric()
	switch evt.Type() {
	case aguievents.EventTypeRunStarted:
		holdInflight(meta)
	case aguievents.EventTypeRunFinished:
		recordRunDuration(meta, enumor.AiagentRunStateFinished, scene, rid)
		releaseInflight(meta)
	case aguievents.EventTypeRunError:
		recordRunDuration(meta, enumor.AiagentRunStateError, scene, rid)
		releaseInflight(meta)
	default:
	}
}

// recordRunDuration 对终态 run 打点耗时直方图，并打原始秒数日志供后续校准分桶。
func recordRunDuration(meta *RunMeta, state enumor.AiagentRunState, scene, rid string) {
	cost := calcRunDuration(meta)
	metrics.IncAiagentRunTotal(meta.BkBizID, state, scene)
	metrics.ObserveAiagentRunDuration(meta.BkBizID, state, scene, cost)
	logs.Infof("observe aiagent run duration success, state: %s, scene: %s, bk_biz_id: %d, "+
		"cost_seconds: %.3f, rid: %s", state, scene, meta.BkBizID, cost.Seconds(), rid)
}

// AfterTranslate observes events at runner emitEvent, including RUN_STARTED
// which never goes through Translate. It must return the original event and
// a nil error; a non-nil error would abort the run.
func AfterTranslate(ctx context.Context, evt aguievents.Event) (aguievents.Event, error) {
	if evt != nil {
		ObserveEvents(ctx, []aguievents.Event{evt})
	}
	return evt, nil
}

// BeginRun holds inflight without an AG-UI event. Production holds on the
// real RUN_STARTED in AfterTranslate; tests still use this helper.
func BeginRun(ctx context.Context) {
	holdInflight(RunMetaFromCtx(ctx))
}

// EndRun drops the inflight hold if a terminal AG-UI event was not observed
// (client disconnect, HITL interrupt, Translate miss). Idempotent.
func EndRun(ctx context.Context) {
	releaseInflight(RunMetaFromCtx(ctx))
}

// holdInflight 对本轮 /agui 请求将 run_inflight +1。
// CAS 保证同一 RunMeta 只持有一次。
func holdInflight(meta *RunMeta) {
	if meta == nil {
		return
	}
	if !meta.inflightHeld.CompareAndSwap(false, true) {
		return
	}
	metrics.AddAiagentRunInflight(meta.BkBizID, 1)
}

// releaseInflight 对本轮已持有的 inflight 将 run_inflight -1。
// CAS 保证未持有或已释放时不把 gauge 减成负数。
func releaseInflight(meta *RunMeta) {
	if meta == nil {
		return
	}
	if !meta.inflightHeld.CompareAndSwap(true, false) {
		return
	}
	metrics.AddAiagentRunInflight(meta.BkBizID, -1)
}

// calcRunDuration 计算本轮从 StartedAt 到现在的耗时；StartedAt 为零则返回 0。
func calcRunDuration(meta *RunMeta) time.Duration {
	if meta == nil || meta.StartedAt.IsZero() {
		return 0
	}
	return time.Since(meta.StartedAt)
}
