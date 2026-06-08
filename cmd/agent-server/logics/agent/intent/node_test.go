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

package intent

import (
	"context"
	"errors"
	"testing"

	"hcm/cmd/agent-server/logics/prompt"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

// mockModel implements trpcmodel.Model for testing.
type mockModel struct {
	// response is the text content returned by GenerateContent.
	response string
	// err is returned by GenerateContent when non-nil.
	err error
	// apiErr is placed in Response.Error when non-nil.
	apiErr *trpcmodel.ResponseError
}

func (m *mockModel) GenerateContent(_ context.Context, _ *trpcmodel.Request) (<-chan *trpcmodel.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan *trpcmodel.Response, 1)
	ch <- &trpcmodel.Response{
		Choices: []trpcmodel.Choice{
			{Message: trpcmodel.Message{Content: m.response}},
		},
		Error: m.apiErr,
	}
	close(ch)
	return ch, nil
}

func (m *mockModel) Info() trpcmodel.Info {
	return trpcmodel.Info{Name: "mock"}
}

const defaultTestPrompt = "classify intent"

// defaultContextWindowSize is the context window size used by most test cases.
const defaultContextWindowSize = 5

// newTestPromptStore returns an in-memory prompt.Store with the given content
// stored under constant.IntentRecognitionPromptKey.
func newTestPromptStore(content string) *prompt.Store {
	s := prompt.NewStore("")
	_ = s.Set(constant.IntentRecognitionPromptKey, prompt.PromptEntry{Content: content})
	return s
}

// userMsg creates a user message.
func userMsg(content string) trpcmodel.Message {
	return trpcmodel.Message{Role: trpcmodel.RoleUser, Content: content}
}

// assistantMsg creates an assistant message without tool calls.
func assistantMsg(content string) trpcmodel.Message {
	return trpcmodel.Message{Role: trpcmodel.RoleAssistant, Content: content}
}

// toolCallMsg creates an assistant message that contains tool calls.
func toolCallMsg() trpcmodel.Message {
	return trpcmodel.Message{
		Role:      trpcmodel.RoleAssistant,
		ToolCalls: []trpcmodel.ToolCall{{ID: "call-1"}},
	}
}

// toolResultMsg creates a tool result message.
func toolResultMsg() trpcmodel.Message {
	return trpcmodel.Message{Role: trpcmodel.RoleTool, Content: "result"}
}

// stateWithMessages builds a graph.State containing the provided messages.
func stateWithMessages(msgs []trpcmodel.Message) graph.State {
	return graph.State{graph.StateKeyMessages: msgs}
}

func TestMakeIntentRecognitionNode_HostApply(t *testing.T) {
	mdl := &mockModel{response: "host_apply"}
	node := MakeIntentRecognitionNode(mdl, newTestPromptStore(defaultTestPrompt), defaultContextWindowSize)

	state := stateWithMessages([]trpcmodel.Message{userMsg("帮我申请一台主机")})
	result, err := node(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := assertIntentState(t, result)
	if got != string(enumor.IntentTypeHostApply) {
		t.Errorf("intent = %q, want %q", got, enumor.IntentTypeHostApply)
	}
}

func TestMakeIntentRecognitionNode_ResourceQuery(t *testing.T) {
	mdl := &mockModel{response: "resource_query"}
	node := MakeIntentRecognitionNode(mdl, newTestPromptStore(defaultTestPrompt), defaultContextWindowSize)

	state := stateWithMessages([]trpcmodel.Message{userMsg("查看我的主机列表")})
	result, err := node(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := assertIntentState(t, result)
	if got != string(enumor.IntentTypeResourceQuery) {
		t.Errorf("intent = %q, want %q", got, enumor.IntentTypeResourceQuery)
	}
}

func TestMakeIntentRecognitionNode_Chat(t *testing.T) {
	mdl := &mockModel{response: "chat"}
	node := MakeIntentRecognitionNode(mdl, newTestPromptStore(defaultTestPrompt), defaultContextWindowSize)

	state := stateWithMessages([]trpcmodel.Message{userMsg("你好")})
	result, err := node(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := assertIntentState(t, result)
	if got != string(enumor.IntentTypeChat) {
		t.Errorf("intent = %q, want %q", got, enumor.IntentTypeChat)
	}
}

func TestMakeIntentRecognitionNode_UnknownResponseFallbackToChat(t *testing.T) {
	// The model returns a formatted string that cannot be matched to any IntentType.
	mdl := &mockModel{response: "intent: host_apply"}
	node := MakeIntentRecognitionNode(mdl, newTestPromptStore(defaultTestPrompt), defaultContextWindowSize)

	state := stateWithMessages([]trpcmodel.Message{userMsg("申请主机")})
	result, err := node(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := assertIntentState(t, result)
	if got != string(enumor.IntentTypeChat) {
		t.Errorf("intent = %q, want fallback %q", got, enumor.IntentTypeChat)
	}
}

func TestMakeIntentRecognitionNode_LLMCallErrorFallbackToChat(t *testing.T) {
	mdl := &mockModel{err: errors.New("network timeout")}
	node := MakeIntentRecognitionNode(mdl, newTestPromptStore(defaultTestPrompt), defaultContextWindowSize)

	state := stateWithMessages([]trpcmodel.Message{userMsg("申请主机")})
	result, err := node(context.Background(), state)
	if err != nil {
		t.Fatalf("node must not propagate LLM errors, got: %v", err)
	}

	got := assertIntentState(t, result)
	if got != string(enumor.IntentTypeChat) {
		t.Errorf("intent = %q, want fallback %q", got, enumor.IntentTypeChat)
	}
}

func TestMakeIntentRecognitionNode_APIErrorFallbackToChat(t *testing.T) {
	mdl := &mockModel{
		response: "",
		apiErr:   &trpcmodel.ResponseError{Message: "rate limit exceeded"},
	}
	node := MakeIntentRecognitionNode(mdl, newTestPromptStore(defaultTestPrompt), defaultContextWindowSize)

	state := stateWithMessages([]trpcmodel.Message{userMsg("申请主机")})
	result, err := node(context.Background(), state)
	if err != nil {
		t.Fatalf("node must not propagate API errors, got: %v", err)
	}

	got := assertIntentState(t, result)
	if got != string(enumor.IntentTypeChat) {
		t.Errorf("intent = %q, want fallback %q", got, enumor.IntentTypeChat)
	}
}

func TestMakeIntentRecognitionNode_EmptyMessagesFallbackToChat(t *testing.T) {
	mdl := &mockModel{response: "host_apply"}
	node := MakeIntentRecognitionNode(mdl, newTestPromptStore(defaultTestPrompt), defaultContextWindowSize)

	result, err := node(context.Background(), graph.State{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := assertIntentState(t, result)
	if got != string(enumor.IntentTypeChat) {
		t.Errorf("intent = %q, want fallback %q", got, enumor.IntentTypeChat)
	}
}

func TestMakeIntentRecognitionNode_NoUserMessageFallbackToChat(t *testing.T) {
	// State contains only an assistant message — no user message.
	mdl := &mockModel{response: "host_apply"}
	node := MakeIntentRecognitionNode(mdl, newTestPromptStore(defaultTestPrompt), defaultContextWindowSize)

	state := stateWithMessages([]trpcmodel.Message{assistantMsg("你好，有什么可以帮你？")})
	result, err := node(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := assertIntentState(t, result)
	if got != string(enumor.IntentTypeChat) {
		t.Errorf("intent = %q, want fallback %q", got, enumor.IntentTypeChat)
	}
}

func TestMakeIntentRecognitionNode_UsesIntentRecognitionPromptAsSystemMessage(t *testing.T) {
	var capturedReq *trpcmodel.Request
	capturingMdl := &capturingModel{
		response: "host_apply",
		capture:  func(r *trpcmodel.Request) { capturedReq = r },
	}
	const customPrompt = "custom intent recognition prompt"
	node := MakeIntentRecognitionNode(capturingMdl, newTestPromptStore(customPrompt), defaultContextWindowSize)

	state := stateWithMessages([]trpcmodel.Message{userMsg("帮我申请一台主机")})
	_, err := node(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReq == nil || len(capturedReq.Messages) == 0 {
		t.Fatal("no request was captured")
	}
	if capturedReq.Messages[0].Content != customPrompt {
		t.Errorf("system message = %q, want %q", capturedReq.Messages[0].Content, customPrompt)
	}
}

func TestMakeIntentRecognitionNode_ToolMessagesFiltered(t *testing.T) {
	// Verify that only user messages are passed to the classification LLM.
	// Tool messages and assistant replies must both be excluded.
	var capturedReq *trpcmodel.Request
	capturingMdl := &capturingModel{
		response: "resource_query",
		capture:  func(r *trpcmodel.Request) { capturedReq = r },
	}
	node := MakeIntentRecognitionNode(capturingMdl, newTestPromptStore(defaultTestPrompt), defaultContextWindowSize)

	state := stateWithMessages([]trpcmodel.Message{
		userMsg("查主机"),
		toolCallMsg(),   // should be filtered
		toolResultMsg(), // should be filtered
		assistantMsg("已找到以下主机"),
		userMsg("再查一次"),
	})
	_, err := node(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("no request was captured")
	}
	// First message is the system prompt. Remaining messages must be user-only.
	for _, m := range capturedReq.Messages[1:] {
		if m.Role != trpcmodel.RoleUser {
			t.Errorf("non-user message leaked into classification request: role=%s", m.Role)
		}
	}
}

func TestMakeIntentRecognitionNode_ContextWindowRespected(t *testing.T) {
	// With ContextWindowSize=2, only the 2 most recent non-tool messages should be passed.
	var capturedReq *trpcmodel.Request
	capturingMdl := &capturingModel{
		response: "chat",
		capture:  func(r *trpcmodel.Request) { capturedReq = r },
	}
	node := MakeIntentRecognitionNode(capturingMdl, newTestPromptStore("classify"), 2)

	state := stateWithMessages([]trpcmodel.Message{
		userMsg("msg1"),
		assistantMsg("reply1"),
		userMsg("msg2"),
		assistantMsg("reply2"),
		userMsg("msg3"), // only msg2+reply2+msg3 → last 2 non-tool = reply2, msg3
	})
	_, err := node(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Messages in request: [system] + up to 2 context messages.
	if capturedReq == nil {
		t.Fatal("no request was captured")
	}
	contextCount := len(capturedReq.Messages) - 1 // subtract system message
	if contextCount > 2 {
		t.Errorf("context window not respected: got %d messages, want <= 2", contextCount)
	}
}

// assertIntentState asserts that result is a graph.State containing constant.StateKeyIntent
// and returns the intent string value.
func assertIntentState(t *testing.T, result any) string {
	t.Helper()
	s, ok := result.(graph.State)
	if !ok {
		t.Fatalf("result is %T, want graph.State", result)
	}
	v, ok := s[constant.StateKeyIntent].(string)
	if !ok {
		t.Fatalf("state[%q] is %T, want string", constant.StateKeyIntent, s[constant.StateKeyIntent])
	}
	return v
}

func TestIntentState(t *testing.T) {
	tests := []struct {
		name   string
		intent enumor.IntentType
	}{
		{name: "host_apply", intent: enumor.IntentTypeHostApply},
		{name: "chat", intent: enumor.IntentTypeChat},
		{name: "resource_query", intent: enumor.IntentTypeResourceQuery},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := intentState(tc.intent)

			intentVal, _ := state[constant.StateKeyIntent].(string)
			if enumor.IntentType(intentVal) != tc.intent {
				t.Errorf("intent = %q, want %q", intentVal, tc.intent)
			}
			if len(state) != 1 {
				t.Errorf("state keys = %d, want only intent", len(state))
			}
		})
	}
}

// capturingModel captures the last request sent to GenerateContent for inspection.
type capturingModel struct {
	response string
	capture  func(*trpcmodel.Request)
}

func (m *capturingModel) GenerateContent(_ context.Context, req *trpcmodel.Request) (<-chan *trpcmodel.Response, error) {
	if m.capture != nil {
		m.capture(req)
	}
	ch := make(chan *trpcmodel.Response, 1)
	ch <- &trpcmodel.Response{
		Choices: []trpcmodel.Choice{
			{Message: trpcmodel.Message{Content: m.response}},
		},
	}
	close(ch)
	return ch, nil
}

func (m *capturingModel) Info() trpcmodel.Info {
	return trpcmodel.Info{Name: "capturing-mock"}
}
