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

// Package hitl 为 agent 图提供统一的人在环（Human-in-the-Loop, HITL）中断节点。
//
// 单个节点处理所有需要人工中断的工具调用，由以工具名为键的 Handler 注册表分派。两类 handler 共用该节点：
//   - LLM 主动发起的提问（human_confirm）：向用户提问，随后在 llm 节点继续。
//   - 真实工具的执行前确认门禁（如 create_biz_apply）：在工具执行前确认，然后路由到 tool 节点（放行）
//     或回退到 llm 节点（取消/校验未通过）。
//
// 整体流程如下：
//  1. llm 节点产出一个 tool_call；当存在已注册的 Handler 时，路由将其送往 hitl 节点。
//  2. 节点构造 handler payload 并调用 graph.Interrupt，暂停执行并保存 checkpoint。
//  3. 前端收到对应的自定义事件（hitl.interrupt / tool.confirm）并展示 UI。
//  4. 用户响应后图恢复执行，handler 的 OnResume 产出消息增量与下一跳路由（tool / llm）。
package hitl

import (
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// declaredToolWrapper 将一个声明包装为不可执行工具。
// 用于没有执行逻辑的纯声明型工具。
type declaredToolWrapper struct{ d *tool.Declaration }

// Declaration 返回该工具的声明。
func (w declaredToolWrapper) Declaration() *tool.Declaration { return w.d }

// GetTool 返回 HITL 工具声明（human_confirm）。
// 这是一个没有执行逻辑的纯声明型工具。
func GetTool() *tool.Declaration {
	return HumanConfirmTool()
}

// GetToolWrapper 以 tool.Tool 接口形式返回 HITL 工具，使其可被注册进工具集。
func GetToolWrapper() tool.Tool {
	return declaredToolWrapper{d: HumanConfirmTool()}
}

// GetNode 返回由给定 handler 注册表驱动的统一 HITL 节点函数。
// 该节点处理所有存在已注册 Handler 的工具调用（human_confirm、执行前工具门禁……）：
// 触发中断、发出对应的自定义事件，并在恢复时应用 handler 的 ResumeResult。
func GetNode(reg *Registry) graph.NodeFunc {
	return makeHITLNode(reg)
}
