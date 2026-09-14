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

package agent

import (
	"context"
	"testing"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// gateCtx 构造带 invocation 的 ctx；accountID 非空时写入 RuntimeState 表示账号已解析。
func gateCtx(accountID string) context.Context {
	rs := map[string]any{}
	if accountID != "" {
		rs[constant.SessionAccountIDTempKey] = accountID
	}
	inv := &trpcagent.Invocation{RunOptions: trpcagent.RunOptions{RuntimeState: rs}}
	return trpcagent.NewInvocationContext(context.Background(), inv)
}

// 未选账号时，经 tool_proxy 信封调用推荐类工具被 CustomResult 拦截并引导 select_account。
func TestAccountGate_BlockRecommendWhenUnresolved_Proxy(t *testing.T) {
	gate := makeAccountGateBeforeTool()
	env := []byte(`{"tool_name":"bkhcm-devhk/get_biz_apply_recommend_by_static",` +
		`"parameters":{},"schema_token":"tok"}`)

	res, err := gate(gateCtx(""), &trpctool.BeforeToolArgs{
		ToolName:  constant.ProxyExecuteToolFullName,
		Arguments: env,
	})
	if err != nil {
		t.Fatalf("gate err: %v", err)
	}
	if res == nil || res.CustomResult != constant.SelectAccountRequiredMsg {
		t.Fatalf("expect block with guide, got %+v", res)
	}
}

// 账号已解析时放行提单工具（tool_proxy 信封），不双重拦截。
func TestAccountGate_AllowWhenResolved_Proxy(t *testing.T) {
	gate := makeAccountGateBeforeTool()
	env := []byte(`{"tool_name":"bkhcm-devhk/create_biz_apply","parameters":{},"schema_token":"tok"}`)

	res, err := gate(gateCtx("acc-1"), &trpctool.BeforeToolArgs{
		ToolName:  constant.ProxyExecuteToolFullName,
		Arguments: env,
	})
	if err != nil {
		t.Fatalf("gate err: %v", err)
	}
	if res != nil {
		t.Fatalf("expect allow (nil) when account resolved, got %+v", res)
	}
}

// 信封顶层多带 tool_intent 时，门禁判据不变：仍按解开后的真实工具名拦截/放行。
func TestAccountGate_UnaffectedByToolIntent(t *testing.T) {
	gate := makeAccountGateBeforeTool()

	blocked := []byte(`{"tool_name":"bkhcm-devhk/create_biz_apply","parameters":{},` +
		`"schema_token":"tok","tool_intent":"正在提交主机申领单"}`)
	res, err := gate(gateCtx(""), &trpctool.BeforeToolArgs{
		ToolName:  constant.ProxyExecuteToolFullName,
		Arguments: blocked,
	})
	if err != nil {
		t.Fatalf("gate err: %v", err)
	}
	if res == nil || res.CustomResult != constant.SelectAccountRequiredMsg {
		t.Fatalf("expect block with guide when account unresolved, got %+v", res)
	}

	res, err = gate(gateCtx("acc-1"), &trpctool.BeforeToolArgs{
		ToolName:  constant.ProxyExecuteToolFullName,
		Arguments: blocked,
	})
	if err != nil {
		t.Fatalf("gate err: %v", err)
	}
	if res != nil {
		t.Fatalf("expect allow (nil) when account resolved, got %+v", res)
	}
}

// 直连（非 proxy）方式调用提单工具，未选账号同样被拦截。
func TestAccountGate_BlockCreateBizApplyDirectWhenUnresolved(t *testing.T) {
	gate := makeAccountGateBeforeTool()

	res, err := gate(gateCtx(""), &trpctool.BeforeToolArgs{
		ToolName:  constant.ToolNameCreateBizApply,
		Arguments: []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("gate err: %v", err)
	}
	if res == nil || res.CustomResult != constant.SelectAccountRequiredMsg {
		t.Fatalf("expect block for direct create_biz_apply, got %+v", res)
	}
}

// tool_proxy 只读元工具即使账号未解析也放行。元工具到达门禁时名字带 toolset 前缀
//（框架 NamedToolSet 按 "<toolset>_<tool>" 命名），白名单登记的正是这个全名。
func TestAccountGate_AllowProxyReadOnlyMetaTool(t *testing.T) {
	gate := makeAccountGateBeforeTool()

	for _, name := range []string{constant.ProxySearchToolsFullName, constant.ProxyGetToolSchemaFullName} {
		res, err := gate(gateCtx(""), &trpctool.BeforeToolArgs{
			ToolName:  name,
			Arguments: []byte(`{}`),
		})
		if err != nil {
			t.Fatalf("gate err: %v", err)
		}
		if res != nil {
			t.Fatalf("expect allow for read-only meta-tool %s, got %+v", name, res)
		}
	}
}

// 只读查询类 MCP 业务工具不在白名单内，账号未解析时同样被拦（默认拦截语义）。
func TestAccountGate_BlockReadOnlyMCPToolWhenUnresolved(t *testing.T) {
	gate := makeAccountGateBeforeTool()
	env := []byte(`{"tool_name":"bkhcm-devhk/list_biz_apply_order","parameters":{},"schema_token":"tok"}`)

	res, err := gate(gateCtx(""), &trpctool.BeforeToolArgs{
		ToolName:  constant.ProxyExecuteToolFullName,
		Arguments: env,
	})
	if err != nil {
		t.Fatalf("gate err: %v", err)
	}
	if res == nil || res.CustomResult != constant.SelectAccountRequiredMsg {
		t.Fatalf("expect block for read-only MCP tool, got %+v", res)
	}
}

// select_account 自身绝不被门禁拦截，避免死锁。
func TestAccountGate_IgnoreSelectAccountItself(t *testing.T) {
	gate := makeAccountGateBeforeTool()

	res, err := gate(gateCtx(""), &trpctool.BeforeToolArgs{
		ToolName:  string(enumor.DeclToolSelectAccount),
		Arguments: []byte(`{"account_id":"acc-1"}`),
	})
	if err != nil {
		t.Fatalf("gate err: %v", err)
	}
	if res != nil {
		t.Fatalf("select_account must never be gated, got %+v", res)
	}
}

// isAccountGateAllowed 覆盖白名单边界：只有本地声明工具与 proxy 只读元工具放行。
func TestIsAccountGateAllowed(t *testing.T) {
	allowed := []string{
		constant.SkillLoadToolName,
		constant.SkillListDocsToolName,
		constant.SkillSelectDocsToolName,
		string(enumor.DeclToolHumanConfirm),
		string(enumor.DeclToolSelectAccount),
		constant.ProxySearchToolsFullName,
		constant.ProxyGetToolSchemaFullName,
	}
	for _, n := range allowed {
		if !isAccountGateAllowed(n) {
			t.Fatalf("%s should be allowed by account gate", n)
		}
	}
	// 业务工具（推荐、提单、只读查询）与 execute_tool 信封本身都不放行；
	// 元工具裸名（未经框架前缀）也不放行，框架不会以该形式下发。
	blocked := []string{
		"get_biz_apply_recommend_by_static",
		"get_biz_apply_recommend_by_plan",
		"get_biz_apply_recommend_split_suborder",
		constant.ToolNameCreateBizApply,
		"list_biz_apply_order",
		"list_biz_cvm",
		constant.ProxyExecuteToolFullName,
		constant.SearchToolsToolName,
	}
	for _, n := range blocked {
		if isAccountGateAllowed(n) {
			t.Fatalf("%s should not be allowed by account gate", n)
		}
	}
}

// 白名单必须覆盖全部本地声明工具，否则新增本地工具会被门禁误拦。
func TestAccountGateAllowlistCoversLocalTools(t *testing.T) {
	localTools := buildSkillTools(nil)
	// select_account 由 buildHostApplySubgraph 在 buildSkillTools 之后追加，此处补齐。
	localTools[string(enumor.DeclToolSelectAccount)] = nil

	for name := range localTools {
		if !isAccountGateAllowed(name) {
			t.Fatalf("local declared tool %s missing from accountGateAllowedTools", name)
		}
	}
}
