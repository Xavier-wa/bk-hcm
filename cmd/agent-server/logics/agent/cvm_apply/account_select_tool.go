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

package cvmapply

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	protocloud "hcm/pkg/api/data-service/cloud"
	cloudserver "hcm/pkg/client/cloud-server"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/session"
	trpctool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// reselectByUserGuide 是账号不可用时附在工具结果末尾的引导语：要求模型把账号选择权交回用户。
// 刻意不列出业务下的可用账号——列出等于向模型提示「换一个就能过」，反而诱导它静默改选，
// 让用户在无感知的情况下用错账号提单。
const reselectByUserGuide = "账号选择权属于用户，严禁自行改用其它账号继续执行。" +
	"请调用 human_confirm 工具，把该账号不可用的情况告知用户并请其重新选择账号；" +
	"待用户答复后，再调用 select_account 上报用户选定的 account_id。"

// selectAccountTool 是模型上报所选 account_id 的本地声明工具（不经 tool_proxy，与 skill_load 同类）。
// 模型在多账号场景确定用户所选账号后调用它；工具校验该账号属于当前业务可用（tcloud-ziyan）账号后，
// 复用 injectAccountIDToRuntimeState 注入 RuntimeState 并落库，供后续轮次 tryReuseAccountID 复用。
type selectAccountTool struct {
	cloudClient *cloudserver.Client
	sessSvc     session.Service
	appName     string
}

// NewSelectAccountTool 创建 select_account 工具，注入账号校验与持久化所需依赖。
func NewSelectAccountTool(cloudClient *cloudserver.Client, sessSvc session.Service,
	appName string) trpctool.CallableTool {

	return &selectAccountTool{cloudClient: cloudClient, sessSvc: sessSvc, appName: appName}
}

// selectAccountArgs 定义 select_account 工具入参。
type selectAccountArgs struct {
	// AccountID 是模型确定的云账号 ID，须来自账号选项列表。
	AccountID string `json:"account_id"`
}

// Declaration 返回 select_account 工具声明。
func (t *selectAccountTool) Declaration() *trpctool.Declaration {
	return &trpctool.Declaration{
		Name: string(enumor.DeclToolSelectAccount),
		Description: "【选定云账号】在多账号场景下，确定用户要使用的云账号后调用本工具上报其 account_id，" +
			"系统据此记录选中账号；执行申领类操作前必须先调用本工具。account_id 取值须来自账号选项列表。",
		InputSchema: &trpctool.Schema{
			Type: "object",
			Properties: map[string]*trpctool.Schema{
				constant.SelectAccountArgKey: {
					Type:        "string",
					Description: "选中的云账号 ID，须来自账号选项列表中的 account_id",
				},
			},
			Required: []string{constant.SelectAccountArgKey},
		},
	}
}

// Call 校验 account_id 属于当前业务可用（tcloud-ziyan）账号后注入 RuntimeState 并落库。
// 校验失败仅返回可读提示（不写 RuntimeState、不落库、不返回 error），引导模型重新选择。
func (t *selectAccountTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	rid := rest.RidFromContext(ctx)

	var args selectAccountArgs
	if err := json.Unmarshal(jsonArgs, &args); err != nil {
		logs.Errorf("select_account failed, err: %v, args: %s, rid: %s", err, string(jsonArgs), rid)
		return "参数解析失败，请传入合法的 account_id", nil
	}
	accountID := strings.TrimSpace(args.AccountID)
	if accountID == "" {
		logs.Errorf("select_account failed, err: account_id is required, rid: %s", rid)
		return "account_id 不能为空，请从账号选项列表中选择一个账号", nil
	}

	inv, ok := trpcagent.InvocationFromContext(ctx)
	if !ok || inv == nil {
		logs.Errorf("select_account failed, err: invocation not found in context, rid: %s", rid)
		return "系统异常：未获取到运行上下文，暂时无法选定账号", nil
	}

	bkBizID := parseBkBizID(ctx, inv, nil)
	if bkBizID <= 0 {
		logs.Errorf("select_account failed, err: bk_biz_id not resolved, rid: %s", rid)
		return "系统异常：未确定当前业务，暂时无法选定账号", nil
	}

	accounts, err := listAccountsForBiz(ctx, t.cloudClient, bkBizID)
	if err != nil {
		logs.Errorf("select_account: list accounts failed, bk_biz_id: %d, err: %v, rid: %s",
			bkBizID, err, rid)
		return "查询业务账号失败，请稍后重试", nil
	}
	if !isEnabledAccountID(accounts, accountID) {
		logs.Warnf("select_account: account_id: %s is not an enabled account of biz: %d, rid: %s",
			accountID, bkBizID, rid)
		return fmt.Sprintf("account_id=%s 不是当前业务下可用的自研云账号，已拒绝选定。%s",
			accountID, reselectByUserGuide), nil
	}

	injectAccountIDToRuntimeState(ctx, inv, accountID, t.sessSvc, t.appName)
	logs.Infof("select_account: selected account_id: %s for biz: %d, rid: %s", accountID, bkBizID, rid)
	return fmt.Sprintf("已选定云账号：%s", accountID), nil
}

// isEnabledAccountID 判断 accountID 是否属于业务下可用（tcloud-ziyan）账号。
func isEnabledAccountID(accounts []*protocloud.AccountBizRelWithAccount, accountID string) bool {
	for _, acc := range filterEnabledAccounts(accounts) {
		if acc.ID == accountID {
			return true
		}
	}
	return false
}
