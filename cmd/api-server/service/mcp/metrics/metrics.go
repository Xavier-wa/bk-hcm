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

// Package metrics 提供 api-server MCP / A2A 路径的 Prometheus 指标注册与访问入口。
//
// 完整指标名遵循 `<namespace>_<subsystem>_<name>` 格式：
//   - hcm_api_server_mcp_tools_call_total{tool, mcp_server_name, status}
//   - hcm_api_server_mcp_tools_call_duration_seconds{tool, mcp_server_name}
//   - hcm_api_server_mcp_bridge_active_tasks
//   - hcm_api_server_mcp_internal_schema_stale
//
// 调用方应在 api-server 启动期调用 InitMCPMetrics()，之后通过包级访问函数获取
// metric 对象进行 Observe / Inc / Set。
package metrics

import (
	"sync"

	"hcm/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

// 子系统名称：归到 hcm_api_server_mcp_* 命名空间。
const (
	// SubSysMCP 是对外部提供服务的 ingress + bridge 共用的指标子系统。
	SubSysMCP = "api_server_mcp"

	// SubSysMCPInternal 是对内部提供服务的 MCP server 专用的指标子系统。
	SubSysMCPInternal = "api_server_mcp_internal"
)

// 调用状态标签取值，作为 Counter 的 status 维度。
const (
	StatusSuccess    = "success"
	StatusError      = "error"
	StatusCanceled   = "canceled"
	StatusInvalidArg = "invalid_arg"
)

// mcpMetrics 持有本包注册的所有 metric 对象，启动期初始化后只读。
type mcpMetrics struct {
	toolsCallTotal           *prometheus.CounterVec
	toolsCallDurationSeconds *prometheus.HistogramVec
	bridgeActiveTasks        prometheus.Gauge
	internalSchemaStale      prometheus.Gauge
}

var (
	holder *mcpMetrics
	once   sync.Once
)

// InitMCPMetrics 注册全部 MCP 指标到全局 Prometheus registerer。
//
// 该函数幂等（sync.Once 保护），可在 app 启动时调用一次。
func InitMCPMetrics() {
	once.Do(func() {
		initWithRegisterer(metrics.Register())
	})
}

// initWithRegisterer 构造 holder 并把全部 metric 注册到指定 registerer。
// 拆出该函数主要服务两个场景：
//   - 生产路径：InitMCPMetrics → metrics.Register() 全局 registerer；
//   - 测试路径：传入独立 prometheus.NewRegistry()，避免与全局 registerer 冲突。
func initWithRegisterer(reg prometheus.Registerer) {
	holder = &mcpMetrics{
		toolsCallTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: metrics.Namespace,
				Subsystem: SubSysMCP,
				Name:      "tools_call_total",
				Help: "the total number of tools/call requests processed by " +
					"api-server MCP ingress, labeled by tool name, mcp_server_name " +
					"path variable and final status",
			},
			[]string{"tool", "mcp_server_name", "status"},
		),
		toolsCallDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: metrics.Namespace,
				Subsystem: SubSysMCP,
				Name:      "tools_call_duration_seconds",
				Help: "duration of tools/call from request entry to final " +
					"CallToolResult, including upstream A2A streaming time",
				Buckets: prometheus.ExponentialBuckets(0.05, 2, 12),
			},
			[]string{"tool", "mcp_server_name"},
		),
		bridgeActiveTasks: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: metrics.Namespace,
				Subsystem: SubSysMCP,
				Name:      "bridge_active_tasks",
				Help: "current size of the progressToken→taskId map in the " +
					"MCP↔A2A bridge; reflects the count of in-flight tools/call " +
					"that have not yet reached A2A terminal state",
			},
		),
		internalSchemaStale: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: metrics.Namespace,
				Subsystem: SubSysMCPInternal,
				Name:      "schema_stale",
				Help: "1 when the southbound MCP tool schema has not been " +
					"successfully synced from the BlueKing gateway MCP-proxy " +
					"for longer than the configured threshold; 0 when fresh",
			},
		),
	}

	reg.MustRegister(holder.toolsCallTotal)
	reg.MustRegister(holder.toolsCallDurationSeconds)
	reg.MustRegister(holder.bridgeActiveTasks)
	reg.MustRegister(holder.internalSchemaStale)
}



// IncToolsCall 累计一次 tools/call 调用。空 holder 安全（未初始化时静默 no-op）。
func IncToolsCall(tool, mcpServerName, status string) {
	if holder == nil {
		return
	}
	holder.toolsCallTotal.WithLabelValues(tool, mcpServerName, status).Inc()
}

// ObserveToolsCallDuration 记录一次 tools/call 的耗时（秒）。
func ObserveToolsCallDuration(tool, mcpServerName string, seconds float64) {
	if holder == nil {
		return
	}
	holder.toolsCallDurationSeconds.WithLabelValues(tool, mcpServerName).Observe(seconds)
}

// SetBridgeActiveTasks 设置当前 bridge 内进行中的 A2A task 数量。
func SetBridgeActiveTasks(n float64) {
	if holder == nil {
		return
	}
	holder.bridgeActiveTasks.Set(n)
}

// IncBridgeActiveTasks 给 bridge 活跃 task 计数加 delta（可为负数，表示完成）。
func IncBridgeActiveTasks(delta float64) {
	if holder == nil {
		return
	}
	holder.bridgeActiveTasks.Add(delta)
}

// SetInternalSchemaStale 把对内部提供服务的 MCP schema 同步状态设置为 stale(1) 或 fresh(0)。
func SetInternalSchemaStale(stale bool) {
	if holder == nil {
		return
	}
	v := 0.0
	if stale {
		v = 1.0
	}
	holder.internalSchemaStale.Set(v)
}
