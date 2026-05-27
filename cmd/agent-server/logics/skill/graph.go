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

	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"
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

		if inv.Session.State == nil {
			inv.Session.State = make(map[string][]byte)
		}

		for _, tc := range assistantMsg.ToolCalls {
			logs.Infof("[skill_graph] processing tool call: name=%s, rid: %s", tc.Function.Name, rid)
			switch tc.Function.Name {
			case constant.SkillLoadToolName:
				recordSkillLoadedToState(tc, agentName, inv.Session.State, rid)
			case constant.SkillSelectDocsToolName:
				recordSkillSelectedDocsToState(tc, agentName, inv.Session.State, rid)
			}
		}

		logs.Infof("[skill_graph] AfterNodeCallback completed, node=%s, rid: %s", callbackCtx.NodeName, rid)
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

// recordSkillLoadedToState skill_load 调用后将 skill 的加载状态写入 session.State
func recordSkillLoadedToState(tc model.ToolCall, agentName string, state map[string][]byte, rid string) {
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
	state[loadedKey] = []byte("1")
	logs.Infof("[skill_graph] skill loaded: agent=%s skill=%s, rid: %s", agentName, params.Skill, rid)
}

// recordSkillSelectedDocsToState skill_select_docs 调用后将 skill 的选中的 docs 写入 session.State
func recordSkillSelectedDocsToState(tc model.ToolCall, agentName string, state map[string][]byte, rid string) {
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
	if params.IncludeAllDocs {
		state[docsKey] = []byte("*")
	} else if len(params.Docs) > 0 {
		docsBytes, err := json.Marshal(params.Docs)
		if err != nil {
			logs.Warnf("[skill_graph] failed to marshal docs: %v, rid: %s", err, rid)
			return
		}
		state[docsKey] = docsBytes
	}
	logs.Infof("[skill_graph] skill docs selected: agent=%s skill=%s all=%v docs=%v, rid: %s",
		agentName, params.Skill, params.IncludeAllDocs, params.Docs, rid)
}
