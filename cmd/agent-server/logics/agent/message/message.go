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

// Package message provides the message handling logic.
package message

import (
	"context"
	"time"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/uuid"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

// EmitFallbackMessage emits the fallback text as a proper model execution event so it appears
// as an assistant text message in the AG-UI stream. This is necessary when the graph routes
// directly from intent_recognition to fallback (skipping the llm node), because no LLM response
// is available to produce the TextMessage event sequence.
func EmitFallbackMessage(ctx context.Context, messages []trpcmodel.Message, state graph.State,
	interruptKey, nodeID, message string) {

	// 判断是否需要发送 fallback 消息：resume 重放、尾部已有 assistant 回复、或响应内容为空时均跳过
	if !shouldEmitFallbackResponse(state, interruptKey, messages, message) {
		return
	}

	rid := rest.RidFromContext(ctx)
	emitter := graph.GetEventEmitterWithContext(ctx, state)
	now := time.Now()
	responseID := uuid.UUID()
	evt := graph.NewModelExecutionEvent(
		graph.WithModelEventNodeID(nodeID),
		graph.WithModelEventResponseID(responseID),
		graph.WithModelEventOutput(message),
		graph.WithModelEventPhase(graph.ModelExecutionPhaseComplete),
		graph.WithModelEventStartTime(now),
		graph.WithModelEventEndTime(now),
	)
	if err := emitter.Emit(evt); err != nil {
		logs.Warnf("%s node: emit fallback message failed, err: %v, rid: %s", nodeID, err, rid)
	}
}

// hasAssistantTailWithContent 判断消息尾部是否已存在指定内容的 assistant 回复。
func hasAssistantTailWithContent(messages []trpcmodel.Message, content string) bool {
	if content == "" || len(messages) == 0 {
		return false
	}
	last := messages[len(messages)-1]
	return last.Role == trpcmodel.RoleAssistant && last.Content == content
}

// shouldEmitFallbackResponse 判断 fallback 节点是否需要 emit model execution event。
// 在以下情况跳过 emit 防止重复或无效输出：
// 1. interrupt resume 重放；
// 2. 消息尾部已存在 assistant 回复；
// 3. 响应内容为空。
func shouldEmitFallbackResponse(state graph.State, interruptKey string, messages []trpcmodel.Message,
	lastResp string) bool {

	// 查看是否是resume重放，不能单靠下面最后一条assistant回复来判断，因为触发中断的时候，delta并不会被更新到message中
	// 这里通过检查： 1. ResumeChannel 是否有值； 2. ResumeMap 是否有值判断是否为中断恢复
	if graph.HasResumeValue(state, interruptKey) {
		return false
	}

	if len(messages) > 0 && messages[len(messages)-1].Role == trpcmodel.RoleAssistant {
		return false
	}
	return lastResp != ""
}

// BuildFallbackResumeDelta builds state delta after fallback resumes with next user input.
// For unsupported intent turns, it rebuilds history to assistant+user to avoid stale anchoring.
//
// StateKeyUserInput is explicitly cleared in every resume delta. The framework's
// mergeInitialStateNonInternal only merges keys absent from the restored checkpoint,
// so a stale user_input written during a run that never reached the LLM node
// (e.g. "查看预测" → fallback interrupt) persists across checkpoint/resume cycles.
// Without the explicit clear, the LLM node's executeUserInputStage would use the
// stale value and overwrite the correctly-rebuilt messages tail.
func BuildFallbackResumeDelta(ctx context.Context, state graph.State, messages []trpcmodel.Message, lastResp,
	userInput string) graph.State {

	rid := rest.RidFromContext(ctx)
	delta := graph.State{
		graph.StateKeyLastResponse: "",
		// 清空 StateKeyUserInput：LLM 节点正常执行后会在自己的 delta 里将其置空，
		// 但不支持意图时流程绕过了 LLMNode
		// 旧值会残留在 checkpoint 里。若不清空，下一轮 resume 进入 LLM 节点时，
		// executeUserInputStage 会用旧的 user_input 覆盖正确重建的 messages 末尾消息。
		// resume的时候是把用户的输入追加到user message里面，所以这里可以清空
		graph.StateKeyUserInput: "",
	}

	intentStr, _ := state[constant.StateKeyIntent].(string)
	intentType := enumor.IntentType(intentStr)
	clearingUnsupported := intentStr != "" && !intentType.IsSupportedScene()

	if clearingUnsupported {
		logs.Infof("[fallback] clear unsupported intent=%s for re-recognition, rid: %s", intentStr, rid)
		delta[constant.StateKeyIntent] = ""
		delta[graph.StateKeyMessages] = []graph.MessageOp{
			graph.AppendMessages{
				Items: []trpcmodel.Message{
					{Role: trpcmodel.RoleAssistant, Content: lastResp},
					{Role: trpcmodel.RoleUser, Content: userInput},
				},
			},
		}
		return delta
	}

	msgDelta := []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: userInput}}
	if !hasAssistantTailWithContent(messages, lastResp) && lastResp != "" {
		msgDelta = []trpcmodel.Message{
			{Role: trpcmodel.RoleAssistant, Content: lastResp},
			{Role: trpcmodel.RoleUser, Content: userInput},
		}
	}

	delta[graph.StateKeyMessages] = msgDelta
	return delta
}
