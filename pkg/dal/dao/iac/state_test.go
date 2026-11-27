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

package daoiac

import (
	"strings"
	"testing"

	tableiac "hcm/pkg/dal/table/iac"
	"hcm/pkg/dal/table/types"
)

func TestBuildIacStateUpdateSQL_NullCurrentAndBlankImportSpec(t *testing.T) {
	empty := ""
	model := &tableiac.IacStateTable{
		Current:          types.JsonField("null"),
		ImportFromSpecID: &empty,
		Reviser:          "admin",
	}

	setExpr, toUpdate, err := buildIacStateUpdateSQL(model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(setExpr, "current = :current") {
		t.Fatalf("expected setExpr to contain current assignment, got: %s", setExpr)
	}
	val, exists := toUpdate["current"]
	if !exists || val != nil {
		t.Fatalf("expected toUpdate[\"current\"] == nil, got exists=%v val=%#v", exists, val)
	}
	if !strings.Contains(setExpr, "import_from_spec_id = :import_from_spec_id") {
		t.Fatalf("expected setExpr to contain import_from_spec_id assignment, got: %s", setExpr)
	}
	gotPtr, ok := toUpdate["import_from_spec_id"].(*string)
	if !ok || gotPtr == nil || *gotPtr != "" {
		t.Fatalf("expected *string pointing to empty string for import_from_spec_id, got %#v",
			toUpdate["import_from_spec_id"])
	}

	skipModel := &tableiac.IacStateTable{
		LastFlowID: "flow-1",
		Reviser:    "admin",
	}
	skipExpr, skipUpdate, err := buildIacStateUpdateSQL(skipModel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(skipExpr, "current") {
		t.Fatalf("expected empty Current to skip current column, got: %s", skipExpr)
	}
	if _, exists := skipUpdate["current"]; exists {
		t.Fatalf("expected current key absent when Current is empty")
	}
}
