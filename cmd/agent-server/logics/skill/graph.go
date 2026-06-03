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

package skill

import (
	"context"
	"encoding/json"
	"time"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
)

// MakeSkillLoadAfterToolCallback returns an AfterNodeCallback that detects
// skill_load / skill_select_docs tool calls and persists the loading state
// into the invocation's session state.
func MakeSkillLoadAfterToolCallback(agentName string) graph.AfterNodeCallback {
	return func(ctx context.Context, callbackCtx *graph.NodeCallbackContext, state graph.State, result any,
		nodeErr error) (any, error) {

		rid := rest.RidFromContext(ctx)

		if nodeErr != nil {
			logs.Warnf("[skill_graph] nodeErr != nil, skip: %v, rid: %s", nodeErr, rid)
			return result, nil
		}

		inv, ok := trpcagent.InvocationFromContext(ctx)
		if !ok || inv == nil || inv.Session == nil {
			logs.Warnf("[skill_graph] no invocation or session in context, skip skill load tracking "+
				"(ok=%v inv=%v session=%v), rid: %s",
				ok, inv != nil, inv != nil && inv.Session != nil, rid)
			return result, nil
		}

		msgs, ok := state[graph.StateKeyMessages].([]model.Message)
		if !ok || len(msgs) == 0 {
			logs.Warnf("[skill_graph] no messages in state (ok=%v len=%d), skip skill load tracking, rid: %s", ok,
				len(msgs), rid)
			return result, nil
		}

		assistantMsg := extractLastAssistantWithToolCalls(msgs)
		if assistantMsg == nil {
			logs.Infof("[skill_graph] no assistant message with tool calls found, skip skill load, rid: %s", rid)
			return result, nil
		}

		for _, tc := range assistantMsg.ToolCalls {
			logs.Infof("[skill_graph] processing tool call: name=%s, rid: %s", tc.Function.Name, rid)
			switch tc.Function.Name {
			case constant.SkillLoadToolName:
				recordSkillLoadedToState(ctx, tc, agentName, inv.Session, inv.SessionService)
			case constant.SkillSelectDocsToolName:
				recordSkillSelectedDocsToState(ctx, tc, agentName, inv.Session, inv.SessionService)
			}
		}

		logs.Infof("[skill_graph] AfterNodeCallback completed, node=%s, sessionStateKeys=%d, rid: %s",
			callbackCtx.NodeName, len(inv.Session.State), rid)
		return result, nil
	}
}

// extractLastAssistantWithToolCalls extracts the last assistant message with tool calls from the messages.
func extractLastAssistantWithToolCalls(msgs []model.Message) *model.Message {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == model.RoleAssistant && len(msgs[i].ToolCalls) > 0 {
			return &msgs[i]
		}
	}
	return nil
}

// persistStateToService calls svc.UpdateSessionState to write stateMap into the session backend (e.g. MySQL).
// It also updates the in-memory sess.State for consistency within the current process.
func persistStateToService(ctx context.Context, sess *session.Session, svc session.Service,
	stateMap session.StateMap) {

	rid := rest.RidFromContext(ctx)
	// Log the keys being persisted for debugging.
	keys := make([]string, 0, len(stateMap))
	for k, v := range stateMap {
		keys = append(keys, k)
		// Update in-memory first for consistency within the current invocation.
		sess.SetState(k, v)
	}

	if svc == nil {
		logs.Warnf("[skill_graph] SessionService is nil, skip persisting state to backend, rid: %s", rid)
		return
	}
	// 框架对 state 的更新存在并发且未进行加锁，如果 after callback 执行的过快，add event 对 state 的更新可能会产生覆盖，因此这里等待
	time.Sleep(constant.SessionStateUpdateConcurrentWait)

	key := session.Key{
		AppName:   sess.AppName,
		UserID:    sess.UserID,
		SessionID: sess.ID,
	}
	if err := svc.UpdateSessionState(ctx, key, stateMap); err != nil {
		logs.Errorf("[skill_graph] UpdateSessionState failed: key=%+v err=%v, rid: %s", key, err, rid)
	}
}

// recordSkillLoadedToState persists the skill_load result into the session backend so that
// the loaded skill survives process restarts.
func recordSkillLoadedToState(ctx context.Context, tc model.ToolCall, agentName string,
	sess *session.Session, svc session.Service) {

	rid := rest.RidFromContext(ctx)
	var params struct {
		Skill string `json:"skill"`
	}
	if err := json.Unmarshal(tc.Function.Arguments, &params); err != nil {
		logs.Warnf("[skill_graph] failed to unmarshal skill_load args: %v, rid: %s", err, rid)
		return
	}
	if params.Skill == "" {
		return
	}
	loadedKey := skillpkg.LoadedKey(agentName, params.Skill)
	persistStateToService(ctx, sess, svc, session.StateMap{loadedKey: []byte("1")})
	logs.Infof("[skill_graph] skill loaded: agent=%s skill=%s, rid: %s", agentName, params.Skill, rid)
}

// recordSkillSelectedDocsToState persists the skill_select_docs result into the session backend.
func recordSkillSelectedDocsToState(ctx context.Context, tc model.ToolCall, agentName string,
	sess *session.Session, svc session.Service) {

	rid := rest.RidFromContext(ctx)
	var params struct {
		Skill          string   `json:"skill"`
		IncludeAllDocs bool     `json:"include_all_docs"`
		Docs           []string `json:"docs"`
	}
	if err := json.Unmarshal(tc.Function.Arguments, &params); err != nil {
		logs.Warnf("[skill_graph] failed to unmarshal skill_select_docs args: %v, rid: %s", err, rid)
		return
	}
	if params.Skill == "" {
		return
	}
	docsKey := skillpkg.DocsKey(agentName, params.Skill)
	var val []byte
	if params.IncludeAllDocs {
		val = []byte("*")
	} else if len(params.Docs) > 0 {
		docsBytes, err := json.Marshal(params.Docs)
		if err != nil {
			logs.Warnf("[skill_graph] failed to marshal docs: %v, rid: %s", err, rid)
			return
		}
		val = docsBytes
	}
	if val != nil {
		persistStateToService(ctx, sess, svc, session.StateMap{docsKey: val})
	}
	logs.Infof("[skill_graph] skill docs selected: agent=%s skill=%s all=%v docs=%v, rid: %s",
		agentName, params.Skill, params.IncludeAllDocs, params.Docs, rid)
}
