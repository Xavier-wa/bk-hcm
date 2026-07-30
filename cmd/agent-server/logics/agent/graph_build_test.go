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
	"encoding/json"
	"testing"

	"hcm/cmd/agent-server/logics/agent/hitl"
	"hcm/cmd/agent-server/logics/agent/message"
	"hcm/cmd/agent-server/logics/agent/toolgate"
	"hcm/pkg/cc"
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
		{enumor.IntentTypeResourceQuery, true},
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

func TestSceneNodeTarget(t *testing.T) {
	tests := []struct {
		scene enumor.IntentType
		want  string
	}{
		{enumor.IntentTypeHostApply, string(enumor.SubgraphAgentNodeHostApply)},
		{enumor.IntentTypeResourceQuery, string(enumor.SubgraphAgentNodeResourceQuery)},
	}
	for _, tc := range tests {
		if got := sceneNodeTarget(tc.scene); got != tc.want {
			t.Errorf("sceneNodeTarget(%q) = %q, want %q", tc.scene, got, tc.want)
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
			name:    "recognised resource_query commits session_tag",
			state:   graph.State{constant.StateKeyIntent: string(enumor.IntentTypeResourceQuery)},
			wantTag: enumor.IntentTypeResourceQuery,
		},
		{
			name:    "already tagged re-commits same tag",
			state:   graph.State{constant.StateKeySessionTag: enumor.IntentTypeHostApply},
			wantTag: enumor.IntentTypeHostApply,
		},
		{
			name:    "already tagged as string re-commits same tag",
			state:   graph.State{constant.StateKeySessionTag: string(enumor.IntentTypeHostApply)},
			wantTag: enumor.IntentTypeHostApply,
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
			tag := parseSessionTag(st)
			if tag != tc.wantTag {
				t.Errorf("committed session_tag = %q, want %q", tag, tc.wantTag)
			}
		})
	}
}

// newTestRegistry builds a hitl registry with human_confirm and the active confirm gates.
func newTestRegistry(enabled bool) *hitl.Registry {
	reg := hitl.NewRegistry()
	reg.Register(hitl.NewHumanConfirmHandler())
	for _, h := range toolgate.GetEnabledGateHandlers(cc.AgentConfirmGateConfig{Enabled: enabled}, nil) {
		reg.Register(h)
	}
	return reg
}

func TestMakeRoutingFuncGate(t *testing.T) {
	route := makeSubgraphRoutingFunc(string(enumor.SubgraphAgentNodeHostApply), newTestRegistry(true))
	ctx := context.Background()

	gatedCall := trpcmodel.ToolCall{
		ID:       "c1",
		Function: trpcmodel.FunctionDefinitionParam{Name: constant.ToolNameCreateBizApply},
	}
	otherCall := trpcmodel.ToolCall{
		ID:       "c2",
		Function: trpcmodel.FunctionDefinitionParam{Name: "list_biz_host"},
	}
	confirmCall := trpcmodel.ToolCall{
		ID:       "c3",
		Function: trpcmodel.FunctionDefinitionParam{Name: constant.HumanConfirmToolName},
	}
	// 真实场景：LLM 经 proxy execute_tool 调用 create_biz_apply，真实工具名在 arguments 内。
	proxyGatedCall := trpcmodel.ToolCall{
		ID: "c4",
		Function: trpcmodel.FunctionDefinitionParam{
			Name: constant.ProxyExecuteToolFullName,
			Arguments: []byte(`{"tool_name":"bkhcm-devhk/create_biz_apply",` +
				`"parameters":{"path_param":{"bk_biz_id":1}},"schema_token":"t"}`),
		},
	}
	proxyOtherCall := trpcmodel.ToolCall{
		ID: "c5",
		Function: trpcmodel.FunctionDefinitionParam{
			Name:      constant.ProxyExecuteToolFullName,
			Arguments: []byte(`{"tool_name":"bkhcm-devhk/list_biz_host","parameters":{},"schema_token":"t"}`),
		},
	}

	tests := []struct {
		name       string
		toolCalls  []trpcmodel.ToolCall
		wantTarget string
		wantErr    bool
	}{
		{
			name:       "gated tool alone routes to hitl",
			toolCalls:  []trpcmodel.ToolCall{gatedCall},
			wantTarget: "hitl",
		},
		{
			name:      "gated tool with other tool errors",
			toolCalls: []trpcmodel.ToolCall{gatedCall, otherCall},
			wantErr:   true,
		},
		{
			name:      "human_confirm with other tool errors",
			toolCalls: []trpcmodel.ToolCall{confirmCall, otherCall},
			wantErr:   true,
		},
		{
			name:       "other tool routes to tool",
			toolCalls:  []trpcmodel.ToolCall{otherCall},
			wantTarget: "tool",
		},
		{
			name:       "human_confirm routes to hitl",
			toolCalls:  []trpcmodel.ToolCall{confirmCall},
			wantTarget: "hitl",
		},
		{
			name:       "proxy-wrapped gated tool routes to hitl",
			toolCalls:  []trpcmodel.ToolCall{proxyGatedCall},
			wantTarget: "hitl",
		},
		{
			name:       "proxy-wrapped other tool routes to tool",
			toolCalls:  []trpcmodel.ToolCall{proxyOtherCall},
			wantTarget: "tool",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := graph.State{graph.StateKeyMessages: []trpcmodel.Message{
				{Role: trpcmodel.RoleAssistant, ToolCalls: tc.toolCalls},
			}}
			got, err := route(ctx, state)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got target %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("route() error = %v", err)
			}
			if got != tc.wantTarget {
				t.Errorf("route() = %q, want %q", got, tc.wantTarget)
			}
		})
	}
}

func TestMakeRoutingFuncGateDisabled(t *testing.T) {
	route := makeSubgraphRoutingFunc(string(enumor.SubgraphAgentNodeHostApply), newTestRegistry(false))

	state := graph.State{graph.StateKeyMessages: []trpcmodel.Message{
		{Role: trpcmodel.RoleAssistant, ToolCalls: []trpcmodel.ToolCall{
			{ID: "c1", Function: trpcmodel.FunctionDefinitionParam{Name: constant.ToolNameCreateBizApply}},
		}},
	}}
	got, err := route(context.Background(), state)
	if err != nil {
		t.Fatalf("route() error = %v", err)
	}
	// When the gate is disabled the guarded tool is not registered, so it falls back to the tool path.
	if got != "tool" {
		t.Errorf("route() = %q, want tool (gate disabled)", got)
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
			name:       "supported session_tag routes to host_apply subgraph",
			state:      graph.State{constant.StateKeySessionTag: enumor.IntentTypeHostApply},
			wantTarget: string(enumor.SubgraphAgentNodeHostApply),
		},
		{
			name:       "resource_query session_tag routes to resource_query subgraph",
			state:      graph.State{constant.StateKeySessionTag: enumor.IntentTypeResourceQuery},
			wantTarget: "resource_query",
		},
		{
			name:       "this-turn unsupported intent routes to fallback",
			state:      graph.State{constant.StateKeyIntent: string(enumor.IntentTypeChat)},
			wantTarget: string(enumor.MainGraphAgentNodeFallback),
		},
		{
			name:       "supported intent without committed tag routes to intent_recognition",
			state:      graph.State{constant.StateKeyIntent: string(enumor.IntentTypeResourceQuery)},
			wantTarget: string(enumor.MainGraphAgentNodeIntentRecognition),
		},
		{
			name:       "no tag no intent routes to intent_recognition",
			state:      graph.State{},
			wantTarget: string(enumor.MainGraphAgentNodeIntentRecognition),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			target, err := route(ctx, tc.state)
			if err != nil {
				t.Fatalf("route() error = %v", err)
			}
			if target != tc.wantTarget {
				t.Errorf("route() = %q, want %q", target, tc.wantTarget)
			}
		})
	}
}

// TestSceneDispatchNodeThenRouting 校验节点提交标签后，路由按场景直达对应入口节点的组合行为。
func TestSceneDispatchNodeThenRouting(t *testing.T) {
	node := makeSceneDispatchNode()
	route := makeSceneDispatchRoutingFunc()
	ctx := context.Background()

	tests := []struct {
		name       string
		intent     enumor.IntentType
		wantTarget string
	}{
		{name: "host_apply dispatch then route to host_apply subgraph", intent: enumor.IntentTypeHostApply,
			wantTarget: string(enumor.SubgraphAgentNodeHostApply)},
		{name: "resource_query dispatch then route to subgraph", intent: enumor.IntentTypeResourceQuery,
			wantTarget: "resource_query"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := graph.State{constant.StateKeyIntent: string(tc.intent)}
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
			if target != tc.wantTarget {
				t.Errorf("route() = %q, want %q", target, tc.wantTarget)
			}
		})
	}
}

func TestUnsupportedIntentFallbackMessage(t *testing.T) {
	const (
		unsupportedMsg = "目前AI助手仅支持主机申领相关能力，其他云资源管理功能即将上线，如需要申领主机，请直接描述您的配置需求。"
		genericMsg     = "抱歉，我暂时无法处理您的请求。"
	)

	tests := []struct {
		name   string
		intent enumor.IntentType
		want   string
	}{
		{
			name:   "unsupported chat intent returns guidance message",
			intent: enumor.IntentTypeChat,
			want:   unsupportedMsg,
		},
		{
			name:   "empty intent returns guidance message",
			intent: "",
			want:   unsupportedMsg,
		},
		{
			name:   "supported resource_query intent returns generic message",
			intent: enumor.IntentTypeResourceQuery,
			want:   genericMsg,
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
	fallbackText := "目前AI助手仅支持主机申领相关能力，其他云资源管理功能即将上线，如需要申领主机，请直接描述您的配置需求。"
	// Use chat intent (genuinely unsupported) to test clearing logic.
	// resource_query is now a supported intent and its history is preserved (not cleared).
	state := graph.State{constant.StateKeyIntent: string(enumor.IntentTypeChat)}
	messages := []trpcmodel.Message{
		{Role: trpcmodel.RoleUser, Content: "给我讲个故事"},
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
	if len(ops) != 1 {
		t.Fatalf("message ops len = %d, want 1", len(ops))
	}

	rebuilt := ops[0].Apply(nil)
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
	fallbackText := "目前AI助手仅支持主机申领相关能力，其他云资源管理功能即将上线，如需要申领主机，请直接描述您的配置需求。"

	tests := []struct {
		name      string
		state     graph.State
		messages  []trpcmodel.Message
		userInput string
	}{
		{
			name:      "clearing unsupported intent path clears user_input",
			state:     graph.State{constant.StateKeyIntent: string(enumor.IntentTypeChat)},
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

// TestMakeSubgraphInputMapperStripsForwardedResumeValue verifies the entry-point key stripping:
// StateKeyForwardedResumeValue must NOT enter the persisted subgraph state. The value is rebuilt
// into inv.RunOptions.RuntimeState on every resume Run by tryPrepareAutoResume, so the subgraph
// state dropping it forces mergeInitialStateNonInternal to backfill the current round's value and
// avoids stale-checkpoint dirty reads on the after_tool_hitl loop edge.
func TestMakeSubgraphInputMapperStripsForwardedResumeValue(t *testing.T) {
	mapper := makeSubgraphInputMapper("host_apply")
	parent := graph.State{
		graph.StateKeyMessages:                []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "hi"}},
		graph.StateKeyUserInput:               "再申请一单试试",
		constant.StateKeyForwardedResumeValue: `{"plan":"x"}`,
		graph.CfgKeyCheckpointID:              "parent-cp",
	}

	child := mapper(parent)

	if _, exists := child[constant.StateKeyForwardedResumeValue]; exists {
		t.Errorf("subgraph child state must not contain %q (entry-point strip required)",
			constant.StateKeyForwardedResumeValue)
	}
	// Sanity: regular (non-internal) keys still pass through, and the framework-internal
	// checkpoint id is also stripped.
	if _, exists := child[graph.CfgKeyCheckpointID]; exists {
		t.Errorf("subgraph child state must not contain %q", graph.CfgKeyCheckpointID)
	}
	if _, exists := child[graph.StateKeyUserInput]; exists {
		t.Errorf("subgraph child state must not contain %q (avoid createInitialState overwrite)",
			graph.StateKeyUserInput)
	}
	if _, exists := child[graph.StateKeyMessages]; !exists {
		t.Errorf("subgraph child state should preserve normal keys like %q", graph.StateKeyMessages)
	}
}

func TestMakeSubgraphInputMapperDeepCopiesMessages(t *testing.T) {
	mapper := makeSubgraphInputMapper("resource_query")
	parentMsgs := []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "hi"}}
	parent := graph.State{graph.StateKeyMessages: parentMsgs}

	child := mapper(parent)
	childMsgs, ok := child[graph.StateKeyMessages].([]trpcmodel.Message)
	if !ok {
		t.Fatalf("child messages type = %T, want []model.Message", child[graph.StateKeyMessages])
	}
	if len(childMsgs) != 1 {
		t.Fatalf("child messages len = %d, want 1", len(childMsgs))
	}

	childMsgs[0].Content = "mutated"
	if parentMsgs[0].Content != "hi" {
		t.Fatalf("parent message mutated after child edit, got %q", parentMsgs[0].Content)
	}
	if &childMsgs[0] == &parentMsgs[0] {
		t.Fatal("child messages should not share element pointers with parent")
	}
}

func TestMakeSubgraphOutputMapperReturnsDeltaOnly(t *testing.T) {
	mapper := makeSubgraphOutputMapper("resource_query")
	parent := graph.State{
		graph.StateKeyMessages: []trpcmodel.Message{
			{Role: trpcmodel.RoleUser, Content: "q1"},
			{Role: trpcmodel.RoleUser, Content: "q2"},
		},
	}
	decoded := []trpcmodel.Message{
		{Role: trpcmodel.RoleUser, Content: "q1"},
		{Role: trpcmodel.RoleUser, Content: "q2"},
		{Role: trpcmodel.RoleAssistant, Content: "thinking"},
		{Role: trpcmodel.RoleTool, ToolID: "tooluse_a", Content: "skill-a"},
		{Role: trpcmodel.RoleTool, ToolID: "tooluse_b", Content: "skill-b"},
	}
	raw, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal decoded messages failed, err: %v", err)
	}

	out := mapper(parent, graph.SubgraphResult{
		RawStateDelta: map[string][]byte{graph.StateKeyMessages: raw},
	})
	delta, ok := out[graph.StateKeyMessages].([]trpcmodel.Message)
	if !ok {
		t.Fatalf("output messages type = %T, want []model.Message", out[graph.StateKeyMessages])
	}
	if len(delta) != 3 {
		t.Fatalf("delta len = %d, want 3", len(delta))
	}
	if delta[0].Role != trpcmodel.RoleAssistant || delta[1].ToolID != "tooluse_a" || delta[2].ToolID != "tooluse_b" {
		t.Fatalf("unexpected delta messages: %+v", delta)
	}
}

func TestMakeSubgraphOutputMapperSkipsWhenNoGrowth(t *testing.T) {
	mapper := makeSubgraphOutputMapper("resource_query")
	parentMsgs := []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "q1"}}
	parent := graph.State{graph.StateKeyMessages: parentMsgs}
	raw, err := json.Marshal(parentMsgs)
	if err != nil {
		t.Fatalf("marshal parent messages failed, err: %v", err)
	}

	out := mapper(parent, graph.SubgraphResult{
		RawStateDelta: map[string][]byte{graph.StateKeyMessages: raw},
	})
	if out != nil {
		t.Fatalf("expected nil output when decoded len <= parent len, got %+v", out)
	}
}

func TestMakeSubgraphInputMapperUsesTurnNamespace(t *testing.T) {
	mapper := makeSubgraphInputMapper("host_apply")
	parentMsgs := []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "我要申请主机"}}

	// 首轮：父图无 turn 记录，input mapper 自增到 1，ns = host_apply_1。
	child1 := mapper(graph.State{graph.StateKeyMessages: parentMsgs})
	ns1, _ := child1[graph.CfgKeyCheckpointNS].(string)
	if ns1 != "host_apply_1" {
		t.Fatalf("first round checkpoint ns = %q, want host_apply_1", ns1)
	}
	turn1, _ := child1[subgraphTurnKey("host_apply")].(int)
	if turn1 != 1 {
		t.Fatalf("first round child turn = %d, want 1", turn1)
	}

	// 第二轮：父图带有第一轮回填的 turn=1，input mapper 继续自增到 2，ns = host_apply_2。
	// 关键：即便两轮父图消息数完全相同，ns 仍不同（不靠 len(msgs) 区分轮次）。
	parent2 := graph.State{
		graph.StateKeyMessages:        parentMsgs,
		subgraphTurnKey("host_apply"): 1,
	}
	child2 := mapper(parent2)
	ns2, _ := child2[graph.CfgKeyCheckpointNS].(string)
	if ns2 != "host_apply_2" {
		t.Fatalf("second round checkpoint ns = %q, want host_apply_2 (distinct from round 1 even with equal msg count)", ns2)
	}
	turn2, _ := child2[subgraphTurnKey("host_apply")].(int)
	if turn2 != 2 {
		t.Fatalf("second round child turn = %d, want 2", turn2)
	}
}

func TestMakeSubgraphOutputMapperFeedsTurnBackToParent(t *testing.T) {
	mapper := makeSubgraphOutputMapper("host_apply")
	// 子图完成态 RawStateDelta 携带本轮自增后的 turn=2（与 messages 同口径由框架序列化）。
	parentMsgs := []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "q1"}}
	decoded := []trpcmodel.Message{
		{Role: trpcmodel.RoleUser, Content: "q1"},
		{Role: trpcmodel.RoleAssistant, Content: "r1"},
	}
	rawMsgs, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal decoded messages failed, err: %v", err)
	}
	// RawStateDelta 是 map[string][]byte，需包含 turn key 以模拟子图状态回填。
	turnRaw, err := json.Marshal(2)
	if err != nil {
		t.Fatalf("marshal turn failed, err: %v", err)
	}
	rawDelta := map[string][]byte{
		graph.StateKeyMessages:        rawMsgs,
		subgraphTurnKey("host_apply"): turnRaw,
	}

	out := mapper(graph.State{graph.StateKeyMessages: parentMsgs}, graph.SubgraphResult{RawStateDelta: rawDelta})
	if out == nil {
		t.Fatal("expected non-nil output when decoded grows, got nil")
	}
	// turn 必须随 messages 一起回填父图，供下一轮 input mapper 续递增。
	gotTurn, ok := out[subgraphTurnKey("host_apply")].(int)
	if !ok {
		t.Fatalf("output missing turn key %q", subgraphTurnKey("host_apply"))
	}
	if gotTurn != 2 {
		t.Fatalf("output turn = %d, want 2", gotTurn)
	}
}

func TestMakeSubgraphOutputMapperFeedsTurnBackWhenNoGrowth(t *testing.T) {
	mapper := makeSubgraphOutputMapper("host_apply")
	// 零账号路径：子图未新增 assistant 消息（decoded == parent），但 turn 已自增到 1。
	// 即便无消息可 merge，turn 也必须回填父图，否则下一轮复用同一 namespace 撞已结束 checkpoint。
	parentMsgs := []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "申请一台主机"}}
	rawMsgs, err := json.Marshal(parentMsgs)
	if err != nil {
		t.Fatalf("marshal parent messages failed, err: %v", err)
	}
	turnRaw, err := json.Marshal(1)
	if err != nil {
		t.Fatalf("marshal turn failed, err: %v", err)
	}
	out := mapper(graph.State{graph.StateKeyMessages: parentMsgs}, graph.SubgraphResult{
		RawStateDelta: map[string][]byte{
			graph.StateKeyMessages:        rawMsgs,
			subgraphTurnKey("host_apply"): turnRaw,
		},
	})
	if out == nil {
		t.Fatal("expected turn feedback when subgraph ends without message growth, got nil")
	}
	// 无增长场景不应带回 messages delta，只回填 turn。
	if _, ok := out[graph.StateKeyMessages]; ok {
		t.Fatalf("no-growth output must not carry messages delta, got %+v", out[graph.StateKeyMessages])
	}
	gotTurn, ok := out[subgraphTurnKey("host_apply")].(int)
	if !ok || gotTurn != 1 {
		t.Fatalf("output turn = %v ok=%v, want 1", out[subgraphTurnKey("host_apply")], ok)
	}
}

func TestMakeSubgraphOutputMapperNoGrowthNoTurnReturnsNil(t *testing.T) {
	mapper := makeSubgraphOutputMapper("host_apply")
	// 无增长且 RawStateDelta 未携带 turn 时，保持原有「跳过」语义返回 nil。
	parentMsgs := []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "q1"}}
	rawMsgs, err := json.Marshal(parentMsgs)
	if err != nil {
		t.Fatalf("marshal parent messages failed, err: %v", err)
	}
	out := mapper(graph.State{graph.StateKeyMessages: parentMsgs}, graph.SubgraphResult{
		RawStateDelta: map[string][]byte{graph.StateKeyMessages: rawMsgs},
	})
	if out != nil {
		t.Fatalf("expected nil when no growth and no turn in delta, got %+v", out)
	}
}
