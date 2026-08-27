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

package enumor

import (
	"fmt"
	"slices"
)

// AIModel is the identifier of an AI language model supported by the platform.
type AIModel string

// Validate checks whether the model name is one of the declared supported models.
func (m AIModel) Validate() error {
	switch m {
	case AIModelDeepSeekV3, AIModelDSV32Thinking, AIModelDSV32,
		AIModelDeepSeekR1, AIModelHunyuan2Thinking, AIModelHunyuanTurbo,
		AIModelGPTOSS120B, AIModelQwen3, AIModelQwen3NoThinking:
		return nil
	default:
		return fmt.Errorf("unsupported AI model: %s", m)
	}
}

const (
	// AIModelDeepSeekV3 is DeepSeek-V3.
	AIModelDeepSeekV3 AIModel = "deepseek-v3"
	// AIModelDSV32Thinking is DSV32 with thinking mode enabled.
	AIModelDSV32Thinking AIModel = "dsv32-thinking"
	// AIModelDSV32 is DSV32.
	AIModelDSV32 AIModel = "dsv32"
	// AIModelDeepSeekR1 is DeepSeek-R1.
	AIModelDeepSeekR1 AIModel = "deepseek-r1"
	// AIModelHunyuan2Thinking is Tencent Hunyuan2 with thinking mode enabled.
	AIModelHunyuan2Thinking AIModel = "hunyuan2-thinking"
	// AIModelHunyuanTurbo is Tencent Hunyuan Turbo.
	AIModelHunyuanTurbo AIModel = "hunyuan-turbo"
	// AIModelGPTOSS120B is GPT-OSS 120B.
	AIModelGPTOSS120B AIModel = "gptoss-120b"
	// AIModelQwen3 is Qwen3 (thinking mode enabled by default).
	AIModelQwen3 AIModel = "qwen3"
	// AIModelQwen3NoThinking is Qwen3 with thinking mode disabled.
	AIModelQwen3NoThinking AIModel = "qwen3-nothinking"
)

// DefaultAllowedAIModels is the platform default list of allowed AI models,
// used when no explicit allowedModels list is configured.
var DefaultAllowedAIModels = []AIModel{
	AIModelDeepSeekV3,
	AIModelDSV32Thinking,
	AIModelDSV32,
	AIModelDeepSeekR1,
	AIModelHunyuan2Thinking,
	AIModelHunyuanTurbo,
	AIModelGPTOSS120B,
	AIModelQwen3,
	AIModelQwen3NoThinking,
}

// AgentModelProviderType is the type of the agent model provider.
type AgentModelProviderType string

const (
	// AgentModelProviderTypeBKAPIGW is the type of the BK API gateway provider.
	AgentModelProviderTypeBKAPIGW AgentModelProviderType = "bkapigw"
	// AgentModelProviderTypeOpenAI is the type of the OpenAI provider.
	AgentModelProviderTypeOpenAI AgentModelProviderType = "openai"
)

// Validate validates the agent model provider type.
func (t AgentModelProviderType) Validate() error {
	switch t {
	case AgentModelProviderTypeBKAPIGW:
	case AgentModelProviderTypeOpenAI:
	default:
		return fmt.Errorf("unsupported agent model provider type: %s", t)
	}
	return nil
}

type AgentMode string

const (
	// AgentModeAgent is the agent mode.
	AgentModeAgent AgentMode = "agent"
	// AgentModeGraph is the graph mode.
	AgentModeGraph AgentMode = "graph"
)

// Validate validates the agent mode.
func (m AgentMode) Validate() error {
	switch m {
	case AgentModeAgent, AgentModeGraph:
	default:
		return fmt.Errorf("unsupported agent mode: %s", m)
	}
	return nil
}

// IntentType represents the user's intent category recognised by the intent recognition node.
type IntentType string

const (
	// IntentTypeHostApply indicates the user wants to apply for host resources.
	IntentTypeHostApply IntentType = "host_apply"
	// IntentTypeResourceQuery indicates the user wants to query resource information.
	IntentTypeResourceQuery IntentType = "resource_query"
	// IntentTypeChat indicates the user is engaging in general conversation.
	IntentTypeChat IntentType = "chat"
	// IntentTypeUnsupported 表示本轮意图无法归入任何已识别的类别，是意图分类的降级取值。
	//
	// 它不是一个真实场景：模型不会输出该值，它也不会被写回 session_tag，因此既不在 IntentTypes 中，
	// 也不被 Validate 接受。分类不可用（提示词缺失、LLM 调用失败、响应无法识别等）时用它替代 chat，
	// 使「分类失败」与「用户确在闲聊」两种语义分离——否则一旦 chat 被实现为受支持场景，
	// 所有降级路径都会被 IsSupportedScene 判为可切换，导致分类失败时误切场景。
	IntentTypeUnsupported IntentType = "unsupported"
)

// IntentTypes lists all recognised intent categories.
var IntentTypes = []IntentType{
	IntentTypeHostApply,
	IntentTypeResourceQuery,
	IntentTypeChat,
}

// GetAllIntentTypes returns a copy of all intent types to keep the internal slice immutable.
func GetAllIntentTypes() []IntentType {
	return slices.Clone(IntentTypes)
}

// Validate checks whether the intent type is one of the recognised categories.
// IntentTypeUnsupported 是内部降级取值，不属于合法的分类结果，故不在此放行。
func (t IntentType) Validate() error {
	switch t {
	case IntentTypeHostApply, IntentTypeResourceQuery, IntentTypeChat:
		return nil
	default:
		return fmt.Errorf("unsupported intent type: %s", t)
	}
}

// IsSupportedScene reports whether the intent has an implemented scene flow.
func (t IntentType) IsSupportedScene() bool {
	switch t {
	case IntentTypeHostApply, IntentTypeResourceQuery:
		return true
	default:
		return false
	}
}

// GraphCheckpointBackend is the backend type for the graph checkpoint storage.
type GraphCheckpointBackend string

const (
	// GraphCheckpointBackendInMemory is the in-memory checkpoint backend.
	GraphCheckpointBackendInMemory GraphCheckpointBackend = "inmemory"
	// GraphCheckpointBackendSQLite is the SQLite checkpoint backend.
	GraphCheckpointBackendSQLite GraphCheckpointBackend = "sqlite"
	// GraphCheckpointBackendRedis is the Redis checkpoint backend.
	GraphCheckpointBackendRedis GraphCheckpointBackend = "redis"
)

// Validate validates the graph checkpoint backend.
func (b GraphCheckpointBackend) Validate() error {
	switch b {
	case GraphCheckpointBackendInMemory, GraphCheckpointBackendSQLite, GraphCheckpointBackendRedis:
	default:
		return fmt.Errorf("unsupported graph checkpoint backend: %s", b)
	}
	return nil
}

// MCPFilterMode is the mode of the MCP filter.
type MCPFilterMode string

const (
	// MCPFilterModeInclude is the include mode of the MCP filter.
	MCPFilterModeInclude MCPFilterMode = "include"
	// MCPFilterModeExclude is the exclude mode of the MCP filter.
	MCPFilterModeExclude MCPFilterMode = "exclude"
)

// Validate validates the MCP filter mode.
func (m MCPFilterMode) Validate() error {
	switch m {
	case MCPFilterModeInclude, MCPFilterModeExclude:
	default:
		return fmt.Errorf("unsupported MCP filter mode: %s", m)
	}
	return nil
}

// MainGraphAgentNode is the name of a built-in agent node in the main graph.
type MainGraphAgentNode string

const (
	// MainGraphAgentNodeSceneDispatch is the scene dispatch routing hub node.
	// 意图识别已并入该节点（intent.Classify），主图不再有独立的 intent_recognition 节点。
	MainGraphAgentNodeSceneDispatch MainGraphAgentNode = "scene_dispatch"
	// MainGraphAgentNodeFallback is the fallback interrupt node.
	MainGraphAgentNodeFallback MainGraphAgentNode = "fallback"
)

// Validate validates the main graph agent node.
func (n MainGraphAgentNode) Validate() error {
	switch n {
	case MainGraphAgentNodeSceneDispatch, MainGraphAgentNodeFallback:
		return nil
	default:
		return fmt.Errorf("unsupported main graph agent node: %s", n)
	}
}

// SubgraphAgentNode is the name of a subgraph agent node in the main graph
// (AddSubgraphNode / AddAgentNode).
type SubgraphAgentNode string

const (
	// SubgraphAgentNodeHostApply is the host apply subgraph agent node.
	SubgraphAgentNodeHostApply SubgraphAgentNode = "host_apply"
	// SubgraphAgentNodeResourceQuery is the resource query subgraph agent node.
	SubgraphAgentNodeResourceQuery SubgraphAgentNode = "resource_query"
)

// Validate validates the subgraph agent node.
func (n SubgraphAgentNode) Validate() error {
	switch n {
	case SubgraphAgentNodeHostApply, SubgraphAgentNodeResourceQuery:
		return nil
	default:
		return fmt.Errorf("unsupported subgraph agent node: %s", n)
	}
}

// IsSubgraphAgentNode reports whether nodeID is a main-graph subgraph agent node.
// Interrupt metadata on these nodes is a propagated copy from an inner subgraph node
// that has already reported the interrupt.
func IsSubgraphAgentNode(nodeID string) bool {
	return SubgraphAgentNode(nodeID).Validate() == nil
}

// ResourceQueryNode is the name of a graph node in the resource query workflow.
type ResourceQueryNode string

const (
	// ResourceQueryNodeLLM is the LLM node.
	ResourceQueryNodeLLM ResourceQueryNode = "llm"
	// ResourceQueryNodeFallback is the fallback node.
	ResourceQueryNodeFallback ResourceQueryNode = "fallback"
	// ResourceQueryNodeTool is the tool node.
	ResourceQueryNodeTool ResourceQueryNode = "tool"
	// ResourceQueryNodeHITL is the human in the loop node.
	ResourceQueryNodeHITL ResourceQueryNode = "hitl"
)

// Validate validates the resource query node.
func (n ResourceQueryNode) Validate() error {
	switch n {
	case ResourceQueryNodeLLM, ResourceQueryNodeFallback, ResourceQueryNodeTool, ResourceQueryNodeHITL:
		return nil
	default:
		return fmt.Errorf("unsupported resource query node: %s", n)
	}
}

// CvmApplyNode is the name of a graph node in the CVM apply workflow.
type CvmApplyNode string

const (
	// CvmApplyNodeLLM is the LLM node.
	CvmApplyNodeLLM CvmApplyNode = "llm"
	// CvmApplyNodeAccountSelect is the account selection node.
	CvmApplyNodeAccountSelect CvmApplyNode = "account_select"
	// CvmApplyNodeFallback is the fallback node.
	CvmApplyNodeFallback CvmApplyNode = "fallback"
	// CvmApplyNodeTool is the tool node.
	CvmApplyNodeTool CvmApplyNode = "tool"
	// CvmApplyNodeAfterToolHITL is the after-tool human-in-the-loop node, which interrupts
	// after recommend tools to let the user pick a plan.
	CvmApplyNodeAfterToolHITL CvmApplyNode = "after_tool_hitl"
	// CvmApplyNodeHITL is the human-in-the-loop node in the cvm subgraph.
	CvmApplyNodeHITL CvmApplyNode = "hitl"
)

// Validate validates the CVM apply node.
func (n CvmApplyNode) Validate() error {
	switch n {
	case CvmApplyNodeLLM, CvmApplyNodeAccountSelect, CvmApplyNodeFallback, CvmApplyNodeTool, CvmApplyNodeAfterToolHITL,
		CvmApplyNodeHITL:
		return nil
	default:
		return fmt.Errorf("unsupported CVM apply node: %s", n)
	}
}

// ToolName is the name of an MCP tool.
type ToolName string

const (
	// ToolNameRecommendByStatic 离线偏好 + 库存在线推荐。
	ToolNameRecommendByStatic ToolName = "get_biz_apply_recommend_by_static"
	// ToolNameRecommendByPlan 预测余量 + 库存在线推荐。
	ToolNameRecommendByPlan ToolName = "get_biz_apply_recommend_by_plan"
	// ToolNameRecommendSplitSuborder 主机申请单据拆分试算。
	ToolNameRecommendSplitSuborder ToolName = "get_biz_apply_recommend_split_suborder"
)

// Validate validates the tool name.
func (n ToolName) Validate() error {
	switch n {
	case ToolNameRecommendByStatic, ToolNameRecommendByPlan, ToolNameRecommendSplitSuborder:
		return nil
	default:
		return fmt.Errorf("unsupported tool name: %s", n)
	}
}

// IsRecommend 判断是否为推荐类工具
func (n ToolName) IsRecommend() bool {
	return n == ToolNameRecommendByStatic || n == ToolNameRecommendByPlan || n == ToolNameRecommendSplitSuborder
}

// DeclarativeToolName is the name of a locally declared agent tool
// (not routed through MCP tool_proxy), such as human_confirm / select_account.
type DeclarativeToolName string

const (
	// DeclToolHumanConfirm is the human confirmation tool.
	// LLM calls this tool when it needs user confirmation or choice.
	DeclToolHumanConfirm DeclarativeToolName = "human_confirm"
	// DeclToolSelectAccount 是模型在多账号场景下上报所选 account_id 的本地声明工具名，
	// 与 skill_load / human_confirm 同类（不经 tool_proxy），供后端校验并持久化选中账号。
	DeclToolSelectAccount DeclarativeToolName = "select_account"
)

// Validate validates the declarative tool name.
func (n DeclarativeToolName) Validate() error {
	switch n {
	case DeclToolHumanConfirm, DeclToolSelectAccount:
		return nil
	default:
		return fmt.Errorf("unsupported declarative tool name: %s", n)
	}
}

// AiagentRunState is the lifecycle state of an AI agent run used as metric labels.
type AiagentRunState string

const (
	// AiagentRunStateRunning is the in-flight run status. It is not a run_total
	// state; in-progress runs are exposed by run_inflight.
	AiagentRunStateRunning AiagentRunState = "running"
	// AiagentRunStateFinished is the run_total state when AG-UI emits RUN_FINISHED.
	AiagentRunStateFinished AiagentRunState = "finished"
	// AiagentRunStateError is the run_total state when AG-UI emits RUN_ERROR.
	AiagentRunStateError AiagentRunState = "error"
	// AiagentRunStateUnknown is the run_total state written only by orphan sweep.
	AiagentRunStateUnknown AiagentRunState = "unknown"
)

// Validate checks whether the run state is one of the declared values.
func (s AiagentRunState) Validate() error {
	switch s {
	case AiagentRunStateRunning, AiagentRunStateFinished, AiagentRunStateError, AiagentRunStateUnknown:
		return nil
	default:
		return fmt.Errorf("unsupported aiagent run state: %s", s)
	}
}
