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

// Package cvmapply provides Function Nodes for the CVM apply workflow in graph mode.
package cvmapply

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"hcm/cmd/agent-server/logics/agent/message"
	"hcm/cmd/agent-server/logics/agent/state"
	"hcm/cmd/agent-server/logics/auth"
	protocloud "hcm/pkg/api/data-service/cloud"
	cloudserver "hcm/pkg/client/cloud-server"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

const (
	// accountQueryTimeout is the maximum duration allowed for the account list HTTP call.
	accountQueryTimeout = 3 * time.Second

	// unsupportedVendorReason is returned in options.reason for non-tcloud-ziyan accounts.
	unsupportedVendorReason = "当前仅支持自研云（tcloud-ziyan）账号"
)

// accountSelectOption holds one account option for the account_select.interrupt payload.
type accountSelectOption struct {
	AccountID   string `json:"account_id"`
	AccountName string `json:"account_name"`
	Vendor      string `json:"vendor"`
	Enabled     bool   `json:"enabled"`
	Reason      string `json:"reason"`
}

// NewAccountSelectNode returns a Function Node that identifies the resource account for the
// current biz and either auto-selects (single account) or triggers an HITL interrupt
// (multiple accounts) before routing to the llm node.
//
// The cloudClient is captured by closure; no bk_ticket is required for the internal call.
func NewAccountSelectNode(cloudClient *cloudserver.Client) graph.NodeFunc {
	return func(ctx context.Context, state graph.State) (any, error) {
		rid := rest.RidFromContext(ctx)
		logs.Infof("account_select: start, rid: %s", rid)

		inv, ok := trpcagent.InvocationFromContext(ctx)
		if !ok || inv == nil {
			return nil, fmt.Errorf("account_select: invocation not found in context, rid: %s", rid)
		}

		// 已经历过 account 选择的，直接从 session state 读取并跳过，避免多次 run 反复选择
		if accountID, ok := getSelectedAccountIDFromSession(inv); ok {
			logs.Infof("account_select: reuse persisted account_id=%s, skip selection, rid: %s", accountID, rid)
			injectAccountIDToRuntimeState(ctx, inv, accountID)
			return graph.State{
				constant.SessionAccountIDTempKey:  accountID,
				constant.AccountSelectNextNodeKey: enumor.CvmApplyNodeLLM,
			}, nil
		}

		bkBizID, ok := inv.RunOptions.RuntimeState[constant.SessionBkBizIDStateKey].(int64)
		if !ok || bkBizID <= 0 {
			return nil, fmt.Errorf("account_select: bk_biz_id not found or invalid in RuntimeState, rid: %s", rid)
		}
		logs.Infof("account_select: bk_biz_id=%d, rid: %s", bkBizID, rid)

		accounts, err := listAccountsForBiz(ctx, cloudClient, bkBizID)
		if err != nil {
			logs.Errorf("account_select: list accounts failed, bk_biz_id=%d, err: %v, rid: %s", bkBizID, err, rid)
			return nil, fmt.Errorf("account_select: list accounts failed: %w", err)
		}
		logs.Infof("account_select: bk_biz_id=%d account count=%d, rid: %s", bkBizID, len(accounts), rid)

		return routeByAccountCount(ctx, state, inv, accounts)
	}
}

// listAccountsForBiz queries resource accounts for the given biz with a bounded timeout.
// No bk_ticket is required for this internal service call.
func listAccountsForBiz(ctx context.Context, client *cloudserver.Client, bkBizID int64) (
	[]*protocloud.AccountBizRelWithAccount, error) {

	timeoutCtx, cancel := context.WithTimeout(ctx, accountQueryTimeout)
	defer cancel()

	kt := kit.New()
	kt.User = auth.BKUsernameFromContext(ctx)
	kt.AppCode = constant.AgentSourceAppCode
	// TODO 内部版使用多租户后，这里需要添加租户属性
	kt.Rid = rest.RidFromContext(timeoutCtx)
	kt.Ctx = timeoutCtx
	return client.Account.ListByUsageBizID(kt, bkBizID, enumor.ResourceAccount)
}

// routeByAccountCount selects the next graph node based on the number of available accounts.
func routeByAccountCount(ctx context.Context, state graph.State, inv *trpcagent.Invocation,
	accounts []*protocloud.AccountBizRelWithAccount) (any, error) {

	rid := rest.RidFromContext(ctx)
	switch count := len(accounts); {
	case count == 0:
		return graph.State{constant.AccountSelectNextNodeKey: enumor.CvmApplyNodeFallback}, nil

	case count == 1:
		accountID := accounts[0].ID
		logs.Infof("account_select: auto-select account_id=%s, rid: %s", accountID, rid)
		injectAccountIDToRuntimeState(ctx, inv, accountID)
		return graph.State{
			constant.SessionAccountIDTempKey:  accountID,
			constant.AccountSelectNextNodeKey: enumor.CvmApplyNodeLLM,
		}, nil

	default:
		messages, _ := state[graph.StateKeyMessages].([]trpcmodel.Message)
		// 上一步没有产生助手回复，需要进行emit事件封装，生成AGUI消息
		lastResp := "已进入主机申领模式。请先选择需要操作的云账号。"
		// HITL中断处理：返回用户在 resume 时的自由输入文本
		userInput, err := promptAccountSelection(ctx, messages, state, accounts, lastResp)
		if err != nil {
			return nil, err
		}
		// account_id 来自前端 forwardedProps（即用户从选项中选定的账号）
		accountID := resolveSelectedAccountID(ctx, inv)
		if accountID != "" {
			injectAccountIDToRuntimeState(ctx, inv, accountID)
		}

		// 用户看到的账号选项并不在模型对话上下文中，这里将其作为助手消息注入，
		// 让模型理解 resume 后用户输入所针对的选项
		optionsMsg := buildAccountOptionsMessage(ctx, accounts)
		lastResp = lastResp + "\n" + optionsMsg
		delta := message.BuildFallbackResumeDelta(ctx, state, messages, lastResp, userInput)
		delta[constant.SessionAccountIDTempKey] = accountID
		delta[constant.AccountSelectNextNodeKey] = enumor.CvmApplyNodeLLM
		return delta, nil
	}
}

// promptAccountSelection triggers an HITL interrupt with the available account options and
// returns the free-form user input text supplied on resume. The structured account_id is
// resolved separately via resolveSelectedAccountID, because the user input is not guaranteed
// to be an account_id.
func promptAccountSelection(ctx context.Context, messages []trpcmodel.Message, state graph.State,
	accounts []*protocloud.AccountBizRelWithAccount, lastResp string) (string, error) {

	rid := rest.RidFromContext(ctx)

	interruptKey := buildInterruptKey(state, constant.AccountSelectInterruptKey)
	message.EmitFallbackMessage(ctx, messages, state, interruptKey, string(enumor.CvmApplyNodeAccountSelect), lastResp)

	resumeValue, interruptErr := graph.Interrupt(ctx, state, interruptKey, map[string]any{
		"type":    constant.AccountSelectInterruptKey,
		"options": buildAccountOptions(accounts),
	})
	if interruptErr != nil {
		logs.Infof("account_select: interrupt triggered (multi-account), rid: %s", rid)
		return "", interruptErr
	}

	userInput, _ := resumeValue.(string)
	logs.Infof("account_select: resumed with user input=%q, rid: %s", userInput, rid)
	return userInput, nil
}

// resolveSelectedAccountID reads the account_id that the frontend passed via forwardedProps
// (stored in RuntimeState under StateKeyForwardedResumeValue) when the user picked an account
// option. It is kept separate from the free-form user input.
func resolveSelectedAccountID(ctx context.Context, inv *trpcagent.Invocation) string {
	rid := rest.RidFromContext(ctx)

	if inv == nil || inv.RunOptions.RuntimeState == nil {
		return ""
	}

	raw := inv.RunOptions.RuntimeState[constant.StateKeyForwardedResumeValue]
	accountID, ok := raw.(string)
	if !ok || accountID == "" {
		logs.Warnf("account_select: invalid forwarded account_id, expected non-empty string, got %T(%v), rid: %s",
			raw, raw, rid)
		return ""
	}

	logs.Infof("account_select: user selected account_id=%s, rid: %s", accountID, rid)
	return accountID
}

// buildAccountOptionsMessage serializes the account options shown to the user into an assistant
// message, so the resumed conversation context reflects the choices the user actually saw.
func buildAccountOptionsMessage(ctx context.Context, accounts []*protocloud.AccountBizRelWithAccount) string {
	rid := rest.RidFromContext(ctx)

	optionsJSON, err := json.Marshal(buildAccountOptions(accounts))
	if err != nil {
		logs.Warnf("account_select: marshal account options failed, err: %v, rid: %s", err, rid)
		return ""
	}
	return string(optionsJSON)
}

// buildAccountOptions converts account records into the interrupt payload option list,
// marking non-tcloud-ziyan accounts as disabled with an explanatory reason.
func buildAccountOptions(accounts []*protocloud.AccountBizRelWithAccount) []accountSelectOption {
	options := make([]accountSelectOption, 0, len(accounts))
	for _, acc := range accounts {
		opt := accountSelectOption{
			AccountID:   acc.ID,
			AccountName: acc.Name,
			Vendor:      string(acc.Vendor),
			Enabled:     acc.Vendor == enumor.TCloudZiyan,
		}
		// TODO 目前仅支持自研云申领，后续公有云支持后可放开
		if !opt.Enabled {
			opt.Reason = unsupportedVendorReason
		}
		options = append(options, opt)
	}
	return options
}

// injectAccountIDToRuntimeState writes the selected account_id into the invocation's
// RuntimeState so that MakeAccountIDInjectCallback can read it and expose it as the
// {temp:account_id?} instruction placeholder before each LLM call. It also persists the
// account_id into the session state so that subsequent runs can skip account selection.
func injectAccountIDToRuntimeState(ctx context.Context, inv *trpcagent.Invocation, accountID string) {
	if inv == nil || inv.RunOptions.RuntimeState == nil {
		return
	}
	// 注入到RuntimeState，供后续LLM调用时使用
	inv.RunOptions.RuntimeState[constant.SessionAccountIDTempKey] = accountID

	// 持久化到session state，供后续run时使用
	state.PersistStateToService(ctx, session.StateMap{
		constant.SessionSelectedAccountIDStateKey: []byte(accountID),
	})
}

// getSelectedAccountIDFromSession reads the previously selected account_id persisted in the
// session state. It returns the account_id and true when a non-empty value exists.
func getSelectedAccountIDFromSession(inv *trpcagent.Invocation) (string, bool) {
	if inv == nil || inv.Session == nil {
		return "", false
	}
	raw, ok := inv.Session.GetState(constant.SessionSelectedAccountIDStateKey)
	if !ok || len(raw) == 0 {
		return "", false
	}
	return string(raw), true
}

// buildInterruptKey returns a unique interrupt key by appending the current message count to the
// base key, ensuring that repeated entries into the same node (e.g. after retry) produce distinct
// keys and do not conflict with stale checkpoints. This mirrors the pattern used by
// buildFallbackInterruptKey in graph_build.go.
func buildInterruptKey(state graph.State, base string) string {
	msgs, _ := state[graph.StateKeyMessages].([]trpcmodel.Message)
	return fmt.Sprintf("%s%s%d", base, constant.InterruptKeySeparator, len(msgs))
}
