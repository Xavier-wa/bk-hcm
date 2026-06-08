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

package agent

import (
	"context"
	"testing"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

func TestMakeIntentRoutingFunc(t *testing.T) {
	route := makeIntentRoutingFunc()
	ctx := context.Background()

	tests := []struct {
		name       string
		state      graph.State
		wantTarget string
	}{
		{
			name: "host_apply routes to llm",
			state: graph.State{
				constant.StateKeyIntent: string(enumor.IntentTypeHostApply),
			},
			wantTarget: "llm",
		},
		{
			name: "resource_query routes to fallback",
			state: graph.State{
				constant.StateKeyIntent: string(enumor.IntentTypeResourceQuery),
			},
			wantTarget: "fallback",
		},
		{
			name: "chat routes to fallback",
			state: graph.State{
				constant.StateKeyIntent: string(enumor.IntentTypeChat),
			},
			wantTarget: "fallback",
		},
		{
			name:       "missing intent routes to fallback",
			state:      graph.State{},
			wantTarget: "fallback",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := route(ctx, tc.state)
			if err != nil {
				t.Fatalf("route() error = %v", err)
			}
			if got != tc.wantTarget {
				t.Errorf("route() = %q, want %q", got, tc.wantTarget)
			}
		})
	}
}

func TestMakePostFallbackRoutingFunc(t *testing.T) {
	route := makePostFallbackRoutingFunc()
	ctx := context.Background()

	tests := []struct {
		name       string
		state      graph.State
		wantTarget string
	}{
		{
			name: "host_apply routes to llm",
			state: graph.State{
				constant.StateKeyIntent: string(enumor.IntentTypeHostApply),
			},
			wantTarget: "llm",
		},
		{
			name: "chat routes to intent_recognition",
			state: graph.State{
				constant.StateKeyIntent: string(enumor.IntentTypeChat),
			},
			wantTarget: "intent_recognition",
		},
		{
			name: "resource_query routes to intent_recognition",
			state: graph.State{
				constant.StateKeyIntent: string(enumor.IntentTypeResourceQuery),
			},
			wantTarget: "intent_recognition",
		},
		{
			name:       "missing intent routes to intent_recognition",
			state:      graph.State{},
			wantTarget: "intent_recognition",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := route(ctx, tc.state)
			if err != nil {
				t.Fatalf("route() error = %v", err)
			}
			if got != tc.wantTarget {
				t.Errorf("route() = %q, want %q", got, tc.wantTarget)
			}
		})
	}
}

func TestUnsupportedIntentFallbackMessage(t *testing.T) {
	tests := []struct {
		name   string
		intent enumor.IntentType
		want   string
	}{
		{
			name:   "resource_query message",
			intent: enumor.IntentTypeResourceQuery,
			want:   "资源查询功能正在建设中，敬请期待。如需主机申领，请直接描述您的申领需求。",
		},
		{
			name:   "chat message",
			intent: enumor.IntentTypeChat,
			want:   "您好，当前我主要支持主机申领相关能力。如需申请主机，请描述您的配置与业务需求。",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := graph.State{constant.StateKeyIntent: string(tc.intent)}
			got := unsupportedIntentFallbackMessage(state)
			if got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestShouldEmitFallbackResponse(t *testing.T) {
	fallbackText := "资源查询功能正在建设中，敬请期待。如需主机申领，请直接描述您的申领需求。"

	tests := []struct {
		name     string
		state    graph.State
		messages []trpcmodel.Message
		lastResp string
		want     bool
	}{
		{
			name:     "skip emit on resume replay",
			state:    graph.State{graph.ResumeChannel: "new user input"},
			lastResp: fallbackText,
			want:     false,
		},
		{
			name: "skip emit when assistant tail already present",
			messages: []trpcmodel.Message{
				{Role: trpcmodel.RoleAssistant, Content: fallbackText},
			},
			lastResp: fallbackText,
			want:     false,
		},
		{
			name: "emit on first unsupported intent entry",
			state: graph.State{
				constant.StateKeyIntent: string(enumor.IntentTypeResourceQuery),
			},
			messages: []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "查预测"}},
			lastResp: fallbackText,
			want:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldEmitFallbackResponse(tc.state, tc.messages, tc.lastResp); got != tc.want {
				t.Errorf("shouldEmitFallbackResponse() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildFallbackResumeDeltaClearsUnsupportedIntentHistory(t *testing.T) {
	fallbackText := "资源查询功能正在建设中，敬请期待。如需主机申领，请直接描述您的申领需求。"
	state := graph.State{constant.StateKeyIntent: string(enumor.IntentTypeResourceQuery)}
	messages := []trpcmodel.Message{
		{Role: trpcmodel.RoleUser, Content: "我要看一下我的预测"},
	}

	delta := buildFallbackResumeDelta(context.Background(), state, messages, fallbackText, "那帮我申领一台主机吧")

	intentVal, _ := delta[constant.StateKeyIntent].(string)
	if intentVal != "" {
		t.Fatalf("intent = %q, want cleared", intentVal)
	}

	ops, ok := delta[graph.StateKeyMessages].([]graph.MessageOp)
	if !ok {
		t.Fatalf("messages update = %T, want []graph.MessageOp", delta[graph.StateKeyMessages])
	}
	if len(ops) != 2 {
		t.Fatalf("message ops len = %d, want 2", len(ops))
	}

	rebuilt := ops[0].Apply(nil)
	rebuilt = ops[1].Apply(rebuilt)
	if len(rebuilt) != 2 {
		t.Fatalf("rebuilt messages len = %d, want 2", len(rebuilt))
	}
	if rebuilt[0].Role != trpcmodel.RoleAssistant || rebuilt[0].Content != fallbackText {
		t.Fatalf("first rebuilt message = %+v, want assistant fallback text", rebuilt[0])
	}
	if rebuilt[1].Role != trpcmodel.RoleUser || rebuilt[1].Content != "那帮我申领一台主机吧" {
		t.Fatalf("second rebuilt message = %+v, want new user input", rebuilt[1])
	}
}

// TestBuildFallbackResumeDeltaClearsUserInput verifies that the resume delta always
// clears StateKeyUserInput. mergeInitialStateNonInternal skips keys that already
// exist in the restored checkpoint, so a stale user_input written during a run that
// never reached the LLM node (e.g. "查看预测" → fallback interrupt) survives into
// the next turn. Without the explicit clear, executeUserInputStage would use that
// stale value and overwrite the correctly-rebuilt messages tail with the old input.
func TestBuildFallbackResumeDeltaClearsUserInput(t *testing.T) {
	fallbackText := "资源查询功能正在建设中，敬请期待。如需主机申领，请直接描述您的申领需求。"

	tests := []struct {
		name      string
		state     graph.State
		messages  []trpcmodel.Message
		userInput string
	}{
		{
			name:      "clearing unsupported intent path clears user_input",
			state:     graph.State{constant.StateKeyIntent: string(enumor.IntentTypeResourceQuery)},
			messages:  []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "查看预测"}},
			userInput: "我要申请主机",
		},
		{
			name:  "normal resume path also clears user_input",
			state: graph.State{},
			messages: []trpcmodel.Message{
				{Role: trpcmodel.RoleAssistant, Content: fallbackText},
			},
			userInput: "继续",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			delta := buildFallbackResumeDelta(tc.state, tc.messages, fallbackText, tc.userInput, "test-rid")

			userInputVal, exists := delta[graph.StateKeyUserInput]
			if !exists {
				t.Fatalf("StateKeyUserInput not present in delta, want explicit clear")
			}
			if s, _ := userInputVal.(string); s != "" {
				t.Fatalf("StateKeyUserInput = %q, want empty string", s)
			}
		})
	}
}
