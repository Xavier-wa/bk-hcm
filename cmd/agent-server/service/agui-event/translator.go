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
	if hitlPayload := buildHITLPayload(evt); hitlPayload != "" {
		out = append(out, aguievents.NewCustomEvent("hitl.interrupt", aguievents.WithValue(hitlPayload)))
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

func buildHITLPayload(evt *event.Event) string {
	if evt == nil || evt.StateDelta == nil {
		return ""
	}
	raw, ok := evt.StateDelta[graph.MetadataKeyPregel]
	if !ok || len(raw) == 0 {
		return ""
	}
	var meta graph.PregelStepMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return ""
	}
	if !isHITLInterruptKey(meta.InterruptKey) {
		return ""
	}
	payload := map[string]any{
		"value":         meta.InterruptValue,
		"checkpoint_id": meta.CheckpointID,
		"lineage_id":    meta.LineageID,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(b)
}

func isHITLInterruptKey(key string) bool {
	return key == constant.HITLInterruptKey ||
		strings.HasPrefix(key, constant.HITLInterruptKey+constant.InterruptKeySeparator)
}
