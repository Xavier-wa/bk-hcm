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

package eval

import (
	"reflect"
	"testing"

	"hcm/pkg/cc"
)

func TestComputeFixedSample884667(t *testing.T) {
	got := Compute(DimScores{
		"no_text_plan_listing":  5,
		"faithfulness":          5,
		"guidance_at_interrupt": 4,
		"error_handling":        4,
		"param_inference":       3,
		"conclusion_first":      5,
		"outcome_complete":      1,
		"concise_closure":       0,
		"intent_match":          1,
	}, cc.DefaultEvalProcessWeights, cc.DefaultEvalOutcomeWeights)
	if got.Process != 88 || got.Outcome != 46 || got.Quality != 67 {
		t.Fatalf("got process/outcome/quality=%d/%d/%d, want 88/46/67",
			got.Process, got.Outcome, got.Quality)
	}
}

func TestComputeIgnoresOutOfRange(t *testing.T) {
	got := Compute(DimScores{
		"no_text_plan_listing": 9,
		"faithfulness":         -3,
	}, cc.DefaultEvalProcessWeights, cc.DefaultEvalOutcomeWeights)
	if got.Process < 0 || got.Process > 100 {
		t.Fatalf("process out of range: %d", got.Process)
	}
}

func TestComputeCustomWeights(t *testing.T) {
	got := Compute(DimScores{"only": 5}, map[string]float64{"only": 1}, map[string]float64{"only": 1})
	if got.Process != 100 || got.Outcome != 100 || got.Quality != 100 {
		t.Fatalf("got %+v, want 100/100/100", got)
	}
}

// 无工具失败、无中断的一轮，判官省略两维后仍应满分，而不是被当 0 分压到 67。
func TestComputeSkipsMissingDims(t *testing.T) {
	dims := DimScores{
		"no_text_plan_listing": 5,
		"faithfulness":         5,
		"param_inference":      5,
		"conclusion_first":     5,
		"outcome_complete":     5,
		"concise_closure":      5,
		"intent_match":         5,
	}
	got := Compute(dims, cc.DefaultEvalProcessWeights, cc.DefaultEvalOutcomeWeights)
	if got.Process != 100 || got.Outcome != 100 || got.Quality != 100 {
		t.Fatalf("got %+v, want 100/100/100", got)
	}
}

func TestComputeAllDimsMissing(t *testing.T) {
	got := Compute(DimScores{}, cc.DefaultEvalProcessWeights, cc.DefaultEvalOutcomeWeights)
	if got.Process != 0 || got.Outcome != 0 || got.Quality != 0 {
		t.Fatalf("got %+v, want 0/0/0", got)
	}
}

func TestMissingDims(t *testing.T) {
	dims := DimScores{"faithfulness": 5, "intent_match": 4}
	got := MissingDims(dims, cc.DefaultEvalProcessWeights, cc.DefaultEvalOutcomeWeights)
	want := []string{
		"concise_closure", "conclusion_first", "error_handling", "guidance_at_interrupt",
		"no_text_plan_listing", "outcome_complete", "param_inference",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if len(MissingDims(fullDims(), cc.DefaultEvalProcessWeights, cc.DefaultEvalOutcomeWeights)) != 0 {
		t.Fatal("full dims should report nothing missing")
	}
}

func fullDims() DimScores {
	dims := make(DimScores)
	for name := range cc.DefaultEvalProcessWeights {
		dims[name] = 5
	}
	for name := range cc.DefaultEvalOutcomeWeights {
		dims[name] = 5
	}
	return dims
}
