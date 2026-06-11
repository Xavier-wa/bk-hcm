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

package constant

import "time"

// AG-UI
const (
	// AGUIPath is the HTTP path for the AG-UI endpoint.
	AGUIPath = "/api/v1/agent/agui"
	// AGUICancelPath is the HTTP path for the AG-UI cancel endpoint.
	AGUICancelPath = "/api/v1/agent/cancel"
	// AGUIHistoryPath is the HTTP path for the AG-UI history (MessagesSnapshot) endpoint.
	AGUIHistoryPath = "/api/v1/agent/history"
)

// MCP
const (
	// MCPTypeBKAIDev is the MCP toolset type that enables automatic
	// X-Bkapi-Authorization header injection for BK AI Dev gateways.
	MCPTypeBKAIDev = "bkaidev"

	// DefaultProviderName is the well-known provider name that the "aidev" config
	// section is mapped to. Models without an explicit provider use this one.
	DefaultProviderName = MCPTypeBKAIDev
)

// Skill
const (
	// SkillLoadToolName is the name of the skill load tool.
	SkillLoadToolName = "skill_load"
	// SkillListDocsToolName is the name of the skill list docs tool.
	SkillListDocsToolName = "skill_list_docs"
	// SkillSelectDocsToolName is the name of the skill select docs tool.
	SkillSelectDocsToolName = "skill_select_docs"
)

// MCP proxy tool
const (
	// ProxyToolSetName is the name of the MCP proxy toolset.
	ProxyToolSetName = "tool_proxy"
	// ProxySchemaTokenLen is the length of the token for the proxy schema.
	ProxySchemaTokenLen = 16

	// SearchToolsToolName is the meta-tool for semantic tool search.
	SearchToolsToolName = "search_tools"
	// GetToolSchemaToolName is the meta-tool for fetching a tool schema by name.
	GetToolSchemaToolName = "get_tool_schema"
	// ExecuteToolToolName is the meta-tool for executing an MCP tool by name.
	ExecuteToolToolName = "execute_tool"
)

// Default upper bounds for the agent invocation loop.
const (
	// DefaultMaxLLMCalls caps LLM requests per invocation to prevent runaway loops.
	DefaultMaxLLMCalls = 50
	// DefaultMaxToolIterations caps tool-call iterations per invocation.
	DefaultMaxToolIterations = 25
	// DefaultMaxHistoryRuns limits preserved full-message history runs when session
	// summary is enabled; older runs are represented by the summary only.
	DefaultMaxHistoryRuns = 10

	// DefaultLLMRequestBodyLogLimit is the limit of the LLM request body log.
	DefaultLLMRequestBodyLogLimit = 16 * 1024
)

// session constant
const (
	// SessionCacheResolverCapacity is the capacity of the session cache resolver.
	SessionCacheResolverCapacity = 10000

	// SessionCacheResolverTTL is the TTL of the session cache resolver.
	SessionCacheResolverTTL = 30 * time.Minute

	// SessionIncrContentCountTimeout is the timeout of the session incr content count.
	SessionIncrContentCountTimeout = 5 * time.Second

	// SessionStateUpdateConcurrentWait is the wait time of the session state update to avoid concurrent update.
	SessionStateUpdateConcurrentWait = 300 * time.Millisecond

	// ApproxRunesPerToken matches the default in model.SimpleTokenCounter.
	ApproxRunesPerToken = 4.0
)

// agent state key
const (
	// SessionStateLastIncludedTS is the session state key the framework's summary checkers
	// use to track which events have already been included in a summary.
	SessionStateLastIncludedTS = "summary:last_included_ts"

	// SessionBkBizIDStateKey is the runtime-state key used to propagate the session's
	// bk_biz_id into the Graph run, allowing Function nodes (e.g. fetch_plans) to
	// consume it directly without asking the user.
	SessionBkBizIDStateKey = "bk_biz_id"

	// SessionBkBizIDTempKey is the session temp-state key written by the BeforeModel
	// callback so that the instruction placeholder {temp:session_bk_biz_id?} is
	// expanded before each LLM call.
	SessionBkBizIDTempKey = "session_bk_biz_id"

	// SessionUserDisplayNameKey is the session user-state key written by the BeforeModel
	// callback so that the instruction placeholder {user:display_name?} is
	// expanded before each LLM call.
	SessionUserDisplayNameKey = "display_name"

	// RetrievedToolsCacheKey is the cache key for the retrieved tools.
	RetrievedToolsCacheKey = "custom:retrieved_tools"

	// AvailableSkillsInjected is the session state key for the available skills injected.
	AvailableSkillsInjected = "available_skills_injected"
)

// StateKeyIntent is the graph state key for the intent recognised in the current turn.
// Values are enumor.IntentType strings (e.g. "host_apply", "chat", "resource_query").
const StateKeyIntent = "intent"

// StateKeySessionTag is the graph state key for the session-level scene tag.
// 会话级场景标签，区别于本轮意图 StateKeyIntent；取值为 enumor.IntentType 字符串。
const StateKeySessionTag = "session_tag"

// ForwardedPropSessionTag is the forwardedProps key used to pass the session tag into a graph run.
const ForwardedPropSessionTag = "sessionTag"

// HITL (Human-in-the-Loop) constants
const (
	// HumanConfirmToolName is the name of the human confirmation tool.
	// LLM calls this tool when it needs user confirmation or choice.
	HumanConfirmToolName = "human_confirm"

	// HITLInterruptKey is the key used for graph.Interrupt in HITL flow.
	// This key is used to identify the interrupt in ResumeMap.
	HITLInterruptKey = "human_confirm"
	// InterruptKeySeparator separates interrupt key parts.
	InterruptKeySeparator = ":"

	// FallbackInterruptKey is the key used for graph.Interrupt in fallback flow.
	// When LLM responds without tool calls, fallback pauses here until the user sends the next message.
	FallbackInterruptKey = "fallback"
	// FallbackInterruptKeyHashLen is the short hash length used in fallback interrupt key.
	FallbackInterruptKeyHashLen = 16
)

// bkaidev
const (
	// BKAIDEVGroupTypeSpace is the space group type for bkaidev api.
	BKAIDEVGroupTypeSpace = "space"
)

// prompt-key
const (
	// SystemPromptKey is the well-known prompt key injected as the LLM system message.
	SystemPromptKey = "system_prompt"
	// InstructionKey is the ll-known prompt key appended after the system message.
	InstructionKey = "instruction_prompt"
	// MemoryExtractPromptKey is the well-known prompt key used by the memory extractor.
	MemoryExtractPromptKey = "memory_extract_prompt"
	// IntentRecognitionPromptKey is the well-known prompt key used by the intent recognition.
	IntentRecognitionPromptKey = "intent_recognition_prompt"
)

// PermissionDeniedMsg is the message for permission denied.
const PermissionDeniedMsg = "当前用户无权限执行该工具，请联系管理员或确认工具可见范围"
