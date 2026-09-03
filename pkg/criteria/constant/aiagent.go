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

import (
	"fmt"
	"strings"
	"time"
)

// AG-UI
const (
	// AGUIPath is the HTTP path for the AG-UI endpoint.
	AGUIPath = "/api/v1/agent/agui"
	// AGUICancelPath is the HTTP path for the AG-UI cancel endpoint.
	AGUICancelPath = "/api/v1/agent/cancel"
	// AGUIHistoryPath is the HTTP path for the AG-UI history (MessagesSnapshot) endpoint.
	AGUIHistoryPath = "/api/v1/agent/history"
)

// MCPToolSetType is the enum-like type for MCP toolset category.
type MCPToolSetType string

// MCP
const (
	// MCPTypeBKAIDev enables automatic X-Bkapi-Authorization
	// header injection for BK AI Dev gateways.
	MCPTypeBKAIDev MCPToolSetType = "bkaidev"

	// MCPTypeInternal is for internal HCM MCP calls
	// (e.g. agent-server LLM → api-server 内置 HCM MCP).
	// 仅注入 X-Bkapi-User-Name（来自 request context 中的 bk_username），
	// 不注入 X-Bkapi-Authorization / bk_ticket / access_token；基于内网信任的纯内部微服务调用。
	MCPTypeInternal MCPToolSetType = "internal"

	// DefaultProviderName is the well-known provider name that the "aidev" config
	// section is mapped to. Models without an explicit provider use this one.
	DefaultProviderName = string(MCPTypeBKAIDev)
)

// Normalize returns canonical enum value for compatible inputs.
func (t MCPToolSetType) Normalize() MCPToolSetType {
	raw := strings.TrimSpace(string(t))
	switch {
	case raw == "":
		return ""
	case strings.EqualFold(raw, string(MCPTypeBKAIDev)):
		return MCPTypeBKAIDev
	case strings.EqualFold(raw, string(MCPTypeInternal)):
		return MCPTypeInternal
	default:
		return MCPToolSetType(raw)
	}
}

// Validate validates the MCP toolset type value.
func (t MCPToolSetType) Validate() error {
	nt := t.Normalize()
	if nt == "" {
		return nil
	}
	switch nt {
	case MCPTypeBKAIDev, MCPTypeInternal:
		return nil
	default:
		return fmt.Errorf("mcp type %q is invalid, expect one of [%q, %q]",
			string(t), string(MCPTypeBKAIDev), string(MCPTypeInternal))
	}
}

// IsBKAIDev reports whether type is bkaidev.
func (t MCPToolSetType) IsBKAIDev() bool {
	return t.Normalize() == MCPTypeBKAIDev
}

// MCP ingress / internal endpoint paths exposed by api-server.
const (
	// MCPIngressBasePathDefault 是对外部提供服务的 MCP ingress 的路径前缀。
	// 完整路径 = <basePath>/<mcp_server_name>/mcp/（streamable HTTP）。
	MCPIngressBasePathDefault = "/api/v1/mcp/servers"

	// MCPInternalDefaultServerName 是对内部提供服务的 HCM MCP server 的默认服务名称
	MCPInternalDefaultServerName = "hcm-internal-mcp"

	// MCPInternalBasePathDefault 是对内部提供服务的 HCM MCP server 的路径前缀，
	// 仅供 agent-server 等内部组件调用，外部不可达。
	MCPInternalBasePathDefault = "/api/v1/mcp/internal/hcm/mcp"

	// MCPIngressAggregatedToolName 是对外提供服务的 MCP ingress 对外暴露的唯一聚合工具名，
	// MCP 客户端通过该工具向 HCM agent 发送自然语言指令。
	MCPIngressAggregatedToolName = "send_message"

	// MCPInternalServerDefaultVersion 是对内部提供服务的 HCM MCP server的默认版本号。
	MCPInternalServerDefaultVersion = "1.0.0"
)

// 注：MCP/A2A 调用来源 header 名 MCPCallerSourceHeader 见 header.go；
// 取值字面量请使用 cc.APIServerName / cc.AgentServerName，避免重复维护。

// A2A protocol
const (
	// A2ABasePathDefault is the default base path for A2A endpoints, kept
	// consistent with the AG-UI endpoint prefix.
	A2ABasePathDefault = "/api/v1/agent"

	// A2AJSONRPCSubPath is the JSON-RPC entry sub-path under A2A basePath.
	// Final path = <basePath>/a2a (e.g. /api/v1/agent/a2a).
	A2AJSONRPCSubPath = "/a2a"

	// A2AWellKnownAgentCardPath is the A2A v0.2.2 AgentCard discovery path.
	A2AWellKnownAgentCardPath = "/.well-known/agent-card.json"

	// A2AWellKnownAgentLegacyPath is the legacy AgentCard discovery path kept
	// for client compatibility with A2A 0.1.x.
	A2AWellKnownAgentLegacyPath = "/.well-known/agent.json"

	// MCPBridgeDefaultConnectTimeout is the default timeout for establishing an A2A connection.
	MCPBridgeDefaultConnectTimeout = 5 * time.Second
	// MCPBridgeDefaultReadTimeout is the default timeout for an A2A streaming request.
	MCPBridgeDefaultReadTimeout = 300 * time.Second
	// MCPBridgeDefaultMaxIdleConnsPerHost is the default keep-alive connection pool size.
	MCPBridgeDefaultMaxIdleConnsPerHost = 100
	// MCPBridgeDefaultKeepAlive is the default keep-alive interval for an A2A connection.
	MCPBridgeDefaultKeepAlive = 30 * time.Second
	// MCPBridgeDefaultIdleConnTimeout is the default idle connection timeout.
	MCPBridgeDefaultIdleConnTimeout = 90 * time.Second
	// MCPBridgeDefaultTLSHandshakeTimeout is the default TLS handshake timeout.
	MCPBridgeDefaultTLSHandshakeTimeout = 10 * time.Second
	// MCPBridgeDefaultExpectContinueTimeout is the default timeout for an HTTP 100-continue response.
	MCPBridgeDefaultExpectContinueTimeout = time.Second
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

	// ProxyExecuteToolFullName is the LLM-facing name of the proxy execute_tool meta-tool,
	// composed as "<toolset>_<tool>" (e.g. "tool_proxy_execute_tool"). The LLM calls real MCP
	// tools through this meta-tool, carrying the real tool name in the "tool_name" argument.
	ProxyExecuteToolFullName = ProxyToolSetName + "_" + ExecuteToolToolName
	// ProxySearchToolsFullName is the LLM-facing name of the proxy search_tools meta-tool,
	// composed as "<toolset>_<tool>" (e.g. "tool_proxy_search_tools").
	ProxySearchToolsFullName = ProxyToolSetName + "_" + SearchToolsToolName
	// ProxyGetToolSchemaFullName is the LLM-facing name of the proxy get_tool_schema meta-tool,
	// composed as "<toolset>_<tool>" (e.g. "tool_proxy_get_tool_schema").
	ProxyGetToolSchemaFullName = ProxyToolSetName + "_" + GetToolSchemaToolName
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

	// DefaultLLMRequestBodyLogLimit is the fallback limit of the LLM request body log.
	// agent-server 未配置 llmRequestBodyLogLimit 时使用该值。
	DefaultLLMRequestBodyLogLimit = 16 * 1024
)

// history sync
const (
	// AiagentRunIDMaxLen is aiagent_run.run_id VARCHAR(64).
	AiagentRunIDMaxLen = 64
	// MaxHistorySyncSessionLimit caps session_codes per history sync call.
	MaxHistorySyncSessionLimit = 20
	// AiagentRunHistorySyncReason marks ledger rows created from session history.
	AiagentRunHistorySyncReason = "history_sync"
)

// session constant
const (
	// SessionCacheResolverCapacity is the capacity of the session cache resolver.
	SessionCacheResolverCapacity = 10000

	// SessionCacheResolverTTL is the TTL of the session cache resolver.
	SessionCacheResolverTTL = 30 * time.Minute

	// SessionIncrContentCountTimeout is the timeout of the session incr content count.
	SessionIncrContentCountTimeout = 5 * time.Second

	// SessionTagWriteBackTimeout is the timeout of the in-node session tag write-back.
	// 该回写在 scene_dispatch 节点内同步执行，超时上限直接计入本轮 run 的延迟，
	// 因此比 Run 结束后异步对账用的 SessionIncrContentCountTimeout 更短。
	SessionTagWriteBackTimeout = 3 * time.Second

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

	// SessionAccountIDTempKey is the session temp-state key written by the BeforeModel
	// callback so that the instruction placeholder {temp:account_id?} is
	// expanded before each LLM call.
	SessionAccountIDTempKey = "account_id"

	// SessionRidStateKey is the runtime-state key that carries the request id (rid)
	// from the HTTP context into the Graph run. It is injected at the run entry point
	// (makeRunOptionResolver) so that subgraph input/output mappers — which only
	// receive graph.State — can attach the rid to [hcm graph trace] logs for
	// cross-turn/cross-node correlation. Not persisted as business state.
	SessionRidStateKey = "session_rid"

	// SessionSelectedAccountIDStateKey 持久化 account_select 节点选定的账号，
	// 使其跨多次 run 保留，避免重复触发账号选择。
	SessionSelectedAccountIDStateKey = "cvm_apply:selected_account_id"

	// SessionUserDisplayNameKey is the session user-state key written by the BeforeModel
	// callback so that the instruction placeholder {user:display_name?} is
	// expanded before each LLM call.
	SessionUserDisplayNameKey = "display_name"

	// RetrievedToolsCacheKey is the cache key for the retrieved tools.
	RetrievedToolsCacheKey = "custom:retrieved_tools"

	// AvailableSkillsInjected is the session state key for the available skills injected.
	AvailableSkillsInjected = "available_skills_injected"
)

// StateKeySessionTag is the graph state key for the session-level scene tag.
// 会话级场景标签，是主图中「当前处于哪个场景」的唯一真相来源；取值为 enumor.IntentType 字符串。
// 每个轮次边界由 scene_dispatch 依据本轮意图分类结果重新提交；标签相对上一轮有变化时，
// scene_dispatch 在判定点同步回写 DB，Run 结束后 service 层的差异对账仅作兜底。
const StateKeySessionTag = "session_tag"

// StateKeySceneDispatchNext is the graph state key carrying the scene_dispatch routing decision.
// scene_dispatch 节点完成「当前会话标签 × 本轮意图分类结果」的判定后，把目标节点名写入该键；
// 条件边路由函数只按该键查表，不重复实现判定逻辑，避免两处判定随决策分支增多而发散。
const StateKeySceneDispatchNext = "scene_dispatch_next"

// SceneSwitchedCustomEventName is the AG-UI CUSTOM event name emitted by scene_dispatch when a
// session switches from one supported scene to another. 事件 payload 携带切换前后的场景，
// 供前端同步会话标签 UI。
const SceneSwitchedCustomEventName = "scene.switched"

// ForwardedPropSessionTag is the forwardedProps key used to pass the session tag into a graph run.
const ForwardedPropSessionTag = "sessionTag"

// ForwardedPropResumeValue is the forwardedProps key used by the frontend to pass structured
// resume data (e.g. a selected account_id) when resuming from an HITL interrupt.
const ForwardedPropResumeValue = "resumeValue"

// StateKeyForwardedResumeValue is the runtime-state key that carries the structured resume value
// from forwardedProps. It is kept separate from the resume command (which always carries the
// free-form user input text), so nodes can consume the structured selection independently.
const StateKeyForwardedResumeValue = "forwarded_resume_value"

// StateKeySubgraphTurnPrefix is the prefix of the per-subgraph turn counter state key.
// 子图按「轮次」分配独立 checkpoint namespace 时，需要一个严格单调递增的轮次序号，
// 用以替代「进入子图时的消息条数」这种脆弱标识（两轮消息数恰好相等会触发 namespace 碰撞、
// 误命中旧完成态 checkpoint）。该序号按 nodePrefix 隔离（避免 host_apply / resource_query 互相干扰），
// 完整 key 由 stateKeySubgraphTurn(nodePrefix) 生成，存储在父图 state 中并随子图 checkpoint 持久化，
// 由 makeSubgraphInputMapper 自增、makeSubgraphOutputMapper 经子图完成态回填父图，跨轮次保持递增。
const StateKeySubgraphTurnPrefix = "subgraph_turn_"

// HITL (Human-in-the-Loop) constants
const (
	// HITLInterruptKey is the key used for graph.Interrupt in HITL flow.
	// This key is used to identify the interrupt in ResumeMap.
	HITLInterruptKey = "hitl.interrupt"
	// InterruptKeySeparator separates interrupt key parts.
	InterruptKeySeparator = ":"

	// FallbackInterruptKey is the key used for graph.Interrupt in fallback flow.
	// When LLM responds without tool calls, fallback pauses here until the user sends the next message.
	// NOTE: fallback是唯一不产生 custom 事件的 interrupt
	FallbackInterruptKey = "fallback"
	// FallbackInterruptKeyHashLen is the short hash length used in fallback interrupt key.
	FallbackInterruptKeyHashLen = 16

	// AccountSelectInterruptKey is the key used for graph.Interrupt when multiple accounts are
	// detected and the user must choose one to proceed with the CVM apply workflow.
	AccountSelectInterruptKey = "account_select.interrupt"

	// AccountUnavailableEmitKey 是「当前业务无可用云账号」提示的 emit 去重键。
	// 该路径只发消息、不触发 interrupt，独立成键可避免复用 AccountSelectInterruptKey 时
	// 撞上账号卡片的 resume 记录，被误判为「恢复重放」而跳过提示。
	AccountUnavailableEmitKey = "account_select.no_usable_account"

	// AfterToolHITLRecommendSelectInterruptKey 推荐方案选择场景的中断 key
	AfterToolHITLRecommendSelectInterruptKey = "after_tool_hitl.recommend_select.interrupt"
	// AfterToolHITLRecommendSuborderConfirmInterruptKey 推荐方案拆单试算确认场景的中断key
	AfterToolHITLRecommendSuborderConfirmInterruptKey = "after_tool_hitl.recommend_suborder_confirm.interrupt"

	// AfterToolHITLResumeForwardedEventName 中断恢复时携带前端 forwardedProps 结构化回复的自定义事件名。
	AfterToolHITLResumeForwardedEventName = "after_tool_hitl.resume_forwarded"
)

// CVM apply graph state keys
const (
	// AccountSelectNextNodeKey is an internal routing key written by the account_select node so the
	// conditional edge function can decide the next node.
	AccountSelectNextNodeKey = "account_select.next"

	// StateKeyRecommendCandidates 累积推荐候选的 graph state key。
	StateKeyRecommendCandidates = "recommend:candidates"
)

// Apply recommend defaults consumed by the host_apply agent.
const (
	// DefaultRecommendLimit 推荐返回方案数缺省值。
	DefaultRecommendLimit = 5
)

// tool confirm gate constants
const (
	// ToolConfirmCreateCvmApplyInterruptKey is the interrupt key for create_cvm_apply confirm gate.
	ToolConfirmCreateCvmApplyInterruptKey = "tool_confirm.interrupt.create_cvm_apply"
	// ToolConfirmInterruptKeyPrefix is the key prefix used for graph.Interrupt in tool confirm gates.
	// The translator emits the "tool.confirm" custom event when an interrupt key carries this prefix.
	ToolConfirmInterruptKeyPrefix = "tool_confirm.interrupt"
	// ToolConfirmCustomEventName is the AG-UI custom event name carrying the tool confirm card payload.
	ToolConfirmCustomEventName = "tool.confirm"

	// StateKeyHITLRoute is the graph state key recording the next node the hitl node routes to
	// after resume. 取值为 enumor.HITLRouteTool / enumor.HITLRouteLLM。
	StateKeyHITLRoute = "hitl_route"

	// ToolNameCreateBizApply is the host apply submit tool guarded by the tool confirm gate.
	ToolNameCreateBizApply = "create_biz_apply"
)

// MCP / OpenClaw HITL confirm constants (northbound send_message).
const (
	// MCPConfirmActionConfirm resumes a tool.confirm interrupt and proceeds to execute the tool.
	MCPConfirmActionConfirm = "confirm"
	// MCPConfirmActionCancel resumes a tool.confirm interrupt and cancels the pending tool call.
	MCPConfirmActionCancel = "cancel"

	// MCPResultMetaConfirmKey is the CallToolResult._meta key that carries the structured
	// confirm payload when the agent graph is interrupted for HITL confirmation.
	MCPResultMetaConfirmKey = "confirm"

	// MCPConfirmPendingMessage is the human-readable content returned when a tool confirm
	// interrupt is waiting for the next send_message with a confirm payload.
	MCPConfirmPendingMessage = "需要您确认后才能继续执行。请使用同一 contextId 再次调用 send_message，" +
		"并在 confirm 参数中传入 {\"action\":\"confirm\",\"args\":<上一次返回的 _meta.confirm.data>} 确认，" +
		"或 {\"action\":\"cancel\"} 取消。"

	// MCPSelectPendingMessage is the human-readable content returned when a non-tool.confirm HITL
	// interrupt (account/recommendation/suborder selection) is waiting for the user's choice.
	// 这类中断以纯文本恢复：把可选项展示给用户，用同一 contextId 将用户选择以 text 续聊即可。
	MCPSelectPendingMessage = "需要先向用户展示以下可选项并由其确认后才能继续。请使用同一 contextId 再次调用 " +
		"send_message，将用户的选择以纯文本传入 text 即可（无需 confirm 参数）。"
)

// account select tool constants
const (
	// SelectAccountArgKey 是 select_account 工具的账号入参字段名。
	SelectAccountArgKey = "account_id"

	// SelectAccountRequiredMsg 是账号未解析时门禁返回给模型的引导文案，要求先调用 select_account。
	// 门禁默认拦截全部业务工具（含只读查询），文案不限定「申领」以免只读工具被拦时语义不符。
	SelectAccountRequiredMsg = "尚未选定云账号，请先调用 select_account 工具选定账号后再重新调用本工具"
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
	// EvalScopePromptKey is the well-known prompt key used by eval stage-1.
	EvalScopePromptKey = "eval_scope_prompt"
	// EvalRubricPromptKey is the well-known prompt key used by eval stage-2.
	EvalRubricPromptKey = "eval_rubric_prompt"
)

// eval score
const (
	// MinEvalScore is the inclusive lower bound of process/outcome/quality scores.
	MinEvalScore = 0
	// MaxEvalScore is the inclusive upper bound of process/outcome/quality scores.
	MaxEvalScore = 100
	// DefaultAiagentRunEvalGapLimit is the default page size for listing terminal
	// runs that have no eval row. Use AiagentRunEvalGapLimit instead of a raw 20.
	DefaultAiagentRunEvalGapLimit = 20
)

// AiagentRunEvalGapLimit returns limit, or DefaultAiagentRunEvalGapLimit when limit is 0.
func AiagentRunEvalGapLimit(limit uint) uint {
	if limit == 0 {
		return DefaultAiagentRunEvalGapLimit
	}
	return limit
}

// PromptSystemKey 按场景派生 system prompt 的 store key。
// scene 为空时返回默认场景的 SystemPromptKey；非空时返回 "{scene}_system_prompt"。
func PromptSystemKey(scene string) string {
	if scene == "" {
		return SystemPromptKey
	}
	return scene + "_system_prompt"
}

// PromptInstructionKey 按场景派生 instruction 的 store key。
// scene 为空时返回默认场景的 InstructionKey；非空时返回 "{scene}_instruction_prompt"。
func PromptInstructionKey(scene string) string {
	if scene == "" {
		return InstructionKey
	}
	return scene + "_instruction_prompt"
}

// PermissionDeniedMsg is the message for permission denied.
const PermissionDeniedMsg = "当前用户无权限执行该工具，请联系管理员或确认工具可见范围"

// NoPermissionFallbackMessage is the fallback reply shown when no available cloud account exists
// under the current biz, blocking the CVM apply workflow.
const NoPermissionFallbackMessage = "当前没有可用的云账号，请联系该业务管理员开通权限：[联系管理员](wxwork://message?username=HCM)"

// ExtractDataMaxDepth bounds the recursion when unwrapping nested result/data envelopes.
const ExtractDataMaxDepth = 5
