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
	"encoding/json"
	"sync/atomic"
	"time"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

type runMetaCtxKey struct{}

// RunMeta 是一轮真实 /agui 对话的观测上下文。中间件在请求进来时把它挂到 context 上。
// 后面打点、算耗时、配对 inflight 都从这里读，而不是再去解析请求体。
// *RunMeta 会同时被 handler（EndRun）与 runner（RefreshScene / AfterTranslate）持有。
type RunMeta struct {
	BkBizID   int64
	StartedAt time.Time
	// scene 是本轮打点用的场景，原子存取。入口写入当前 session_tag，
	// graph 提交合法 tag 后覆盖为最终场景；未分类或非法 tag 留空。
	scene atomic.Pointer[string]
	// inflightHeld is true while this /agui request is counted in run_inflight.
	// Hold happens on the real RUN_STARTED at AfterTranslate; EndRun releases
	// if the SSE ends without a terminal event.
	inflightHeld atomic.Bool
}

// NewRunMeta 构造一轮 /agui 的观测上下文。scene 经原子写入。
func NewRunMeta(bkBizID int64, scene enumor.IntentType, startedAt time.Time) *RunMeta {
	meta := &RunMeta{
		BkBizID:   bkBizID,
		StartedAt: startedAt,
	}
	meta.SetScene(scene)
	return meta
}

// SetScene 原子写入本轮打点场景，供 handler 与 runner 跨 goroutine 安全读写。
func (m *RunMeta) SetScene(scene enumor.IntentType) {
	if m == nil {
		return
	}
	if scene == "" {
		m.scene.Store(nil)
		return
	}
	s := string(scene)
	m.scene.Store(&s)
}

// WithRunMeta stores RunMeta into the request context.
func WithRunMeta(ctx context.Context, meta *RunMeta) context.Context {
	return context.WithValue(ctx, runMetaCtxKey{}, meta)
}

// RunMetaFromCtx returns the attached RunMeta, or nil when absent (e.g. /history).
func RunMetaFromCtx(ctx context.Context) *RunMeta {
	meta, _ := ctx.Value(runMetaCtxKey{}).(*RunMeta)
	return meta
}

// SceneForMetric 原子读取终态指标的 scene 标签；未分类时为空。
func (m *RunMeta) SceneForMetric() string {
	if m == nil {
		return ""
	}
	p := m.scene.Load()
	if p == nil {
		return ""
	}
	return *p
}

// RefreshScene 用 graph 事件 StateDelta 里的 session_tag 覆盖为最终 Scene。
//
// graph 完成时会把最终 state（含 session_tag）放进 StateDelta，且早于 RUN_FINISHED。
// 无 tag、或 tag 非法时不覆盖，避免把入口已有的合法场景冲掉。
func RefreshScene(ctx context.Context, stateDelta map[string][]byte) {
	rid := rest.RidFromContext(ctx)
	meta := RunMetaFromCtx(ctx)
	if meta == nil || len(stateDelta) == 0 {
		return
	}
	raw, ok := stateDelta[constant.StateKeySessionTag]
	if !ok || len(raw) == 0 {
		return
	}
	var tag string
	if err := json.Unmarshal(raw, &tag); err != nil {
		logs.Errorf("refresh scene failed, err: %v, rid: %s", err, rid)
		return
	}
	if tag == "" {
		return
	}
	scene := NormalizeScene(enumor.IntentType(tag))
	if scene == "" {
		return
	}
	meta.SetScene(scene)
}

// NormalizeScene maps a session tag to a bounded scene label.
// Empty, illegal, or unsupported tags stay empty.
func NormalizeScene(tag enumor.IntentType) enumor.IntentType {
	if tag.Validate() == nil {
		return tag
	}
	return ""
}
