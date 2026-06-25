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
)

// IntentTypes lists all recognised intent categories.
var IntentTypes = []IntentType{
	IntentTypeHostApply,
	IntentTypeResourceQuery,
	IntentTypeChat,
}

// GetAllIntentTypes returns all intent types.
func GetAllIntentTypes() []IntentType {
	return IntentTypes
}

// Validate checks whether the intent type is one of the declared values.
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

// ResourceQueryNode is the name of a graph node in the resource query workflow.
type ResourceQueryNode string

const (
	// ResourceQueryGraphNode is the name of the graph node in the resource query workflow.
	ResourceQueryGraphNode ResourceQueryNode = "resource_query"
)

// HostApplyNode is the name of the host apply subgraph node in the main graph.
type HostApplyNode string

const (
	// HostApplyGraphNode is the name of the host apply subgraph node in the main graph.
	HostApplyGraphNode HostApplyNode = "host_apply"
)

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

)

// Validate validates the CVM apply node.
func (n CvmApplyNode) Validate() error {
	switch n {
	case CvmApplyNodeLLM, CvmApplyNodeAccountSelect, CvmApplyNodeFallback, CvmApplyNodeTool:
	default:
		return fmt.Errorf("unsupported CVM apply node: %s", n)
	}
	return nil
}
