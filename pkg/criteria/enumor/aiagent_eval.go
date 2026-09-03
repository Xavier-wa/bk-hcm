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

// AiagentEvalReasonCode is the primary issue category of one evaluation.
type AiagentEvalReasonCode string

const (
	// AiagentEvalReasonIntentMismatch means the reply missed the user intent.
	AiagentEvalReasonIntentMismatch AiagentEvalReasonCode = "intent_mismatch"
	// AiagentEvalReasonToolError means a tool call failed.
	AiagentEvalReasonToolError AiagentEvalReasonCode = "tool_error"
	// AiagentEvalReasonIncomplete means the outcome was incomplete.
	AiagentEvalReasonIncomplete AiagentEvalReasonCode = "incomplete"
	// AiagentEvalReasonOutOfScope means the assistant overstepped.
	AiagentEvalReasonOutOfScope AiagentEvalReasonCode = "out_of_scope"
	// AiagentEvalReasonFabricatedData means numbers or facts were invented.
	AiagentEvalReasonFabricatedData AiagentEvalReasonCode = "fabricated_data"
	// AiagentEvalReasonOK means no primary issue.
	AiagentEvalReasonOK AiagentEvalReasonCode = "ok"
)

// Validate checks whether the reason code is one of the declared values.
func (c AiagentEvalReasonCode) Validate() error {
	switch c {
	case AiagentEvalReasonIntentMismatch, AiagentEvalReasonToolError, AiagentEvalReasonIncomplete,
		AiagentEvalReasonOutOfScope, AiagentEvalReasonFabricatedData, AiagentEvalReasonOK:
		return nil
	default:
		return fmt.Errorf("unsupported aiagent eval reason_code: %s", c)
	}
}

// AiagentEvalRedline is a semantic redline hit by the judge.
type AiagentEvalRedline string

const (
	// AiagentEvalRedlineTextDumpedPlan 把执行计划当正文倾倒给用户（应用内已有结构化计划/卡片却仍大段罗列步骤）。
	AiagentEvalRedlineTextDumpedPlan AiagentEvalRedline = "text_dumped_plan"
	// AiagentEvalRedlineFabricatedData 回复与 tool_result 不一致，或编造数字/事实。
	AiagentEvalRedlineFabricatedData AiagentEvalRedline = "fabricated_data"
	// AiagentEvalRedlineOverpromise 承诺了当前能力/权限做不到的事。
	AiagentEvalRedlineOverpromise AiagentEvalRedline = "overpromise"
	// AiagentEvalRedlineWrongScene 错误切换或混用场景（如查询聊着聊着按申领流程走）。
	AiagentEvalRedlineWrongScene AiagentEvalRedline = "wrong_scene"
)

// Validate checks whether the redline is one of the declared values.
func (r AiagentEvalRedline) Validate() error {
	switch r {
	case AiagentEvalRedlineTextDumpedPlan, AiagentEvalRedlineFabricatedData,
		AiagentEvalRedlineOverpromise, AiagentEvalRedlineWrongScene:
		return nil
	default:
		return fmt.Errorf("unsupported aiagent eval redline: %s", r)
	}
}

// ValidateAiagentEvalRedlines validates a redlines slice.
func ValidateAiagentEvalRedlines(redlines []string) error {
	for _, item := range redlines {
		if err := AiagentEvalRedline(item).Validate(); err != nil {
			return err
		}
	}
	return nil
}

// AiagentEvalSnapshotRole is the role of a window member in context_snapshot.
type AiagentEvalSnapshotRole string

const (
	// AiagentEvalSnapshotRoleContext is an earlier run in the window.
	AiagentEvalSnapshotRoleContext AiagentEvalSnapshotRole = "context"
	// AiagentEvalSnapshotRoleTarget is the run being evaluated.
	AiagentEvalSnapshotRoleTarget AiagentEvalSnapshotRole = "target"
)

// Validate checks whether the snapshot role is one of the declared values.
func (r AiagentEvalSnapshotRole) Validate() error {
	switch r {
	case AiagentEvalSnapshotRoleContext, AiagentEvalSnapshotRoleTarget:
		return nil
	default:
		return fmt.Errorf("unsupported aiagent eval snapshot role: %s", r)
	}
}
