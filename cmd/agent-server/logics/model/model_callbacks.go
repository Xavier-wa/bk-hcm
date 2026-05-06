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

package model

import (
	"context"

	"hcm/pkg/logs"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

// historicalToolResultPlaceholder replaces the content of tool-result messages from
// historical turns. Keeping the message in place (with empty-ish content) is required
// because the OpenAI API mandates a tool response for every tool_call in the preceding
// assistant message; deleting the message would cause an API validation error.
const historicalToolResultPlaceholder = "[historical result omitted]"

// MakeHistoricalToolResultFilter returns a BeforeModelCallbackStructured that trims
// token usage by replacing the content of tool-result messages that belong to
// historical turns with a short placeholder.
//
// "Historical" is defined as every message that appears before the last role:"user"
// message in the request. Tool results within the current turn (after the last user
// message) are always preserved in full so the model can act on them.
func MakeHistoricalToolResultFilter() model.BeforeModelCallbackStructured {
	return func(ctx context.Context, args *model.BeforeModelArgs) (*model.BeforeModelResult, error) {
		if args == nil || args.Request == nil {
			return nil, nil
		}

		msgs := args.Request.Messages

		// Locate the last user message; everything before it is historical context.
		lastUserIdx := -1
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == model.RoleUser {
				lastUserIdx = i
				break
			}
		}

		// Nothing to filter: no user message found, or it is the very first message.
		if lastUserIdx <= 0 {
			return nil, nil
		}

		// Count historical tool messages that still carry real content.
		filteredCount := 0
		for i := 0; i < lastUserIdx; i++ {
			if msgs[i].Role == model.RoleTool && msgs[i].Content != historicalToolResultPlaceholder {
				filteredCount++
			}
		}
		if filteredCount == 0 {
			return nil, nil
		}

		// Shallow-copy the slice so we do not mutate the original backing array,
		// then overwrite content on the copied elements.
		filtered := make([]model.Message, len(msgs))
		copy(filtered, msgs)
		for i := 0; i < lastUserIdx; i++ {
			if filtered[i].Role == model.RoleTool {
				filtered[i].Content = historicalToolResultPlaceholder
			}
		}
		args.Request.Messages = filtered

		logs.Infof("[model] historical tool results filtered: count=%d total_msgs=%d", filteredCount, len(msgs))
		return nil, nil
	}
}
