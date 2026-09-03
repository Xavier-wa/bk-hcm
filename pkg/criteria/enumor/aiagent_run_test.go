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

import "testing"

func TestAiagentRunStatus_Validate(t *testing.T) {
	tests := []struct {
		status  AiagentRunStatus
		wantErr bool
	}{
		{AiagentRunStatusRunning, false},
		{AiagentRunStatusFinished, false},
		{AiagentRunStatusError, false},
		{AiagentRunStatusCancel, false},
		{AiagentRunStatusUnknown, false},
		{AiagentRunStatus("done"), true},
		{AiagentRunStatus(""), true},
	}
	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			err := tc.status.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestAiagentRunStatus_IsTerminal(t *testing.T) {
	if AiagentRunStatusRunning.IsTerminal() {
		t.Fatal("running must not be terminal")
	}
	if !AiagentRunStatusFinished.IsTerminal() {
		t.Fatal("finished must be terminal")
	}
	if !AiagentRunStatusCancel.IsTerminal() {
		t.Fatal("cancel must be terminal")
	}
}

func TestTerminalAiagentRunStatuses(t *testing.T) {
	got := TerminalAiagentRunStatuses()
	if len(got) == 0 {
		t.Fatal("terminal statuses must not be empty")
	}
	seen := make(map[AiagentRunStatus]bool, len(got))
	for _, status := range got {
		if !status.IsTerminal() {
			t.Fatalf("%s must be terminal", status)
		}
		if seen[status] {
			t.Fatalf("duplicate terminal status: %s", status)
		}
		seen[status] = true
	}
	all := []AiagentRunStatus{
		AiagentRunStatusRunning, AiagentRunStatusFinished, AiagentRunStatusError,
		AiagentRunStatusCancel, AiagentRunStatusUnknown,
	}
	for _, status := range all {
		if status.IsTerminal() != seen[status] {
			t.Fatalf("%s IsTerminal=%t not listed in TerminalAiagentRunStatuses",
				status, status.IsTerminal())
		}
	}
}

func TestIntentType_ValidateRunScene(t *testing.T) {
	if err := IntentTypeHostApply.ValidateRunScene(); err != nil {
		t.Fatalf("host_apply should pass, err: %v", err)
	}
	if err := IntentTypeUnsupported.ValidateRunScene(); err != nil {
		t.Fatalf("unsupported should pass as run scene, err: %v", err)
	}
	if err := IntentType("other").ValidateRunScene(); err == nil {
		t.Fatal("other should fail")
	}
}

func TestNormalizeRunScene(t *testing.T) {
	if got := NormalizeRunScene(IntentTypeHostApply); got != IntentTypeHostApply {
		t.Fatalf("got %s, want host_apply", got)
	}
	if got := NormalizeRunScene(IntentType("bogus")); got != IntentTypeUnsupported {
		t.Fatalf("got %s, want unsupported", got)
	}
}

func TestAiagentEvalReasonCode_Validate(t *testing.T) {
	if err := AiagentEvalReasonToolError.Validate(); err != nil {
		t.Fatalf("tool_error should pass, err: %v", err)
	}
	if err := AiagentEvalReasonCode("slow").Validate(); err == nil {
		t.Fatal("slow should fail")
	}
	if err := AiagentEvalReasonCode("").Validate(); err == nil {
		t.Fatal("empty should fail")
	}
}

func TestAiagentEvalRedline_Validate(t *testing.T) {
	if err := AiagentEvalRedlineFabricatedData.Validate(); err != nil {
		t.Fatalf("fabricated_data should pass, err: %v", err)
	}
	if err := AiagentEvalRedline("unconfirmed_write").Validate(); err == nil {
		t.Fatal("unconfirmed_write should fail")
	}
	if err := ValidateAiagentEvalRedlines([]string{string(AiagentEvalRedlineOverpromise)}); err != nil {
		t.Fatalf("valid slice should pass, err: %v", err)
	}
	if err := ValidateAiagentEvalRedlines([]string{"unconfirmed_write"}); err == nil {
		t.Fatal("invalid slice should fail")
	}
}
