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

package a2a

import (
	"context"
	"encoding/json"
	"strings"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-a2a-go/protocol"
	"trpc.group/trpc-go/trpc-a2a-go/taskmanager"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	agoa2a "trpc.group/trpc-go/trpc-agent-go/server/a2a"
)

// autoResumeProcessor wraps the A2A message processor so that MCP/OpenClaw
// continuations on an interrupted checkpoint behave like the AG-UI path:
// inject checkpoint_id + resume command, and normalize structured confirm
// payload into StateKeyForwardedResumeValue.
type autoResumeProcessor struct {
	next  taskmanager.MessageProcessor
	saver graph.CheckpointSaver
}

// ProcessMessage implements taskmanager.MessageProcessor.
func (p *autoResumeProcessor) ProcessMessage(
	ctx context.Context,
	message protocol.Message,
	options taskmanager.ProcessOptions,
	handler taskmanager.TaskHandler,
) (*taskmanager.MessageProcessingResult, error) {

	if p == nil || p.next == nil {
		return nil, nil
	}
	enrichA2AAutoResume(ctx, p.saver, &message)
	return p.next.ProcessMessage(ctx, message, options, handler)
}

// newAutoResumeHook returns a ProcessMessageHook that injects AG-UI-equivalent
// auto-resume RuntimeState keys into A2A message metadata before the runner runs.
func newAutoResumeHook(saver graph.CheckpointSaver) agoa2a.ProcessMessageHook {
	return func(next taskmanager.MessageProcessor) taskmanager.MessageProcessor {
		return &autoResumeProcessor{next: next, saver: saver}
	}
}

// enrichA2AAutoResume mutates message.Metadata in place when the context has an
// interrupted checkpoint. Safe to call with nil saver / nil ContextID.
func enrichA2AAutoResume(ctx context.Context, saver graph.CheckpointSaver, message *protocol.Message) {
	if message == nil || message.ContextID == nil {
		return
	}
	contextID := strings.TrimSpace(*message.ContextID)
	if contextID == "" {
		return
	}

	rid := rest.RidFromContext(ctx)
	if message.Metadata == nil {
		message.Metadata = make(map[string]any)
	}

	// Normalize MCP confirm / AG-UI-style resumeValue into the shared runtime key.
	normalizeForwardedResumeValue(message.Metadata, rid)

	// Always bind lineage to contextId so checkpoint lookup / resume are consistent
	// with AG-UI (lineageID = threadID = contextId).
	message.Metadata[graph.CfgKeyLineageID] = contextID

	if saver == nil {
		return
	}
	cm := graph.NewCheckpointManager(saver)
	tuple, err := cm.Latest(ctx, contextID, "")
	if err != nil {
		logs.Warnf("a2a auto-resume: get latest checkpoint failed, contextId=%s, err: %v, rid: %s",
			contextID, err, rid)
		return
	}
	if tuple == nil || tuple.Checkpoint == nil || !tuple.Checkpoint.IsInterrupted() {
		return
	}

	message.Metadata[graph.CfgKeyCheckpointID] = tuple.Checkpoint.ID

	// resume command 决定 graph.Interrupt 在中断节点的返回值。与 AG-UI tryPrepareAutoResume 对齐：
	// 优先把前端结构化 forwarded 值（tool.confirm 的 JSON 提单参数 / 选账号 id 字符串）作为 resume 命令。
	//
	// 子图循环节点（如 create_biz_apply 提单门禁）读不到父图 RuntimeState 里的
	// StateKeyForwardedResumeValue —— makeSubgraphInputMapper 在子图入口剥键，且子 agent 的
	// invocation 不携带父图 RuntimeState。因此结构化载荷只能经 resume command 带进
	// graph.Interrupt 的返回值，供 hitl 节点 resolveStructuredResumeValue 识别。
	resumeCmd := forwardedResumeValueString(message.Metadata)
	if resumeCmd == "" {
		resumeCmd = a2aMessageText(message)
	}
	if resumeCmd == "" {
		// Resume still needs a command to leave the interrupt; empty text is valid for
		// structured-only confirm (forwarded_resume_value already set).
		resumeCmd = "confirm"
	}
	message.Metadata[graph.StateKeyCommand] = graph.NewResumeCommand().WithResume(resumeCmd)
	logs.Infof("a2a auto-resume: interrupt checkpoint prepared, contextId=%s, checkpointId=%s, "+
		"has_forwarded=%t, rid: %s",
		contextID, tuple.Checkpoint.ID,
		message.Metadata[constant.StateKeyForwardedResumeValue] != nil, rid)
}

// normalizeForwardedResumeValue accepts either:
//   - forwarded_resume_value (already the runtime key)
//   - resumeValue (AG-UI forwardedProps key, string or object)
//
// and writes a JSON string into StateKeyForwardedResumeValue for HITL handlers.
func normalizeForwardedResumeValue(metadata map[string]any, rid string) {
	if metadata == nil {
		return
	}

	raw, ok := metadata[constant.StateKeyForwardedResumeValue]
	if !ok || raw == nil {
		raw, ok = metadata[constant.ForwardedPropResumeValue]
		if !ok || raw == nil {
			return
		}
	}

	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return
		}
		metadata[constant.StateKeyForwardedResumeValue] = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			logs.Warnf("a2a auto-resume: marshal forwarded resume value failed, err: %v, rid: %s", err, rid)
			return
		}
		metadata[constant.StateKeyForwardedResumeValue] = string(b)
	}
	// Avoid leaving the alternate key around so RuntimeState stays unambiguous.
	delete(metadata, constant.ForwardedPropResumeValue)
}

// forwardedResumeValueString 返回 metadata 中归一化后的 StateKeyForwardedResumeValue（字符串形态）。
// normalizeForwardedResumeValue 已保证该值要么是字符串、要么被 marshal 成 JSON 字符串；
// 非字符串或缺省时返回空串。
func forwardedResumeValueString(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	if s, ok := metadata[constant.StateKeyForwardedResumeValue].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func a2aMessageText(message *protocol.Message) string {
	if message == nil {
		return ""
	}
	var parts []string
	for _, part := range message.Parts {
		switch p := part.(type) {
		case *protocol.TextPart:
			if strings.TrimSpace(p.Text) != "" {
				parts = append(parts, p.Text)
			}
		case protocol.TextPart:
			if strings.TrimSpace(p.Text) != "" {
				parts = append(parts, p.Text)
			}
		}
	}
	return strings.Join(parts, "")
}
