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

func TestCreateAiagentRunReqValidateOccurredAt(t *testing.T) {
	req := &CreateAiagentRunReq{
		RunID: "r1", SessionCode: "s1", User: "u1",
		Status:     enumor.AiagentRunStatusFinished,
		OccurredAt: "2026-09-03 12:00:00",
		EndedAt:    "2026-09-03 12:01:00",
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("valid occurred_at must pass, err: %v", err)
	}

	req.OccurredAt = "not-a-time"
	if err := req.Validate(); err == nil {
		t.Fatal("invalid occurred_at must fail")
	}

	req.OccurredAt = ""
	req.EndedAt = "2026-09-03 12:01:00"
	if err := req.Validate(); err == nil {
		t.Fatal("ended_at without occurred_at must fail")
	}
}
