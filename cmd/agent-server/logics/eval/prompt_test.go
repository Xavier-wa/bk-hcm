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
	"reflect"
	"testing"

	"hcm/pkg/criteria/enumor"
	tableaiagent "hcm/pkg/dal/table/aiagent"
	"hcm/pkg/kit"
)

func TestParseScopeOutputFallback(t *testing.T) {
	candidates := []CandidateRun{
		{Run: tableaiagent.RunTable{RunID: "a"}, Role: enumor.AiagentEvalSnapshotRoleContext},
		{Run: tableaiagent.RunTable{RunID: "b"}, Role: enumor.AiagentEvalSnapshotRoleTarget},
	}
	got, illegal := parseScopeOutput(`not-json`, candidates, "b")
	if got != "b" || !illegal {
		t.Fatalf("illegal json: got %s illegal=%v", got, illegal)
	}
	got, illegal = parseScopeOutput(`{"start_run_id":"missing"}`, candidates, "b")
	if got != "b" || !illegal {
		t.Fatalf("unknown id: got %s illegal=%v", got, illegal)
	}
	got, illegal = parseScopeOutput("```json\n{\"start_run_id\":\"a\"}\n```", candidates, "b")
	if got != "a" || illegal {
		t.Fatalf("valid id: got %s illegal=%v", got, illegal)
	}
	got, illegal = parseScopeOutput("好的，结果如下：\n```json\n{\"start_run_id\":\"a\"}\n```", candidates, "b")
	if got != "a" || illegal {
		t.Fatalf("prefixed prose: got %s illegal=%v", got, illegal)
	}
}

// 判官在 JSON 前加了一句说明时仍要能解析，否则整轮评估作废。
func TestParseRubricOutputToleratesProse(t *testing.T) {
	raw := "以下是本轮评估结果：\n```json\n" +
		`{"dims":{"faithfulness":4},"redlines":[],"reason_code":"ok","summary":"ok","briefs":{"r1":"x"}}` +
		"\n```\n希望有帮助。"
	got, err := parseRubricOutput(kit.New(), raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if got.Dims["faithfulness"] != 4 || got.Summary != "ok" || got.Briefs["r1"] != "x" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

// 非法红线只丢自己，合法项必须留下；非法 reason_code 降级为 ok。
func TestParseRubricOutputKeepsValidRedlines(t *testing.T) {
	raw := `{"dims":{},"redlines":["fabricated_data","hallucination"],"reason_code":"intent_mismatched"}`
	got, err := parseRubricOutput(kit.New(), raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !reflect.DeepEqual(got.Redlines, []string{"fabricated_data"}) {
		t.Fatalf("redlines: got %v, want [fabricated_data]", got.Redlines)
	}
	if got.ReasonCode != string(enumor.AiagentEvalReasonOK) {
		t.Fatalf("reason_code: got %s, want ok", got.ReasonCode)
	}
}

func TestParseRubricOutputRejectsNonJSON(t *testing.T) {
	if _, err := parseRubricOutput(kit.New(), "完全没有 JSON"); err == nil {
		t.Fatal("non-json output must fail")
	}
}

func TestPassedHelper(t *testing.T) {
	if !isPassedLocal(80, nil) {
		t.Fatal("80 with empty redlines should pass")
	}
	if isPassedLocal(80, []string{"fabricated_data"}) {
		t.Fatal("redline should fail")
	}
}

func isPassedLocal(quality int, redlines []string) bool {
	return quality >= 80 && len(redlines) == 0
}
