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
	"strings"
	"time"

	"hcm/cmd/agent-server/logics/agent/message"
	"hcm/cmd/agent-server/logics/auth"
	protocloud "hcm/pkg/api/data-service/cloud"
	cloudserver "hcm/pkg/client/cloud-server"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/util"

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

// NewAccountSelectNode 返回账号选择 Function Node：按业务查可用云账号，单账号自动选中，多账号 HITL 中断。
//
// sessSvc：父图持有的 session.Service，用于跨 Run 持久化/复用已选账号（key 规则见 session_key.go）。
// appName：须与 service.loadSelectedAccountID 一致，保证读写同一 session 后端记录。
func NewAccountSelectNode(cloudClient *cloudserver.Client, sessSvc session.Service,
	appName string) graph.NodeFunc {

	return func(ctx context.Context, state graph.State) (any, error) {
		rid := rest.RidFromContext(ctx)
		logs.Infof("account_select: start, rid: %s", rid)

		inv, ok := trpcagent.InvocationFromContext(ctx)
		if !ok || inv == nil {
			return nil, fmt.Errorf("account_select: invocation not found in context, rid: %s", rid)
		}

		// bk_biz_id 由 makeRunOptionResolver 以 int64 类型注入到 inv.RunOptions.RuntimeState。
		// 子图从检查点恢复时，graph State 会被 JSON 反序列化（int64 → float64），
		// 且子图的 inv 可能未携带父图的 RuntimeState 最新值。
		// parseBkBizID 会从两个来源读取，并同时兼容 int64 与 float64。
		// 账号记忆按业务隔离，复用/持久化前必须先拿到 bk_biz_id。
		bkBizID := parseBkBizID(ctx, inv, state)
		if bkBizID <= 0 {
			return nil, fmt.Errorf("account_select: bk_biz_id not found or invalid, rid: %s", rid)
		}
		logs.Infof("account_select: bk_biz_id=%d, rid: %s", bkBizID, rid)

		accounts, err := listAccountsForBiz(ctx, cloudClient, bkBizID)
		if err != nil {
			logs.Errorf("account_select: list accounts failed, bk_biz_id=%d, err: %v, rid: %s",
				bkBizID, err, rid)
			return nil, fmt.Errorf("account_select: list accounts failed: %w", err)
		}
		logs.Infof("account_select: bk_biz_id=%d account count=%d, rid: %s", bkBizID, len(accounts), rid)

		// 同会话再次进入 host_apply 时，子图仍从 account_select 起跑（namespace 按轮次隔离），
		// 此处按优先级复用「当前业务」已选账号，命中则跳过 HITL，不再弹「请选择云账号」。
		// 顺序：session 后端（跨 Run）→ RuntimeState（run 起点回填）→ graph State（InputMapper 带入）。
		// 复用前必须校验该账号仍存在于当前业务可用列表且 vendor 受支持：账号被禁用/回收/移出业务时
		// 不复用，重新走账号选择（TAPD 136108620 AC-004）。
		if accountID, ok := tryReuseAccountID(ctx, inv, sessSvc, appName, state); ok {
			if isAccountUsable(accountID, accounts) {
				logs.Infof("account_select: reuse persisted account_id=%s for biz=%d, skip selection, rid: %s",
					accountID, bkBizID, rid)
				injectAccountIDToRuntimeState(ctx, inv, accountID, sessSvc, appName)
				return graph.State{
					constant.SessionAccountIDTempKey:  accountID,
					constant.AccountSelectNextNodeKey: enumor.CvmApplyNodeLLM,
				}, nil
			}
			logs.Infof("account_select: persisted account_id=%s no longer usable in biz=%d, re-select, rid: %s",
				accountID, bkBizID, rid)
		}

		return routeByAccountCount(ctx, state, inv, sessSvc, appName, accounts)
	}
}

// isAccountUsable 判断待复用的 account_id 是否仍存在于当前业务可用账号列表中，且 vendor 受支持
// （当前仅 tcloud-ziyan）。用于 AC-004：已记住账号失效时不复用、重新弹出选择。
func isAccountUsable(accountID string, accounts []*protocloud.AccountBizRelWithAccount) bool {
	if accountID == "" {
		return false
	}
	for _, acc := range accounts {
		if acc.ID == accountID {
			return acc.Vendor == enumor.TCloudZiyan
		}
	}
	return false
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
	sessSvc session.Service, appName string, accounts []*protocloud.AccountBizRelWithAccount) (
	any, error) {

	rid := rest.RidFromContext(ctx)
	switch count := len(accounts); {
	case count == 0:
		return graph.State{constant.AccountSelectNextNodeKey: enumor.CvmApplyNodeFallback}, nil

	case count == 1:
		accountID := accounts[0].ID
		logs.Infof("account_select: auto-select account_id=%s, rid: %s", accountID, rid)
		injectAccountIDToRuntimeState(ctx, inv, accountID, sessSvc, appName)
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
		// 解析 account_id：优先 forwardedProps；子图 resume 时该值不在子图 RuntimeState
		//（makeSubgraphInputMapper 会剥键），回退用 interrupt 返回的 userInput 匹配选项
		//（前端选账号时 resume 命令即为 account_id 字符串，如 "0000002b"）。
		accountID := resolveSelectedAccountID(ctx, inv, userInput, accounts)
		if accountID != "" {
			injectAccountIDToRuntimeState(ctx, inv, accountID, sessSvc, appName)
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

// resolveSelectedAccountID 解析用户选定的 account_id。
//
// 优先级：
//  1. RuntimeState[forwarded_resume_value] — 父图 run 起点由 tryPrepareAutoResume 注入；
//  2. userInput 匹配账号选项 — 子图 resume 时的兜底（子图 inv.RuntimeState 不含 forwarded）。
func resolveSelectedAccountID(ctx context.Context, inv *trpcagent.Invocation, userInput string,
	accounts []*protocloud.AccountBizRelWithAccount) string {

	rid := rest.RidFromContext(ctx)

	if inv != nil && inv.RunOptions.RuntimeState != nil {
		raw := inv.RunOptions.RuntimeState[constant.StateKeyForwardedResumeValue]
		if accountID, ok := raw.(string); ok && accountID != "" {
			logs.Infof("account_select: user selected account_id=%s (forwarded), rid: %s", accountID, rid)
			return accountID
		}
		if raw != nil {
			logs.Warnf("account_select: invalid forwarded account_id, expected non-empty string, got %T(%v), rid: %s",
				raw, raw, rid)
		}
	}

	if accountID := matchAccountIDFromUserInput(userInput, accounts); accountID != "" {
		logs.Infof("account_select: user selected account_id=%s (matched input), rid: %s", accountID, rid)
		return accountID
	}
	return ""
}

// matchAccountIDFromUserInput 将 interrupt 恢复文本精确匹配为 account_id。
// 仅覆盖确定性输入：前端卡片点选（resume 值即 account_id）与用户手打裸 ID。
// 自然语言选账号（如「我选择云账号：xxx」）不在此处理，交由 select_account 工具结构化接住，
// 避免脆弱的账号名包含匹配被否定句（如「我不选 xxx」）误命中。
func matchAccountIDFromUserInput(userInput string, accounts []*protocloud.AccountBizRelWithAccount) string {
	userInput = strings.TrimSpace(userInput)
	if userInput == "" || len(accounts) == 0 {
		return ""
	}
	for _, acc := range accounts {
		if userInput == acc.ID {
			return acc.ID
		}
	}
	return ""
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

// injectAccountIDToRuntimeState 写入当轮 RuntimeState（供 prompt 占位符）并持久化到 session 后端（供跨 Run 复用）。
func injectAccountIDToRuntimeState(ctx context.Context, inv *trpcagent.Invocation, accountID string,
	sessSvc session.Service, appName string) {

	if inv == nil || inv.RunOptions.RuntimeState == nil {
		return
	}
	inv.RunOptions.RuntimeState[constant.SessionAccountIDTempKey] = accountID

	// 持久化 key 与 service.loadSelectedAccountID 一致；不依赖子图 inv.Session（InputMapper 会过滤）。
	if sessSvc == nil {
		logs.Warnf("account_select: session service not injected, skip persist selected account, rid: %s",
			rest.RidFromContext(ctx))
		return
	}
	key := sessionKeyFromInvocation(ctx, inv, appName)
	if key.SessionID == "" || key.AppName == "" {
		logs.Errorf("account_select: skip persist selected account due to invalid key=%+v, rid: %s",
			key, rest.RidFromContext(ctx))
		return
	}

	//将选中账号写入 session 后端（key: cvm_apply:selected_account_id）。
	if err := sessSvc.UpdateSessionState(ctx, key, session.StateMap{
		constant.SessionSelectedAccountIDStateKey: []byte(accountID)}); err != nil {
		// 持久化失败不阻断本轮：账号已在 RuntimeState；打 Error 便于对照写成功/读失败。
		logs.Errorf("account_select: persist selected account_id failed, key=%+v err=%v, rid: %s",
			key, err, rest.RidFromContext(ctx))
		return
	}
	logs.Infof("account_select: persist selected account_id=%s success, key=%+v, rid: %s",
		accountID, key, rest.RidFromContext(ctx))
}

// tryReuseAccountID 查找本会话可复用的已选账号，命中则跳过账号选择 HITL。
//
// 三源按优先级（任一命中即可）：
//  1. session 后端 — 跨 Run 权威来源，account_select 持久化的 cvm_apply:selected_account_id；
//  2. RuntimeState[account_id] — makeRunOptionResolver 在 run 起点从后端回填；
//  3. graph State[account_id] — 同轮 host_apply 子图 InputMapper 从父图 state 拷贝。
func tryReuseAccountID(ctx context.Context, inv *trpcagent.Invocation, sessSvc session.Service,
	appName string, state graph.State) (string, bool) {

	if accountID, ok := getSelectedAccountIDFromSession(ctx, inv, sessSvc, appName); ok {
		return accountID, true
	}
	if inv != nil && inv.RunOptions.RuntimeState != nil {
		if accountID, ok := inv.RunOptions.RuntimeState[constant.SessionAccountIDTempKey].(string); ok &&
			accountID != "" {
			return accountID, true
		}
	}
	if state != nil {
		if accountID, ok := state[constant.SessionAccountIDTempKey].(string); ok && accountID != "" {
			return accountID, true
		}
	}
	return "", false
}

// getSelectedAccountIDFromSession 从 session 后端读取持久化的 account_id（统一 session key）。
func getSelectedAccountIDFromSession(ctx context.Context, inv *trpcagent.Invocation,
	sessSvc session.Service, appName string) (string, bool) {

	if sessSvc == nil || inv == nil {
		return "", false
	}
	key := sessionKeyFromInvocation(ctx, inv, appName)
	if key.SessionID == "" || key.AppName == "" {
		return "", false
	}
	sess, err := sessSvc.GetSession(ctx, key)
	if err != nil {
		// 读后端失败不阻断：走正常账号选择分支或 RuntimeState 回填分支。
		logs.Warnf("account_select: GetSession failed, key=%+v err=%v, rid: %s",
			key, err, rest.RidFromContext(ctx))
		return "", false
	}
	if sess == nil {
		return "", false
	}
	raw, ok := sess.GetState(constant.SessionSelectedAccountIDStateKey)
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

// parseBkBizID 解析 bk_biz_id，兼容 checkpoint 反序列化后 int64→float64 的类型变化。
// 优先读 RuntimeState（每轮 fresh），回退 graph State（子图 resume 时父 RuntimeState 可能未带入）。
func parseBkBizID(ctx context.Context, inv *trpcagent.Invocation, state graph.State) int64 {
	rid := rest.RidFromContext(ctx)
	if inv != nil && inv.RunOptions.RuntimeState != nil {
		if v, ok := inv.RunOptions.RuntimeState[constant.SessionBkBizIDStateKey].(int64); ok && v > 0 {
			return v
		}
	}

	id, err := util.GetInt64ByInterface(state[constant.SessionBkBizIDStateKey])
	if err != nil {
		logs.Errorf("account_select: parse bk_biz_id failed, err: %v, rid: %s", err, rid)
		return 0
	}
	return id
}
