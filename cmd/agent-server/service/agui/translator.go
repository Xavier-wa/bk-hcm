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

// Package agui ...
package agui

import (
	"context"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/translator"
)

type customTranslator struct {
	inner translator.Translator
}

// NewCustomTranslator 创建自定义翻译器
func NewCustomTranslator(inner translator.Translator) translator.Translator {
	return &customTranslator{inner: inner}
}

var _ translator.PostRunFinalizingTranslator = (*customTranslator)(nil)

// Translate event and add custom event
func (t *customTranslator) Translate(ctx context.Context, evt *event.Event) ([]aguievents.Event, error) {
	out, err := t.inner.Translate(ctx, evt)
	if err != nil {
		return nil, err
	}
	if payload := buildCustomPayload(evt); payload != nil {
		out = append(out, aguievents.NewCustomEvent("trace.metadata", aguievents.WithValue(payload)))
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

func buildCustomPayload(evt *event.Event) map[string]any {
	if evt == nil || evt.Response == nil {
		return nil
	}
	return map[string]any{
		"object":    evt.Response.Object,
		"timestamp": evt.Response.Timestamp,
	}
}
