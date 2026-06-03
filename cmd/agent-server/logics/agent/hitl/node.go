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

package hitl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// makeHITLNode returns a Function Node that handles HITL (Human-in-the-Loop) logic.
//
// Execution flow:
//  1. First call: Parse human_confirm tool call from messages, call graph.Interrupt,
//     which throws InterruptError, causing the graph to pause and save checkpoint.
//  2. Resume call: graph.Interrupt returns the resume value (user input),
//     append user choice as a user message to messages, return updated state.
func makeHITLNode() graph.NodeFunc {
	return func(ctx context.Context, state graph.State) (any, error) {
		rid := rest.RidFromContext(ctx)
		messages, _ := state[graph.StateKeyMessages].([]model.Message)
		logs.Infof("hitl node: start, total messages=%d, rid: %s", len(messages), rid)

		// 1. Find the human_confirm tool call from the last assistant message
		toolCall := extractHumanConfirmToolCall(messages)
		if toolCall == nil {
			return nil, errors.New("hitl node: no human_confirm tool call found")
		}

		// 2. Parse tool call arguments
		var args HumanConfirmArgs
		if err := json.Unmarshal(toolCall.Function.Arguments, &args); err != nil {
			logs.Errorf("hitl node: failed to parse human_confirm args, err: %v, rid: %s", err, rid)
			return nil, fmt.Errorf("hitl node: failed to parse human_confirm args: %v", err)
		}
		logs.Infof("hitl node: parsed args, question=%s, options=%v, rid: %s", args.Question, args.Options, rid)

		interruptKey := fmt.Sprintf("%s%s%s", constant.HITLInterruptKey, constant.InterruptKeySeparator, toolCall.ID)

		// 3. Call graph.Interrupt
		// First call: throws InterruptError, graph pauses
		// Resume call: returns resume value (user input)
		resumeValue, err := graph.Interrupt(ctx, state, interruptKey, map[string]any{
			"question": args.Question,
			"options":  args.Options,
		})
		if err != nil {
			// First call: err is InterruptError, propagate up to pause execution
			logs.Errorf("hitl node: first call, graph.Interrupt returned err: %v, rid: %s", err, rid)
			return nil, err
		}
		logs.Infof("hitl node: resume call, graph.Interrupt returned resumeValue=%v, rid: %s", resumeValue, rid)

		// 4. Resume: get user choice from resume value
		userChoice, ok := resumeValue.(string)
		if !ok {
			logs.Errorf("hitl node: invalid resume value type, expected string, got %T, rid: %s", resumeValue, rid)
			return nil, fmt.Errorf("hitl node: invalid resume value type, expected string, got %T", resumeValue)
		}
		logs.Infof("hitl node: userChoice=%s, rid: %s", userChoice, rid)

		// 5. Construct delta messages to close the human_confirm tool call and pass user choice.
		//
		// Note: MessagesStateSchema uses an append reducer for StateKeyMessages,
		// so we return ONLY the delta (new messages), not the full history.
		toolResultMsg := model.Message{
			Role:    model.RoleTool,
			ToolID:  toolCall.ID,
			Content: userChoice,
		}
		userMsg := model.Message{
			Role:    model.RoleUser,
			Content: userChoice,
		}
		logs.Infof("hitl node: returning message delta, toolID=%s, userChoice=%s, rid: %s", toolCall.ID, userChoice, rid)

		return graph.State{
			graph.StateKeyMessages: []model.Message{toolResultMsg, userMsg},
		}, nil
	}
}

// extractHumanConfirmToolCall extracts the human_confirm tool call from messages.
// It searches backwards from the last message to find the most recent assistant
// message containing a human_confirm tool call.
func extractHumanConfirmToolCall(messages []model.Message) *model.ToolCall {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == model.RoleAssistant {
			for _, tc := range messages[i].ToolCalls {
				if tc.Function.Name == constant.HumanConfirmToolName {
					// Return a copy to avoid reference issues
					return &tc
				}
			}
		}
	}
	return nil
}
