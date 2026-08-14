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

package message

import (
	"context"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	trpcagent "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
)

// SceneSwitchedPayload is the payload of the scene.switched CUSTOM event.
type SceneSwitchedPayload struct {
	// From 切换前的会话场景标签
	From enumor.IntentType `json:"from"`
	// To 切换后的会话场景标签
	To enumor.IntentType `json:"to"`
}

// EmitSceneSwitched emits the scene.switched CUSTOM event so that the frontend can sync the
// session scene tag UI when a turn-boundary scene switch happens.
//
// 事件走框架原生的 node custom event 通道，由 AG-UI translator 无条件转成同名 CUSTOM 事件，
// 因此不依赖中断（interrupt）产生，也无需改动 agui-event 的自定义 translator。
//
// 调用方只在真正发生切换时调用（原标签为受支持场景且与新标签不同）——无标签会话首次提交标签
// 不算切换，前端不需要提示。emit 失败仅记录 Warn，不阻断本轮对话。
func EmitSceneSwitched(ctx context.Context, state graph.State, nodeID string, from, to enumor.IntentType) {
	rid := rest.RidFromContext(ctx)

	var invocationID string
	if inv, ok := trpcagent.InvocationFromContext(ctx); ok && inv != nil {
		invocationID = inv.InvocationID
	}

	evt := graph.NewNodeCustomEvent(
		graph.WithNodeCustomEventInvocationID(invocationID),
		graph.WithNodeCustomEventNodeID(nodeID),
		graph.WithNodeCustomEventEventType(constant.SceneSwitchedCustomEventName),
		graph.WithNodeCustomEventPayload(SceneSwitchedPayload{From: from, To: to}),
	)

	if err := graph.GetEventEmitterWithContext(ctx, state).Emit(evt); err != nil {
		logs.Warnf("%s node: emit %s event failed, from: %s, to: %s, err: %v, rid: %s",
			nodeID, constant.SceneSwitchedCustomEventName, from, to, err, rid)
		return
	}

	logs.Infof("%s node: emit %s event, from: %s, to: %s, rid: %s",
		nodeID, constant.SceneSwitchedCustomEventName, from, to, rid)
}
