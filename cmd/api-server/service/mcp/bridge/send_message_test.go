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
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package bridge

import (
	"context"
	"errors"
	"testing"
	"time"

	"hcm/cmd/api-server/service/mcp/ingress"
	"hcm/cmd/api-server/service/mcp/middleware"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"

	a2aclient "trpc.group/trpc-go/trpc-a2a-go/client"
	"trpc.group/trpc-go/trpc-a2a-go/protocol"
	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

type mockProvider struct {
	client   *mockA2AClient
	endpoint string
	err      error
	calls    int
}

func (m *mockProvider) NewClient(context.Context) (a2aStreamClient, string, error) {
	m.calls++
	if m.err != nil {
		return nil, "", m.err
	}
	return m.client, m.endpoint, nil
}

type mockA2AClient struct {
	events       []protocol.StreamingMessageEvent
	streamErr    error
	streamParams []protocol.SendMessageParams
	cancelIDs    []string
	cancelErr    error
}

func (m *mockA2AClient) StreamMessage(_ context.Context, params protocol.SendMessageParams,
	_ ...a2aclient.RequestOption) (<-chan protocol.StreamingMessageEvent, error) {

	m.streamParams = append(m.streamParams, params)
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	ch := make(chan protocol.StreamingMessageEvent, len(m.events))
	for _, event := range m.events {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func (m *mockA2AClient) CancelTasks(_ context.Context, params protocol.TaskIDParams,
	_ ...a2aclient.RequestOption) (*protocol.Task, error) {

	m.cancelIDs = append(m.cancelIDs, params.ID)
	if m.cancelErr != nil {
		return nil, m.cancelErr
	}
	return &protocol.Task{
		ID:        params.ID,
		ContextID: "ctx-cancel",
		Status:    protocol.TaskStatus{State: protocol.TaskStateCanceled},
	}, nil
}

func TestHandler_SendMessageHappyPath(t *testing.T) {
	client := &mockA2AClient{
		events: []protocol.StreamingMessageEvent{
			statusEvent("task-1", "ctx-1", protocol.TaskStateWorking, "working", false),
			artifactEvent("task-1", "ctx-1", "hello "),
			artifactEvent("task-1", "ctx-1", "world"),
			statusEvent("task-1", "ctx-1", protocol.TaskStateCompleted, "done", true),
		},
	}
	handler := newTestHandler(client)

	var progress []string
	handler.progressSender = func(_ context.Context, _ interface{}, _ float64, message string) {
		progress = append(progress, message)
	}

	result, err := handler.SendMessage(testCtx(), &ingress.SendMessageRequest{
		Text:          "列出 CVM",
		ContextID:     "ctx-1",
		BkBizID:       123,
		ModelName:     "hunyuan",
		ProgressToken: "p1",
		MCPServerName: "server-a",
	}, nil)
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result IsError = true, content=%v", result.Content)
	}
	if got := resultText(result); got != "hello world" {
		t.Fatalf("result text = %q, want %q", got, "hello world")
	}
	if result.Meta[resultMetaContextIDKey] != "ctx-1" {
		t.Fatalf("contextId meta = %v", result.Meta[resultMetaContextIDKey])
	}
	if result.Meta[resultMetaTaskIDKey] != "task-1" {
		t.Fatalf("taskId meta = %v", result.Meta[resultMetaTaskIDKey])
	}
	if handler.taskMap.Len() != 0 {
		t.Fatalf("task map len = %d, want 0", handler.taskMap.Len())
	}
	if len(progress) != 4 {
		t.Fatalf("progress count = %d, want 4, progress=%v", len(progress), progress)
	}

	if len(client.streamParams) != 1 {
		t.Fatalf("stream calls = %d, want 1", len(client.streamParams))
	}
	params := client.streamParams[0]
	if params.Message.ContextID == nil || *params.Message.ContextID != "ctx-1" {
		t.Fatalf("message contextId = %v", params.Message.ContextID)
	}
	if params.Metadata[metadataBkBizIDKey] != int64(123) {
		t.Fatalf("bk_biz_id metadata = %v", params.Metadata[metadataBkBizIDKey])
	}
}

func TestHandler_SendMessageGeneratedContextID(t *testing.T) {
	client := &mockA2AClient{
		events: []protocol.StreamingMessageEvent{
			statusEvent("task-new", "ctx-new", protocol.TaskStateCompleted, "created", true),
		},
	}
	handler := newTestHandler(client)
	result, err := handler.SendMessage(testCtx(), &ingress.SendMessageRequest{
		Text:          "你好",
		ProgressToken: "p1",
	}, nil)
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if client.streamParams[0].Message.ContextID != nil {
		t.Fatalf("first call contextId = %v, want nil", client.streamParams[0].Message.ContextID)
	}
	if result.Meta[resultMetaContextIDKey] != "ctx-new" {
		t.Fatalf("returned contextId = %v", result.Meta[resultMetaContextIDKey])
	}
}

func TestHandler_SendMessageMissingText(t *testing.T) {
	client := &mockA2AClient{}
	handler := newTestHandler(client)
	_, err := handler.SendMessage(testCtx(), &ingress.SendMessageRequest{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(client.streamParams) != 0 {
		t.Fatalf("stream calls = %d, want 0", len(client.streamParams))
	}
}

func TestHandler_SendMessageFiveArtifactsProgress(t *testing.T) {
	events := make([]protocol.StreamingMessageEvent, 0, 6)
	for i := 0; i < 5; i++ {
		events = append(events, artifactEvent("task-1", "ctx-1", "chunk"))
	}
	events = append(events, statusEvent("task-1", "ctx-1", protocol.TaskStateCompleted, "done", true))

	client := &mockA2AClient{events: events}
	handler := newTestHandler(client)

	progressCount := 0
	handler.progressSender = func(context.Context, interface{}, float64, string) {
		progressCount++
	}

	result, err := handler.SendMessage(testCtx(), &ingress.SendMessageRequest{
		Text:          "stream",
		ProgressToken: "p1",
	}, nil)
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if resultText(result) != "chunkchunkchunkchunkchunk" {
		t.Fatalf("unexpected result text: %q", resultText(result))
	}
	if progressCount != 6 {
		t.Fatalf("progress count = %d, want 6", progressCount)
	}
}

func TestHandler_SendMessageFailedState(t *testing.T) {
	client := &mockA2AClient{
		events: []protocol.StreamingMessageEvent{
			statusEvent("task-1", "ctx-1", protocol.TaskStateFailed, "模型调用超时", true),
		},
	}
	handler := newTestHandler(client)
	result, err := handler.SendMessage(testCtx(), &ingress.SendMessageRequest{
		Text:          "fail",
		ProgressToken: "p1",
	}, nil)
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if !result.IsError {
		t.Fatal("result IsError = false, want true")
	}
	if got := resultText(result); got != "模型调用超时" {
		t.Fatalf("error text = %q", got)
	}
}

func TestHandler_CancelByProgressToken(t *testing.T) {
	client := &mockA2AClient{}
	handler := newTestHandler(client)
	handler.taskMap.Set("p1", "task-1")

	handler.CancelByProgressToken(testCtx(), "p1")
	if len(client.cancelIDs) != 1 || client.cancelIDs[0] != "task-1" {
		t.Fatalf("cancelIDs = %v, want [task-1]", client.cancelIDs)
	}
	if handler.taskMap.Len() != 0 {
		t.Fatalf("task map len = %d, want 0", handler.taskMap.Len())
	}
}

func TestHandler_StreamMessageError(t *testing.T) {
	client := &mockA2AClient{streamErr: errors.New("network down")}
	handler := newTestHandler(client)
	_, err := handler.SendMessage(testCtx(), &ingress.SendMessageRequest{Text: "hello"}, nil)
	if err == nil {
		t.Fatal("expected stream error")
	}
}

func TestProgressTaskMapLRUEviction(t *testing.T) {
	m := NewProgressTaskMap(2, time.Minute)
	m.Set("p1", "task-1")
	m.Set("p2", "task-2")
	if _, ok := m.Get("p1"); !ok {
		t.Fatal("p1 should exist")
	}
	m.Set("p3", "task-3")

	if _, ok := m.Get("p2"); ok {
		t.Fatal("p2 should be evicted as least recently used")
	}
	if taskID, ok := m.Get("p1"); !ok || taskID != "task-1" {
		t.Fatalf("p1 = %q, %v", taskID, ok)
	}
	if taskID, ok := m.Get("p3"); !ok || taskID != "task-3" {
		t.Fatalf("p3 = %q, %v", taskID, ok)
	}
}

func TestBuildA2AEndpointURL(t *testing.T) {
	got, err := buildA2AEndpointURL("127.0.0.1:8080")
	if err != nil {
		t.Fatalf("buildA2AEndpointURL error: %v", err)
	}
	want := "http://127.0.0.1:8080/api/v1/agent/a2a"
	if got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestRequestHeaders(t *testing.T) {
	headers := requestHeaders(&kit.Kit{Rid: "rid-1", User: "admin", AppCode: "app", TenantID: "tenant"})
	if headers[constant.UserKey] != "admin" {
		t.Fatalf("user header = %q", headers[constant.UserKey])
	}
	if headers[constant.MCPCallerSourceHeader] != string(cc.APIServerName) {
		t.Fatalf("caller source = %q", headers[constant.MCPCallerSourceHeader])
	}
}

func newTestHandler(client *mockA2AClient) *Handler {
	return newHandlerWithProvider(cc.MCPIngressSetting{
		AggregatedToolName:      constant.MCPIngressAggregatedToolName,
		ProgressTaskMapMaxSize:  100,
		ProgressTaskMapEntryTTL: time.Minute,
	}, &mockProvider{client: client, endpoint: "http://agent/api/v1/agent/a2a"})
}

func testCtx() context.Context {
	return middleware.WithKit(context.Background(), &kit.Kit{
		Rid:      "rid-1",
		User:     "admin",
		AppCode:  "app",
		TenantID: "tenant",
	})
}

func statusEvent(taskID, contextID string, state protocol.TaskState, text string,
	final bool) protocol.StreamingMessageEvent {

	return protocol.StreamingMessageEvent{Result: &protocol.TaskStatusUpdateEvent{
		TaskID:    taskID,
		ContextID: contextID,
		Kind:      protocol.KindTaskStatusUpdate,
		Final:     final,
		Status: protocol.TaskStatus{
			State:   state,
			Message: textMessage(text),
		},
	}}
}

func artifactEvent(taskID, contextID, text string) protocol.StreamingMessageEvent {
	return protocol.StreamingMessageEvent{Result: &protocol.TaskArtifactUpdateEvent{
		TaskID:    taskID,
		ContextID: contextID,
		Kind:      protocol.KindTaskArtifactUpdate,
		Artifact: protocol.Artifact{
			ArtifactID: "artifact-1",
			Parts:      []protocol.Part{protocol.NewTextPart(text)},
		},
	}}
}

func textMessage(text string) *protocol.Message {
	msg := protocol.NewMessage(protocol.MessageRoleAgent, []protocol.Part{protocol.NewTextPart(text)})
	return &msg
}

func resultText(result *mcpsdk.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	if text, ok := result.Content[0].(mcpsdk.TextContent); ok {
		return text.Text
	}
	return ""
}
