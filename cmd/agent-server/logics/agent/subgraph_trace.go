/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package agent

import (
	"fmt"

	"hcm/cmd/agent-server/logics/logger"
	"hcm/pkg/logs"

	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

// logSubgraphInput 记录进入子图时的摘要（turn、checkpoint_ns、消息条数），不 dump 全文。
func logSubgraphInput(nodePrefix, rid string, turn int, parentMsgs, childMsgs []trpcmodel.Message,
	checkpointNS string) {
	logs.Infof("%s tag=subgraph_input node=%s checkpoint_ns=%s turn=%d parent_len=%d child_len=%d rid: %s",
		logger.GraphTraceLogPrefix, nodePrefix, checkpointNS, turn, len(parentMsgs), len(childMsgs), rid)
}

// logSubgraphOutputSkip 记录子图输出无需合并回主图（decoded 不长于 parent）的摘要。
func logSubgraphOutputSkip(nodePrefix, rid string, parentMsgs, decoded []trpcmodel.Message) {
	logs.Infof("%s tag=subgraph_output node=%s stage=skip_merge decoded_len=%d parent_len=%d rid: %s",
		logger.GraphTraceLogPrefix, nodePrefix, len(decoded), len(parentMsgs), rid)
}

// logSubgraphOutputMerge 记录子图 delta 成功合并回主图的摘要。
func logSubgraphOutputMerge(nodePrefix, rid string, parentTurn int, parentMsgs, decoded, delta []trpcmodel.Message) {
	logs.Infof("%s tag=subgraph_output node=%s stage=merge_done parent_turn=%d decoded_len=%d "+
		"parent_len=%d delta_len=%d rid: %s",
		logger.GraphTraceLogPrefix, nodePrefix, parentTurn, len(decoded), len(parentMsgs), len(delta), rid)
}

// logSubgraphOutputGiveUp 记录因缺少/空 messages 而放弃合并的告警。
func logSubgraphOutputGiveUp(nodePrefix, rid, reason string) {
	logs.Warnf("%s tag=subgraph_output node=%s stage=give_up reason=%s rid: %s",
		logger.GraphTraceLogPrefix, nodePrefix, reason, rid)
}

// logSubgraphOutputDecodeFailed 记录子图 RawStateDelta messages JSON 解码失败。
func logSubgraphOutputDecodeFailed(nodePrefix, rid string, err error, rawLen int) {
	logs.Errorf("%s tag=subgraph_output node=%s stage=decode_failed err: %v raw_len=%d rid: %s",
		logger.GraphTraceLogPrefix, nodePrefix, err, rawLen, rid)
}

// formatSubgraphGiveUpReason 生成放弃合并时的原因字符串。
func formatSubgraphGiveUpReason(missingKey bool, emptyDecoded bool) string {
	switch {
	case missingKey:
		return fmt.Sprintf("raw_state_delta_missing_%s", graph.StateKeyMessages)
	case emptyDecoded:
		return "decoded_messages_empty"
	default:
		return "unknown"
	}
}
