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

// Package intent provides user intent classification for the GraphAgent.
// It runs a lightweight LLM call to classify the user's intent of the current turn
// into one of the predefined categories. The classification is invoked in-process by
// the scene_dispatch node rather than exposed as a graph node, so the result stays a
// node-local value and never enters (nor is persisted in) graph.State.
package intent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hcm/cmd/agent-server/logics/prompt"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

// intentMaxTokens caps the response length for the intent classification call.
const intentMaxTokens = 16

// Classify recognises the user intent of the current turn with a lightweight LLM call.
//
// It never returns an error: every failure mode (prompt store unavailable, prompt missing,
// LLM call error, unrecognised response) degrades to enumor.IntentTypeUnsupported.
// 该取值在 scene_dispatch 的决策矩阵中落入「不受支持」分支，对已有场景标签的会话意味着
// 「保持原场景」，因此分类失败永远不会误切场景。降级值刻意不用 chat：chat 是一个真实的
// 分类结果，未来可能被实现为受支持场景，届时复用它会让所有降级路径变成「切到闲聊场景」。
//
// The messages parameter is the full graph message history; only the most recent
// contextWindowSize user/assistant messages are sent to the model.
func Classify(ctx context.Context, mdl trpcmodel.Model, promptStore *prompt.Store,
	messages []trpcmodel.Message, contextWindowSize int) enumor.IntentType {

	rid := rest.RidFromContext(ctx)
	if promptStore == nil {
		logs.Errorf("[intent recognition] prompt store is nil, fallback to unsupported, rid: %s", rid)
		return enumor.IntentTypeUnsupported
	}

	intentPrompt, ok := promptStore.Get(constant.IntentRecognitionPromptKey)
	if !ok {
		logs.Errorf("[intent recognition] intent prompt not found in prompt store, fallback to unsupported, rid: %s",
			rid)
		return enumor.IntentTypeUnsupported
	}

	contextMsgs := extractContextMessages(messages, contextWindowSize)
	if !hasUserMessage(contextMsgs) {
		return enumor.IntentTypeUnsupported
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

	// 分类调用发生在每个轮次边界上，耗时直接计入用户可感知的首字延迟，因此固定打点。
	start := time.Now()
	intent, err := LLMParseIntent(ctx, mdl, req)
	if err != nil {
		logs.Errorf("[intent recognition] classify failed, fallback to unsupported, err: %v, cost: %v, rid: %s",
			err, time.Since(start), rid)
		return enumor.IntentTypeUnsupported
	}

	logs.Infof("[intent recognition] classified intent=%s, cost: %v, rid: %s", intent, time.Since(start), rid)
	return intent
}

// LLMParseIntent sends the classification request to the model and parses the response.
// It returns (IntentTypeUnsupported, nil) when the model returns an unrecognised value,
// and (IntentTypeUnsupported, error) on a communication-level failure.
func LLMParseIntent(ctx context.Context, mdl trpcmodel.Model, req *trpcmodel.Request) (enumor.IntentType, error) {
	rid := rest.RidFromContext(ctx)
	respCh, err := mdl.GenerateContent(ctx, req)
	if err != nil {
		logs.Errorf("[intent recognition] LLM recognize intent failed, err: %v, rid: %s", err, rid)
		return enumor.IntentTypeUnsupported, fmt.Errorf("generate content: %w", err)
	}

	var sb strings.Builder
	for resp := range respCh {
		if resp.Error != nil {
			logs.Errorf("[intent recognition] get LLM intent recognization failed, err: %v, rid: %s", resp.Error, rid)
			return enumor.IntentTypeUnsupported, fmt.Errorf("model response error: %s", resp.Error.Message)
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
		logs.Warnf("[intent recognition]: unrecognised model response %q, fallback to unsupported, rid: %s", raw, rid)
		return enumor.IntentTypeUnsupported, nil
	}

	return intent, nil
}

// extractContextMessages returns up to n recent conversation messages, oldest first.
// The classification prompt still ends with the latest user turn in normal resume flow.
func extractContextMessages(messages []trpcmodel.Message, n int) []trpcmodel.Message {
	filtered := make([]trpcmodel.Message, 0, len(messages))
	for _, m := range messages {
		if !isConversationMessage(m) {
			continue
		}
		filtered = append(filtered, m)
	}

	if len(filtered) <= n {
		return filtered
	}

	return filtered[len(filtered)-n:]
}

// isConversationMessage reports whether m is a human-readable conversation turn:
// a user message, or an assistant reply that carries no tool calls.
//
// 工具调用与工具结果对意图判断没有信息量，且 tool_calls 消息的 Content 通常为空，
// 混进来只会白白挤占上下文窗口。
func isConversationMessage(m trpcmodel.Message) bool {
	switch m.Role {
	case trpcmodel.RoleUser:
		return true
	case trpcmodel.RoleAssistant:
		return len(m.ToolCalls) == 0
	default:
		return false
	}
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
