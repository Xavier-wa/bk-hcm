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

// capturingModel captures the last request sent to GenerateContent for inspection.
type capturingModel struct {
	response string
	capture  func(*trpcmodel.Request)
}

func (m *capturingModel) GenerateContent(_ context.Context, req *trpcmodel.Request) (
	<-chan *trpcmodel.Response, error) {

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

// TestClassify covers the recognised intents and every degradation path. Classify never
// returns an error: all failures must degrade to unsupported so that the caller's decision
// matrix falls into the "unsupported" branch and keeps the session in its current scene.
func TestClassify(t *testing.T) {
	tests := []struct {
		name        string
		mdl         trpcmodel.Model
		promptStore *prompt.Store
		messages    []trpcmodel.Message
		want        enumor.IntentType
	}{
		{
			name:        "host apply",
			mdl:         &mockModel{response: "host_apply"},
			promptStore: newTestPromptStore(defaultTestPrompt),
			messages:    []trpcmodel.Message{userMsg("帮我申请一台主机")},
			want:        enumor.IntentTypeHostApply,
		},
		{
			name:        "resource query",
			mdl:         &mockModel{response: "resource_query"},
			promptStore: newTestPromptStore(defaultTestPrompt),
			messages:    []trpcmodel.Message{userMsg("查看我的主机列表")},
			want:        enumor.IntentTypeResourceQuery,
		},
		{
			name:        "chat",
			mdl:         &mockModel{response: "chat"},
			promptStore: newTestPromptStore(defaultTestPrompt),
			messages:    []trpcmodel.Message{userMsg("你好")},
			want:        enumor.IntentTypeChat,
		},
		{
			name:        "unrecognised model response degrades to unsupported",
			mdl:         &mockModel{response: "intent: host_apply"},
			promptStore: newTestPromptStore(defaultTestPrompt),
			messages:    []trpcmodel.Message{userMsg("申请主机")},
			want:        enumor.IntentTypeUnsupported,
		},
		{
			name:        "llm call error degrades to unsupported",
			mdl:         &mockModel{err: errors.New("network timeout")},
			promptStore: newTestPromptStore(defaultTestPrompt),
			messages:    []trpcmodel.Message{userMsg("申请主机")},
			want:        enumor.IntentTypeUnsupported,
		},
		{
			name:        "model api error degrades to unsupported",
			mdl:         &mockModel{apiErr: &trpcmodel.ResponseError{Message: "rate limit exceeded"}},
			promptStore: newTestPromptStore(defaultTestPrompt),
			messages:    []trpcmodel.Message{userMsg("申请主机")},
			want:        enumor.IntentTypeUnsupported,
		},
		{
			name:        "nil prompt store degrades to unsupported without panicking",
			mdl:         &mockModel{response: "host_apply"},
			promptStore: nil,
			messages:    []trpcmodel.Message{userMsg("申请主机")},
			want:        enumor.IntentTypeUnsupported,
		},
		{
			name:        "missing intent prompt degrades to unsupported",
			mdl:         &mockModel{response: "host_apply"},
			promptStore: prompt.NewStore(""),
			messages:    []trpcmodel.Message{userMsg("申请主机")},
			want:        enumor.IntentTypeUnsupported,
		},
		{
			name:        "empty messages degrade to unsupported",
			mdl:         &mockModel{response: "host_apply"},
			promptStore: newTestPromptStore(defaultTestPrompt),
			messages:    nil,
			want:        enumor.IntentTypeUnsupported,
		},
		{
			name:        "no user message degrades to unsupported",
			mdl:         &mockModel{response: "host_apply"},
			promptStore: newTestPromptStore(defaultTestPrompt),
			messages:    []trpcmodel.Message{assistantMsg("你好，有什么可以帮你？")},
			want:        enumor.IntentTypeUnsupported,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(context.Background(), tc.mdl, tc.promptStore, tc.messages, defaultContextWindowSize)
			if got != tc.want {
				t.Errorf("Classify() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyUsesIntentRecognitionPromptAsSystemMessage(t *testing.T) {
	var capturedReq *trpcmodel.Request
	mdl := &capturingModel{
		response: "host_apply",
		capture:  func(r *trpcmodel.Request) { capturedReq = r },
	}
	const customPrompt = "custom intent recognition prompt"

	Classify(context.Background(), mdl, newTestPromptStore(customPrompt),
		[]trpcmodel.Message{userMsg("帮我申请一台主机")}, defaultContextWindowSize)

	if capturedReq == nil || len(capturedReq.Messages) == 0 {
		t.Fatal("no request was captured")
	}
	if capturedReq.Messages[0].Content != customPrompt {
		t.Errorf("system message = %q, want %q", capturedReq.Messages[0].Content, customPrompt)
	}
}

// TestClassifyFiltersToolMessages verifies that tool artifacts never reach the classification
// LLM: only user and plain assistant messages are forwarded.
func TestClassifyFiltersToolMessages(t *testing.T) {
	var capturedReq *trpcmodel.Request
	mdl := &capturingModel{
		response: "resource_query",
		capture:  func(r *trpcmodel.Request) { capturedReq = r },
	}

	messages := []trpcmodel.Message{
		userMsg("查主机"),
		toolCallMsg(),   // should be filtered
		toolResultMsg(), // should be filtered
		assistantMsg("已找到以下主机"),
		userMsg("再查一次"),
	}
	Classify(context.Background(), mdl, newTestPromptStore(defaultTestPrompt), messages, defaultContextWindowSize)

	if capturedReq == nil {
		t.Fatal("no request was captured")
	}
	// First message is the system prompt; the rest must carry no tool artifacts.
	for _, m := range capturedReq.Messages[1:] {
		if m.Role == trpcmodel.RoleTool || len(m.ToolCalls) > 0 {
			t.Errorf("tool artifact leaked into classification request: role=%s, tool_calls=%d",
				m.Role, len(m.ToolCalls))
		}
	}
}

func TestClassifyRespectsContextWindow(t *testing.T) {
	const contextWindowSize = 2

	var capturedReq *trpcmodel.Request
	mdl := &capturingModel{
		response: "chat",
		capture:  func(r *trpcmodel.Request) { capturedReq = r },
	}

	messages := []trpcmodel.Message{
		userMsg("msg1"),
		assistantMsg("reply1"),
		userMsg("msg2"),
		assistantMsg("reply2"),
		userMsg("msg3"),
	}
	Classify(context.Background(), mdl, newTestPromptStore(defaultTestPrompt), messages, contextWindowSize)

	if capturedReq == nil {
		t.Fatal("no request was captured")
	}
	if contextCount := len(capturedReq.Messages) - 1; contextCount > contextWindowSize {
		t.Errorf("context window not respected: got %d messages, want <= %d", contextCount, contextWindowSize)
	}
}
