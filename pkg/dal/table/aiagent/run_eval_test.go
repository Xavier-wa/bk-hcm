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
	"hcm/pkg/dal/table/types"
)

func validEval() RunEvalTable {
	return RunEvalTable{
		ID:              "1",
		RunID:           "r1",
		SessionID:       "sess",
		User:            "u1",
		BkBizID:         100,
		Scene:           enumor.IntentTypeChat,
		Query:           "hello",
		StartRunID:      "r1",
		ProcessScore:    80,
		OutcomeScore:    80,
		QualityScore:    80,
		Redlines:        types.StringArray{},
		ReasonCode:      enumor.AiagentEvalReasonOK,
		RubricVersion:   "v1",
		EvalResult:      types.JsonField(`{}`),
		ContextSnapshot: types.JsonField(`{"runs":[]}`),
		Creator:         "u1",
		Reviser:         "u1",
	}
}

func TestRunEvalInsertValidate(t *testing.T) {
	r := validEval()
	if err := r.InsertValidate(); err != nil {
		t.Fatalf("valid eval failed, err: %v", err)
	}
	r.QualityScore = 101
	if err := r.InsertValidate(); err == nil {
		t.Fatal("quality 101 must fail")
	}
	missingUser := validEval()
	missingUser.User = ""
	if err := missingUser.InsertValidate(); err == nil {
		t.Fatal("empty user must fail")
	}

	missingScene := validEval()
	missingScene.Scene = ""
	if err := missingScene.InsertValidate(); err == nil {
		t.Fatal("empty scene must fail")
	}
}

// TestRunEvalColumnsHaveSceneQueryOmitSessionCodeAndTranscript asserts the aiagent_run_eval
// column set: scene/query are denormalized from aiagent_run (dashboard filter/sort pushdown),
// while session_code/passed/transcript are intentionally still not stored (session_id already
// covers the thread key, passed is derived from quality_score+redlines, transcript is looked up
// from aiagent_run on demand).
func TestRunEvalColumnsHaveSceneQueryOmitSessionCodeAndTranscript(t *testing.T) {
	hasUser, hasBkBizID, hasScene, hasQuery := false, false, false, false
	for _, col := range RunEvalColumnDescriptor {
		switch col.Column {
		case "user":
			hasUser = true
		case "bk_biz_id":
			hasBkBizID = true
		case "scene":
			hasScene = true
		case "query":
			hasQuery = true
		case "session_code", "passed", "transcript":
			t.Fatalf("aiagent_run_eval must not store %s", col.Column)
		}
	}
	if !hasUser || !hasBkBizID {
		t.Fatal("aiagent_run_eval must store user and bk_biz_id")
	}
	if !hasScene || !hasQuery {
		t.Fatal("aiagent_run_eval must store scene and query")
	}
}
