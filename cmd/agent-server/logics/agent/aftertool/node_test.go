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
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package aftertool

import (
	"context"
	"testing"

	"hcm/pkg/criteria/constant"

	agent "trpc.group/trpc-go/trpc-agent-go/agent"
)

// ctxWithForwarded builds a context carrying an invocation whose RuntimeState holds the given
// forwarded resume value (as written by service.go: tryPrepareAutoResume every resume Run).
func ctxWithForwarded(forwarded any) context.Context {
	ctx, inv := agent.EnsureInvocation(context.Background())
	inv.RunOptions = agent.RunOptions{RuntimeState: map[string]any{
		constant.StateKeyForwardedResumeValue: forwarded,
	}}
	return ctx
}

// TestResolveAfterToolForwardedResumeValue covers the resume source-separation decision used by
// doInterrupt: a non-empty forwarded value in RuntimeState means "frontend structured protocol"
// (emit resume_forwarded event and adopt it); its absence means the free-form resumeValue should be
// used instead.
func TestResolveAfterToolForwardedResumeValue(t *testing.T) {
	planJSON := `{"path_param":{"bk_biz_id":"213"},"body_param":{"bk_username":"u"}}`

	t.Run("forwarded present is adopted", func(t *testing.T) {
		got := resolveForwardedResumeValue(ctxWithForwarded(planJSON))
		if got != planJSON {
			t.Errorf("got %q, want forwarded plan JSON", got)
		}
	})

	t.Run("no forwarded falls back to empty", func(t *testing.T) {
		if got := resolveForwardedResumeValue(context.Background()); got != "" {
			t.Errorf("got %q, want empty (no invocation / no runtime state)", got)
		}
	})

	t.Run("empty forwarded string falls back to empty", func(t *testing.T) {
		if got := resolveForwardedResumeValue(ctxWithForwarded("")); got != "" {
			t.Errorf("got %q, want empty (empty forwarded string)", got)
		}
	})

	// 关键回归：自由文本即使是合法 JSON，也不应被当作 forwarded 结构化选择。
	// resolveAfterToolForwardedResumeValue 只读 RuntimeState，从不嗅探 resumeValue，
	// 因此 RuntimeState 无 forwarded 时返回空，doInterrupt 会回退到 resumeValue 自由文本通道，
	// 不再误发 resume_forwarded 事件。
	t.Run("free text JSON is not treated as forwarded", func(t *testing.T) {
		if got := resolveForwardedResumeValue(context.Background()); got != "" {
			t.Errorf("got %q, want empty (JSON free text must not be sniffed as forwarded)", got)
		}
	})
}
