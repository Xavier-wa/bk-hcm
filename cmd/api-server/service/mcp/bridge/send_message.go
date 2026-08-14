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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hcm/cmd/api-server/service/mcp/ingress"
	mcpmetrics "hcm/cmd/api-server/service/mcp/metrics"
	"hcm/cmd/api-server/service/mcp/middleware"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"

	a2aclient "trpc.group/trpc-go/trpc-a2a-go/client"
	"trpc.group/trpc-go/trpc-a2a-go/protocol"
	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

const (
	metadataSourceKey       = "source"
	metadataSourceMCP       = "mcp"
	metadataBkBizIDKey      = "bk_biz_id"
	metadataModelNameKey    = "model_name"
	metadataMCPServerKey    = "mcp_server_name"
	resultMetaContextIDKey  = "contextId"
	resultMetaTaskIDKey     = "taskId"
	resultMetaEndpointKey   = "a2aEndpoint"
	cancelledResultMessage  = "任务已取消"
	failedResultFallback    = "agent task failed"
	completedResultFallback = "任务已完成"
)

// Handler 实现 ingress.BridgeHandler，把 MCP send_message 转换为 A2A message/stream。
type Handler struct {
	clients        a2aClientProvider
	taskMap        *ProgressTaskMap
	toolName       string
	progressSender progressSenderFunc
}

type progressSenderFunc func(ctx context.Context, progressToken interface{}, progress float64, message string)

type sendMessageStream struct {
	ctx      context.Context
	req      *ingress.SendMessageRequest
	cli      a2aStreamClient
	endpoint string
	headers  map[string]string
	user     string
}

// NewHandler 构造生产用 bridge handler。
func NewHandler(bridgeCfg cc.MCPBridgeSetting, ingressCfg cc.MCPIngressSetting,
	dis serviced.Discover) (*Handler, error) {

	provider, err := NewA2AClientProvider(bridgeCfg, dis)
	if err != nil {
		return nil, err
	}
	return newHandlerWithProvider(ingressCfg, provider), nil
}

func newHandlerWithProvider(ingressCfg cc.MCPIngressSetting, provider a2aClientProvider) *Handler {
	toolName := ingressCfg.AggregatedToolName
	if strings.TrimSpace(toolName) == "" {
		toolName = constant.MCPIngressAggregatedToolName
	}
	return &Handler{
		clients:        provider,
		taskMap:        NewProgressTaskMap(ingressCfg.ProgressTaskMapMaxSize, ingressCfg.ProgressTaskMapEntryTTL),
		toolName:       toolName,
		progressSender: sendProgress,
	}
}

// SendMessage 实现 ingress.BridgeHandler。
func (h *Handler) SendMessage(ctx context.Context, req *ingress.SendMessageRequest, _ *mcpsdk.Server) (
	*mcpsdk.CallToolResult, error) {

	start := time.Now()
	status := mcpmetrics.StatusError
	mcpServerName := ""
	if req != nil {
		mcpServerName = req.MCPServerName
	}
	defer func() {
		mcpmetrics.IncToolsCall(h.metricToolName(), mcpServerName, status)
		mcpmetrics.ObserveToolsCallDuration(h.metricToolName(), mcpServerName, time.Since(start).Seconds())
	}()

	if h == nil || h.clients == nil {
		status = mcpmetrics.StatusError
		return nil, fmt.Errorf("bridge: handler is not initialized")
	}
	if h.progressSender == nil {
		h.progressSender = sendProgress
	}
	if req == nil || strings.TrimSpace(req.Text) == "" {
		status = mcpmetrics.StatusInvalidArg
		return nil, fmt.Errorf("bridge: text is required")
	}

	kt, rid, user := kitFromCtx(ctx)
	logs.Infof("bridge: tools/call send_message received, user=%s, mcp_server_name=%s, "+
		"text_len=%d, contextId=%q, has_progress_token=%t, rid: %s",
		user, req.MCPServerName, len(req.Text), req.ContextID, req.ProgressToken != nil, rid)

	cli, endpoint, err := h.clients.NewClient(ctx)
	if err != nil {
		logs.Errorf("bridge: create a2a client failed, err: %v, rid: %s", err, rid)
		return nil, err
	}

	params := buildSendMessageParams(req)
	headers := requestHeaders(kt)
	events, err := cli.StreamMessage(ctx, params, a2aclient.WithRequestHeaders(headers))
	if err != nil {
		logs.Errorf("bridge: stream message to agent-server failed, endpoint=%s, err: %v, rid: %s",
			endpoint, err, rid)
		return nil, err
	}

	stream := sendMessageStream{
		ctx:      ctx,
		req:      req,
		cli:      cli,
		endpoint: endpoint,
		headers:  headers,
		user:     user,
	}
	result, resultStatus, err := h.handleStreamEvents(stream, events)
	status = resultStatus
	return result, err
}

func (h *Handler) handleStreamEvents(stream sendMessageStream,
	events <-chan protocol.StreamingMessageEvent) (*mcpsdk.CallToolResult, string, error) {

	collector := newArtifactCollector()
	var taskID string
	var contextID string
	var pending *pendingConfirm
	defer func() {
		if stream.req.ProgressToken != nil {
			h.taskMap.Delete(stream.req.ProgressToken)
		}
	}()

	for event := range events {
		if event.Result == nil {
			continue
		}
		switch result := event.Result.(type) {
		case *protocol.Task:
			taskID, contextID = h.trackTask(stream.ctx, stream.req.ProgressToken, taskID, contextID, result.ID,
				result.ContextID)
			pending = collectPendingConfirm(pending, result.Metadata)
			pending = collectPendingConfirm(pending, metadataFromMessage(result.Status.Message))
			h.progressSender(stream.ctx, stream.req.ProgressToken, progressForState(result.Status.State),
				statusMessage(result.Status.Message, string(result.Status.State)))
		case *protocol.Message:
			contextID = firstNonEmpty(contextID, ptrStringValue(result.ContextID))
			pending = collectPendingConfirm(pending, metadataFromMessage(result))
			msg := messageText(result)
			if msg != "" {
				collector.AppendText(msg)
				h.progressSender(stream.ctx, stream.req.ProgressToken, 0.9, msg)
			}
		case *protocol.TaskStatusUpdateEvent:
			pending = collectPendingConfirm(pending, metadataFromStatusUpdate(result))
			finalResult, finalStatus, ok := h.handleStatusUpdate(stream, result, collector, pending, &taskID,
				&contextID)
			if ok {
				return finalResult, finalStatus, nil
			}
		case *protocol.TaskArtifactUpdateEvent:
			taskID, contextID = h.trackTask(stream.ctx, stream.req.ProgressToken, taskID, contextID, result.TaskID,
				result.ContextID)
			pending = collectPendingConfirm(pending, metadataFromArtifact(result.Artifact))
			pending = collectPendingConfirm(pending, result.Metadata)
			msg := collector.AppendArtifact(result.Artifact)
			if msg != "" {
				h.progressSender(stream.ctx, stream.req.ProgressToken, 0.8, msg)
			}
		default:
			_, rid, _ := kitFromCtx(stream.ctx)
			logs.Warnf("bridge: skip unsupported a2a stream event type %T, rid: %s", result, rid)
		}
	}

	return h.handleStreamEnd(stream, taskID, contextID, pending)
}

func (h *Handler) handleStatusUpdate(stream sendMessageStream, result *protocol.TaskStatusUpdateEvent,
	collector *artifactCollector, pending *pendingConfirm, taskID, contextID *string) (
	*mcpsdk.CallToolResult, string, bool) {

	*taskID, *contextID = h.trackTask(stream.ctx, stream.req.ProgressToken, *taskID, *contextID, result.TaskID,
		result.ContextID)
	msg := statusMessage(result.Status.Message, string(result.Status.State))
	h.progressSender(stream.ctx, stream.req.ProgressToken, progressForState(result.Status.State), msg)
	if !result.Final {
		return nil, "", false
	}

	_, rid, _ := kitFromCtx(stream.ctx)
	final := h.finalResult(result.Status.State, collector.Text(), msg, *contextID, *taskID, stream.endpoint, pending)
	logs.Infof("bridge: a2a task reached final state, state=%s, user=%s, mcp_server_name=%s, "+
		"progressToken=%v, taskId=%s, contextId=%s, has_confirm=%t, rid: %s",
		result.Status.State, stream.user, stream.req.MCPServerName, stream.req.ProgressToken, *taskID, *contextID,
		pending != nil, rid)
	return final, statusFromTaskState(result.Status.State), true
}

func (h *Handler) handleStreamEnd(stream sendMessageStream, taskID, contextID string, pending *pendingConfirm) (
	*mcpsdk.CallToolResult, string, error) {

	if err := stream.ctx.Err(); err != nil {
		h.cancelTask(stream.ctx, stream.cli, taskID, stream.headers)
		return errorResult(cancelledResultMessage, contextID, taskID, stream.endpoint, nil),
			mcpmetrics.StatusCanceled, nil
	}

	_, rid, _ := kitFromCtx(stream.ctx)
	logs.Errorf("bridge: a2a stream ended before final event, user=%s, mcp_server_name=%s, "+
		"progressToken=%v, taskId=%s, contextId=%s, rid: %s",
		stream.user, stream.req.MCPServerName, stream.req.ProgressToken, taskID, contextID, rid)
	return nil, mcpmetrics.StatusError, fmt.Errorf("bridge: a2a stream ended before final event")
}

// Cancel 实现 ingress.BridgeHandler。
func (h *Handler) Cancel(ctx context.Context, progressToken interface{}) {
	h.CancelByProgressToken(ctx, progressToken)
}

// CancelByProgressToken 按 MCP progressToken 查找 A2A taskId 并 best-effort 取消。
func (h *Handler) CancelByProgressToken(ctx context.Context, progressToken interface{}) {
	kt, rid, user := kitFromCtx(ctx)
	if h == nil || h.taskMap == nil {
		logs.Warnf("bridge: cancel ignored because handler is not initialized, progressToken=%v, rid: %s",
			progressToken, rid)
		return
	}

	taskID, ok := h.taskMap.Get(progressToken)
	if !ok {
		logs.Warnf("bridge: cancel ignored because progressToken is unknown, user=%s, progressToken=%v, rid: %s",
			user, progressToken, rid)
		return
	}
	if h.clients == nil {
		logs.Warnf("bridge: cancel ignored because a2a client provider is nil, progressToken=%v, taskId=%s, rid: %s",
			progressToken, taskID, rid)
		return
	}

	cli, endpoint, err := h.clients.NewClient(ctx)
	if err != nil {
		logs.Errorf("bridge: create a2a client for cancel failed, err: %v, rid: %s", err, rid)
		return
	}
	if h.cancelTask(ctx, cli, taskID, requestHeaders(kt)) {
		h.taskMap.Delete(progressToken)
		logs.Infof("bridge: cancel a2a task success, user=%s, progressToken=%v, taskId=%s, endpoint=%s, rid: %s",
			user, progressToken, taskID, endpoint, rid)
	}
}

func (h *Handler) trackTask(ctx context.Context, progressToken interface{}, currentTaskID, currentContextID,
	taskID, contextID string) (string, string) {

	if taskID != "" && currentTaskID == "" {
		currentTaskID = taskID
		if progressToken != nil {
			_, rid, _ := kitFromCtx(ctx)
			h.taskMap.Set(progressToken, taskID)
			logs.Infof("bridge: a2a task mapped, progressToken=%v, taskId=%s, rid: %s",
				progressToken, taskID, rid)
		}
	}
	if contextID != "" && currentContextID == "" {
		currentContextID = contextID
	}
	return currentTaskID, currentContextID
}

func (h *Handler) cancelTask(ctx context.Context, cli a2aStreamClient, taskID string,
	headers map[string]string) bool {

	if cli == nil || taskID == "" {
		return false
	}
	_, err := cli.CancelTasks(ctx, protocol.TaskIDParams{
		RPCID: protocol.GenerateRPCID(),
		ID:    taskID,
	}, a2aclient.WithRequestHeaders(headers))
	if err != nil {
		_, rid, _ := kitFromCtx(ctx)
		logs.Errorf("bridge: cancel a2a task failed, taskId=%s, err: %v, rid: %s", taskID, err, rid)
		return false
	}
	return true
}

func (h *Handler) finalResult(state protocol.TaskState, text, statusText, contextID, taskID,
	endpoint string, pending *pendingConfirm) *mcpsdk.CallToolResult {

	switch state {
	case protocol.TaskStateCompleted, protocol.TaskStateInputRequired:
		if strings.TrimSpace(text) == "" {
			text = statusText
		}
		if pending != nil {
			text = formatConfirmContent(pending, text)
		} else if strings.TrimSpace(text) == "" {
			text = completedResultFallback
		}
		return textResult(text, contextID, taskID, endpoint, pending)
	case protocol.TaskStateCanceled:
		return errorResult(cancelledResultMessage, contextID, taskID, endpoint, nil)
	case protocol.TaskStateFailed, protocol.TaskStateRejected, protocol.TaskStateAuthRequired:
		if strings.TrimSpace(statusText) == "" || statusText == string(state) {
			statusText = failedResultFallback
		}
		return errorResult(statusText, contextID, taskID, endpoint, nil)
	default:
		if pending != nil {
			text = formatConfirmContent(pending, firstNonEmpty(text, statusText))
		}
		return textResult(firstNonEmpty(text, statusText), contextID, taskID, endpoint, pending)
	}
}

func (h *Handler) metricToolName() string {
	if h == nil || h.toolName == "" {
		return constant.MCPIngressAggregatedToolName
	}
	return h.toolName
}

func buildSendMessageParams(req *ingress.SendMessageRequest) protocol.SendMessageParams {
	metadata := map[string]interface{}{
		metadataSourceKey: metadataSourceMCP,
	}
	if req.BkBizID > 0 {
		metadata[metadataBkBizIDKey] = req.BkBizID
	}
	if strings.TrimSpace(req.ModelName) != "" {
		metadata[metadataModelNameKey] = req.ModelName
	}
	if strings.TrimSpace(req.MCPServerName) != "" {
		metadata[metadataMCPServerKey] = req.MCPServerName
	}
	// Align with AG-UI forwardedProps.resumeValue so agent-server HITL can resume
	// create_biz_apply / other tool.confirm gates from OpenClaw.
	if resume, ok := buildConfirmResumeValue(req.Confirm); ok {
		metadata[constant.StateKeyForwardedResumeValue] = resume
		metadata[constant.ForwardedPropResumeValue] = resume
	}

	var contextID *string
	if strings.TrimSpace(req.ContextID) != "" {
		contextID = &req.ContextID
	}
	message := protocol.NewMessageWithContext(
		protocol.MessageRoleUser,
		[]protocol.Part{protocol.NewTextPart(req.Text)},
		nil,
		contextID,
	)
	message.Metadata = metadata

	return protocol.SendMessageParams{
		RPCID:    protocol.GenerateRPCID(),
		Message:  message,
		Metadata: metadata,
	}
}

func requestHeaders(kt *kit.Kit) map[string]string {
	h := http.Header{}
	middleware.InjectInternalHeaders(kt, h, string(cc.APIServerName))

	headers := make(map[string]string, len(h))
	for key, values := range h {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return headers
}

func kitFromCtx(ctx context.Context) (*kit.Kit, string, string) {
	kt, ok := middleware.KitFromCtx(ctx)
	if !ok || kt == nil {
		return nil, unknownRid, ""
	}
	rid := kt.Rid
	if strings.TrimSpace(rid) == "" {
		rid = unknownRid
	}
	return kt, rid, kt.User
}

func sendProgress(ctx context.Context, progressToken interface{}, progress float64, message string) {
	if progressToken == nil || strings.TrimSpace(message) == "" {
		return
	}
	sender, ok := mcpsdk.GetNotificationSender(ctx)
	if !ok {
		return
	}
	if err := sender.SendProgress(progress, message); err != nil {
		_, rid, _ := kitFromCtx(ctx)
		logs.Warnf("bridge: send mcp progress notification failed, err: %v, rid: %s", err, rid)
	}
}

func progressForState(state protocol.TaskState) float64 {
	switch state {
	case protocol.TaskStateSubmitted:
		return 0.1
	case protocol.TaskStateWorking, protocol.TaskStateInputRequired:
		return 0.5
	case protocol.TaskStateCompleted, protocol.TaskStateCanceled, protocol.TaskStateFailed,
		protocol.TaskStateRejected, protocol.TaskStateAuthRequired:
		return 1
	default:
		return 0.5
	}
}

func statusFromTaskState(state protocol.TaskState) string {
	switch state {
	case protocol.TaskStateCompleted:
		return mcpmetrics.StatusSuccess
	case protocol.TaskStateCanceled:
		return mcpmetrics.StatusCanceled
	case protocol.TaskStateFailed, protocol.TaskStateRejected, protocol.TaskStateAuthRequired:
		return mcpmetrics.StatusError
	default:
		return mcpmetrics.StatusSuccess
	}
}

func textResult(text, contextID, taskID, endpoint string, pending *pendingConfirm) *mcpsdk.CallToolResult {
	result := mcpsdk.NewTextResult(text)
	result.Meta = resultMeta(contextID, taskID, endpoint, pending)
	return result
}

func errorResult(text, contextID, taskID, endpoint string, pending *pendingConfirm) *mcpsdk.CallToolResult {
	result := mcpsdk.NewErrorResult(text)
	result.Meta = resultMeta(contextID, taskID, endpoint, pending)
	return result
}

func resultMeta(contextID, taskID, endpoint string, pending *pendingConfirm) map[string]interface{} {
	meta := make(map[string]interface{})
	if contextID != "" {
		meta[resultMetaContextIDKey] = contextID
	}
	if taskID != "" {
		meta[resultMetaTaskIDKey] = taskID
	}
	if endpoint != "" {
		meta[resultMetaEndpointKey] = endpoint
	}
	return mergeConfirmIntoMeta(meta, pending)
}

func statusMessage(msg *protocol.Message, fallback string) string {
	text := messageText(msg)
	if strings.TrimSpace(text) == "" {
		return fallback
	}
	return text
}

func messageText(msg *protocol.Message) string {
	if msg == nil {
		return ""
	}

	var parts []string
	for _, part := range msg.Parts {
		text := partText(part)
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "")
}

func partText(part protocol.Part) string {
	switch p := part.(type) {
	case *protocol.TextPart:
		return p.Text
	case protocol.TextPart:
		return p.Text
	case *protocol.DataPart:
		return jsonString(p.Data)
	case protocol.DataPart:
		return jsonString(p.Data)
	case *protocol.FilePart:
		return filePartText(*p)
	case protocol.FilePart:
		return filePartText(p)
	default:
		return ""
	}
}

func jsonString(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func filePartText(p protocol.FilePart) string {
	switch f := p.File.(type) {
	case *protocol.FileWithBytes:
		name := ptrStringValue(f.Name)
		if name == "" {
			name = "embedded-file"
		}
		return fmt.Sprintf("[file:%s]", name)
	case *protocol.FileWithURI:
		name := ptrStringValue(f.Name)
		if name == "" {
			name = f.URI
		}
		return fmt.Sprintf("[file:%s]", name)
	default:
		return "[file]"
	}
}

type artifactCollector struct {
	builder strings.Builder
}

func newArtifactCollector() *artifactCollector {
	return &artifactCollector{}
}

func (c *artifactCollector) AppendText(text string) {
	c.builder.WriteString(text)
}

func (c *artifactCollector) AppendArtifact(artifact protocol.Artifact) string {
	var progressParts []string
	for _, part := range artifact.Parts {
		switch p := part.(type) {
		case *protocol.TextPart:
			progressParts = append(progressParts, p.Text)
			if !isThought(p.Metadata) {
				c.builder.WriteString(p.Text)
			}
		case protocol.TextPart:
			progressParts = append(progressParts, p.Text)
			if !isThought(p.Metadata) {
				c.builder.WriteString(p.Text)
			}
		default:
			text := partText(part)
			if strings.TrimSpace(text) != "" {
				progressParts = append(progressParts, text)
			}
		}
	}
	return strings.Join(progressParts, "")
}

func (c *artifactCollector) Text() string {
	return c.builder.String()
}

func isThought(metadata map[string]interface{}) bool {
	v, ok := metadata["thought"]
	if !ok {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.EqualFold(val, "true")
	default:
		return false
	}
}

func ptrStringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
