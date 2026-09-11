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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"hcm/cmd/agent-server/logics/agent/hitl"
	authlogic "hcm/cmd/agent-server/logics/auth"
	agenttool "hcm/cmd/agent-server/logics/tool"
	"hcm/cmd/agent-server/logics/toolproxy"
	woatypes "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/auth"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/util"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

const (
	// applyCancelToolResult 是用户未点击确认按钮时写回该 tool_call 的结果。
	applyCancelToolResult = "本次申领提单未提交：用户没有点击申领确认卡片上的「确认提交」按钮。"
	// applyCancelNotice 是用户以自由文本回复确认卡片时，注入上下文用于引导模型下一条回复的说明。
	// 用户常直接输入「确认提交」这类文字，模型容易把它当成提交许可就直接重新调用提单工具，结果又弹一次
	// 确认卡片且没有任何文字说明。这里的要求是：先用文字讲清「文字不能替代点击」，再由模型按用户意愿
	// 决定是否重新调用提单工具重新弹卡片，让用户可以接着点击按钮完成提交。
	applyCancelNotice = "提单工具已被确认门禁拦截，单据未提交。用户在输入框输入的文字" +
		"（包括「确认」「确认提交」「提交」等）都不能作为提交依据，只有点击申领确认卡片上的" +
		"「确认提交」按钮才会真正提单。回复时必须先用文字说明这一点：如需提交本次申领，" +
		"请在申领确认卡片上点击「确认提交」按钮；若用户已表达继续提交的意愿，" +
		"可在该说明之后再次调用提单工具重新弹出确认卡片，方便用户直接点击按钮。"
	// applyArgsParseFailed 是申领参数无法解析时写回该 tool_call 的结果。
	applyArgsParseFailed = "申领参数解析失败，暂时无法提单。"
	// applyMissingBizID 是无法确定申领所属业务时写回该 tool_call 的结果。
	applyMissingBizID = "无法确定本次申领所属业务（缺少 bk_biz_id），暂时无法提单。请重新发起申领。"
	// applyAuthCheckFailed 是主动鉴权本身失败（IAM 报错/超时/authorizer 未注入）时写回该 tool_call 的结果。
	// 此时尚未判定用户是否无权限，因此按系统异常提示，且不提供权限申请地址。
	applyAuthCheckFailed = "申领权限校验失败，暂时无法提单，请稍后重试。"
	// applyPreCheckFailedPrefix 是 woa 预检以非权限错误失败时写回该 tool_call 的结果前缀。
	applyPreCheckFailedPrefix = "申领前置校验失败，暂时无法提单："
	// applyPreCheckRejectPrefix 是 woa 预检业务不通过（额度/库存/参数等）时写回该 tool_call 的结果前缀。
	applyPreCheckRejectPrefix = "申领前置校验未通过："
	// applyNoPermMsgFmt 是无权限且申请地址可用时面向用户的说明，占位为用户名与业务 ID。
	applyNoPermMsgFmt = "当前用户 %s 在业务 %d 下没有主机申领权限，可点击权限申请链接申请后重试。"
	// applyNoPermDegradeMsgFmt 是无权限但申请地址不可用时的降级说明，占位为用户名与业务 ID。
	applyNoPermDegradeMsgFmt = "当前用户 %s 在业务 %d 下没有主机申领权限，且当前无法直接跳转申请，" +
		"请联系该业务管理员开通主机申领权限后再重试。"
)

// applyCheckFunc 提单前只读校验调用，默认走 woa-server，便于单测注入桩。
type applyCheckFunc func(ctx context.Context, bizID int64, req *woatypes.ApplyReq) (*woatypes.CheckApplyOrderResp,
	error)

// applyAuthorizeFunc 主机申领权限的主动判定，默认走 auth.Authorizer.Authorize，便于单测注入桩。
// 返回 (authorized, err)：err 非空表示尚不能判定用户是否有权限（IAM 报错/超时）。
type applyAuthorizeFunc func(kt *kit.Kit, res meta.ResourceAttribute) (bool, error)

// applyURLFunc 生成权限申请地址，默认走 auth.Authorizer 的 GetPermissionToApply + GetApplyPermUrl，
// 便于单测注入桩。
type applyURLFunc func(kt *kit.Kit, res meta.ResourceAttribute) (string, error)

// applyDeniedResult 是无权限拒绝写回该 tool_call 的结构化结果。
// 仅无权限路径使用该结构；其它失败仍写回现网风格的纯文本。
type applyDeniedResult struct {
	// Code 无权限标识，固定为 errf.PermissionDenied（2030403），供消费方区分无权限与其它失败。
	Code int32 `json:"code"`
	// Message 面向用户的简短原因，含用户名与业务。
	Message string `json:"message"`
	// ApplyURL 可直接打开的权限申请地址；生成失败时省略，不给空串或伪造地址。
	ApplyURL string `json:"apply_url,omitempty"`
}

// createCvmApplyGate 守护主机申领提单工具（create_biz_apply），实现 hitl.Handler。
// 门禁持有 authorizer 与 woa-server 客户端：用户确认后先对当前用户、当前业务主动判定主机申领权限，
// 鉴权通过后再调用 woa 提单前只读校验接口执行真实前置校验。
type createCvmApplyGate struct {
	client     *client.ClientSet
	authorizer auth.Authorizer
	check      applyCheckFunc
	authorize  applyAuthorizeFunc
	applyURL   applyURLFunc
}

var (
	_ hitl.Handler       = (*createCvmApplyGate)(nil)
	_ hitl.CancelNoticer = (*createCvmApplyGate)(nil)
)

// newCreateCvmApplyGate 创建主机申领门禁，注入用于 woa 校验调用的 client set 与用于主动鉴权的 authorizer。
// 仅需工具名的场景（如 GetEnabledGateToolNames）可对两者传 nil：此时主动鉴权会 fail closed。
func newCreateCvmApplyGate(clientSet *client.ClientSet, authorizer auth.Authorizer) hitl.Handler {
	g := &createCvmApplyGate{client: clientSet, authorizer: authorizer}
	g.check = g.callWoaCheck
	g.authorize = g.callAuthorize
	g.applyURL = g.callApplyURL
	return g
}

// newGateKit 从运行上下文构造后端 kit，供 woa 侧调用使用。
// 用户身份取自请求上下文（X-Bkapi-User-Name），缺失时回退到提单参数中的提单人。
func newGateKit(ctx context.Context, fallbackUser string) *kit.Kit {
	kt := core.NewBackendKit()
	kt.Ctx = ctx
	kt.Rid = rest.RidFromContext(ctx)
	if user := authlogic.BKUsernameFromContext(ctx); user != "" {
		kt.User = user
		return kt
	}
	if fallbackUser != "" {
		kt.User = fallbackUser
	}
	return kt
}

// newAuthKit 构造用于主机申领主动鉴权的 kit，判定主体只认请求上下文中的蓝鲸用户名。
// 申领参数中的提单人由模型生成、且用户可在确认卡片上编辑，不能作为权限判定主体；
// core.NewBackendKit 预置的后端操作用户同样不能顶替真实用户。
// 上下文缺用户名时返回 nil，由调用方按 fail closed 拒绝，不得当作有权限。
func newAuthKit(ctx context.Context) *kit.Kit {
	if authlogic.BKUsernameFromContext(ctx) == "" {
		return nil
	}
	return newGateKit(ctx, "")
}

// hostApplyResource 返回主机申领的鉴权资源属性。
// 必须与 woa-server CheckBizApplyOrder 的鉴权点保持一致（cmd/woa-server/service/task/check.go）：
// 业务 + 创建/申领 + 当前业务 ID。两侧不一致会出现门禁放行但 woa 拒绝、或反之。
func hostApplyResource(bizID int64) meta.ResourceAttribute {
	return meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.Biz, Action: meta.Create},
		BizID: bizID,
	}
}

// callAuthorize 调用注入的 authorizer 判定当前用户对该资源是否有权限。
func (g *createCvmApplyGate) callAuthorize(kt *kit.Kit, res meta.ResourceAttribute) (bool, error) {
	if g.authorizer == nil {
		return false, errors.New("authorizer not injected")
	}

	_, authorized, err := g.authorizer.Authorize(kt, res)
	if err != nil {
		return false, err
	}
	return authorized, nil
}

// callApplyURL 为给定资源生成权限申请地址。
func (g *createCvmApplyGate) callApplyURL(kt *kit.Kit, res meta.ResourceAttribute) (string, error) {
	if g.authorizer == nil {
		return "", errors.New("authorizer not injected")
	}

	permission, err := g.authorizer.GetPermissionToApply(kt, res)
	if err != nil {
		return "", err
	}
	return g.authorizer.GetApplyPermUrl(kt, permission)
}

// callWoaCheck 从运行上下文构造后端 kit，并调用 woa-server 的提单前只读校验。
func (g *createCvmApplyGate) callWoaCheck(ctx context.Context, bizID int64, req *woatypes.ApplyReq) (
	*woatypes.CheckApplyOrderResp, error) {

	if g.client == nil {
		return nil, errors.New("woa client not injected")
	}

	return g.client.WoaServer().Task.CheckBizApplyOrder(newGateKit(ctx, req.User), bizID, req)
}

// ToolName 返回受门禁守护的工具名。
func (g *createCvmApplyGate) ToolName() string {
	return constant.ToolNameCreateBizApply
}

// EventKind 将中断路由到 tool.confirm 前端事件。
func (g *createCvmApplyGate) EventKind() string {
	return constant.ToolConfirmCreateCvmApplyInterruptKey
}

// CancelNotice 返回取消提单后引导模型下一条回复的说明，实现 hitl.CancelNoticer。
func (g *createCvmApplyGate) CancelNotice() string {
	return applyCancelNotice
}

// BuildPayload 从工具调用入参构造申领确认卡片的 payload。
func (g *createCvmApplyGate) BuildPayload(ctx context.Context, tc *model.ToolCall) (any, error) {
	rid := rest.RidFromContext(ctx)

	args, err := unmarshalArgs(ctx, tc)
	if err != nil {
		logs.Errorf("apply gate: unmarshal args failed, err: %v, rid: %s", err, rid)
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	return &ConfirmPayload{
		Tool: g.ToolName(),
		Data: args,
	}, nil
}

// OnResume routes the resume to onConfirm or onCancel based on the resume value origin:
//   - hitl.CancelActionSignal: the HITL node detected no forwardedProps.resumeValue → cancel.
//   - any other string: forwardedProps.resumeValue from the frontend carrying confirmed args JSON → confirm.
func (g *createCvmApplyGate) OnResume(ctx context.Context, tc *model.ToolCall, resumeValue any) (
	hitl.ResumeResult, error) {

	rid := rest.RidFromContext(ctx)

	args, ok := resumeValue.(string)
	if !ok || args == "" {
		return g.onCancel(ctx, tc, "no resume value or empty"), nil
	}

	if args == hitl.CancelActionSignal {
		return g.onCancel(ctx, tc, "cancel action signal"), nil
	}

	// 将 args 反序列化为 map[string]any
	var confirmedArgs map[string]any
	if err := json.Unmarshal([]byte(args), &confirmedArgs); err != nil {
		logs.Errorf("apply gate: unmarshal confirmed args failed, err: %v, rid: %s", err, rid)
		return g.reject(tc, applyArgsParseFailed), nil
	}

	return g.onConfirm(ctx, tc, confirmedArgs)
}

// onConfirm executes the pre-submit checks after the user confirmed; routes to the tool node
// on pass, or falls back to the llm node on missing biz ID / no permission / validation failure.
// confirmedArgs carries the args returned by the frontend via forwardedProps.resumeValue;
// if empty, the original tool-call args are used.
func (g *createCvmApplyGate) onConfirm(ctx context.Context, tc *model.ToolCall, confirmedArgs map[string]any) (
	hitl.ResumeResult, error) {

	rid := rest.RidFromContext(ctx)
	finalArgs, err := g.resolveFinalArgs(ctx, tc, confirmedArgs)
	if err != nil {
		return g.reject(tc, applyArgsParseFailed), nil
	}

	bizID, found := g.resolveBizID(ctx, tc, finalArgs)
	if !found {
		logs.Errorf("apply gate: extract bk_biz_id from path_param failed, rid: %s", rid)
		return g.reject(tc, applyMissingBizID), nil
	}

	body := extractApplyBody(finalArgs)
	kt := newAuthKit(ctx)
	if kt == nil {
		logs.Errorf("apply gate: no bk_username in request context, cannot authorize host apply, "+
			"bizID: %d, rid: %s", bizID, rid)
		return g.reject(tc, applyAuthCheckFailed), nil
	}
	res := hostApplyResource(bizID)

	authorized, err := g.doAuthorize(kt, res)
	if err != nil {
		logs.Errorf("apply gate: authorize host apply failed, err: %v, bizID: %d, rid: %s", err, bizID, rid)
		return g.reject(tc, applyAuthCheckFailed), nil
	}
	if !authorized {
		logs.Infof("apply gate: no host apply permission, user: %s, bizID: %d, rid: %s", kt.User, bizID, rid)
		return g.reject(tc, g.buildDeniedResult(kt, res, bizID)), nil
	}

	ok, reason, err := g.validateApply(ctx, bizID, body)
	if err != nil {
		// 窄兜底：主动鉴权已通过但 woa 仍以无权限拒绝（两次调用间权限被回收、或鉴权点漂移），
		// 仍按无权限引导处理，不拼成「暂时无法提单」的系统异常文案。
		if errf.Error(err).Code == errf.PermissionDenied {
			logs.Warnf("apply gate: woa check denied on permission after gate authorized, err: %v, "+
				"bizID: %d, rid: %s", err, bizID, rid)
			return g.reject(tc, g.buildDeniedResult(kt, res, bizID)), nil
		}
		logs.Errorf("apply gate: validate apply failed, err: %v, bizID: %d, rid: %s", err, bizID, rid)
		return g.reject(tc, applyPreCheckFailedPrefix+err.Error()), nil
	}
	if !ok {
		logs.Infof("apply gate: validate apply not passed, reason: %s, bizID: %d, rid: %s", reason, bizID, rid)
		return g.reject(tc, applyPreCheckRejectPrefix+reason), nil
	}

	logs.Infof("apply gate: validate apply passed, proceed to submit, rid: %s", rid)
	return g.proceed(tc, confirmedArgs, rid)
}

// resolveFinalArgs 返回本次确认最终生效的申领参数：优先用户在确认卡片上回传的参数，
// 为空时回退到原始工具调用参数。
func (g *createCvmApplyGate) resolveFinalArgs(ctx context.Context, tc *model.ToolCall,
	confirmedArgs map[string]any) (map[string]any, error) {

	if len(confirmedArgs) > 0 {
		return confirmedArgs, nil
	}

	args, err := unmarshalArgs(ctx, tc)
	if err != nil {
		logs.Errorf("apply gate: unmarshal original args failed, err: %v, rid: %s", err, rest.RidFromContext(ctx))
		return nil, err
	}
	return args, nil
}

// resolveBizID 提取本次申领所属业务 ID。
// 用户在确认卡片编辑参数后可能不回传 path_param，故回退到原始工具调用参数再取一次。
func (g *createCvmApplyGate) resolveBizID(ctx context.Context, tc *model.ToolCall, finalArgs map[string]any) (
	int64, bool) {

	if bizID, found := extractApplyPathParam(finalArgs); found {
		return bizID, true
	}
	origArgs, err := unmarshalArgs(ctx, tc)
	if err != nil {
		return 0, false
	}
	return extractApplyPathParam(origArgs)
}

// doAuthorize 对当前用户、当前业务主动判定主机申领权限。
// 鉴权入口未初始化时返回 error，由调用方按 fail closed 拒绝，不得当作有权限。
func (g *createCvmApplyGate) doAuthorize(kt *kit.Kit, res meta.ResourceAttribute) (bool, error) {
	if g.authorize == nil {
		return false, errors.New("apply authorize func not initialized")
	}
	return g.authorize(kt, res)
}

// buildDeniedResult 构造无权限拒绝写回该 tool_call 的 JSON 结果。
// 申请地址可用时带 apply_url，不可用时省略该字段并给出降级说明；两种情况都带无权限标识。
func (g *createCvmApplyGate) buildDeniedResult(kt *kit.Kit, res meta.ResourceAttribute, bizID int64) string {
	rst := &applyDeniedResult{
		Code:     errf.PermissionDenied,
		ApplyURL: g.resolveApplyURL(kt, res, bizID),
	}
	rst.Message = fmt.Sprintf(applyNoPermDegradeMsgFmt, kt.User, bizID)
	if rst.ApplyURL != "" {
		rst.Message = fmt.Sprintf(applyNoPermMsgFmt, kt.User, bizID)
	}

	raw, err := json.Marshal(rst)
	if err != nil {
		logs.Errorf("apply gate: marshal denied result failed, err: %v, bizID: %d, rid: %s", err, bizID, kt.Rid)
		return fmt.Sprintf("%d：%s", rst.Code, rst.Message)
	}
	return string(raw)
}

// resolveApplyURL 为已判定无权限的资源生成权限申请地址，取不到则返回空串走降级。
// 与鉴权本身失败不同：此处已经判定无权限，申请地址失败只降级文案，不升格为系统异常。
func (g *createCvmApplyGate) resolveApplyURL(kt *kit.Kit, res meta.ResourceAttribute, bizID int64) string {
	if g.applyURL == nil {
		logs.Warnf("apply gate: apply url func not initialized, degrade without apply_url, bizID: %d, rid: %s",
			bizID, kt.Rid)
		return ""
	}

	url, err := g.applyURL(kt, res)
	if err != nil {
		logs.Warnf("apply gate: get apply perm url failed, err: %v, bizID: %d, rid: %s", err, bizID, kt.Rid)
		return ""
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		logs.Warnf("apply gate: apply perm url is empty or not http(s): %q, bizID: %d, rid: %s",
			url, bizID, kt.Rid)
		return ""
	}
	return url
}

// onCancel 处理取消动作：关闭该 tool_call 并回退到 llm 节点。
// reason 说明本次取消的判定来源，仅用于日志。
// 引导用户重新点击确认按钮的说明由 CancelNotice 提供，经 hitl 节点以 assistant 消息注入：
// 用户手动输入时该 tool 结果会被 llm 节点的历史工具结果过滤器替换为占位符，无法承载引导内容。
func (g *createCvmApplyGate) onCancel(ctx context.Context, tc *model.ToolCall, reason string) hitl.ResumeResult {
	logs.Infof("apply gate: user cancelled apply, reason: %s, rid: %s", reason, rest.RidFromContext(ctx))
	return g.reject(tc, applyCancelToolResult)
}

// proceed 构造放行到 tool 节点的 ResumeResult。
//
// HITL 恢复后若直接复用原始 tool_call_id（c1）继续执行，tool 节点会再次流式输出相同 ID 的
// ToolCallStart/End 事件，导致前端历史回放出现重复 ID 而失败。
// 为此采用 ID 置换策略：
//  1. 以合成的 tool 结果消息关闭原始 c1（前端视角：c1 已结束）；
//  2. 注入新的 assistant 消息，携带新 ID（c2）和最终入参；
//  3. 路由到 tool 节点执行 c2 ——c2 为全新 ID，不会产生重复事件。
func (g *createCvmApplyGate) proceed(tc *model.ToolCall, confirmedArgs map[string]any, rid string) (
	hitl.ResumeResult, error) {

	finalArgs, err := buildFinalArgs(tc, confirmedArgs, rid)
	if err != nil {
		return hitl.ResumeResult{}, err
	}

	// 生成新 tool_call_id：保留原始 ID 的厂商前缀，仅替换末段唯一标识
	newCallID := agenttool.RegenToolCallID(tc.ID)
	newCall := model.ToolCall{
		ID: newCallID,
		Function: model.FunctionDefinitionParam{
			Name:      tc.Function.Name,
			Arguments: finalArgs,
		},
	}
	logs.Infof("apply gate: proceed, original_id=%s, new_id=%s, rid: %s", tc.ID, newCallID, rid)
	return hitl.ResumeResult{
		Next: enumor.CvmApplyNodeTool,
		// 原子追加两条消息：关闭原始 c1 + 注入新调用 c2，tool 节点将执行 c2
		AppendMessages: []model.Message{
			{Role: model.RoleTool, ToolID: tc.ID, ToolName: tc.Function.Name,
				Content: "user approved, proceeding with submission"},
			{Role: model.RoleAssistant, ToolCalls: []model.ToolCall{newCall}},
		},
	}, nil
}

// buildFinalArgs 优先使用用户在确认卡上编辑过的参数，否则回退到原始 tool_call 参数。
func buildFinalArgs(tc *model.ToolCall, confirmedArgs map[string]any, rid string) ([]byte, error) {
	if len(confirmedArgs) == 0 {
		return tc.Function.Arguments, nil
	}
	newArgs, err := rewriteToolCallArgs(tc, confirmedArgs)
	if err != nil {
		logs.Errorf("apply gate: rewrite tool call args failed, err: %v, rid: %s", err, rid)
		return nil, fmt.Errorf("rewrite tool call args: %w", err)
	}
	return newArgs, nil
}

// reject 构造以面向用户提示关闭该 tool_call 并回退到 llm 节点的结果。
func (g *createCvmApplyGate) reject(tc *model.ToolCall, message string) hitl.ResumeResult {
	return hitl.ResumeResult{
		Next:           enumor.CvmApplyNodeLLM,
		ClearUserInput: true,
		AppendMessages: []model.Message{
			{Role: model.RoleTool, ToolID: tc.ID, ToolName: tc.Function.Name, Content: message},
		},
	}
}

// validateApply 调用 woa-server 提单前校验接口执行真实的库存/预测校验。
// 返回 (pass, reason, err)：业务拒绝以 (false, reason, nil) 返回，供 LLM 转述给用户；
// 系统异常（客户端缺失、下游失败）以 err 返回。
func (g *createCvmApplyGate) validateApply(ctx context.Context, bizID int64, body map[string]any) (
	bool, string, error) {

	rid := rest.RidFromContext(ctx)

	if g.check == nil {
		logs.Errorf("apply gate: check func not initialized, rid: %s", rid)
		return false, "", errors.New("apply check func not initialized")
	}
	if bizID <= 0 {
		logs.Errorf("apply gate: invalid bk_biz_id: %d, rid: %s", bizID, rid)
		return false, "", fmt.Errorf("invalid bk_biz_id: %d", bizID)
	}

	req, err := buildApplyReq(body)
	if err != nil {
		logs.Errorf("apply gate: build apply request failed, err: %v, bizID: %d, rid: %s", err, bizID, rid)
		return false, "", fmt.Errorf("build apply request: %w", err)
	}
	req.BkBizId = bizID

	rst, err := g.check(ctx, bizID, req)
	if err != nil {
		logs.Errorf("apply gate: call woa check apply failed, err: %v, bizID: %d, rid: %s", err, bizID, rid)
		return false, "", err
	}
	if rst == nil {
		logs.Errorf("apply gate: woa check apply returned nil result, bizID: %d, rid: %s", bizID, rid)
		return false, "", errors.New("woa check apply returned nil result")
	}

	logs.Infof("apply gate: woa check apply done, pass: %v, bizID: %d, rid: %s", rst.Pass, bizID, rid)
	return rst.Pass, rst.Reason, nil
}

// buildApplyReq 通过 JSON 往返把通用申领 body 转换为 woa-server 的 ApplyReq，
// 复用共享的 json tag，使门禁与具体字段列表解耦。
func buildApplyReq(body map[string]any) (*woatypes.ApplyReq, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal apply body: %w", err)
	}
	req := new(woatypes.ApplyReq)
	if err = json.Unmarshal(raw, req); err != nil {
		return nil, fmt.Errorf("unmarshal apply request: %w", err)
	}
	return req, nil
}

// extractApplyPathParam 从 MCP 工具入参的 path_param 包装中取出 bk_biz_id。
// create_biz_apply 的 bk_biz_id 放在 path_param 中，值可能解码为 JSON 数字或字符串。
// 字段缺失或非法时返回 (0, false)。
func extractApplyPathParam(args map[string]any) (int64, bool) {
	pathParam, ok := args["path_param"].(map[string]any)
	if !ok {
		return 0, false
	}
	raw, ok := pathParam["bk_biz_id"]
	if !ok || raw == nil {
		return 0, false
	}
	id, err := util.GetInt64ByInterface(raw)
	if err != nil {
		return 0, false
	}
	return id, true
}

// unmarshalArgs 将有效的工具调用入参反序列化为通用 map。
// 真实工具经 proxy execute_tool 调用时，有效入参是信封内的 "parameters"；否则是 tool_call 自身入参。
func unmarshalArgs(ctx context.Context, tc *model.ToolCall) (map[string]any, error) {
	rid := rest.RidFromContext(ctx)
	resolved := toolproxy.ResolveToolCall(tc)
	args := make(map[string]any)
	if len(resolved.Arguments) == 0 {
		return args, nil
	}
	if err := json.Unmarshal(resolved.Arguments, &args); err != nil {
		logs.Errorf("apply gate: unmarshal args failed, resolved args: %v, err: %v, rid: %s",
			resolved.Arguments, err, rid)
		return nil, fmt.Errorf("apply gate: unmarshal args failed: %w", err)
	}
	return args, nil
}

// rewriteToolCallArgs 在用户于确认卡片上编辑申领参数后，构造新的原始 tool_call 入参。
// 对 proxy execute_tool 调用，仅替换内层 "parameters" 对象，并保留 tool_name / schema_token
// （否则 proxy 的 schema_token 校验会拒绝提交）；对直接调用，则直接序列化编辑后的入参。
func rewriteToolCallArgs(tc *model.ToolCall, editedArgs map[string]any) ([]byte, error) {
	if !toolproxy.ResolveToolCall(tc).Proxy {
		return json.Marshal(editedArgs)
	}

	envelope := make(map[string]any)
	if len(tc.Function.Arguments) > 0 {
		if err := json.Unmarshal(tc.Function.Arguments, &envelope); err != nil {
			return nil, fmt.Errorf("unmarshal proxy envelope: %w", err)
		}
	}
	envelope["parameters"] = editedArgs
	return json.Marshal(envelope)
}

// extractApplyBody 返回真实的申领 body，存在 MCP body_param 包装时自动解包。
func extractApplyBody(args map[string]any) map[string]any {
	if body, ok := args["body_param"].(map[string]any); ok {
		return body
	}
	return args
}
