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

package enumor

import "fmt"

// AiagentRunStatus is the status of one aiagent_run row.
type AiagentRunStatus string

const (
	// AiagentRunStatusRunning is the in-progress status.
	AiagentRunStatusRunning AiagentRunStatus = "running"
	// AiagentRunStatusFinished is the successful terminal status.
	AiagentRunStatusFinished AiagentRunStatus = "finished"
	// AiagentRunStatusError is the failed terminal status.
	AiagentRunStatusError AiagentRunStatus = "error"
	// AiagentRunStatusCancel is the user-cancelled terminal status.
	AiagentRunStatusCancel AiagentRunStatus = "cancel"
	// AiagentRunStatusUnknown is the orphan-sweep terminal status.
	AiagentRunStatusUnknown AiagentRunStatus = "unknown"
)

// Validate checks whether the run status is one of the declared values.
func (s AiagentRunStatus) Validate() error {
	switch s {
	case AiagentRunStatusRunning, AiagentRunStatusFinished, AiagentRunStatusError,
		AiagentRunStatusCancel, AiagentRunStatusUnknown:
		return nil
	default:
		return fmt.Errorf("unsupported aiagent run status: %s", s)
	}
}

// TerminalAiagentRunStatuses returns all terminal run statuses.
func TerminalAiagentRunStatuses() []AiagentRunStatus {
	return []AiagentRunStatus{
		AiagentRunStatusFinished,
		AiagentRunStatusError,
		AiagentRunStatusCancel,
		AiagentRunStatusUnknown,
	}
}

// IsTerminal reports whether the status is a terminal state.
func (s AiagentRunStatus) IsTerminal() bool {
	for _, status := range TerminalAiagentRunStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// AiagentTranscriptItemType is the type of one transcript item.
type AiagentTranscriptItemType string

const (
	// AiagentTranscriptItemUser is a user text bubble.
	AiagentTranscriptItemUser AiagentTranscriptItemType = "user"
	// AiagentTranscriptItemAssistant is an assistant text bubble.
	AiagentTranscriptItemAssistant AiagentTranscriptItemType = "assistant"
	// AiagentTranscriptItemTool is a tool call with result.
	AiagentTranscriptItemTool AiagentTranscriptItemType = "tool"
	// AiagentTranscriptItemCustom is a HITL custom event.
	AiagentTranscriptItemCustom AiagentTranscriptItemType = "custom"
)

// Validate checks whether the transcript item type is one of the declared values.
func (t AiagentTranscriptItemType) Validate() error {
	switch t {
	case AiagentTranscriptItemUser, AiagentTranscriptItemAssistant, AiagentTranscriptItemTool,
		AiagentTranscriptItemCustom:
		return nil
	default:
		return fmt.Errorf("unsupported transcript item type: %s", t)
	}
}
