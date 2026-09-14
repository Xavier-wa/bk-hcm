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

package tool

import (
	"hcm/pkg/logs"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// trpc-agent-go 的 function-call 处理器不是只认 tool.Tool / tool.CallableTool，而是对工具做
// 结构化类型断言来发现可选能力（见 internal/flow/processor/functioncall.go）。装饰器必须原样
// 保留这些能力，否则例如 skill_load 的 StateDeltaForInvocation 会断言失败，skill 状态静默不落。
// 因此这里按内层实际实现的能力挑选装饰变体，而不是统一包成一种类型。
type (
	// stateDeltaProvider 是内层工具按 tool_call 维度写状态的能力。
	stateDeltaProvider interface {
		StateDelta(string, []byte, []byte) map[string][]byte
	}

	// invocationStateDeltaProvider 是内层工具按 invocation 维度写状态的能力。
	invocationStateDeltaProvider interface {
		StateDeltaForInvocation(*agent.Invocation, string, []byte, []byte) map[string][]byte
	}

	// 以下接口是本装饰器尚未覆盖的可选能力，仅用于探测：命中则放弃包装，
	// 不冒静默丢能力的风险。签名与框架 functioncall.go 里的局部断言保持一致。
	streamInnerPreference interface {
		StreamInner() bool
	}

	innerTextModePreference interface {
		InnerTextMode() tool.InnerTextMode
	}

	longRunner interface {
		LongRunning() bool
	}

	summarizationSkipper interface {
		SkipSummarization() bool
	}

	structuredStreamErrorOptIn interface {
		TRPCAgentGoStructuredStreamErrorsOptIn() bool
	}
)

// ToolWithIntent decorates a declaration-only tool so that its input schema exposes the
// optional tool_intent argument. 其余方法由内嵌接口原样透传。
type ToolWithIntent struct {
	tool.Tool
}

// Declaration returns the inner declaration with the optional tool_intent argument added.
func (w ToolWithIntent) Declaration() *tool.Declaration {
	return DeclWithToolIntent(w.Tool.Declaration())
}

// CallableToolWithIntent is the ToolWithIntent variant for callable tools.
type CallableToolWithIntent struct {
	tool.CallableTool
}

// Declaration returns the inner declaration with the optional tool_intent argument added.
func (w CallableToolWithIntent) Declaration() *tool.Declaration {
	return DeclWithToolIntent(w.CallableTool.Declaration())
}

// statefulToolWithIntent is the ToolWithIntent variant for callable tools that write graph state
// through both interfaces (skill_load / skill_select_docs)。StateDelta 系列方法必须显式转发，
// 否则框架的结构化断言会落空。
type statefulToolWithIntent struct {
	CallableToolWithIntent
	stateDelta           stateDeltaProvider
	invocationStateDelta invocationStateDeltaProvider
}

// StateDelta forwards to the inner tool.
func (w statefulToolWithIntent) StateDelta(toolCallID string, args, resultJSON []byte) map[string][]byte {
	return w.stateDelta.StateDelta(toolCallID, args, resultJSON)
}

// StateDeltaForInvocation forwards to the inner tool.
func (w statefulToolWithIntent) StateDeltaForInvocation(inv *agent.Invocation, toolCallID string,
	args, resultJSON []byte) map[string][]byte {

	return w.invocationStateDelta.StateDeltaForInvocation(inv, toolCallID, args, resultJSON)
}

// stateDeltaToolWithIntent is the variant for callable tools that implement StateDelta only
// （如框架 skill 包的 ExecTool / WriteStdinTool / PollSessionTool）。
// 外层方法集必须与内层一致：框架优先断言 StateDeltaForInvocation，外层若多出该方法，
// 内层的 StateDelta 就再也不会被调用，状态会静默丢失。
type stateDeltaToolWithIntent struct {
	CallableToolWithIntent
	stateDelta stateDeltaProvider
}

// StateDelta forwards to the inner tool.
func (w stateDeltaToolWithIntent) StateDelta(toolCallID string, args, resultJSON []byte) map[string][]byte {
	return w.stateDelta.StateDelta(toolCallID, args, resultJSON)
}

// invocationStateDeltaToolWithIntent is the variant for callable tools that implement
// StateDeltaForInvocation only。同样按内层方法集精确转发，不额外暴露 StateDelta。
type invocationStateDeltaToolWithIntent struct {
	CallableToolWithIntent
	invocationStateDelta invocationStateDeltaProvider
}

// StateDeltaForInvocation forwards to the inner tool.
func (w invocationStateDeltaToolWithIntent) StateDeltaForInvocation(inv *agent.Invocation, toolCallID string,
	args, resultJSON []byte) map[string][]byte {

	return w.invocationStateDelta.StateDeltaForInvocation(inv, toolCallID, args, resultJSON)
}

// NewToolWithIntent returns inner decorated so that its declaration exposes the optional
// tool_intent argument, letting the model state what the current call is for.
//
// 装饰以「能力保留」为前提：变体按内层实现的可选接口挑选。若内层带有本装饰器无法转发的能力
// （流式调用、跳过总结等），原样返回 inner —— 宁可少一个说明字段，也不改变工具行为。
func NewToolWithIntent(inner tool.Tool) tool.Tool {
	if inner == nil {
		return nil
	}

	name := ""
	if decl := inner.Declaration(); decl != nil {
		name = decl.Name
	}

	// 命中任一转发不了的能力，直接放弃包装、原样返回内层工具：宁可这个工具少一个 tool_intent
	// 说明字段，也不冒着半包装、悄悄改变工具行为的风险。
	if unpreservable, capability := hasUnpreservableCapability(inner); unpreservable {
		logs.Warnf("[tool:intent] tool=%q implements %s which the intent wrapper cannot forward, "+
			"skip decorating", name, capability)
		return inner
	}

	callable, isCallable := inner.(tool.CallableTool)
	if !isCallable {
		// 纯声明型工具（如 human_confirm）不参与执行，只需补声明字段。
		return ToolWithIntent{Tool: inner}
	}

	// 按内层实现的状态写入接口组合精确挑选变体，四种组合全部覆盖：外层多暴露或少暴露任一方法，
	// 都会让框架的断言选错分支，进而静默丢状态。框架优先断言 StateDeltaForInvocation。
	base := CallableToolWithIntent{CallableTool: callable}
	sdp, hasStateDelta := inner.(stateDeltaProvider)
	isdp, hasInvocationStateDelta := inner.(invocationStateDeltaProvider)
	switch {
	case hasStateDelta && hasInvocationStateDelta:
		return statefulToolWithIntent{
			CallableToolWithIntent: base,
			stateDelta:             sdp,
			invocationStateDelta:   isdp,
		}
	case hasStateDelta:
		return stateDeltaToolWithIntent{CallableToolWithIntent: base, stateDelta: sdp}
	case hasInvocationStateDelta:
		return invocationStateDeltaToolWithIntent{CallableToolWithIntent: base, invocationStateDelta: isdp}
	default:
		return base
	}
}

// hasUnpreservableCapability 探测内层是否实现了当前装饰变体转不出去的可选能力。
// 框架靠类型断言发现这些能力，包装后外层类型变了；若外层没实现同名方法，能力会静默丢失
// （例如流式工具被当成一次性 Call）。命中则放弃包装，第二个返回值仅用于日志。
func hasUnpreservableCapability(inner tool.Tool) (bool, string) {
	switch inner.(type) {
	case tool.StreamableTool:
		// 结果按 chunk 流式返回；只包 Declaration 会丢掉 StreamableCall，框架会退化走 Call。
		// TODO: 引进 skill_exec / skill_run 前在此补流式装饰变体（见 graph_build.buildSkillTools）。
		// 须转发 StreamableCall，并按内层有无精确挂 StreamInner/InnerTextMode/结构化流式错误与
		// StateDelta*（方法集必须与内层一致，多挂 StateDeltaForInvocation 会让仅有 StateDelta
		// 的 ExecTool 状态静默丢失）。
		return true, "tool.StreamableTool"
	case streamInnerPreference:
		// 控制「有 StreamableCall 时是否真走流式」；漏转会让本该走 Call 的工具被改成走流式。
		return true, "StreamInner"
	case innerTextModePreference:
		// 控制流式内层事件的文本是否转发；漏转会改变用户可见的流式文本。
		return true, "InnerTextMode"
	case longRunner:
		// LongRunning=true 时框架允许不回 function response；漏转会多发一条工具响应。
		return true, "LongRunning"
	case summarizationSkipper:
		// 工具结果是否跳过外层总结；漏转会让本不该总结的结果再被总结一遍。
		return true, "SkipSummarization"
	case structuredStreamErrorOptIn:
		// 流式错误是否走结构化形态；漏转会改变错误的呈现方式。
		return true, "TRPCAgentGoStructuredStreamErrorsOptIn"
	default:
		return false, ""
	}
}
