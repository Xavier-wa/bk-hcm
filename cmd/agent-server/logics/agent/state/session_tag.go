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

package state

import (
	"context"

	"hcm/cmd/agent-server/logics/auth"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
)

// ParseSessionTag 从 graph state 中容错解析会话场景标签。
//
// session_tag 在 state 中的具体类型并不稳定：service 层首轮注入时是 enumor.IntentType，
// 而节点写回或经 checkpoint JSON 序列化/反序列化恢复后会变成 plain string。这里统一兼容两种类型，
// 避免直接做 .(enumor.IntentType) 断言因类型不符而拿到零值。
func ParseSessionTag(state graph.State) enumor.IntentType {
	switch v := state[constant.StateKeySessionTag].(type) {
	case enumor.IntentType:
		return v
	case string:
		return enumor.IntentType(v)
	default:
		return ""
	}
}

// SessionTagUpdater 抽象会话场景标签的落库能力，只取 data-service 会话客户端的 Update 方法。
//
// 在使用侧定义窄接口而不是直接依赖 *dataservice.Client：一是节点只需要这一个方法，
// 二是 scene_dispatch 的单测用打桩实现即可覆盖回写行为，不必架起整个 data-service 客户端。
type SessionTagUpdater interface {
	Update(kt *kit.Kit, req *dsaiagent.UpdateAiagentSessionReq) error
}

// PersistSessionTag 把本轮判定出的会话场景标签同步回写到 aiagent_session。
//
// 调用点在 scene_dispatch 判定出标签变化处，且排在 scene.switched 事件之前：DB 先于前端事件更新，
// 用户在本轮输出期间切走再切回会话时才不会从 DB 读到旧标签。
// 所有失败路径只记日志、不返回错误——标签落库只是 checkpoint 的投影更新，不能影响本轮路由与回答；
// 漏写由 service 层在 Run 结束后的差异对账兜底。
func PersistSessionTag(ctx context.Context, updater SessionTagUpdater, tag enumor.IntentType) {
	rid := rest.RidFromContext(ctx)
	if updater == nil {
		logs.Warnf("[session_tag] updater not injected, skip write back, tag: %s, rid: %s", tag, rid)
		return
	}

	threadID := threadIDFromContext(ctx)
	if threadID == "" {
		// 更新请求以会话主键定位，拿不到 threadID 时写入会打到非预期记录上，只能放弃本次回写。
		logs.Errorf("[session_tag] empty thread id, skip write back, tag: %s, rid: %s", tag, rid)
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, constant.SessionTagWriteBackTimeout)
	defer cancel()

	// 与 account_select 等节点内的跨服务调用保持同一套 kit 拼装方式（不用 core.NewBackendKit：
	// 它会读全局配置判定多租户，把配置依赖带进节点路径）。
	// Reviser 是必填字段，ctx 缺蓝鲸用户名时回落到后端操作用户，留空会被请求校验直接拒掉。
	// TODO 内部版使用多租户后，这里需要添加租户属性
	kt := kit.New()
	kt.AppCode = constant.AgentSourceAppCode
	kt.Rid = rid
	kt.Ctx = timeoutCtx
	kt.User = auth.BKUsernameFromContext(ctx)
	if kt.User == "" {
		kt.User = constant.BackendOperationUserKey
		logs.Warnf("[session_tag] no bk_username in context, fallback to %s, thread: %s, rid: %s",
			constant.BackendOperationUserKey, threadID, rid)
	}

	req := &dsaiagent.UpdateAiagentSessionReq{ID: threadID, Reviser: kt.User, SessionTag: tag}
	if err := updater.Update(kt, req); err != nil {
		logs.Warnf("[session_tag] write back failed, thread: %s, tag: %s, err: %v, rid: %s",
			threadID, tag, err, rid)
		return
	}

	logs.Infof("[session_tag] write back success, thread: %s, tag: %s, rid: %s", threadID, tag, rid)
}

// threadIDFromContext 取本轮的 thread（lineage）ID，它等于会话记录主键，是回写的定位依据。
// 来源为 makeRunOptionResolver 每轮注入的 RuntimeState[lineage_id]，而非 graph state——
// 后者在子图 mapper 与 checkpoint 恢复链路上都可能不带该键。
func threadIDFromContext(ctx context.Context) string {
	inv, ok := trpcagent.InvocationFromContext(ctx)
	if !ok || inv == nil || inv.RunOptions.RuntimeState == nil {
		return ""
	}
	threadID, _ := inv.RunOptions.RuntimeState[graph.CfgKeyLineageID].(string)
	return threadID
}
