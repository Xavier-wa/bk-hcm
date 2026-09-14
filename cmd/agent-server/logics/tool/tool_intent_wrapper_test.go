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
	"context"
	"testing"

	"hcm/pkg/criteria/constant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// declOnlyTool mimics human_confirm: a pure declaration tool that is never executed.
type declOnlyTool struct {
	decl *tool.Declaration
}

func (t *declOnlyTool) Declaration() *tool.Declaration { return t.decl }

// callableIntentTool mimics select_account / skill_list_docs.
type callableIntentTool struct {
	declOnlyTool
	gotArgs []byte
}

func (t *callableIntentTool) Call(_ context.Context, jsonArgs []byte) (any, error) {
	t.gotArgs = append([]byte(nil), jsonArgs...)
	return "called", nil
}

// statefulIntentTool mimics skill_load / skill_select_docs, which the framework discovers
// through structural assertions on the StateDelta interfaces.
type statefulIntentTool struct {
	callableIntentTool
}

func (t *statefulIntentTool) StateDelta(toolCallID string, _, _ []byte) map[string][]byte {
	return map[string][]byte{"state_delta": []byte(toolCallID)}
}

func (t *statefulIntentTool) StateDeltaForInvocation(_ *agent.Invocation, toolCallID string,
	_, _ []byte) map[string][]byte {

	return map[string][]byte{"invocation_state_delta": []byte(toolCallID)}
}

// stateDeltaOnlyIntentTool mimics 框架 skill 包的 ExecTool / PollSessionTool：只实现 StateDelta。
type stateDeltaOnlyIntentTool struct {
	callableIntentTool
}

func (t *stateDeltaOnlyIntentTool) StateDelta(toolCallID string, _, _ []byte) map[string][]byte {
	return map[string][]byte{"state_delta": []byte(toolCallID)}
}

// invocationStateDeltaOnlyIntentTool 只实现 StateDeltaForInvocation。
type invocationStateDeltaOnlyIntentTool struct {
	callableIntentTool
}

func (t *invocationStateDeltaOnlyIntentTool) StateDeltaForInvocation(_ *agent.Invocation, toolCallID string,
	_, _ []byte) map[string][]byte {

	return map[string][]byte{"invocation_state_delta": []byte(toolCallID)}
}

// streamableIntentTool implements a capability the wrapper cannot forward.
type streamableIntentTool struct {
	callableIntentTool
}

func (t *streamableIntentTool) StreamableCall(_ context.Context, _ []byte) (*tool.StreamReader, error) {
	return nil, nil
}

// longRunningIntentTool mimics tools that opt into LongRunning (no function response when result is nil).
type longRunningIntentTool struct {
	callableIntentTool
}

func (t *longRunningIntentTool) LongRunning() bool { return true }

// innerTextModeIntentTool mimics streamable tools that customize forwarded inner text.
type innerTextModeIntentTool struct {
	callableIntentTool
}

func (t *innerTextModeIntentTool) StreamableCall(_ context.Context, _ []byte) (*tool.StreamReader, error) {
	return nil, nil
}

func (t *innerTextModeIntentTool) InnerTextMode() tool.InnerTextMode {
	return tool.InnerTextModeExclude
}

func newIntentTestDecl(name string) *tool.Declaration {
	return &tool.Declaration{
		Name: name,
		InputSchema: &tool.Schema{
			Type:       "object",
			Required:   []string{"skill"},
			Properties: map[string]*tool.Schema{"skill": {Type: "string"}},
		},
	}
}

func TestNewToolWithIntent_Declaration(t *testing.T) {
	testCases := []struct {
		name  string
		inner tool.Tool
	}{
		{
			name:  "declaration only tool",
			inner: &declOnlyTool{decl: newIntentTestDecl("human_confirm")},
		},
		{
			name: "callable tool",
			inner: &callableIntentTool{
				declOnlyTool: declOnlyTool{decl: newIntentTestDecl("select_account")},
			},
		},
		{
			name: "stateful callable tool",
			inner: &statefulIntentTool{callableIntentTool: callableIntentTool{
				declOnlyTool: declOnlyTool{decl: newIntentTestDecl("skill_load")},
			}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decl := NewToolWithIntent(tc.inner).Declaration()

			prop, ok := decl.InputSchema.Properties[constant.ToolIntentArgKey]
			require.True(t, ok, "wrapped tool must expose tool_intent")
			assert.Equal(t, "string", prop.Type)
			// 原有参数与 Required 不受影响，tool_intent 始终可选。
			assert.Contains(t, decl.InputSchema.Properties, "skill")
			assert.Equal(t, []string{"skill"}, decl.InputSchema.Required)
			assert.NotContains(t, decl.InputSchema.Required, constant.ToolIntentArgKey)
		})
	}
}

func TestNewToolWithIntent_ForwardsCall(t *testing.T) {
	inner := &callableIntentTool{
		declOnlyTool: declOnlyTool{decl: newIntentTestDecl("select_account")},
	}

	wrapped := NewToolWithIntent(inner)
	callable, ok := wrapped.(tool.CallableTool)
	require.True(t, ok, "callable tool must stay callable after decorating")

	args := []byte(`{"skill":"calc","tool_intent":"正在加载技能"}`)
	result, err := callable.Call(context.Background(), args)

	require.NoError(t, err)
	assert.Equal(t, "called", result)
	// 装饰器不裁剪入参：内层 json.Unmarshal 忽略未知字段，无需为 tool_intent 做特殊处理。
	assert.Equal(t, args, inner.gotArgs)
}

// TestNewToolWithIntent_PreservesStateDelta 锁住框架用于发现状态写入能力的结构化断言：
// 包装后若断言落空，skill_load / skill_select_docs 的状态会静默不落盘。
func TestNewToolWithIntent_PreservesStateDelta(t *testing.T) {
	inner := &statefulIntentTool{callableIntentTool: callableIntentTool{
		declOnlyTool: declOnlyTool{decl: newIntentTestDecl("skill_load")},
	}}

	wrapped := NewToolWithIntent(inner)

	sdp, ok := wrapped.(stateDeltaProvider)
	require.True(t, ok, "StateDelta capability must survive decorating")
	assert.Equal(t, map[string][]byte{"state_delta": []byte("call-1")},
		sdp.StateDelta("call-1", nil, nil))

	isdp, ok := wrapped.(invocationStateDeltaProvider)
	require.True(t, ok, "StateDeltaForInvocation capability must survive decorating")
	assert.Equal(t, map[string][]byte{"invocation_state_delta": []byte("call-2")},
		isdp.StateDeltaForInvocation(nil, "call-2", nil, nil))
}

// TestNewToolWithIntent_PreservesStateDeltaOnly 内层只实现 StateDelta 时，外层必须同样只暴露 StateDelta：
// 框架优先断言 StateDeltaForInvocation，外层一旦多出该方法，内层 StateDelta 再也不会被调用，状态静默丢失。
func TestNewToolWithIntent_PreservesStateDeltaOnly(t *testing.T) {
	inner := &stateDeltaOnlyIntentTool{callableIntentTool: callableIntentTool{
		declOnlyTool: declOnlyTool{decl: newIntentTestDecl("skill_exec")},
	}}

	wrapped := NewToolWithIntent(inner)

	assert.Contains(t, wrapped.Declaration().InputSchema.Properties, constant.ToolIntentArgKey)

	sdp, ok := wrapped.(stateDeltaProvider)
	require.True(t, ok, "StateDelta capability must survive decorating")
	assert.Equal(t, map[string][]byte{"state_delta": []byte("call-1")},
		sdp.StateDelta("call-1", nil, nil))

	_, overExposed := wrapped.(invocationStateDeltaProvider)
	assert.False(t, overExposed, "wrapper must not expose an interface the inner tool lacks")
}

// TestNewToolWithIntent_PreservesInvocationStateDeltaOnly 内层只实现 StateDeltaForInvocation 时，
// 外层同样按内层方法集精确转发，不额外暴露 StateDelta。
func TestNewToolWithIntent_PreservesInvocationStateDeltaOnly(t *testing.T) {
	inner := &invocationStateDeltaOnlyIntentTool{callableIntentTool: callableIntentTool{
		declOnlyTool: declOnlyTool{decl: newIntentTestDecl("todo_update")},
	}}

	wrapped := NewToolWithIntent(inner)

	assert.Contains(t, wrapped.Declaration().InputSchema.Properties, constant.ToolIntentArgKey)

	isdp, ok := wrapped.(invocationStateDeltaProvider)
	require.True(t, ok, "StateDeltaForInvocation capability must survive decorating")
	assert.Equal(t, map[string][]byte{"invocation_state_delta": []byte("call-2")},
		isdp.StateDeltaForInvocation(nil, "call-2", nil, nil))

	_, overExposed := wrapped.(stateDeltaProvider)
	assert.False(t, overExposed, "wrapper must not expose an interface the inner tool lacks")
}

// TestNewToolWithIntent_DeclarationOnlyStaysNonCallable 纯声明工具不能因为包装而变成可执行，
// 否则 tool 节点会尝试执行本应路由到 hitl 的 human_confirm。
func TestNewToolWithIntent_DeclarationOnlyStaysNonCallable(t *testing.T) {
	inner := &declOnlyTool{decl: newIntentTestDecl("human_confirm")}

	wrapped := NewToolWithIntent(inner)

	_, callable := wrapped.(tool.CallableTool)
	assert.False(t, callable)
}

// TestNewToolWithIntent_SkipsUnpreservableCapability 无法转发的能力（如流式调用）宁可不加字段，
// 也不改变工具行为。
func TestNewToolWithIntent_SkipsUnpreservableCapability(t *testing.T) {
	testCases := []struct {
		name  string
		inner tool.Tool
	}{
		{
			name: "streamable",
			inner: &streamableIntentTool{callableIntentTool: callableIntentTool{
				declOnlyTool: declOnlyTool{decl: newIntentTestDecl("skill_exec")},
			}},
		},
		{
			name: "long running",
			inner: &longRunningIntentTool{callableIntentTool: callableIntentTool{
				declOnlyTool: declOnlyTool{decl: newIntentTestDecl("long_runner")},
			}},
		},
		{
			name: "inner text mode",
			inner: &innerTextModeIntentTool{callableIntentTool: callableIntentTool{
				declOnlyTool: declOnlyTool{decl: newIntentTestDecl("stream_text_mode")},
			}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := NewToolWithIntent(tc.inner)

			assert.Same(t, tc.inner, wrapped)
			assert.NotContains(t, wrapped.Declaration().InputSchema.Properties, constant.ToolIntentArgKey)
		})
	}
}

func TestNewToolWithIntent_NilInner(t *testing.T) {
	assert.Nil(t, NewToolWithIntent(nil))
}
