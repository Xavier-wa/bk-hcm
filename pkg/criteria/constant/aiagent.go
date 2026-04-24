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
	DefaultProviderName = "aidev"
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

	// DefaultPreloadMemoryLimit loads the most recent N memories into the system prompt.
	DefaultPreloadMemoryLimit = 20
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

	// ApproxRunesPerToken matches the default in model.SimpleTokenCounter.
	ApproxRunesPerToken = 4.0
)

// agent state key
const (
	// SessionStateLastIncludedTS is the session state key the framework's summary checkers
	// use to track which events have already been included in a summary.
	SessionStateLastIncludedTS = "summary:last_included_ts"

	// RetrievedToolsCacheKey is the cache key for the retrieved tools.
	RetrievedToolsCacheKey = "custom:retrieved_tools"
)
