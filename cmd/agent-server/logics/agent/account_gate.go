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

	"hcm/cmd/agent-server/logics/toolproxy"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// makeAccountGateBeforeTool 返回 host_apply 场景的账号前置门禁 BeforeTool 回调：
// 当模型在尚未选定账号（RuntimeState 无 account_id）时调用白名单外的任意工具，
// 通过 BeforeToolResult.CustomResult 短路工具执行，引导模型先调用 select_account 选定账号。
//
// 门禁默认拦截、白名单放行（名单见 accountGateAllowedTools）：受控侧是持续增长的外部 MCP 工具集，
// 而放行侧是我们自己声明的本地工具，用后者做判定才不会因漏登记而静默放行越权操作。
// 账号已解析时一律放行（提单确认仍由 confirm gate 处理，不双重拦截）。
func makeAccountGateBeforeTool() trpctool.BeforeToolCallbackStructured {
	return func(ctx context.Context, args *trpctool.BeforeToolArgs) (*trpctool.BeforeToolResult, error) {
		if args == nil {
			return nil, nil
		}
		rid := rest.RidFromContext(ctx)

		// 工具经 tool_proxy execute_tool 信封调用时，需解开取真实工具名。
		resolved := toolproxy.ResolveToolCall(&trpcmodel.ToolCall{
			Function: trpcmodel.FunctionDefinitionParam{Name: args.ToolName, Arguments: args.Arguments},
		})
		if isAccountGateAllowed(resolved.Name) {
			return nil, nil
		}

		if AccountExistInContext(ctx) {
			return nil, nil
		}

		logs.Infof("[tool:account_gate] block %s: account not selected, guide select_account, rid: %s",
			resolved.Name, rid)
		return &trpctool.BeforeToolResult{CustomResult: constant.SelectAccountRequiredMsg}, nil
	}
}

// accountGateAllowedTools 是账号门禁的放行白名单：账号未选定时仍允许执行的工具。只含两类：
//
//  1. 本地声明工具（不经 tool_proxy，LLM 侧为裸名）——技能加载、向用户提问、上报选中账号，
//     均不触达云资源；其中 select_account / human_confirm 若被拦会导致选账号死锁；
//  2. tool_proxy 只读元工具（LLM 侧名字被框架 NamedToolSet 前缀成 "<toolset>_<tool>"）——
//     只做工具发现，无副作用。
//
// 其余一律拦截，含全部真实 MCP 业务工具（推荐、提单，以及 list_biz_apply_order 等只读查询）。
// 往 buildSkillTools 新增本地声明工具时必须同步登记于此，否则会被门禁误拦；
// TestAccountGateAllowlistCoversLocalTools 会兜住漏登记。
var accountGateAllowedTools = map[string]struct{}{
	constant.SkillLoadToolName:           {},
	constant.SkillListDocsToolName:       {},
	constant.SkillSelectDocsToolName:     {},
	string(enumor.DeclToolHumanConfirm):  {},
	string(enumor.DeclToolSelectAccount): {},
	constant.ProxySearchToolsFullName:    {},
	constant.ProxyGetToolSchemaFullName:  {},
}

// isAccountGateAllowed 判断工具是否在账号门禁白名单内。
// The name parameter must be the effective tool name resolved by toolproxy.ResolveToolCall,
// i.e. the real MCP tool name once the execute_tool envelope is unwrapped.
func isAccountGateAllowed(name string) bool {
	_, ok := accountGateAllowedTools[name]
	return ok
}

// AccountExistInContext 判断当前 run 是否已解析出 account_id（account_select 节点或 run 起点回填写入）。
func AccountExistInContext(ctx context.Context) bool {
	inv, ok := trpcagent.InvocationFromContext(ctx)
	if !ok || inv == nil || inv.RunOptions.RuntimeState == nil {
		return false
	}
	id, _ := inv.RunOptions.RuntimeState[constant.SessionAccountIDTempKey].(string)
	return id != ""
}
