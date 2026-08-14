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

	mcpmetrics "hcm/cmd/api-server/service/mcp/metrics"

	"trpc.group/trpc-go/trpc-a2a-go/protocol"
)

func TestHandler_CancelBranches(t *testing.T) {
	(*Handler)(nil).Cancel(context.Background(), "p-nil")

	client := &mockA2AClient{}
	handler := newTestHandler(client)
	handler.Cancel(testCtx(), "unknown")
	if len(client.cancelIDs) != 0 {
		t.Fatalf("cancelIDs = %v, want none", client.cancelIDs)
	}

	handler.taskMap.Set("p1", "task-1")
	handler.clients = &mockProvider{err: errors.New("no client")}
	handler.CancelByProgressToken(testCtx(), "p1")
	if handler.taskMap.Len() != 1 {
		t.Fatalf("task map should keep token when client creation fails")
	}

	client.cancelErr = errors.New("cancel failed")
	handler.clients = &mockProvider{client: client, endpoint: "http://agent"}
	handler.CancelByProgressToken(testCtx(), "p1")
	if handler.taskMap.Len() != 1 {
		t.Fatalf("task map should keep token when cancel fails")
	}
}

func TestBridgeProgressAndStatusHelpers(t *testing.T) {
	progressCases := map[protocol.TaskState]float64{
		protocol.TaskStateSubmitted:     0.1,
		protocol.TaskStateWorking:       0.5,
		protocol.TaskStateInputRequired: 0.5,
		protocol.TaskStateCompleted:     1,
		protocol.TaskStateCanceled:      1,
		protocol.TaskStateFailed:        1,
		protocol.TaskStateRejected:      1,
		protocol.TaskStateAuthRequired:  1,
		protocol.TaskState("unknown"):   0.5,
	}
	for state, want := range progressCases {
		if got := progressForState(state); got != want {
			t.Fatalf("progressForState(%s) = %v, want %v", state, got, want)
		}
	}

	statusCases := map[protocol.TaskState]string{
		protocol.TaskStateCompleted:    mcpmetrics.StatusSuccess,
		protocol.TaskStateCanceled:     mcpmetrics.StatusCanceled,
		protocol.TaskStateFailed:       mcpmetrics.StatusError,
		protocol.TaskStateRejected:     mcpmetrics.StatusError,
		protocol.TaskStateAuthRequired: mcpmetrics.StatusError,
		protocol.TaskStateWorking:      mcpmetrics.StatusSuccess,
	}
	for state, want := range statusCases {
		if got := statusFromTaskState(state); got != want {
			t.Fatalf("statusFromTaskState(%s) = %s, want %s", state, got, want)
		}
	}

	sendProgress(context.Background(), nil, 0.1, "skip")
	sendProgress(context.Background(), "p1", 0.1, "")
	sendProgress(context.Background(), "p1", 0.1, "no sender")
}

func TestBridgeResultHelpers(t *testing.T) {
	handler := newTestHandler(&mockA2AClient{})
	if got := resultText(handler.finalResult(protocol.TaskStateCompleted, "", "", "ctx", "task", "", nil)); got !=
		completedResultFallback {
		t.Fatalf("completed fallback = %q", got)
	}
	if got := resultText(handler.finalResult(protocol.TaskStateFailed, "", string(protocol.TaskStateFailed),
		"ctx", "task", "", nil)); got != failedResultFallback {
		t.Fatalf("failed fallback = %q", got)
	}
	if !handler.finalResult(protocol.TaskStateCanceled, "", "", "ctx", "task", "", nil).IsError {
		t.Fatal("canceled result should be error")
	}
	if got := resultText(handler.finalResult(protocol.TaskStateWorking, "text", "", "ctx", "task", "", nil)); got !=
		"text" {
		t.Fatalf("default result = %q", got)
	}
	if (*Handler)(nil).metricToolName() != "send_message" {
		t.Fatal("nil metricToolName should return default tool name")
	}
	if _, rid, user := kitFromCtx(context.Background()); rid != unknownRid || user != "" {
		t.Fatalf("kitFromCtx without kit = rid %q user %q", rid, user)
	}
}

func TestPartTextAndArtifactCollector(t *testing.T) {
	dataText := partText(protocol.NewDataPart(map[string]interface{}{"x": "y"}))
	if dataText != `{"x":"y"}` {
		t.Fatalf("data part text = %q", dataText)
	}

	fileBytes := protocol.NewFilePartWithBytes("report.txt", "text/plain", "YQ==")
	if got := partText(fileBytes); got != "[file:report.txt]" {
		t.Fatalf("file bytes text = %q", got)
	}
	fileURI := protocol.NewFilePartWithURI("", "text/plain", "https://example.com/report.txt")
	if got := partText(fileURI); got != "[file:https://example.com/report.txt]" {
		t.Fatalf("file uri text = %q", got)
	}
	if got := filePartText(protocol.FilePart{}); got != "[file]" {
		t.Fatalf("empty file part text = %q", got)
	}
	if got := partText(nil); got != "" {
		t.Fatalf("unknown part text = %q", got)
	}

	collector := newArtifactCollector()
	collector.AppendText("prefix:")
	progress := collector.AppendArtifact(protocol.Artifact{
		Parts: []protocol.Part{
			protocol.TextPart{Text: "visible"},
			protocol.TextPart{Text: "thought", Metadata: map[string]interface{}{"thought": "true"}},
			protocol.NewDataPart(map[string]interface{}{"k": "v"}),
		},
	})
	if progress != `visiblethought{"k":"v"}` {
		t.Fatalf("progress = %q", progress)
	}
	if collector.Text() != "prefix:visible" {
		t.Fatalf("collector text = %q", collector.Text())
	}

	name := "x"
	if ptrStringValue(&name) != "x" || ptrStringValue(nil) != "" {
		t.Fatal("ptrStringValue unexpected result")
	}
	if firstNonEmpty("", " ", "ok", "fallback") != "ok" || firstNonEmpty("", " ") != "" {
		t.Fatal("firstNonEmpty unexpected result")
	}
	if isThought(map[string]interface{}{"thought": true}) != true ||
		isThought(map[string]interface{}{"thought": 1}) != false {
		t.Fatal("isThought unexpected result")
	}
}
