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

package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"

	openaiopt "github.com/openai/openai-go/option"
	"github.com/tidwall/gjson"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// ModelLoggerCallback creates model callbacks that log LLM reasoning (thinking)
// and response content for debugging.
func ModelLoggerCallback() *model.Callbacks {
	const maxLog = 2048

	truncate := func(s string) string {
		if len(s) <= maxLog {
			return s
		}
		return s[:maxLog] + "...(truncated)"
	}

	cb := model.NewCallbacks()
	cb.AfterModel = append(cb.AfterModel, func(
		ctx context.Context,
		args *model.AfterModelArgs,
	) (*model.AfterModelResult, error) {
		if args == nil {
			return nil, nil
		}
		if args.Error != nil {
			logs.Errorf("[model] LLM call failed: err=%v", args.Error)
			return nil, nil
		}
		rsp := args.Response
		if rsp == nil || len(rsp.Choices) == 0 {
			return nil, nil
		}
		if rsp.IsPartial {
			return nil, nil
		}

		choice := rsp.Choices[0]
		msg := choice.Message

		if msg.ReasoningContent != "" {
			logs.Infof("[model] LLM reasoning: %s", truncate(msg.ReasoningContent))
		}
		if msg.Content != "" {
			logs.Infof("[model] LLM content: %s", truncate(msg.Content))
		}
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				logs.Infof("[model] LLM tool_call: %s args=%s", tc.Function.Name,
					truncate(string(tc.Function.Arguments)))
			}
		}

		if rsp.Usage.TotalTokens > 0 {
			logs.Infof("[model] LLM usage: prompt=%d completion=%d total=%d",
				rsp.Usage.PromptTokens, rsp.Usage.CompletionTokens, rsp.Usage.TotalTokens)
		}
		return nil, nil
	})
	return cb
}

// LLMRequestLogger is an OpenAI middleware that logs request details and estimates input tokens.
func LLMRequestLogger(r *http.Request, next openaiopt.MiddlewareNext) (*http.Response, error) {
	logBodyLimit := constant.DefaultLLMRequestBodyLogLimit
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			body := string(bodyBytes)

			logLLMToolsSummary(body)
			logLLMTokenConfig(body)

			// Estimate input tokens: roughly chars/4 for English, chars/2 for Chinese;
			// use chars/4 as a conservative lower-bound heuristic for mixed content.
			var estTokens int
			for _, msg := range gjson.Get(body, "messages").Array() {
				estTokens += len(msg.Get("content").String()) / 4
			}
			estTokens += len(body) / 10 // account for system prompts, tool definitions, etc.

			if len(body) > logBodyLimit {
				body = body[:logBodyLimit] + fmt.Sprintf("... (truncated, total %d bytes)", len(bodyBytes))
			}
			logs.Infof("LLM request: %s %s body=%s est_input_tokens≈%d", r.Method, r.URL.String(), body, estTokens)
		} else {
			logs.Warnf("LLM request: failed to read body: %v", err)
		}
	} else {
		logs.Infof("LLM request: %s %s (no body)", r.Method, r.URL.String())
	}
	return next(r)
}

// logLLMToolsSummary extracts tool names from the OpenAI request JSON and logs
// a compact summary so operators can verify dynamic tool filtering at a glance.
func logLLMToolsSummary(body string) {
	tools := gjson.Get(body, "tools")
	if !tools.Exists() {
		return
	}
	arr := tools.Array()
	var mcpNames, otherNames []string
	for _, t := range arr {
		name := t.Get("function.name").String()
		if name == "" {
			continue
		}
		if strings.HasPrefix(name, "skill_") || strings.HasPrefix(name, "transfer_to_") ||
			name == "knowledge_search" || name == "agentic_knowledge_search" {
			otherNames = append(otherNames, name)
		} else {
			mcpNames = append(mcpNames, name)
		}
	}
	logs.Infof("LLM request tools: total=%d, mcp=%d %v, framework/skill=%d %v",
		len(mcpNames)+len(otherNames), len(mcpNames), mcpNames, len(otherNames), otherNames)
}

// logLLMTokenConfig extracts token budget fields from the OpenAI request JSON
// to help diagnose max_tokens issues with upstream API gateways.
func logLLMTokenConfig(body string) {
	model := gjson.Get(body, "model").String()
	maxTokens := gjson.Get(body, "max_tokens")
	maxCompletionTokens := gjson.Get(body, "max_completion_tokens")
	stream := gjson.Get(body, "stream")

	msgCount := len(gjson.Get(body, "messages").Array())
	var inputChars int
	for _, msg := range gjson.Get(body, "messages").Array() {
		inputChars += len(msg.Get("content").String())
	}

	logs.Infof("LLM request token config: model=%s, messages=%d, input_chars≈%d, stream=%v, "+
		"max_tokens=%v (present=%v), max_completion_tokens=%v (present=%v)",
		model, msgCount, inputChars, stream.String(),
		maxTokens.String(), maxTokens.Exists(),
		maxCompletionTokens.String(), maxCompletionTokens.Exists())
}
