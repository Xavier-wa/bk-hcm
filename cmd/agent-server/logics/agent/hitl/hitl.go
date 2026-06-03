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

// Package hitl provides Human-in-the-Loop (HITL) functionality for the agent graph.
// It enables the LLM to request user confirmation or choices during execution.
//
// The HITL flow works as follows:
//  1. LLM calls the human_confirm tool when it needs user input
//  2. The graph routes to the hitl node via conditional edges
//  3. The hitl node calls graph.Interrupt, pausing execution and saving checkpoint
//  4. Frontend receives the interrupt event and displays the confirmation UI
//  5. User makes a choice, frontend sends resume request with the choice
//  6. Graph resumes from checkpoint, hitl node receives the choice
//  7. hitl node appends the choice as a user message and returns to llm node
//  8. LLM continues reasoning with the user's choice in context
package hitl

import (
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// declaredToolWrapper wraps a declaration as a non-callable tool.
// This is used for pure declaration tools that have no execution logic.
type declaredToolWrapper struct{ d *tool.Declaration }

// Declaration returns the declaration of the tool.
func (w declaredToolWrapper) Declaration() *tool.Declaration { return w.d }

// GetTool returns the HITL tool declaration (human_confirm).
// This is a pure declaration tool with no execution logic.
func GetTool() *tool.Declaration {
	return HumanConfirmTool()
}

// GetToolWrapper returns the HITL tool as a tool.Tool interface.
// This allows the tool to be registered in the tool set.
func GetToolWrapper() tool.Tool {
	return declaredToolWrapper{d: HumanConfirmTool()}
}

// GetNode returns the HITL node function.
// This node handles the interrupt logic and message injection.
func GetNode() graph.NodeFunc {
	return makeHITLNode()
}
