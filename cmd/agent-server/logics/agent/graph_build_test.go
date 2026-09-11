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
	"errors"
	"testing"

	"hcm/cmd/agent-server/logics/agent/hitl"
	"hcm/cmd/agent-server/logics/agent/message"
	agentstate "hcm/cmd/agent-server/logics/agent/state"
	"hcm/cmd/agent-server/logics/agent/toolgate"
	"hcm/cmd/agent-server/logics/prompt"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

// testAgentName is the agent name passed to scene_dispatch in tests; it only scopes the
// session-level skill state keys, which stay untouched when the context carries no invocation.
const testAgentName = "hcm-agent"

// stubIntentModel is a trpcmodel.Model that always classifies the user intent as a fixed value,
// so that scene_dispatch tests can drive the decision matrix without a real LLM.
type stubIntentModel struct {
	intent enumor.IntentType
}

func newStubIntentModel(intent enumor.IntentType) *stubIntentModel {
	return &stubIntentModel{intent: intent}
}

func (m *stubIntentModel) GenerateContent(_ context.Context, _ *trpcmodel.Request) (
	<-chan *trpcmodel.Response, error) {

	ch := make(chan *trpcmodel.Response, 1)
	ch <- &trpcmodel.Response{
		Choices: []trpcmodel.Choice{{Message: trpcmodel.Message{Content: string(m.intent)}}},
	}
	close(ch)
	return ch, nil
}

func (m *stubIntentModel) Info() trpcmodel.Info {
	return trpcmodel.Info{Name: "stub-intent-model"}
}

// newTestIntentPromptStore returns a prompt store carrying a placeholder intent recognition prompt.
func newTestIntentPromptStore() *prompt.Store {
	s := prompt.NewStore("")
	_ = s.Set(constant.IntentRecognitionPromptKey, prompt.PromptEntry{Content: "classify intent"})
	return s
}

// ridContext returns a context carrying rid, mirroring what the HTTP layer injects.
func ridContext(rid string) context.Context {
	return context.WithValue(context.Background(), constant.RidKey, rid)
}

// newTestSceneDispatchNode 构造 scene_dispatch 节点。
// sessionTagCli 传 nil 表示本用例不关心标签回写——回写侧按「未注入」跳过，无需架起 data-service 客户端。
func newTestSceneDispatchNode(classified enumor.IntentType,
	sessionTagCli agentstate.SessionTagUpdater) graph.NodeFunc {

	return makeSceneDispatchNode(newStubIntentModel(classified), newTestIntentPromptStore(),
		testAgentName, 5, sessionTagCli)
}

// stubSessionTagUpdater 记录 session_tag 回写请求，并可注入错误或观测回调。
type stubSessionTagUpdater struct {
	reqs []*dsaiagent.UpdateAiagentSessionReq
	err  error
	// onUpdate 在记录请求前触发，用于观测回写发生的时刻（如此时事件是否已发出）。
	onUpdate func()
}

func (s *stubSessionTagUpdater) Update(_ *kit.Kit, req *dsaiagent.UpdateAiagentSessionReq) error {
	if s.onUpdate != nil {
		s.onUpdate()
	}
	s.reqs = append(s.reqs, req)
	return s.err
}

// sceneDispatchCtx 在 rid context 上挂 invocation，使节点内的标签回写能解析出 threadID。
func sceneDispatchCtx(rid, threadID string) context.Context {
	inv := &trpcagent.Invocation{
		RunOptions: trpcagent.RunOptions{
			RuntimeState: map[string]any{graph.CfgKeyLineageID: threadID},
		},
	}
	return trpcagent.NewInvocationContext(ridContext(rid), inv)
}

// tagOrNil converts an empty scene tag into nil so that turnBoundaryState leaves the session untagged.
func tagOrNil(tag enumor.IntentType) any {
	if tag == "" {
		return nil
	}
	return tag
}

// turnBoundaryState builds the state scene_dispatch sees at a turn boundary: a pending user
// message plus the session tag the session currently carries (nil means an untagged session).
// 用户消息是必需的——没有它意图分类会直接降级为 chat，测不出决策矩阵的其它分支。
func turnBoundaryState(tag any) graph.State {
	state := graph.State{
		graph.StateKeyMessages: []trpcmodel.Message{
			{Role: trpcmodel.RoleUser, Content: "帮我处理一下"},
		},
	}
	if tag != nil {
		state[constant.StateKeySessionTag] = tag
	}
	return state
}

// stateWithEventChan returns state plus the execution context that graph.GetEventEmitterWithContext
// needs to produce a real (non-noop) emitter, together with the channel receiving emitted events.
func stateWithEventChan(state graph.State) (graph.State, chan *event.Event) {
	eventChan := make(chan *event.Event, 8)
	state[graph.StateKeyExecContext] = &graph.ExecutionContext{
		EventChan:    eventChan,
		InvocationID: "test-invocation",
	}
	return state, eventChan
}

// collectSceneSwitchedPayloads drains ch and returns the payload of every scene.switched event.
func collectSceneSwitchedPayloads(t *testing.T, ch chan *event.Event) []message.SceneSwitchedPayload {
	t.Helper()
	close(ch)

	payloads := make([]message.SceneSwitchedPayload, 0, len(ch))
	for evt := range ch {
		raw, ok := evt.StateDelta[graph.MetadataKeyNodeCustom]
		if !ok {
			continue
		}
		var meta struct {
			EventType string                       `json:"eventType"`
			NodeID    string                       `json:"nodeId"`
			Payload   message.SceneSwitchedPayload `json:"payload"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			t.Fatalf("unmarshal node custom metadata failed, err: %v", err)
		}
		if meta.EventType != constant.SceneSwitchedCustomEventName {
			continue
		}
		if meta.NodeID != string(enumor.MainGraphAgentNodeSceneDispatch) {
			t.Errorf("scene.switched node_id = %q, want %q", meta.NodeID,
				enumor.MainGraphAgentNodeSceneDispatch)
		}
		payloads = append(payloads, meta.Payload)
	}
	return payloads
}

func TestIntentType_IsSupportedScene(t *testing.T) {
	tests := []struct {
		in   enumor.IntentType
		want bool
	}{
		{enumor.IntentTypeHostApply, true},
		{enumor.IntentTypeResourceQuery, true},
		{enumor.IntentTypeChat, false},
		{enumor.IntentTypeUnsupported, false},
		{"unknown", false},
		{"", false},
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
		{"unknown", string(enumor.SubgraphAgentNodeResourceQuery)},
	}
	for _, tc := range tests {
		if got := sceneNodeTarget(tc.scene, "test-rid"); got != tc.want {
			t.Errorf("sceneNodeTarget(%q) = %q, want %q", tc.scene, got, tc.want)
		}
	}
}

// TestDecideSceneDispatch 覆盖「当前会话标签 × 本轮分类结果」决策矩阵的每一格。
func TestDecideSceneDispatch(t *testing.T) {
	tests := []struct {
		name       string
		tag        enumor.IntentType
		classified enumor.IntentType
		want       sceneDispatchDecision
	}{
		{
			name:       "switch to another supported scene",
			tag:        enumor.IntentTypeHostApply,
			classified: enumor.IntentTypeResourceQuery,
			want: sceneDispatchDecision{
				nextNode:   string(enumor.SubgraphAgentNodeResourceQuery),
				sessionTag: enumor.IntentTypeResourceQuery,
				switched:   true,
			},
		},
		{
			name:       "switch back to the other supported scene",
			tag:        enumor.IntentTypeResourceQuery,
			classified: enumor.IntentTypeHostApply,
			want: sceneDispatchDecision{
				nextNode:   string(enumor.SubgraphAgentNodeHostApply),
				sessionTag: enumor.IntentTypeHostApply,
				switched:   true,
			},
		},
		{
			name:       "follow-up in the same scene does not switch",
			tag:        enumor.IntentTypeHostApply,
			classified: enumor.IntentTypeHostApply,
			want: sceneDispatchDecision{
				nextNode:   string(enumor.SubgraphAgentNodeHostApply),
				sessionTag: enumor.IntentTypeHostApply,
			},
		},
		{
			name:       "chat inside a tagged session stays in the current scene",
			tag:        enumor.IntentTypeHostApply,
			classified: enumor.IntentTypeChat,
			want: sceneDispatchDecision{
				nextNode:   string(enumor.SubgraphAgentNodeHostApply),
				sessionTag: enumor.IntentTypeHostApply,
			},
		},
		{
			name:       "first tagging of an untagged session is not a switch",
			tag:        "",
			classified: enumor.IntentTypeHostApply,
			want: sceneDispatchDecision{
				nextNode:   string(enumor.SubgraphAgentNodeHostApply),
				sessionTag: enumor.IntentTypeHostApply,
			},
		},
		{
			name:       "untagged session with unsupported intent falls back",
			tag:        "",
			classified: enumor.IntentTypeChat,
			want:       sceneDispatchDecision{nextNode: string(enumor.MainGraphAgentNodeFallback)},
		},
		{
			name:       "unrecognised tag is overwritten without reporting a switch",
			tag:        "legacy_unknown",
			classified: enumor.IntentTypeResourceQuery,
			want: sceneDispatchDecision{
				nextNode:   string(enumor.SubgraphAgentNodeResourceQuery),
				sessionTag: enumor.IntentTypeResourceQuery,
			},
		},
		{
			name:       "unrecognised tag with unsupported intent falls back",
			tag:        "legacy_unknown",
			classified: enumor.IntentTypeChat,
			want:       sceneDispatchDecision{nextNode: string(enumor.MainGraphAgentNodeFallback)},
		},
		{
			name:       "classification failure keeps the current scene",
			tag:        enumor.IntentTypeResourceQuery,
			classified: enumor.IntentTypeUnsupported,
			want: sceneDispatchDecision{
				nextNode:   string(enumor.SubgraphAgentNodeResourceQuery),
				sessionTag: enumor.IntentTypeResourceQuery,
			},
		},
		{
			name:       "classification failure in an untagged session falls back",
			tag:        "",
			classified: enumor.IntentTypeUnsupported,
			want:       sceneDispatchDecision{nextNode: string(enumor.MainGraphAgentNodeFallback)},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := decideSceneDispatch(tc.tag, tc.classified, "test-rid"); got != tc.want {
				t.Errorf("decideSceneDispatch(%q, %q) = %+v, want %+v", tc.tag, tc.classified, got, tc.want)
			}
		})
	}
}

// TestMakeSceneDispatchNode 校验节点把决策落进 state：目标节点、会话标签、本轮 rid。
func TestMakeSceneDispatchNode(t *testing.T) {
	const testRid = "rid-scene-dispatch"
	ctx := ridContext(testRid)

	tests := []struct {
		name string
		// tag is the session tag already present in state; nil means an untagged session.
		// 用 any 是为了同时覆盖 enumor.IntentType（首轮注入）与 string（checkpoint 反序列化后）两种存法。
		tag          any
		classified   enumor.IntentType
		wantNextNode string
		wantTag      enumor.IntentType // empty means the node must not commit a tag
	}{
		{
			name:         "untagged session commits the classified scene",
			classified:   enumor.IntentTypeHostApply,
			wantNextNode: string(enumor.SubgraphAgentNodeHostApply),
			wantTag:      enumor.IntentTypeHostApply,
		},
		{
			name:         "tagged session switches to the newly classified scene",
			tag:          enumor.IntentTypeHostApply,
			classified:   enumor.IntentTypeResourceQuery,
			wantNextNode: string(enumor.SubgraphAgentNodeResourceQuery),
			wantTag:      enumor.IntentTypeResourceQuery,
		},
		{
			name:         "tag stored as string is parsed and kept",
			tag:          string(enumor.IntentTypeHostApply),
			classified:   enumor.IntentTypeChat,
			wantNextNode: string(enumor.SubgraphAgentNodeHostApply),
			wantTag:      enumor.IntentTypeHostApply,
		},
		{
			name:         "untagged session with unsupported intent routes to fallback without a tag",
			classified:   enumor.IntentTypeChat,
			wantNextNode: string(enumor.MainGraphAgentNodeFallback),
			wantTag:      "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := turnBoundaryState(tc.tag)
			node := newTestSceneDispatchNode(tc.classified, nil)

			got, err := node(ctx, state)
			if err != nil {
				t.Fatalf("node() error = %v", err)
			}
			st, ok := got.(graph.State)
			if !ok {
				t.Fatalf("node() returned %T, want graph.State", got)
			}

			if next, _ := st[constant.StateKeySceneDispatchNext].(string); next != tc.wantNextNode {
				t.Errorf("scene_dispatch_next = %q, want %q", next, tc.wantNextNode)
			}
			if tag := agentstate.ParseSessionTag(st); tag != tc.wantTag {
				t.Errorf("committed session_tag = %q, want %q", tag, tc.wantTag)
			}
			if rid, _ := st[constant.SessionRidStateKey].(string); rid != testRid {
				t.Errorf("session_rid = %q, want refreshed to %q", rid, testRid)
			}
		})
	}
}

// TestMakeSceneDispatchNodeEmitsSceneSwitched 校验 scene.switched 事件的发送时机与内容：
// 只有真正跨场景时才发，首次打标与场景内追问都不发。
func TestMakeSceneDispatchNodeEmitsSceneSwitched(t *testing.T) {
	ctx := ridContext("rid-scene-switched")

	tests := []struct {
		name        string
		tag         enumor.IntentType
		classified  enumor.IntentType
		wantPayload []message.SceneSwitchedPayload
	}{
		{
			name:       "switching scenes emits from and to",
			tag:        enumor.IntentTypeHostApply,
			classified: enumor.IntentTypeResourceQuery,
			wantPayload: []message.SceneSwitchedPayload{
				{From: enumor.IntentTypeHostApply, To: enumor.IntentTypeResourceQuery},
			},
		},
		{
			name:        "first tagging emits nothing",
			tag:         "",
			classified:  enumor.IntentTypeHostApply,
			wantPayload: []message.SceneSwitchedPayload{},
		},
		{
			name:        "follow-up in the same scene emits nothing",
			tag:         enumor.IntentTypeHostApply,
			classified:  enumor.IntentTypeHostApply,
			wantPayload: []message.SceneSwitchedPayload{},
		},
		{
			name:        "chat inside a tagged session emits nothing",
			tag:         enumor.IntentTypeHostApply,
			classified:  enumor.IntentTypeChat,
			wantPayload: []message.SceneSwitchedPayload{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, eventChan := stateWithEventChan(turnBoundaryState(tagOrNil(tc.tag)))

			node := newTestSceneDispatchNode(tc.classified, nil)
			if _, err := node(ctx, state); err != nil {
				t.Fatalf("node() error = %v", err)
			}

			got := collectSceneSwitchedPayloads(t, eventChan)
			if len(got) != len(tc.wantPayload) {
				t.Fatalf("scene.switched events = %+v, want %+v", got, tc.wantPayload)
			}
			for i := range got {
				if got[i] != tc.wantPayload[i] {
					t.Errorf("scene.switched payload[%d] = %+v, want %+v", i, got[i], tc.wantPayload[i])
				}
			}
		})
	}
}

// TestMakeSceneDispatchNodeWritesBackChangedTag 校验标签回写的触发判据：只要本轮要提交的标签与
// 上一轮不同就回写，与是否构成场景切换无关（首次打标不发事件但必须回写）。
func TestMakeSceneDispatchNodeWritesBackChangedTag(t *testing.T) {
	const testThreadID = "thread-scene-dispatch"
	ctx := sceneDispatchCtx("rid-tag-write-back", testThreadID)

	tests := []struct {
		name       string
		tag        any
		classified enumor.IntentType
		// wantWriteTag 为空表示本轮不应发起回写
		wantWriteTag enumor.IntentType
	}{
		{
			name:         "first tagging writes back although no switch event is emitted",
			classified:   enumor.IntentTypeHostApply,
			wantWriteTag: enumor.IntentTypeHostApply,
		},
		{
			name:         "scene switch writes back the new tag",
			tag:          enumor.IntentTypeHostApply,
			classified:   enumor.IntentTypeResourceQuery,
			wantWriteTag: enumor.IntentTypeResourceQuery,
		},
		{
			name:       "follow-up in the same scene does not write back",
			tag:        enumor.IntentTypeHostApply,
			classified: enumor.IntentTypeHostApply,
		},
		{
			name:       "unsupported intent keeping the current tag does not write back",
			tag:        enumor.IntentTypeHostApply,
			classified: enumor.IntentTypeChat,
		},
		{
			name:       "untagged session falling back does not write back",
			classified: enumor.IntentTypeChat,
		},
		{
			name:         "unrecognised tag overwritten by a supported scene writes back",
			tag:          "legacy_unknown",
			classified:   enumor.IntentTypeResourceQuery,
			wantWriteTag: enumor.IntentTypeResourceQuery,
		},
		{
			name:       "unrecognised tag with unsupported intent does not write back",
			tag:        "legacy_unknown",
			classified: enumor.IntentTypeChat,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updater := &stubSessionTagUpdater{}
			node := newTestSceneDispatchNode(tc.classified, updater)

			if _, err := node(ctx, turnBoundaryState(tc.tag)); err != nil {
				t.Fatalf("node() error = %v", err)
			}

			if tc.wantWriteTag == "" {
				if len(updater.reqs) != 0 {
					t.Fatalf("write back requests = %+v, want none", updater.reqs)
				}
				return
			}

			if len(updater.reqs) != 1 {
				t.Fatalf("write back called %d times, want 1", len(updater.reqs))
			}
			req := updater.reqs[0]
			if req.ID != testThreadID {
				t.Errorf("write back id = %q, want %q", req.ID, testThreadID)
			}
			if req.SessionTag != tc.wantWriteTag {
				t.Errorf("write back session_tag = %q, want %q", req.SessionTag, tc.wantWriteTag)
			}
		})
	}
}

// TestMakeSceneDispatchNodeWriteBackPrecedesSceneSwitched 锁定回写与事件的先后：DB 必须先更新，
// 前端才收到 scene.switched。顺序反了会重新打开「切走再切回读到旧标签」的窗口。
func TestMakeSceneDispatchNodeWriteBackPrecedesSceneSwitched(t *testing.T) {
	ctx := sceneDispatchCtx("rid-write-back-order", "thread-order")
	state, eventChan := stateWithEventChan(turnBoundaryState(enumor.IntentTypeHostApply))

	eventsAtWriteBack := -1
	updater := &stubSessionTagUpdater{
		onUpdate: func() { eventsAtWriteBack = len(eventChan) },
	}

	node := newTestSceneDispatchNode(enumor.IntentTypeResourceQuery, updater)
	if _, err := node(ctx, state); err != nil {
		t.Fatalf("node() error = %v", err)
	}

	if len(updater.reqs) != 1 {
		t.Fatalf("write back called %d times, want 1", len(updater.reqs))
	}
	if eventsAtWriteBack != 0 {
		t.Errorf("events already emitted at write-back time = %d, want 0", eventsAtWriteBack)
	}
	if got := collectSceneSwitchedPayloads(t, eventChan); len(got) != 1 {
		t.Fatalf("scene.switched events = %+v, want exactly one", got)
	}
}

// TestMakeSceneDispatchNodeWriteBackFailureKeepsDecision 校验回写失败只是丢一次投影更新：
// 路由结果、提交的标签与切换事件都不受影响，节点也不返回错误。
func TestMakeSceneDispatchNodeWriteBackFailureKeepsDecision(t *testing.T) {
	ctx := sceneDispatchCtx("rid-write-back-failed", "thread-failed")
	state, eventChan := stateWithEventChan(turnBoundaryState(enumor.IntentTypeHostApply))

	updater := &stubSessionTagUpdater{err: errors.New("data-service unavailable")}
	node := newTestSceneDispatchNode(enumor.IntentTypeResourceQuery, updater)

	got, err := node(ctx, state)
	if err != nil {
		t.Fatalf("node() error = %v", err)
	}
	st, ok := got.(graph.State)
	if !ok {
		t.Fatalf("node() returned %T, want graph.State", got)
	}

	if next, _ := st[constant.StateKeySceneDispatchNext].(string); next !=
		string(enumor.SubgraphAgentNodeResourceQuery) {

		t.Errorf("scene_dispatch_next = %q, want %q", next, enumor.SubgraphAgentNodeResourceQuery)
	}
	if tag := agentstate.ParseSessionTag(st); tag != enumor.IntentTypeResourceQuery {
		t.Errorf("committed session_tag = %q, want %q", tag, enumor.IntentTypeResourceQuery)
	}
	if payloads := collectSceneSwitchedPayloads(t, eventChan); len(payloads) != 1 {
		t.Fatalf("scene.switched events = %+v, want exactly one", payloads)
	}
}

// mainGraphNodeNames lists every node the main graph is expected to contain.
var mainGraphNodeNames = []string{
	string(enumor.MainGraphAgentNodeSceneDispatch),
	string(enumor.MainGraphAgentNodeFallback),
	string(enumor.SubgraphAgentNodeHostApply),
	string(enumor.SubgraphAgentNodeResourceQuery),
}

// buildMainGraphTopologyTest compiles the main graph with placeholder nodes so that the topology
// can be asserted without pulling in models, tool sets or client sets.
func buildMainGraphTopologyTest(t *testing.T) *graph.Graph {
	t.Helper()

	stateGraph := graph.NewStateGraph(graph.MessagesStateSchema())
	noop := func(_ context.Context, _ graph.State) (any, error) { return graph.State{}, nil }
	for _, name := range mainGraphNodeNames {
		stateGraph.AddNode(name, noop)
	}
	buildMainGraphTopology(stateGraph)

	g, err := stateGraph.Compile()
	if err != nil {
		t.Fatalf("compile main graph failed, err: %v", err)
	}
	return g
}

// TestWireMainGraphTopologyHasNoIntentRecognitionNode 守住「意图识别已并入 scene_dispatch」这一结构决策：
// 主图不得再出现独立的意图识别节点。
func TestWireMainGraphTopologyHasNoIntentRecognitionNode(t *testing.T) {
	g := buildMainGraphTopologyTest(t)

	got := make(map[string]struct{}, len(g.Nodes()))
	for _, n := range g.Nodes() {
		got[n.ID] = struct{}{}
	}

	if _, exists := got["intent_recognition"]; exists {
		t.Error("main graph still contains an intent_recognition node")
	}
	if len(got) != len(mainGraphNodeNames) {
		t.Errorf("main graph nodes = %v, want exactly %v", got, mainGraphNodeNames)
	}
	for _, name := range mainGraphNodeNames {
		if _, exists := got[name]; !exists {
			t.Errorf("main graph is missing node %q", name)
		}
	}
}

// TestWireMainGraphTopologySceneDispatchIncomingEdges 守住「scene_dispatch 执行 ⟺ 轮次边界」契约：
// 它的入边只能是入口点与 fallback。任何新增入边都会让本节点在非轮次边界上执行，
// 从而在子图中途重跑意图识别、误切场景。
func TestWireMainGraphTopologySceneDispatchIncomingEdges(t *testing.T) {
	g := buildMainGraphTopologyTest(t)
	sceneDispatch := string(enumor.MainGraphAgentNodeSceneDispatch)

	if entry := g.EntryPoint(); entry != sceneDispatch {
		t.Errorf("entry point = %q, want %q", entry, sceneDispatch)
	}

	var sources []string
	for _, name := range mainGraphNodeNames {
		for _, e := range g.Edges(name) {
			if e.To == sceneDispatch {
				sources = append(sources, name)
			}
		}
	}

	want := []string{string(enumor.MainGraphAgentNodeFallback)}
	if len(sources) != len(want) || sources[0] != want[0] {
		t.Errorf("scene_dispatch incoming edges = %v, want %v (plus the entry point)", sources, want)
	}
}

// TestWireMainGraphTopologySceneDispatchTargets 校验条件边的目标集合与路由函数的返回值域一致。
func TestWireMainGraphTopologySceneDispatchTargets(t *testing.T) {
	g := buildMainGraphTopologyTest(t)

	condEdge, ok := g.ConditionalEdge(string(enumor.MainGraphAgentNodeSceneDispatch))
	if !ok {
		t.Fatal("scene_dispatch has no conditional edge")
	}

	want := map[string]string{
		string(enumor.SubgraphAgentNodeHostApply):     string(enumor.SubgraphAgentNodeHostApply),
		string(enumor.SubgraphAgentNodeResourceQuery): string(enumor.SubgraphAgentNodeResourceQuery),
		string(enumor.MainGraphAgentNodeFallback):     string(enumor.MainGraphAgentNodeFallback),
	}
	if len(condEdge.PathMap) != len(want) {
		t.Fatalf("scene_dispatch path map = %v, want %v", condEdge.PathMap, want)
	}
	for key, target := range want {
		if got := condEdge.PathMap[key]; got != target {
			t.Errorf("scene_dispatch path map[%q] = %q, want %q", key, got, target)
		}
	}
}

// newTestRegistry builds a hitl registry with human_confirm and the active confirm gates.
func newTestRegistry(enabled bool) *hitl.Registry {
	reg := hitl.NewRegistry()
	reg.Register(hitl.NewHumanConfirmHandler())
	for _, h := range toolgate.GetEnabledGateHandlers(cc.AgentConfirmGateConfig{Enabled: enabled}, nil, nil) {
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
		Function: trpcmodel.FunctionDefinitionParam{Name: string(enumor.DeclToolHumanConfirm)},
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
			name:       "host_apply decision routes to host_apply subgraph",
			state:      graph.State{constant.StateKeySceneDispatchNext: string(enumor.SubgraphAgentNodeHostApply)},
			wantTarget: string(enumor.SubgraphAgentNodeHostApply),
		},
		{
			name: "resource_query decision routes to resource_query subgraph",
			state: graph.State{
				constant.StateKeySceneDispatchNext: string(enumor.SubgraphAgentNodeResourceQuery),
			},
			wantTarget: string(enumor.SubgraphAgentNodeResourceQuery),
		},
		{
			name:       "fallback decision routes to fallback",
			state:      graph.State{constant.StateKeySceneDispatchNext: string(enumor.MainGraphAgentNodeFallback)},
			wantTarget: string(enumor.MainGraphAgentNodeFallback),
		},
		{
			// 路由函数只查表：即使 session_tag 指向某个场景，缺少决策键也必须安全兜底而非自行判定。
			name:       "missing decision key falls back instead of re-deriving from session_tag",
			state:      graph.State{constant.StateKeySessionTag: enumor.IntentTypeHostApply},
			wantTarget: string(enumor.MainGraphAgentNodeFallback),
		},
		{
			name:       "unknown node name falls back",
			state:      graph.State{constant.StateKeySceneDispatchNext: "no_such_node"},
			wantTarget: string(enumor.MainGraphAgentNodeFallback),
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

// TestSceneDispatchNodeThenRouting 把节点与路由函数串起来，确认判定结果能被条件边如实还原。
func TestSceneDispatchNodeThenRouting(t *testing.T) {
	route := makeSceneDispatchRoutingFunc()
	ctx := ridContext("rid-dispatch-then-route")

	tests := []struct {
		name       string
		tag        enumor.IntentType
		classified enumor.IntentType
		wantTarget string
	}{
		{
			name:       "untagged session classified as host_apply enters host_apply subgraph",
			classified: enumor.IntentTypeHostApply,
			wantTarget: string(enumor.SubgraphAgentNodeHostApply),
		},
		{
			name:       "host_apply session switching to resource_query enters resource_query subgraph",
			tag:        enumor.IntentTypeHostApply,
			classified: enumor.IntentTypeResourceQuery,
			wantTarget: string(enumor.SubgraphAgentNodeResourceQuery),
		},
		{
			name:       "chat inside a tagged session stays in the tagged subgraph",
			tag:        enumor.IntentTypeResourceQuery,
			classified: enumor.IntentTypeChat,
			wantTarget: string(enumor.SubgraphAgentNodeResourceQuery),
		},
		{
			name:       "untagged session classified as chat enters fallback",
			classified: enumor.IntentTypeChat,
			wantTarget: string(enumor.MainGraphAgentNodeFallback),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := turnBoundaryState(tagOrNil(tc.tag))
			node := newTestSceneDispatchNode(tc.classified, nil)
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

func TestUnsupportedSceneFallbackMessage(t *testing.T) {
	const (
		unsupportedMsg = "目前AI助手支持主机申领和云资源查询相关能力，其他云资源管理功能即将上线。" +
			"如需申领主机，请直接描述您的配置需求；如需查询云资源，请告诉我您想查询的内容。"
		genericMsg = "抱歉，我暂时无法处理您的请求。"
	)

	tests := []struct {
		name string
		tag  enumor.IntentType
		want string
	}{
		{
			name: "empty session tag returns guidance message",
			tag:  "",
			want: unsupportedMsg,
		},
		{
			name: "unsupported session tag returns guidance message",
			tag:  enumor.IntentTypeChat,
			want: unsupportedMsg,
		},
		{
			name: "supported resource_query tag returns generic message",
			tag:  enumor.IntentTypeResourceQuery,
			want: genericMsg,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := graph.State{constant.StateKeySessionTag: string(tc.tag)}
			got := unsupportedSceneFallbackMessage(state)
			if got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBuildFallbackResumeDeltaRebuildsUnsupportedSceneHistory 覆盖「无标签会话吃到未支持提示」后的
// resume：历史被重建为 assistant 提示 + 新用户输入，旧的拒识回复不会继续锚定下一轮的场景判断。
func TestBuildFallbackResumeDeltaRebuildsUnsupportedSceneHistory(t *testing.T) {
	fallbackText := "目前AI助手仅支持主机申领相关能力，其他云资源管理功能即将上线，如需要申领主机，请直接描述您的配置需求。"
	// 空标签代表本轮落在「无标签 + 分类不受支持」分支；受支持标签的会话历史则原样保留。
	state := graph.State{}
	messages := []trpcmodel.Message{
		{Role: trpcmodel.RoleUser, Content: "给我讲个故事"},
	}

	delta := message.BuildFallbackResumeDelta(context.Background(), state, messages, fallbackText,
		"那帮我申领一台主机吧")

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

// TestBuildFallbackResumeDeltaKeepsTaggedSceneHistory 覆盖「有标签会话子图跑完」的 resume：
// 历史必须原样保留、只追加本轮消息。判据从 intent 换成 session_tag 后若判反，
// 这里会退化成整段历史重建，用户可见地丢上下文。
func TestBuildFallbackResumeDeltaKeepsTaggedSceneHistory(t *testing.T) {
	const lastResp = "已为你生成 3 个申领方案"

	state := graph.State{constant.StateKeySessionTag: string(enumor.IntentTypeHostApply)}
	messages := []trpcmodel.Message{
		{Role: trpcmodel.RoleUser, Content: "帮我申请一台主机"},
		{Role: trpcmodel.RoleAssistant, Content: lastResp},
	}

	delta := message.BuildFallbackResumeDelta(context.Background(), state, messages, lastResp, "改成 16 核")

	if _, rebuilt := delta[graph.StateKeyMessages].([]graph.MessageOp); rebuilt {
		t.Fatal("history was rebuilt for a tagged session, want plain append")
	}
	appended, ok := delta[graph.StateKeyMessages].([]trpcmodel.Message)
	if !ok {
		t.Fatalf("messages delta = %T, want []trpcmodel.Message", delta[graph.StateKeyMessages])
	}
	// assistant 尾部已是同一条回复，本轮只应追加用户输入。
	if len(appended) != 1 || appended[0].Role != trpcmodel.RoleUser || appended[0].Content != "改成 16 核" {
		t.Fatalf("messages delta = %+v, want only the new user message", appended)
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
			name:      "unsupported scene rebuild path clears user_input",
			state:     graph.State{},
			messages:  []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "查看预测"}},
			userInput: "我要申请主机",
		},
		{
			name:  "normal resume path also clears user_input",
			state: graph.State{constant.StateKeySessionTag: enumor.IntentTypeHostApply},
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

// account_select 无可用账号时自行 emit 提示并把它落成 assistant 消息，主图 fallback 随后要复用同一条
// 消息而不是再追加一条。本用例串起「子图完成态 → output mapper → 主图 fallback 取文案 → resume 回填」
// 整条缝，确认历史里该文案只出现一次。
func TestNoUsableAccountFallbackMessageNotDuplicated(t *testing.T) {
	parentMsgs := []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "申请一台主机"}}
	// 子图完成态：account_select 在父图消息之后追加了一条无权限提示。
	childMsgs := append(append([]trpcmodel.Message{}, parentMsgs...),
		trpcmodel.Message{Role: trpcmodel.RoleAssistant, Content: constant.NoPermissionFallbackMessage})
	rawMsgs, err := json.Marshal(childMsgs)
	if err != nil {
		t.Fatalf("marshal child messages failed, err: %v", err)
	}

	parent := graph.State{
		graph.StateKeyMessages:      parentMsgs,
		constant.StateKeySessionTag: string(enumor.IntentTypeHostApply),
	}
	out := makeSubgraphOutputMapper("host_apply")(parent, graph.SubgraphResult{
		RawStateDelta: map[string][]byte{graph.StateKeyMessages: rawMsgs},
	})
	merged, _ := out[graph.StateKeyMessages].([]trpcmodel.Message)
	if len(merged) != 1 || merged[0].Content != constant.NoPermissionFallbackMessage {
		t.Fatalf("output mapper delta = %+v, want single no-permission assistant message", merged)
	}

	// 主图 fallback 看到的 state：父图消息 + mapper 合并进来的那一条。
	parent[graph.StateKeyMessages] = append(append([]trpcmodel.Message{}, parentMsgs...), merged...)
	lastResp := resolveFallbackLastResp(parent)
	if lastResp != constant.NoPermissionFallbackMessage {
		t.Fatalf("fallback last response = %q, want no-permission message", lastResp)
	}

	// resume 时只应追加用户输入：assistant 尾部已是同一条文案，不能再补一条。
	history, _ := parent[graph.StateKeyMessages].([]trpcmodel.Message)
	delta := message.BuildFallbackResumeDelta(context.Background(), parent, history, lastResp, "换个业务试试")
	resumeMsgs, ok := delta[graph.StateKeyMessages].([]trpcmodel.Message)
	if !ok {
		t.Fatalf("resume delta messages type = %T, want []trpcmodel.Message", delta[graph.StateKeyMessages])
	}
	if len(resumeMsgs) != 1 || resumeMsgs[0].Role != trpcmodel.RoleUser {
		t.Fatalf("resume delta = %+v, want only the user message", resumeMsgs)
	}

	count := 0
	for _, msg := range append(history, resumeMsgs...) {
		if msg.Content == constant.NoPermissionFallbackMessage {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("no-permission message appears %d times in history, want 1", count)
	}
}
