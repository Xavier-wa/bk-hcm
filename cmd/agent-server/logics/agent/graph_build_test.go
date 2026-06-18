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

	"hcm/cmd/agent-server/logics/agent/message"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

func TestIntentType_IsSupportedScene(t *testing.T) {
	tests := []struct {
		in   enumor.IntentType
		want bool
	}{
		{enumor.IntentTypeHostApply, true},
		{enumor.IntentTypeResourceQuery, false},
		{enumor.IntentTypeChat, false},
		{"", false},
		{"unknown", false},
	}
	for _, tc := range tests {
		if got := tc.in.IsSupportedScene(); got != tc.want {
			t.Errorf("IntentType(%q).IsSupportedScene() = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestMakeSceneDispatchNode(t *testing.T) {
	node := makeSceneDispatchNode()
	ctx := context.Background()

	tests := []struct {
		name    string
		state   graph.State
		wantTag enumor.IntentType // expected committed StateKeySessionTag, empty means no-op
	}{
		{
			name:    "recognised host_apply commits session_tag",
			state:   graph.State{constant.StateKeyIntent: string(enumor.IntentTypeHostApply)},
			wantTag: enumor.IntentTypeHostApply,
		},
		{
			name:    "already tagged is no-op",
			state:   graph.State{constant.StateKeySessionTag: enumor.IntentTypeHostApply},
			wantTag: "",
		},
		{
			name:    "unsupported intent is no-op",
			state:   graph.State{constant.StateKeyIntent: string(enumor.IntentTypeChat)},
			wantTag: "",
		},
		{
			name:    "no tag no intent is no-op",
			state:   graph.State{},
			wantTag: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := node(ctx, tc.state)
			if err != nil {
				t.Fatalf("node() error = %v", err)
			}
			st, ok := got.(graph.State)
			if !ok {
				t.Fatalf("node() returned %T, want graph.State", got)
			}
			tag, _ := st[constant.StateKeySessionTag].(enumor.IntentType)
			if tag != tc.wantTag {
				t.Errorf("committed session_tag = %q, want %q", tag, tc.wantTag)
			}
		})
	}
}

func TestMakeSceneDispatchRoutingFunc(t *testing.T) {
	route := makeSceneDispatchRoutingFunc()
	ctx := context.Background()

	tests := []struct {
		name       string
		state      graph.State
		wantTarget string
	}{
		{
			name:       "supported session_tag routes to account_select",
			state:      graph.State{constant.StateKeySessionTag: enumor.IntentTypeHostApply},
			wantTarget: string(enumor.CvmApplyNodeAccountSelect),
		},
		{
			name:       "this-turn unsupported intent routes to fallback",
			state:      graph.State{constant.StateKeyIntent: string(enumor.IntentTypeChat)},
			wantTarget: "fallback",
		},
		{
			name:       "no tag no intent routes to intent_recognition",
			state:      graph.State{},
			wantTarget: "intent_recognition",
		},
	}

	for _, test := range tests {
		target, err := route(ctx, test.state)
		if err != nil {
			t.Fatalf("route() error = %v", err)
		}
		if target != test.wantTarget {
			t.Errorf("route() = %q, want %q", target, test.wantTarget)
		}
	}
}

// TestSceneDispatchNodeThenRouting 校验节点提交标签后路由直达 account_select 的组合行为。
func TestSceneDispatchNodeThenRouting(t *testing.T) {
	node := makeSceneDispatchNode()
	route := makeSceneDispatchRoutingFunc()
	ctx := context.Background()

	state := graph.State{constant.StateKeyIntent: string(enumor.IntentTypeHostApply)}
	got, err := node(ctx, state)
	if err != nil {
		t.Fatalf("node() error = %v", err)
	}
	st, _ := got.(graph.State)
	for k, v := range st {
		state[k] = v
	}

	target, err := route(ctx, state)
	if err != nil {
		t.Fatalf("route() error = %v", err)
	}
	if want := string(enumor.CvmApplyNodeAccountSelect); target != want {
		t.Errorf("route() = %q, want %q", target, want)
	}
}

func TestUnsupportedIntentFallbackMessage(t *testing.T) {
	const unsupportedMsg = "目前AI助手仅支持主机申领相关能力，其他云资源管理功能即将上线，如需要申领主机，请直接描述您的配置需求。"

	tests := []struct {
		name   string
		intent enumor.IntentType
		want   string
	}{
		{
			name:   "resource_query message",
			intent: enumor.IntentTypeResourceQuery,
			want:   unsupportedMsg,
		},
		{
			name:   "chat message",
			intent: enumor.IntentTypeChat,
			want:   unsupportedMsg,
		},
		{
			name:   "unknown intent uses default message",
			intent: "unknown",
			want:   "抱歉，我暂时无法处理您的请求。",
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

func TestBuildFallbackResumeDeltaClearsUnsupportedIntentHistory(t *testing.T) {
	fallbackText := "资源查询功能正在建设中，敬请期待。如需主机申领，请直接描述您的申领需求。"
	state := graph.State{constant.StateKeyIntent: string(enumor.IntentTypeResourceQuery)}
	messages := []trpcmodel.Message{
		{Role: trpcmodel.RoleUser, Content: "我要看一下我的预测"},
	}

	delta := message.BuildFallbackResumeDelta(context.Background(), state, messages, fallbackText,
		"那帮我申领一台主机吧")

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
			delta := message.BuildFallbackResumeDelta(context.Background(), tc.state, tc.messages, fallbackText,
				tc.userInput)

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
