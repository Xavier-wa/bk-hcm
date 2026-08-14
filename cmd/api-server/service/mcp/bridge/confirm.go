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

package bridge

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"hcm/cmd/api-server/service/mcp/ingress"
	"hcm/pkg/criteria/constant"

	"trpc.group/trpc-go/trpc-a2a-go/protocol"
	"trpc.group/trpc-go/trpc-agent-go/graph"
)

const (
	stateDeltaEnvelopeEncodingKey   = "encoding"
	stateDeltaEnvelopePayloadKey    = "payload"
	stateDeltaEnvelopeEncodingBytes = "bytes"
	stateDeltaEnvelopeEncodingNil   = "nil"
)

// pendingConfirm is the structured HITL confirm payload exposed to OpenClaw via
// CallToolResult._meta.confirm. It mirrors the AG-UI tool.confirm event value.
type pendingConfirm struct {
	Kind         string         `json:"kind"`
	Tool         string         `json:"tool,omitempty"`
	InterruptKey string         `json:"interrupt_key,omitempty"`
	CheckpointID string         `json:"checkpoint_id,omitempty"`
	LineageID    string         `json:"lineage_id,omitempty"`
	Data         map[string]any `json:"data,omitempty"`
	Value        any            `json:"value,omitempty"`
}

// buildConfirmResumeValue maps MCP confirm → AG-UI resumeValue JSON string.
//
//   - action=confirm → JSON(args)（args 为空时为 "{}"，门禁回退到原始 tool_call 入参）
//   - action=cancel  → 不设置 forwarded resume（由 HITL 节点合成 CancelActionSignal）
func buildConfirmResumeValue(confirm *ingress.ConfirmRequest) (string, bool) {
	if confirm == nil {
		return "", false
	}
	switch confirm.Action {
	case constant.MCPConfirmActionConfirm:
		if len(confirm.Args) == 0 {
			return "{}", true
		}
		b, err := json.Marshal(confirm.Args)
		if err != nil {
			return "{}", true
		}
		return string(b), true
	default:
		// cancel: omit forwarded resume value
		return "", false
	}
}

// extractPendingConfirmFromMetadata looks for graph Pregel interrupt metadata in
// A2A message/artifact metadata.state_delta and builds a pendingConfirm when the
// interrupt is a tool confirm / HITL kind that OpenClaw must resume structurally.
func extractPendingConfirmFromMetadata(metadata map[string]any) *pendingConfirm {
	if len(metadata) == 0 {
		return nil
	}
	rawDelta, ok := metadata["state_delta"]
	if !ok || rawDelta == nil {
		return nil
	}
	deltaMap, ok := decodeStateDelta(rawDelta)
	if !ok {
		return nil
	}
	rawPregel, ok := deltaMap[graph.MetadataKeyPregel]
	if !ok || len(rawPregel) == 0 {
		return nil
	}
	var meta graph.PregelStepMetadata
	if err := json.Unmarshal(rawPregel, &meta); err != nil {
		return nil
	}
	if meta.InterruptKey == "" {
		return nil
	}
	return pendingConfirmFromPregel(meta)
}

// afterToolHITLInterruptKeyPrefix 是 after_tool_hitl 场景（推荐选择 / 拆单确认）中断 key 的公共前缀。
const afterToolHITLInterruptKeyPrefix = "after_tool_hitl."

func pendingConfirmFromPregel(meta graph.PregelStepMetadata) *pendingConfirm {
	key := meta.InterruptKey
	pc := &pendingConfirm{
		InterruptKey: key,
		CheckpointID: meta.CheckpointID,
		LineageID:    meta.LineageID,
		Value:        meta.InterruptValue,
	}
	// 所有中断都尽量还原 payload 到 Data：tool.confirm 取内层 data（供 confirm.args 回传），
	// 其它中断（选账号 / 推荐 / 拆单）取整个 payload（供向用户展示可选项）。
	if data := confirmDataFromInterruptValue(meta.InterruptValue); data != nil {
		pc.Data = data
	}

	switch {
	case isToolConfirmInterruptKey(key):
		pc.Kind = constant.ToolConfirmCustomEventName
		pc.Tool = toolFromToolConfirmInterrupt(meta)
	case isHITLInterruptKey(key):
		pc.Kind = constant.HITLInterruptKey
	case strings.HasPrefix(key, constant.AccountSelectInterruptKey):
		pc.Kind = constant.AccountSelectInterruptKey
	case strings.HasPrefix(key, afterToolHITLInterruptKeyPrefix):
		pc.Kind = key
	default:
		// fallback / unknown interrupts: still surface for OpenClaw awareness
		pc.Kind = key
	}
	return pc
}

func confirmDataFromInterruptValue(v any) map[string]any {
	switch raw := v.(type) {
	case map[string]any:
		if data, ok := raw["data"].(map[string]any); ok {
			return data
		}
		return raw
	case string:
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return nil
		}
		if data, ok := m["data"].(map[string]any); ok {
			return data
		}
		return m
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return nil
		}
		if data, ok := m["data"].(map[string]any); ok {
			return data
		}
		return m
	}
}

func isToolConfirmInterruptKey(key string) bool {
	return strings.HasPrefix(key, constant.ToolConfirmInterruptKeyPrefix)
}

func isHITLInterruptKey(key string) bool {
	return key == constant.HITLInterruptKey ||
		strings.HasPrefix(key, constant.HITLInterruptKey+constant.InterruptKeySeparator)
}

// toolFromToolConfirmInterrupt 优先取 payload.tool；否则从 interrupt key 解析。
func toolFromToolConfirmInterrupt(meta graph.PregelStepMetadata) string {
	if tool := toolNameFromInterruptValue(meta.InterruptValue); tool != "" {
		return tool
	}
	return toolFromInterruptKey(meta.InterruptKey)
}

func toolNameFromInterruptValue(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	tool, _ := m["tool"].(string)
	return strings.TrimSpace(tool)
}

func toolFromInterruptKey(key string) string {
	parts := strings.Split(key, constant.InterruptKeySeparator)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// decodeStateDelta restores Event.StateDelta from A2A metadata.
// Preferred wire format is the trpc-agent-go envelope:
//
//	{ "<key>": { "encoding": "bytes", "payload": "<base64>" } }
//
// Loose fallbacks keep unit tests and older peers working.
func decodeStateDelta(raw any) (map[string][]byte, bool) {
	switch v := raw.(type) {
	case map[string][]byte:
		if len(v) == 0 {
			return nil, false
		}
		return v, true
	case map[string]any:
		out := make(map[string][]byte, len(v))
		for k, val := range v {
			if entry, ok := val.(map[string]any); ok {
				if bytes, ok := decodeStateDeltaEnvelope(entry); ok {
					out[k] = bytes
					continue
				}
			}
			switch x := val.(type) {
			case []byte:
				out[k] = x
			case string:
				out[k] = []byte(x)
			default:
				b, err := json.Marshal(x)
				if err != nil {
					continue
				}
				out[k] = b
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	case string:
		if v == "" {
			return nil, false
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return nil, false
		}
		return decodeStateDelta(m)
	default:
		return nil, false
	}
}

func decodeStateDeltaEnvelope(entry map[string]any) ([]byte, bool) {
	encoding, ok := entry[stateDeltaEnvelopeEncodingKey].(string)
	if !ok || encoding == "" {
		return nil, false
	}
	switch encoding {
	case stateDeltaEnvelopeEncodingNil:
		return nil, true
	case stateDeltaEnvelopeEncodingBytes:
		payload, ok := entry[stateDeltaEnvelopePayloadKey].(string)
		if !ok {
			return nil, false
		}
		raw, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, false
		}
		return raw, true
	default:
		return nil, false
	}
}

func formatConfirmContent(pc *pendingConfirm, existingText string) string {
	if pc == nil {
		return existingText
	}
	var b strings.Builder
	if strings.TrimSpace(existingText) != "" {
		b.WriteString(existingText)
		b.WriteString("\n\n")
	}

	// 按中断类型给出不同的恢复指引：
	//   - tool.confirm（有副作用的最终提单确认）：必须回传结构化 confirm 对象；
	//   - 其它中断（选账号 / 推荐 / 拆单）：向用户展示可选项，用同一 contextId 纯文本续聊即可。
	isToolConfirm := pc.Kind == constant.ToolConfirmCustomEventName
	if isToolConfirm {
		b.WriteString(constant.MCPConfirmPendingMessage)
	} else {
		b.WriteString(constant.MCPSelectPendingMessage)
	}
	if pc.Tool != "" {
		b.WriteString(fmt.Sprintf("\n待确认工具：%s", pc.Tool))
	}
	if pc.Kind != "" {
		b.WriteString(fmt.Sprintf("\n中断类型：%s", pc.Kind))
	}
	if payload := confirmPayloadForContent(pc); payload != nil {
		if raw, err := json.MarshalIndent(payload, "", "  "); err == nil {
			if isToolConfirm {
				b.WriteString("\n确认参数（请在 confirm.args 中回传）：\n")
			} else {
				b.WriteString("\n可选项/待确认信息（请向用户展示并让其选择，然后用同一 contextId 纯文本回复）：\n")
			}
			b.WriteString(string(raw))
		}
	}
	return b.String()
}

// confirmPayloadForContent 返回渲染进文本内容的 payload：优先结构化 Data，退回原始 Value。
func confirmPayloadForContent(pc *pendingConfirm) any {
	if len(pc.Data) > 0 {
		return pc.Data
	}
	if pc.Value != nil {
		return pc.Value
	}
	return nil
}

func mergeConfirmIntoMeta(meta map[string]interface{}, pc *pendingConfirm) map[string]interface{} {
	if pc == nil {
		return meta
	}
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta[constant.MCPResultMetaConfirmKey] = pc
	return meta
}

func collectPendingConfirm(current *pendingConfirm, metadata map[string]any) *pendingConfirm {
	if pc := extractPendingConfirmFromMetadata(metadata); pc != nil {
		return pc
	}
	return current
}

func metadataFromMessage(msg *protocol.Message) map[string]any {
	if msg == nil {
		return nil
	}
	return msg.Metadata
}

func metadataFromArtifact(artifact protocol.Artifact) map[string]any {
	return artifact.Metadata
}

func metadataFromStatusUpdate(evt *protocol.TaskStatusUpdateEvent) map[string]any {
	if evt == nil {
		return nil
	}
	if evt.Metadata != nil {
		return evt.Metadata
	}
	if evt.Status.Message != nil {
		return evt.Status.Message.Metadata
	}
	return nil
}
