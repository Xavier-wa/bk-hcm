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

package tool

import (
	"context"
	"fmt"
	"strings"

	"hcm/pkg/logs"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

const confirmKeyword = "确认"

// confirmToolSet wraps a tool.ToolSet so that every CallableTool requires
// explicit user confirmation ("确认") before the real call is dispatched.
type confirmToolSet struct {
	inner tool.ToolSet
}

func newConfirmToolSet(inner tool.ToolSet) tool.ToolSet {
	return &confirmToolSet{inner: inner}
}

// Name returns the name of the tool set.
func (s *confirmToolSet) Name() string { return s.inner.Name() }

// Close closes the tool set.
func (s *confirmToolSet) Close() error { return s.inner.Close() }

// Tools returns the tools of the tool set.
func (s *confirmToolSet) Tools(ctx context.Context) []tool.Tool {
	raw := s.inner.Tools(ctx)
	wrapped := make([]tool.Tool, 0, len(raw))
	for _, t := range raw {
		if ct, ok := t.(tool.CallableTool); ok {
			wrapped = append(wrapped, &confirmTool{inner: ct})
		} else {
			wrapped = append(wrapped, t)
		}
	}
	return wrapped
}

// confirmTool wraps a single CallableTool with a confirmation gate.
type confirmTool struct {
	inner tool.CallableTool
}

var _ tool.CallableTool = (*confirmTool)(nil)

// Declaration returns the declaration of the tool.
func (t *confirmTool) Declaration() *tool.Declaration {
	return t.inner.Declaration()
}

// Call calls the tool.
func (t *confirmTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	toolName := ""
	if d := t.inner.Declaration(); d != nil {
		toolName = d.Name
	}

	lastUserInput := lastUserMessage(ctx)
	if strings.TrimSpace(lastUserInput) == confirmKeyword {
		logs.Infof("[tool:confirm] tool=%q user confirmed, proceeding", toolName)
		return t.inner.Call(ctx, jsonArgs)
	}

	logs.Infof("[tool:confirm] tool=%q blocked, last user input=%q (want %q)",
		toolName, lastUserInput, confirmKeyword)

	return fmt.Sprintf(
		"工具 [%s] 需要用户确认后才能执行。请回复「%s」以继续执行此操作。\n在要求用户确认前，先把本次提单的参数通过表格展示给用户",
		toolName, confirmKeyword,
	), nil
}

// lastUserMessage extracts the current user message content from the
// agent invocation stored in ctx.
func lastUserMessage(ctx context.Context) string {
	inv, ok := agent.InvocationFromContext(ctx)
	if !ok || inv == nil {
		return ""
	}
	return strings.TrimSpace(inv.Message.Content)
}
