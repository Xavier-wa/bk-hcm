/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may obtain a copy of the License at http://opensource.org/licenses/MIT
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

package eval

import (
	"testing"

	dsaiagent "hcm/pkg/api/data-service/aiagent"
)

func TestRequireOverwrite(t *testing.T) {
	existingEval := &dsaiagent.GetAiagentRunEvalResult{ID: "e1"}
	if err := requireOverwrite(existingEval, false); err == nil {
		t.Fatal("existing eval without overwrite must fail")
	}
	if err := requireOverwrite(existingEval, true); err != nil {
		t.Fatalf("overwrite=true must pass, err: %v", err)
	}
	if err := requireOverwrite(nil, false); err != nil {
		t.Fatalf("missing eval must pass, err: %v", err)
	}
}
