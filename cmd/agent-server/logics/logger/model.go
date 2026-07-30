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
	"hcm/pkg/rest"
	"hcm/pkg/tools/util"

	openaiopt "github.com/openai/openai-go/option"
	"github.com/tidwall/gjson"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// GraphTraceLogPrefix 是主机申领等场景「可串联可追溯」日志的统一标志词。
// 父图进入子图、子图出父图、调用 LLM 发送的 messages 三类日志均使用本标志词，
// 以便通过单一关键词 grep 串联一次请求的完整消息流转。
const GraphTraceLogPrefix = "[hcm graph trace]"

// traceMessageTruncateLen 单条 message 内容超过该长度则裁剪，避免超长工具结果刷屏。
const traceMessageTruncateLen = 100

// MakeModelLoggerCallback creates model callbacks that log LLM reasoning (thinking)
// and response content for debugging.
func MakeModelLoggerCallback() model.AfterModelCallbackStructured {
	const maxLog = 2048

	return func(ctx context.Context, args *model.AfterModelArgs) (*model.AfterModelResult, error) {
		rid := rest.RidFromContext(ctx)

		if args == nil {
			return nil, nil
		}
		if args.Error != nil {
			logs.Errorf("[model] LLM call failed: err=%v, rid: %s", args.Error, rid)
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
			logs.Infof("[model] LLM reasoning: %s, rid: %s", util.Truncate(msg.ReasoningContent, maxLog), rid)
		}
		if msg.Content != "" {
			logs.Infof("[model] LLM content: %s, rid: %s", util.Truncate(msg.Content, maxLog), rid)
		}
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				logs.Infof("[model] LLM tool_call: %s args=%s, rid: %s", tc.Function.Name,
					util.Truncate(string(tc.Function.Arguments), maxLog), rid)
			}
		}

		if rsp.Usage.TotalTokens > 0 {
			logs.Infof("[model] LLM usage: prompt=%d completion=%d total=%d, rid: %s",
				rsp.Usage.PromptTokens, rsp.Usage.CompletionTokens, rsp.Usage.TotalTokens, rid)
		}
		return nil, nil
	}
}

// LLMRequestLogger is an OpenAI middleware that logs request details and estimates input tokens.
func LLMRequestLogger(r *http.Request, next openaiopt.MiddlewareNext) (*http.Response, error) {
	rid := rest.RidFromContext(r.Context())
	logBodyLimit := constant.DefaultLLMRequestBodyLogLimit
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			body := string(bodyBytes)

			logLLMToolsSummary(body, rid)
			logLLMTokenConfig(body, rid)
			logTraceLLMRequestMessages(body, rid)
			logLLMMessageDuplicateCheck(body, rid)

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
			logs.Infof("LLM request: %s %s body=%s est_input_tokens≈%d, rid: %s", r.Method, r.URL.String(), body, estTokens, rid)
		} else {
			logs.Warnf("LLM request: failed to read body: %v, rid: %s", err, rid)
		}
	} else {
		logs.Infof("LLM request: %s %s (no body), rid: %s", r.Method, r.URL.String(), rid)
	}
	return next(r)
}

// LogTraceMessages 以统一标志词 [hcm graph trace] 在【一条日志】里结构化打印整组 messages。
// 每条消息用 [] 包裹（形如 [idx=.. role=.. tool_id=.. content=.. tool_calls=..]），整组再用
// messages=[...] 包裹，保证一次调用只产生一行可追溯日志，便于按 rid 串联且不被拆分。
// 单条 content 超过 traceMessageTruncateLen 时裁剪为前缀+省略提示，避免超长工具结果刷屏；
// tag 区分阶段（subgraph_input/subgraph_output/llm_request 等），rid 用于请求级串联。
func LogTraceMessages(tag, rid string, msgs []model.Message) {
	if len(msgs) == 0 {
		logs.Infof("%s tag=%s message_count=0 rid: %s", GraphTraceLogPrefix, tag, rid)
		return
	}
	var payload strings.Builder
	payload.WriteString("messages=[")
	for i, m := range msgs {
		if i > 0 {
			payload.WriteString(" ")
		}
		payload.WriteString(formatTraceMessage(i, m))
	}
	payload.WriteString("]")
	logs.Infof("%s tag=%s message_count=%d %s rid: %s",
		GraphTraceLogPrefix, tag, len(msgs), payload.String(), rid)
}

// formatTraceMessage 将单条 message 格式化为带 [] 包裹的结构化字符串。
// tool_calls 内部每一项同样用 [] 包裹，并解包 tool_proxy_* 代理转发的真实 MCP 工具名（real=）。
func formatTraceMessage(i int, m model.Message) string {
	content := m.Content
	if len(content) > traceMessageTruncateLen {
		content = content[:traceMessageTruncateLen] + fmt.Sprintf("...(truncated, total %d)", len(m.Content))
	}
	toolID := m.ToolID
	if toolID == "" && len(m.ToolCalls) > 0 {
		toolID = m.ToolCalls[0].ID
	}
	var toolCallsSummary strings.Builder
	for _, tc := range m.ToolCalls {
		fargs := string(tc.Function.Arguments)
		if len(fargs) > traceMessageTruncateLen {
			fargs = fargs[:traceMessageTruncateLen] + fmt.Sprintf("...(truncated, total %d)", len(fargs))
		}
		realField := ""
		if realName := gjson.Get(string(tc.Function.Arguments), "tool_name").String(); realName != "" {
			realField = " real=" + realName
		}
		toolCallsSummary.WriteString(fmt.Sprintf("[id=%s fn=%s%s args=%s]",
			tc.ID, tc.Function.Name, realField, fargs))
	}
	return fmt.Sprintf("[idx=%d role=%s tool_id=%s content=%s tool_calls=%s]",
		i, m.Role, toolID, content, toolCallsSummary.String())
}

// logLLMToolsSummary extracts tool names from the OpenAI request JSON and logs
// a compact summary so operators can verify dynamic tool filtering at a glance.
func logLLMToolsSummary(body, rid string) {
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
	logs.Infof("LLM request tools: total=%d, mcp=%d %v, framework/skill=%d %v, rid: %s",
		len(mcpNames)+len(otherNames), len(mcpNames), mcpNames, len(otherNames), otherNames, rid)
}

// logLLMTokenConfig extracts token budget fields from the OpenAI request JSON
// to help diagnose max_tokens issues with upstream API gateways.
func logLLMTokenConfig(body, rid string) {
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
		"max_tokens=%v (present=%v), max_completion_tokens=%v (present=%v), rid: %s",
		model, msgCount, inputChars, stream.String(),
		maxTokens.String(), maxTokens.Exists(),
		maxCompletionTokens.String(), maxCompletionTokens.Exists(), rid)
}

// logTraceLLMRequestMessages parses the outgoing LLM request body and logs the whole messages
// array in a SINGLE line under the unified [hcm graph trace] prefix, so the LLM send step can be
// correlated with subgraph input/output logs by the same keyword. Each message is wrapped in [] and
// the whole group in messages=[...]; each message's content is truncated when longer than
// traceMessageTruncateLen to avoid huge tool results flooding the log.
func logTraceLLMRequestMessages(body, rid string) {
	msgs := gjson.Get(body, "messages").Array()
	if len(msgs) == 0 {
		logs.Infof("%s tag=llm_request message_count=0 rid: %s", GraphTraceLogPrefix, rid)
		return
	}
	var payload strings.Builder
	payload.WriteString("messages=[")
	for i, m := range msgs {
		if i > 0 {
			payload.WriteString(" ")
		}
		payload.WriteString(formatTraceMessageFromGJSON(i, m))
	}
	payload.WriteString("]")
	logs.Infof("%s tag=llm_request message_count=%d %s rid: %s",
		GraphTraceLogPrefix, len(msgs), payload.String(), rid)
}

// formatTraceMessageFromGJSON 将单条 gjson 原始消息格式化为带 [] 包裹的结构化字符串，
// 与 formatTraceMessage 输出风格保持一致。content 支持结构化数组降级取 raw；
// tool_calls 内部每一项同样用 [] 包裹，并解包 tool_proxy_* 代理转发的真实 MCP 工具名（real=）。
func formatTraceMessageFromGJSON(i int, m gjson.Result) string {
	role := m.Get("role").String()
	toolID := m.Get("tool_call_id").String()
	content := m.Get("content").String()
	if content == "" {
		// content 可能为结构化数组（如多模态/tool_call 渲染），降级取 raw
		content = m.Get("content").Raw
	}
	if len(content) > traceMessageTruncateLen {
		content = content[:traceMessageTruncateLen] + fmt.Sprintf("...(truncated, total %d)", len(content))
	}
	var toolCallsSummary strings.Builder
	for _, tc := range m.Get("tool_calls").Array() {
		tcid := tc.Get("id").String()
		fname := tc.Get("function.name").String()
		fargs := tc.Get("function.arguments").String()
		if len(fargs) > traceMessageTruncateLen {
			fargs = fargs[:traceMessageTruncateLen] + fmt.Sprintf("...(truncated, total %d)", len(fargs))
		}
		realField := ""
		if realName := gjson.Get(fargs, "tool_name").String(); realName != "" {
			realField = " real=" + realName
		}
		toolCallsSummary.WriteString(fmt.Sprintf("[id=%s fn=%s%s args=%s]", tcid, fname, realField, fargs))
	}
	return fmt.Sprintf("[idx=%d role=%s tool_id=%s content=%s tool_calls=%s]",
		i, role, toolID, content, toolCallsSummary.String())
}

// logLLMMessageDuplicateCheck scans outgoing messages for duplicate tool_call_id values.
func logLLMMessageDuplicateCheck(body, rid string) {
	msgs := gjson.Get(body, "messages").Array()
	if len(msgs) == 0 {
		return
	}
	seen := make(map[string]int, len(msgs))
	var dups []string
	for i, msg := range msgs {
		if msg.Get("role").String() != "tool" {
			continue
		}
		toolID := msg.Get("tool_call_id").String()
		if toolID == "" {
			continue
		}
		if first, ok := seen[toolID]; ok {
			dups = append(dups, fmt.Sprintf("idx=%d dup_of=%d id=%s", i, first, toolID))
			continue
		}
		seen[toolID] = i
	}
	if len(dups) == 0 {
		logs.Infof("[llm request] message check: messages=%d duplicate_tool_ids=0 rid: %s", len(msgs), rid)
		return
	}
	logs.Warnf("[llm request] message check: messages=%d duplicate_tool_ids=%d details=%v rid: %s",
		len(msgs), len(dups), dups, rid)
}
