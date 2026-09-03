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

package aiagent

import (
	"testing"

	"hcm/pkg/criteria/enumor"
)

func validRun() RunTable {
	return RunTable{
		ID:          "1",
		RunID:       "r1",
		SessionCode: "s1",
		User:        "u1",
		Status:      enumor.AiagentRunStatusRunning,
		Scene:       enumor.IntentTypeUnsupported,
		Creator:     "u1",
		Reviser:     "u1",
	}
}

func TestRunInsertValidate(t *testing.T) {
	r := validRun()
	if err := r.InsertValidate(); err != nil {
		t.Fatalf("valid run failed, err: %v", err)
	}
	r.RunID = ""
	if err := r.InsertValidate(); err == nil {
		t.Fatal("empty run_id must fail")
	}
}

func TestRunColumnsHaveNoBriefOrSessionID(t *testing.T) {
	for _, col := range RunColumnDescriptor {
		switch col.Column {
		case "brief", "session_id":
			t.Fatalf("aiagent_run must not store %s", col.Column)
		}
	}
}
