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

package toolgate

import (
	"hcm/cmd/agent-server/logics/agent/hitl"
	"hcm/cmd/agent-server/logics/toolproxy"
	"hcm/pkg/cc"
	"hcm/pkg/client"
)

// builtinGates 列出所有内置的执行前确认门禁。
// 新增受门禁工具：实现 hitl.Handler 后在此处追加即可。
// clientSet 经依赖注入透传给需要跨服务调用的门禁；仅需工具名的场景（如 EnabledGateToolNames）可传 nil。
func builtinGates(clientSet *client.ClientSet) []hitl.Handler {
	return []hitl.Handler{
		newCreateCvmApplyGate(clientSet),
	}
}

// isGateEnabled 判断某个门禁在运行期配置下是否启用：
// 配置启用 且 （工具列表为空 或 包含该工具）。
func isGateEnabled(toolName string, cfg cc.AgentConfirmGateConfig) bool {
	if !cfg.Enabled {
		return false
	}
	if len(cfg.Tools) == 0 {
		return true
	}
	// 配置可写带 toolset 前缀的规范名（proxy 下 LLM 调用用前缀名，如 bkhcm-devhk/create_biz_apply），
	// 而 gate 名为原始名（create_biz_apply）。两侧都删除前缀再比较，兼容两种写法。
	target := toolproxy.StripToolSetPrefix(toolName)
	for _, t := range cfg.Tools {
		if toolproxy.StripToolSetPrefix(t) == target {
			return true
		}
	}
	return false
}

// GetEnabledGateHandlers 返回在给定配置下启用的确认门禁 handler。
// 图构建会把它们注册进 hitl handler 注册表。clientSet 注入给需要跨服务调用的门禁
// （如申领门禁调用 woa-server 校验）。
func GetEnabledGateHandlers(cfg cc.AgentConfirmGateConfig, clientSet *client.ClientSet) []hitl.Handler {
	gates := builtinGates(clientSet)
	out := make([]hitl.Handler, 0, len(gates))
	for _, g := range gates {
		if isGateEnabled(g.ToolName(), cfg) {
			out = append(out, g)
		}
	}
	return out
}

// GetEnabledGateToolNames 返回在给定配置下启用的门禁所守护的工具名集合。
// 用于把受门禁工具从通用确认工具集中排除，避免二次确认。
func GetEnabledGateToolNames(cfg cc.AgentConfirmGateConfig) map[string]struct{} {
	// 仅需工具名，无需跨服务客户端，传 nil 即可。
	gates := builtinGates(nil)
	out := make(map[string]struct{}, len(gates))
	for _, g := range gates {
		if isGateEnabled(g.ToolName(), cfg) {
			out[g.ToolName()] = struct{}{}
		}
	}
	return out
}
