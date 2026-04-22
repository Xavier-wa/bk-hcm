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

package logics

import (
	"context"
	"encoding/json"
	"fmt"

	"hcm/pkg/logs"

	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// makeToolLogger creates tool callbacks that log all tool invocations
// (arguments, results, errors) for debugging.
func makeToolLogger() *tool.Callbacks {
	const maxResultLog = 1024

	truncate := func(s string) string {
		if len(s) <= maxResultLog {
			return s
		}
		return s[:maxResultLog] + "...(truncated)"
	}

	resultStr := func(result any) string {
		if result == nil {
			return "<nil>"
		}
		switch v := result.(type) {
		case string:
			return truncate(v)
		case []byte:
			return truncate(string(v))
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return truncate(fmt.Sprintf("%v", v))
			}
			return truncate(string(b))
		}
	}

	cb := tool.NewCallbacks()
	cb.RegisterBeforeTool(func(
		ctx context.Context,
		args *tool.BeforeToolArgs,
	) (*tool.BeforeToolResult, error) {
		if args == nil {
			return nil, nil
		}
		logs.Infof("[tool] >> %s called: args=%s", args.ToolName, string(args.Arguments))
		return nil, nil
	})
	cb.RegisterAfterTool(func(
		ctx context.Context,
		args *tool.AfterToolArgs,
	) (*tool.AfterToolResult, error) {
		if args == nil {
			return nil, nil
		}
		if args.Error != nil {
			logs.Errorf("[tool] << %s failed: args=%s err=%v", args.ToolName, string(args.Arguments), args.Error)
		} else {
			logs.Infof("[tool] << %s succeeded: args=%s result=%s", args.ToolName, string(args.Arguments),
				resultStr(args.Result))
		}
		return nil, nil
	})
	return cb
}

// makeModelLogger creates model callbacks that log LLM reasoning (thinking)
// and response content for debugging.
func makeModelLogger() *model.Callbacks {
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
