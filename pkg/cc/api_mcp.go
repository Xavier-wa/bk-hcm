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

package cc

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"hcm/pkg/criteria/constant"
)

// MCPServerSetting 描述 api-server 的全部 MCP 子系统配置。
//
// 包含三段彼此独立的子配置：
//   - Ingress：对外部提供服务的 MCP ingress（外部 OpenClaw → 蓝鲸网关 → api-server → agent-server A2A）；
//   - Bridge：MCP↔A2A 桥接的下行 A2A client 参数（仅在 Ingress.Enable=true 时生效；
//     agent-server 实例地址通过 etcd 服务发现 cc.AgentServerName 获取，不在 yaml 中手填）；
//   - Internal：对内部提供服务的 HCM MCP server（agent-server LLM → api-server 本地 MCP server）。
//
// 三段默认全部关闭（Enable=false），对存量部署零影响。
type MCPServerSetting struct {
	Ingress  MCPIngressSetting  `yaml:"ingress"`
	Bridge   MCPBridgeSetting   `yaml:"bridge"`
	Internal MCPInternalSetting `yaml:"internal"`
}

// trySetDefault 为 MCP 子配置补齐默认值。
func (m *MCPServerSetting) trySetDefault() {
	m.Ingress.trySetDefault()
	m.Bridge.trySetDefault()
	m.Internal.trySetDefault()
}

// Validate 校验 MCP 子配置。
//
// 当 Internal.Enable=true 时，Internal.SchemaSync.GatewayURL 必填（蓝鲸网关 MCP-proxy
// 不在 etcd 服务发现范畴，必须手填）。
// 其它情形（含 Ingress.Enable=true）依赖 etcd 服务发现 agent-server，不在配置层校验。
func (m MCPServerSetting) Validate() error {
	if err := m.Ingress.Validate(); err != nil {
		return fmt.Errorf("ingress: %w", err)
	}

	if err := m.Bridge.Validate(); err != nil {
		return fmt.Errorf("bridge: %w", err)
	}

	if err := m.Internal.Validate(); err != nil {
		return fmt.Errorf("internal: %w", err)
	}

	return nil
}

// MCPIngressSetting 描述对外部提供服务的 MCP ingress 配置。
type MCPIngressSetting struct {
	// Enable 控制是否挂载对外部提供服务的 MCP ingress 路径。默认 false。
	Enable bool `yaml:"enable"`
	// BasePath 是对外部提供服务的 ingress 路径前缀。
	// 完整路径 = <basePath>/<mcp_server_name>/mcp/。
	// 默认 constant.MCPIngressBasePathDefault（"/api/v1/mcp/servers"）。
	BasePath string `yaml:"basePath"`
	// ServerName 是 MCP server 的标识名，用于 initialize 响应中的 serverInfo.name。
	// 默认 "hcm-agent"。
	ServerName string `yaml:"serverName"`
	// ServerVersion 是 MCP server 版本号，写入 initialize 响应。默认 "1.0.0"。
	ServerVersion string `yaml:"serverVersion"`
	// AggregatedToolName 是对外暴露的唯一聚合工具名。
	// 默认 constant.MCPIngressAggregatedToolName（"send_message"）。
	AggregatedToolName string `yaml:"aggregatedToolName"`
	// TextMaxLength 是 send_message 工具 text 入参最大字符长度，0 表示不限制。默认 8000。
	TextMaxLength int `yaml:"textMaxLength"`
	// ProgressTaskMapMaxSize 是 progressToken↔taskId 映射表最大条目数，默认 10000。
	ProgressTaskMapMaxSize int `yaml:"progressTaskMapMaxSize"`
	// ProgressTaskMapEntryTTL 是映射表单条记录的 TTL，到期未清理由后台清扫淘汰。默认 30m。
	ProgressTaskMapEntryTTL time.Duration `yaml:"progressTaskMapEntryTTL"`
}

// trySetDefault 为对外部提供服务的 ingress 补齐默认值。
func (s *MCPIngressSetting) trySetDefault() {
	if strings.TrimSpace(s.BasePath) == "" {
		s.BasePath = constant.MCPIngressBasePathDefault
	}
	if strings.TrimSpace(s.ServerName) == "" {
		s.ServerName = defaultA2ACardName
	}
	if strings.TrimSpace(s.ServerVersion) == "" {
		s.ServerVersion = defaultA2ACardVersion
	}
	if strings.TrimSpace(s.AggregatedToolName) == "" {
		s.AggregatedToolName = constant.MCPIngressAggregatedToolName
	}
	if s.TextMaxLength <= 0 {
		s.TextMaxLength = 8000
	}
	if s.ProgressTaskMapMaxSize <= 0 {
		s.ProgressTaskMapMaxSize = 10000
	}
	if s.ProgressTaskMapEntryTTL <= 0 {
		s.ProgressTaskMapEntryTTL = 30 * time.Minute
	}
}

// Validate 校验对外部提供服务的 ingress 配置。Enable=false 时跳过严格校验。
func (s MCPIngressSetting) Validate() error {
	if !s.Enable {
		return nil
	}
	if strings.TrimSpace(s.BasePath) == "" {
		return errors.New("basePath is empty")
	}
	if !strings.HasPrefix(s.BasePath, "/") {
		return fmt.Errorf("basePath must start with '/': %q", s.BasePath)
	}
	if strings.TrimSpace(s.AggregatedToolName) == "" {
		return errors.New("aggregatedToolName is empty")
	}
	return nil
}

// MCPBridgeSetting 描述 MCP↔A2A 桥接的下行 A2A client 参数。
//
// agent-server 实例地址通过 etcd 服务发现 cc.AgentServerName 获取，不在 yaml 中手填；
// 调用路径段固定为 <constant.A2ABasePathDefault><constant.A2AJSONRPCSubPath>（"/api/v1/agent/a2a"）。
type MCPBridgeSetting struct {
	// ConnectTimeout 是 HTTP 连接超时。默认 5s。
	ConnectTimeout time.Duration `yaml:"connectTimeout"`
	// ReadTimeout 是 A2A 长会话单次读超时。默认 300s。
	ReadTimeout time.Duration `yaml:"readTimeout"`
	// MaxIdleConnsPerHost 是 keep-alive 连接池大小。默认 100。
	MaxIdleConnsPerHost int `yaml:"maxIdleConnsPerHost"`
}

// trySetDefault 为 bridge 补齐默认值。
func (s *MCPBridgeSetting) trySetDefault() {
	if s.ConnectTimeout <= 0 {
		s.ConnectTimeout = 5 * time.Second
	}
	if s.ReadTimeout <= 0 {
		s.ReadTimeout = 300 * time.Second
	}
	if s.MaxIdleConnsPerHost <= 0 {
		s.MaxIdleConnsPerHost = 100
	}
}

// Validate 校验 bridge 配置。当前无强校验项（超时与连接池均有合理默认值）。
func (s MCPBridgeSetting) Validate() error {
	return nil
}

// MCPInternalSetting 描述对内部提供服务的 HCM MCP server 配置。
//
// 该 MCP server 仅供 agent-server LLM tool calling 调用，**不**对外暴露。
// 部署侧 SHALL 通过 K8s NetworkPolicy 或蓝鲸网关 Path Strip 限制外部访问。
type MCPInternalSetting struct {
	// Enable 控制是否挂载对内部提供服务的 MCP server 路径。默认 false。
	Enable bool `yaml:"enable"`
	// BasePath 是对内部提供服务的 MCP server 路径前缀。
	// 默认 constant.MCPInternalBasePathDefault（"/api/v1/mcp/internal/hcm/mcp"）。
	BasePath string `yaml:"basePath"`
	// ServerName 是 MCP server 标识名。默认 "hcm-internal-mcp"。
	ServerName string `yaml:"serverName"`
	// ServerVersion 是 MCP server 版本。默认 "1.0.0"。
	ServerVersion string `yaml:"serverVersion"`
	// EnforceCallerSource 是否强制要求请求携带 X-Bkhcm-Caller-Source: agent-server。
	// 默认 true（对内部提供服务的 MCP 必须严格鉴权）。
	EnforceCallerSource bool `yaml:"enforceCallerSource"`
	// SchemaSync 控制从蓝鲸网关 MCP-proxy 同步工具 schema 的策略。
	SchemaSync MCPSchemaSyncSetting `yaml:"schemaSync"`
	// InternalOnlyTools 是仅在对内部提供服务的 MCP 暴露的内部专用工具名列表，
	// 不会从蓝鲸网关同步，schema 由 api-server 本地代码维护。
	InternalOnlyTools []string `yaml:"internalOnlyTools"`
	// ToolFilter 是从网关同步结果中需要排除的工具名列表。
	ToolFilter []string `yaml:"toolFilter"`
}

// trySetDefault 为对内部提供服务的 MCP server 补齐默认值。
func (s *MCPInternalSetting) trySetDefault() {
	if strings.TrimSpace(s.BasePath) == "" {
		s.BasePath = constant.MCPInternalBasePathDefault
	}
	if strings.TrimSpace(s.ServerName) == "" {
		s.ServerName = constant.MCPInternalDefaultServerName
	}
	if strings.TrimSpace(s.ServerVersion) == "" {
		s.ServerVersion = constant.MCPInternalServerDefaultVersion
	}
	// EnforceCallerSource 默认 true，但仅在解析为零值（false）时强制开启。
	// 用户显式配置 false 时不覆盖。注：bool 字段无法区分"未配置"与"显式 false"，
	// 因此对内部提供服务的 MCP 的安全默认值在 service.go ApiServerSetting.trySetDefault 阶段处理。
	s.SchemaSync.trySetDefault()
}

// Validate 校验对内部提供服务的 MCP 配置。Enable=false 时跳过。
func (s MCPInternalSetting) Validate() error {
	if !s.Enable {
		return nil
	}
	if strings.TrimSpace(s.BasePath) == "" {
		return errors.New("basePath is empty")
	}
	if !strings.HasPrefix(s.BasePath, "/") {
		return fmt.Errorf("basePath must start with '/': %q", s.BasePath)
	}
	if err := s.SchemaSync.Validate(); err != nil {
		return fmt.Errorf("schemaSync: %w", err)
	}
	return nil
}

// MCPSchemaSyncSetting 描述从蓝鲸网关 MCP-proxy 同步工具 schema 的策略。
type MCPSchemaSyncSetting struct {
	// GatewayURL 是蓝鲸网关 MCP-proxy 的 tools/list 端点 URL。
	// 对内部提供服务的 MCP（internal.enable=true）时必填。
	GatewayURL string `yaml:"gatewayURL"`
	// Interval 是周期同步间隔。默认 5m。
	Interval time.Duration `yaml:"interval"`
	// Timeout 是单次同步调用的超时时间。默认 30s。
	Timeout time.Duration `yaml:"timeout"`
	// CachePath 是本地缓存文件路径，启动期从此处加载已知 schema 立即提供服务。
	// 默认 "/tmp/hcm-api-server-mcp-schema-cache.json"。
	CachePath string `yaml:"cachePath"`
	// StaleAlertAfter 是连续同步失败多久后标记 schema 为 stale 并暴露告警 metric。
	// 默认 30m。
	StaleAlertAfter time.Duration `yaml:"staleAlertAfter"`
}

// trySetDefault 为 schema 同步补齐默认值。
func (s *MCPSchemaSyncSetting) trySetDefault() {
	if s.Interval <= 0 {
		s.Interval = 5 * time.Minute
	}
	if s.Timeout <= 0 {
		s.Timeout = 30 * time.Second
	}
	if strings.TrimSpace(s.CachePath) == "" {
		s.CachePath = "/tmp/hcm-api-server-mcp-schema-cache.json"
	}
	if s.StaleAlertAfter <= 0 {
		s.StaleAlertAfter = 30 * time.Minute
	}
}

// Validate 校验 schema 同步配置。仅在 internal.enable=true 时被调用。
func (s MCPSchemaSyncSetting) Validate() error {
	if strings.TrimSpace(s.GatewayURL) == "" {
		return errors.New("gatewayURL is empty (required when mcp.internal.enable=true)")
	}
	if !strings.HasPrefix(s.GatewayURL, "http://") && !strings.HasPrefix(s.GatewayURL, "https://") {
		return fmt.Errorf("gatewayURL must start with http:// or https://: %q", s.GatewayURL)
	}
	return nil
}

// A2APassthroughSetting 描述 A2A 透传配置。
//
// 当 Enable=true 时，api-server 在外层 mux 上以反向代理形式暴露 agent-server 的
// /api/v1/agent/a2a JSON-RPC 端点与 /api/v1/agent/.well-known/* AgentCard 端点，
// 使原生 A2A 客户端能通过 api-server 与 HCM agent 通信。
//
// agent-server 实例地址通过 etcd 服务发现 cc.AgentServerName 获取（与 web-server proxy
// 同源），不在 yaml 中手填。该路径与对外部提供服务的 MCP ingress 独立，可单独启用 / 关闭。
// 默认 Enable=false。
type A2APassthroughSetting struct {
	// Enable 控制是否挂载 A2A 透传路径。默认 false。
	Enable bool `yaml:"enable"`
	// BasePath 是 A2A 端点的路径前缀，默认 constant.A2ABasePathDefault（"/api/v1/agent"）。
	// 反代将该前缀下的 /a2a 与 /.well-known/* 子路径透传到 agent-server 同路径。
	BasePath string `yaml:"basePath"`
	// FlushIntervalMS 是反向代理 SSE flush 间隔，单位毫秒；
	// 负数表示每次写入立即 flush（推荐 -1）。默认 -1。
	FlushIntervalMS int `yaml:"flushIntervalMS"`
	// IdleTimeout 是反向代理底层 HTTP 连接空闲超时。默认 600s（支持长会话）。
	IdleTimeout time.Duration `yaml:"idleTimeout"`
}

// trySetDefault 为 A2A 透传补齐默认值。
func (s *A2APassthroughSetting) trySetDefault() {
	if strings.TrimSpace(s.BasePath) == "" {
		s.BasePath = constant.A2ABasePathDefault
	}
	if s.FlushIntervalMS == 0 {
		s.FlushIntervalMS = -1
	}
	if s.IdleTimeout <= 0 {
		s.IdleTimeout = 600 * time.Second
	}
}

// Validate 校验 A2A 透传配置。Enable=false 时跳过。
func (s A2APassthroughSetting) Validate() error {
	if !s.Enable {
		return nil
	}
	if strings.TrimSpace(s.BasePath) == "" {
		return errors.New("basePath is empty")
	}
	if !strings.HasPrefix(s.BasePath, "/") {
		return fmt.Errorf("basePath must start with '/': %q", s.BasePath)
	}
	return nil
}
