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

package prompt

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"text/template"
	"time"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/util"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

const (
	// PromptContentMaxRuneLength is the maximum length of the prompt content in runes.
	PromptContentMaxRuneLength = 80
)

// MakeSystemPromptReplaceCallback returns a BeforeModelCallbackStructured that replaces
// the system message with the latest prompt content from store before each LLM call.
// If the store has no system prompt, the callback is a no-op.
// Use verbosity >= 2 (logs.V(2)) to see per-call injection details.
func MakeSystemPromptReplaceCallback(store *Store) model.BeforeModelCallbackStructured {
	return func(ctx context.Context, args *model.BeforeModelArgs) (*model.BeforeModelResult, error) {
		rid := rest.RidFromContext(ctx)
		if args == nil || args.Request == nil {
			return nil, nil
		}

		systemEntry, _ := store.Get(constant.SystemPromptKey)
		if systemEntry.Content == "" {
			return nil, nil
		}

		instructionEntry, _ := store.Get(constant.InstructionKey)
		// NOTE beforeModelCallback 的触发时点在 instruction 渲染之后，因此动态渲染必须手动实现，不能依赖框架
		rendered := renderInstructionTemplate(ctx, instructionEntry.Content)

		content := BuildSystemPrompt(systemEntry.Content, rendered)

		args.Request.Messages = replaceOrInsertSystem(args.Request.Messages, content)

		if logs.V(2) {
			logs.V(2).Infof("[replace callback]system prompt injected: content=%d preview=%q,rid: %s", len(content),
				util.TruncateRune(content, PromptContentMaxRuneLength), rid)
		}

		return nil, nil
	}
}

// MakeMemoryExtractReplaceCallback returns a BeforeModelCallbackStructured for use
// inside the memory extractor's model callback pipeline. On each extraction LLM call
// it replaces only the base-prompt portion of the system message with the latest
// MemoryExtractKey content from store, re-rendering the {current_date} placeholder
// to match the extractor's own template logic.
// The <available_actions> and <existing_memories> sections that the extractor
// appends after the base prompt are preserved unchanged.
// If the store has no content for that key, the callback is a no-op and the
// extractor continues with its built-in default prompt.
func MakeMemoryExtractReplaceCallback(store *Store) model.BeforeModelCallbackStructured {
	return func(ctx context.Context, args *model.BeforeModelArgs) (*model.BeforeModelResult, error) {
		rid := rest.RidFromContext(ctx)
		if args == nil || args.Request == nil {
			return nil, nil
		}

		e, ok := store.Get(constant.MemoryExtractPromptKey)
		if !ok || e.Content == "" {
			logs.Infof("[memory extractor replace callback] no memory extract prompt from store, rid: %s", rid)
			return nil, nil
		}

		// Replace only the base-prompt part; preserve the <available_actions> and
		// <existing_memories> sections that the extractor appended after it.
		args.Request.Messages = replaceExtractBasePrompt(args.Request.Messages, e.Content)

		logs.V(2).Infof("memory extract prompt injected: contentLen=%d preview=%s, rid: %s",
			len(e.Content), util.TruncateRune(e.Content, PromptContentMaxRuneLength), rid)
		return nil, nil
	}
}

// replaceExtractBasePrompt replaces only the base-prompt portion of the extractor's
// system message. The extractor appends "\n<available_actions>" (and optionally
// "\n<existing_memories>") after the rendered base prompt; those sections are kept
// intact. When the system message contains no appended sections, the whole content
// is replaced. When no system message exists, a new one is inserted.
func replaceExtractBasePrompt(msgs []model.Message, newBasePrompt string) []model.Message {
	// Render {current_date} to match the extractor's own template rendering,
	// so relative time references in the prompt resolve correctly.
	dateStr := time.Now().UTC().Format(time.DateOnly)
	newBasePrompt = strings.ReplaceAll(newBasePrompt, "{current_date}", dateStr)

	// 这里只替换基本的提示词，即memoryExtractor.prompt
	// 在memoryExtractor.prompt后追加的<available_actions>等内容不替换，见memoryExtractor.buildSystemPrompt
	const actionsTag = "\n<available_actions>\n"
	for i := range msgs {
		if msgs[i].Role != model.RoleSystem {
			continue
		}
		if idx := strings.Index(msgs[i].Content, actionsTag); idx >= 0 {
			msgs[i].Content = newBasePrompt + msgs[i].Content[idx:]
		} else {
			msgs[i].Content = newBasePrompt
		}
		return msgs
	}
	return append([]model.Message{model.NewSystemMessage(newBasePrompt)}, msgs...)
}

// replaceOrInsertSystem replaces the first system message in msgs with content,
// or prepends a new system message when none exists.
func replaceOrInsertSystem(msgs []model.Message, content string) []model.Message {
	for i := range msgs {
		if msgs[i].Role == model.RoleSystem {
			msgs[i].Content = content
			return msgs
		}
	}
	return append([]model.Message{model.NewSystemMessage(content)}, msgs...)
}

// BuildSystemPrompt builds the system prompt with instruction.
func BuildSystemPrompt(systemPrompt, instruction string) string {
	prompt := systemPrompt
	if instruction != "" {
		if prompt != "" {
			prompt += "\n\n"
		}
		prompt += instruction
	}
	return prompt
}

// instructionTemplateData holds the dynamic values injected into the instruction Go template.
type instructionTemplateData struct {
	// UserDisplayName is the display name of the current user, sourced from the request context.
	UserDisplayName string
	// BkBizID is the business ID bound to the current session. Empty when the session is
	// platform-level (bk_biz_id = -1) or when the value has not been set.
	BkBizID string
	// AccountID is the cloud account ID identified by the account_select graph node.
	// Empty when no account has been selected yet.
	AccountID string
}

// renderInstructionTemplate executes the instruction Go template with runtime values
// extracted from ctx and the current invocation's RuntimeState.
// On any parse or execution error the original template string is returned unchanged.
func renderInstructionTemplate(ctx context.Context, tmpl string) string {
	if tmpl == "" {
		return ""
	}

	rid := rest.RidFromContext(ctx)

	data := instructionTemplateData{}

	if username, _ := ctx.Value(constant.UserKey).(string); username != "" {
		data.UserDisplayName = username
	}

	if inv, ok := trpcagent.InvocationFromContext(ctx); ok && inv != nil && inv.RunOptions.RuntimeState != nil {
		if bkBizID, ok := inv.RunOptions.RuntimeState[constant.SessionBkBizIDStateKey].(int64); ok && bkBizID > 0 {
			data.BkBizID = strconv.FormatInt(bkBizID, 10)
		}
		if accountID, ok := inv.RunOptions.RuntimeState[constant.SessionAccountIDTempKey].(string); ok {
			data.AccountID = accountID
		}
	}

	t, err := template.New(constant.InstructionKey).Parse(tmpl)
	if err != nil {
		logs.Warnf("[instruction_render] failed to parse instruction template, err: %v, rid: %s", err, rid)
		return tmpl
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		logs.Warnf("[instruction_render] failed to execute instruction template, err: %v, rid: %s", err, rid)
		return tmpl
	}

	return buf.String()
}
