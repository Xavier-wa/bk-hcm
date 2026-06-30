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

// Package aguievent ...
package aguievent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/adapter"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/translator"
)

type customTranslator struct {
	inner translator.Translator
}

// NewCustomTranslator 创建自定义翻译器
func NewCustomTranslator(ctx context.Context, input *adapter.RunAgentInput, opts ...translator.Option) (
	translator.Translator, error) {

	inner, err := translator.New(ctx, input.ThreadID, input.RunID, opts...)
	if err != nil {
		return nil, fmt.Errorf("create inner translator: %w", err)
	}
	return &customTranslator{inner: inner}, nil
}

var _ translator.PostRunFinalizingTranslator = (*customTranslator)(nil)

// Translate event and add custom event
func (t *customTranslator) Translate(ctx context.Context, evt *event.Event) ([]aguievents.Event, error) {
	out, err := t.inner.Translate(ctx, evt)
	if err != nil {
		return nil, err
	}
	if customEvt, ok := buildInterruptCustomEvent(ctx, evt); ok {
		out = append(out, customEvt)
	}
	return out, nil
}

// PostRunFinalizationEvents call the inner translator's PostRunFinalizationEvents
func (t *customTranslator) PostRunFinalizationEvents(ctx context.Context) ([]aguievents.Event, error) {
	finalizer, ok := t.inner.(translator.PostRunFinalizingTranslator)
	if !ok {
		return nil, nil
	}
	return finalizer.PostRunFinalizationEvents(ctx)
}

// buildInterruptCustomEvent 从 evt 提取 PregelStepMetadata，并封装为 CUSTOM 事件；
// 事件类型取 interrupt key 的第一段（第一个 ":" 之前）。
// 若该中断不应面向用户展示，返回 (nil, false)。
func buildInterruptCustomEvent(ctx context.Context, evt *event.Event) (aguievents.Event, bool) {
	rid := rest.RidFromContext(ctx)
	meta, ok := extractPregelMeta(ctx, evt)
	if !ok {
		logs.Warnf("buildInterruptCustomEvent: failed to extract PregelStepMetadata, rid: %s", rid)
		return nil, false
	}
	name, ok := resolveInterruptCustomEventName(meta)
	if !ok {
		return nil, false
	}
	return aguievents.NewCustomEvent(name, aguievents.WithValue(marshalInterruptPayload(ctx, meta))), true
}

// resolveInterruptCustomEventName 判断 Pregel 中断是否应转为 AG-UI CUSTOM 事件，
// 若需要则返回事件名（interrupt key 的第一段）。
//
// 子图 agent 节点（host_apply、resource_query）会在内层节点中断后再次上报同一条中断；
// 这类传播副本应跳过，仅保留内层节点发出的 CUSTOM 事件。
func resolveInterruptCustomEventName(meta graph.PregelStepMetadata) (string, bool) {
	if enumor.IsSubgraphAgentNode(meta.NodeID) {
		return "", false
	}
	baseKey, _, _ := strings.Cut(meta.InterruptKey, constant.InterruptKeySeparator)
	if baseKey == "" || baseKey == constant.FallbackInterruptKey {
		return "", false
	}
	return baseKey, true
}

// extractPregelMeta parses the PregelStepMetadata embedded in evt.StateDelta.
// Returns (meta, true) on success, (zero, false) otherwise.
func extractPregelMeta(ctx context.Context, evt *event.Event) (graph.PregelStepMetadata, bool) {
	rid := rest.RidFromContext(ctx)
	if evt == nil || evt.StateDelta == nil {
		logs.Warnf("extractPregelMeta: evt is nil or evt.StateDelta is nil, rid: %s", rid)
		return graph.PregelStepMetadata{}, false
	}
	raw, ok := evt.StateDelta[graph.MetadataKeyPregel]
	if !ok || len(raw) == 0 {
		logs.Warnf("extractPregelMeta: evt.StateDelta[graph.MetadataKeyPregel] is nil or empty, rid: %s", rid)
		return graph.PregelStepMetadata{}, false
	}
	var meta graph.PregelStepMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		logs.Errorf("extractPregelMeta: failed to unmarshal PregelStepMetadata, err: %v, rid: %s", err, rid)
		return graph.PregelStepMetadata{}, false
	}
	return meta, true
}

// marshalInterruptPayload serializes the interrupt-relevant fields of meta to a JSON string.
// Returns an empty string if marshalling fails.
func marshalInterruptPayload(ctx context.Context, meta graph.PregelStepMetadata) string {
	rid := rest.RidFromContext(ctx)
	b, err := json.Marshal(map[string]any{
		"value":         meta.InterruptValue,
		"checkpoint_id": meta.CheckpointID,
		"lineage_id":    meta.LineageID,
	})
	if err != nil {
		logs.Errorf("marshalInterruptPayload: failed to marshal PregelStepMetadata, err: %v, rid: %s", err, rid)
		return ""
	}

	logs.Infof("marshalInterruptPayload: marshal PregelStepMetadata success, payload: %s, rid: %s",
		string(b), rid)
	return string(b)
}

func isHITLInterruptKey(key string) bool {
	return key == constant.HITLInterruptKey ||
		strings.HasPrefix(key, constant.HITLInterruptKey+constant.InterruptKeySeparator)
}

// buildToolConfirmEvent 当事件携带工具确认门禁中断（key 以 "tool_confirm:" 为前缀）时，
// 构造工具确认自定义事件的名称与 payload，否则返回 ("", "")。事件名按工具维度命名
// （tool.confirm.<tool>），便于前端路由到对应工具的确认卡片；payload 只携带门禁的结构化 data 与 actions。
func buildToolConfirmEvent(ctx context.Context, evt *event.Event) (string, string) {
	rid := rest.RidFromContext(ctx)
	if evt == nil || evt.StateDelta == nil {
		return "", ""
	}
	raw, ok := evt.StateDelta[graph.MetadataKeyPregel]
	if !ok || len(raw) == 0 {
		return "", ""
	}
	var meta graph.PregelStepMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		logs.Errorf("unmarshal tool confirm metadata: %v, rid: %s", err, rid)
		return "", ""
	}
	if !isToolConfirmInterruptKey(meta.InterruptKey) {
		return "", ""
	}
	payload := map[string]any{
		"value":         meta.InterruptValue,
		"checkpoint_id": meta.CheckpointID,
		"lineage_id":    meta.LineageID,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", ""
	}
	name := constant.ToolConfirmCustomEventName
	if tool := toolFromInterruptKey(meta.InterruptKey); tool != "" {
		name = name + "." + tool
	}
	return name, string(b)
}

func isToolConfirmInterruptKey(key string) bool {
	return key == constant.ToolConfirmInterruptKey ||
		strings.HasPrefix(key, constant.ToolConfirmInterruptKey+constant.InterruptKeySeparator)
}

// toolFromInterruptKey 从工具确认中断 key "tool_confirm:<tool>:<callID>" 中提取工具名（第 2 段）。
// 缺少工具段时返回 ""。
func toolFromInterruptKey(key string) string {
	parts := strings.Split(key, constant.InterruptKeySeparator)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}
