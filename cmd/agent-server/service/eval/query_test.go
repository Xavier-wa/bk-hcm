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
	"encoding/json"
	"testing"

	evalogic "hcm/cmd/agent-server/logics/eval"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/table/types"
)

func TestIsPassedShape(t *testing.T) {
	pass := func(quality int, redlines []string, th int) bool {
		return quality >= th && len(redlines) == 0
	}
	if !pass(80, nil, 80) {
		t.Fatal("quality 80 empty redlines should pass")
	}
	if pass(79, nil, 80) {
		t.Fatal("quality 79 should fail")
	}
	if pass(90, []string{"overpromise"}, 80) {
		t.Fatal("redline should fail pass")
	}
}

func TestJsonItems(t *testing.T) {
	got := jsonItems("")
	empty, ok := got.(evalogic.Transcript)
	if !ok || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatalf("empty transcript: %+v", got)
	}

	raw := types.JsonField(`{"items":[{"type":"user","text":"hi"}]}`)
	got = jsonItems(raw)
	tr, ok := got.(evalogic.Transcript)
	if !ok || len(tr.Items) != 1 || tr.Items[0].Type != enumor.AiagentTranscriptItemUser {
		t.Fatalf("decoded transcript: %+v", got)
	}

	got = jsonItems(types.JsonField(`not-json`))
	if _, ok := got.(json.RawMessage); !ok {
		t.Fatalf("invalid json must keep raw, got %T", got)
	}
}
