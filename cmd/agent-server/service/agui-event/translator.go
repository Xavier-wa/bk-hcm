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

// buildInterruptCustomEvent extracts PregelStepMetadata from evt and emits a custom event whose
// type equals the interrupt base key (the portion before the first ":").
// Fallback interrupts are internal flow-control pauses and are excluded.
// Returns (nil, false) when the event carries no interrupt or is excluded.
func buildInterruptCustomEvent(ctx context.Context, evt *event.Event) (aguievents.Event, bool) {
	rid := rest.RidFromContext(ctx)
	meta, ok := extractPregelMeta(ctx, evt)
	if !ok {
		logs.Warnf("buildInterruptCustomEvent: failed to extract PregelStepMetadata, rid: %s", rid)
		return nil, false
	}
	baseKey, _, _ := strings.Cut(meta.InterruptKey, constant.InterruptKeySeparator)
	if baseKey == "" || baseKey == constant.FallbackInterruptKey {
		return nil, false
	}
	return aguievents.NewCustomEvent(baseKey, aguievents.WithValue(marshalInterruptPayload(ctx, meta))), true
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
