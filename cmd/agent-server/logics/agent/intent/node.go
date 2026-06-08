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

// Package intent provides the intent recognition node for the GraphAgent.
// The node runs a lightweight LLM call before the main ReAct loop to classify
// the user's intent into one of the predefined categories and stores the result
// in graph.State so that downstream nodes can adapt their behaviour accordingly.
package intent

import (
	"context"
	"fmt"
	"strings"

	"hcm/cmd/agent-server/logics/prompt"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

// intentMaxTokens caps the response length for the intent classification call.
const intentMaxTokens = 16

// MakeIntentRecognitionNode returns a graph.NodeFunc that classifies user intent
// and writes the result to graph.State under constant.StateKeyIntent.
//
// The node:
//  1. Extracts the most recent user messages (up to contextWindowSize).
//  2. Calls mdl.GenerateContent with the intent classification prompt.
//  3. Parses the response into an enumor.IntentType; falls back to chat on any failure.
//
// contextWindowSize controls how many recent non-tool messages are sent to the model.
func MakeIntentRecognitionNode(mdl trpcmodel.Model, promptStore *prompt.Store, contextWindowSize int) graph.NodeFunc {
	return func(ctx context.Context, state graph.State) (any, error) {
		rid := rest.RidFromContext(ctx)
		if promptStore == nil {
			logs.Errorf("[intent recognition]: prompt store is nil, rid: %s", rid)
			return nil, errf.New(errf.InvalidParameter, "prompt store is nil")
		}
		intentPrompt, ok := promptStore.Get(constant.IntentRecognitionPromptKey)
		if !ok {
			logs.Errorf("[intent recognition]: intent prompt not found from prompt store, rid: %s", rid)
			return nil, errf.New(errf.InvalidParameter, "intent prompt not found")
		}

		messages, _ := state[graph.StateKeyMessages].([]trpcmodel.Message)

		contextMsgs := extractContextMessages(messages, contextWindowSize)
		if !hasUserMessage(contextMsgs) {
			return intentState(enumor.IntentTypeChat), nil
		}

		reqMsgs := make([]trpcmodel.Message, 0, 1+len(contextMsgs))
		reqMsgs = append(reqMsgs, trpcmodel.NewSystemMessage(intentPrompt.Content))
		reqMsgs = append(reqMsgs, contextMsgs...)

		maxTokens := intentMaxTokens
		req := &trpcmodel.Request{
			Messages: reqMsgs,
			GenerationConfig: trpcmodel.GenerationConfig{
				Stream:    false,
				MaxTokens: &maxTokens,
			},
		}

		intent, err := LLMParseIntent(ctx, mdl, req)
		if err != nil {
			logs.Errorf("[intent recognition]: callAndParseIntent failed, err: %v, rid: %s", err, rid)
			return intentState(enumor.IntentTypeChat), nil
		}

		logs.Infof("[intent recognition]: classified intent=%s, rid: %s", intent, rid)
		return intentState(intent), nil
	}
}

// LLMParseIntent sends the classification request to the model and parses the response.
// It returns (IntentTypeChat, nil) when the model returns an unrecognised value,
// and (IntentTypeChat, error) on a communication-level failure.
func LLMParseIntent(ctx context.Context, mdl trpcmodel.Model, req *trpcmodel.Request) (enumor.IntentType, error) {
	rid := rest.RidFromContext(ctx)
	respCh, err := mdl.GenerateContent(ctx, req)
	if err != nil {
		logs.Errorf("[intent recognition] LLM recognize intent failed, err: %v, rid: %s", err, rid)
		return enumor.IntentTypeChat, fmt.Errorf("generate content: %w", err)
	}

	var sb strings.Builder
	for resp := range respCh {
		if resp.Error != nil {
			logs.Errorf("[intent recognition] get LLM intent recognization failed, err: %v, rid: %s", resp.Error, rid)
			return enumor.IntentTypeChat, fmt.Errorf("model response error: %s", resp.Error.Message)
		}
		for _, choice := range resp.Choices {
			if choice.Message.Content != "" {
				sb.WriteString(choice.Message.Content)
			} else if choice.Delta.Content != "" {
				sb.WriteString(choice.Delta.Content)
			}
		}
	}
	logs.Infof("[intent recognition] callAndParseIntent model called message: %s, rid: %s", sb.String(), rid)

	raw := strings.TrimSpace(sb.String())
	intent := enumor.IntentType(raw)
	if err := intent.Validate(); err != nil {
		logs.Warnf("[intent recognition]: unrecognised model response %q, fallback to chat", raw)
		return enumor.IntentTypeChat, nil
	}

	return intent, nil
}

// extractContextMessages returns up to n recent user messages.
//
// Keep user and plain assistant messages, and filter out tool artifacts.
// The classification prompt still ends with the latest user turn in normal resume flow.
func extractContextMessages(messages []trpcmodel.Message, n int) []trpcmodel.Message {
	filtered := make([]trpcmodel.Message, 0, len(messages))
	for _, m := range messages {
		if m.Role != trpcmodel.RoleUser && m.Role != trpcmodel.RoleAssistant {
			continue
		}
		filtered = append(filtered, m)
	}

	logs.Infof("[intent recognition] extract context messages filtered: extract context messages=%v", filtered)

	if len(filtered) <= n {
		return filtered
	}

	return filtered[len(filtered)-n:]
}

// hasUserMessage reports whether messages contains at least one RoleUser message.
func hasUserMessage(messages []trpcmodel.Message) bool {
	for _, m := range messages {
		if m.Role == trpcmodel.RoleUser {
			return true
		}
	}
	return false
}

// intentState returns the graph.State delta that writes the recognised intent.
// host_apply persists in session state so subsequent fallback resumes route to llm.
func intentState(intent enumor.IntentType) graph.State {
	return graph.State{
		constant.StateKeyIntent: string(intent),
	}
}
