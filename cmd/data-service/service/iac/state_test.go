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

package iac

import (
	"encoding/json"
	"testing"

	"hcm/pkg/criteria/enumor"
	tableiac "hcm/pkg/dal/table/iac"
	tabletypes "hcm/pkg/dal/table/types"
)

func TestConvertTableToIacState_NullAndNonNullCurrent(t *testing.T) {
	specID := "spec-1"
	nullRow := &tableiac.IacStateTable{
		ID:               "st-1",
		RepositoryID:     "repo-1",
		BlockID:          "tencentcloud_vpc.main",
		Kind:             enumor.IacStateKindResource,
		Current:          "",
		Deposed:          tabletypes.JsonField(`{"abc12345":{"type":"tencentcloud_vpc"}}`),
		ImportFromSpecID: &specID,
	}

	gotNull := convertTableToIacState(nullRow)
	if gotNull.Current != nil {
		t.Fatalf("expected Current == nil for empty DB current, got %v", *gotNull.Current)
	}
	raw, err := json.Marshal(gotNull)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if m["current"] != nil {
		t.Fatalf("expected JSON current == null, got %#v", m["current"])
	}
	if gotNull.ImportFromSpecID != "spec-1" {
		t.Fatalf("expected import_from_spec_id == spec-1, got %q", gotNull.ImportFromSpecID)
	}

	nonNullRow := &tableiac.IacStateTable{
		ID:           "st-2",
		RepositoryID: "repo-1",
		BlockID:      "tencentcloud_vpc.main",
		Kind:         enumor.IacStateKindResource,
		Current:      tabletypes.JsonField(`{"plugin":"tcloud","type":"tencentcloud_vpc"}`),
		Deposed:      tabletypes.JsonField(`{}`),
	}
	gotNonNull := convertTableToIacState(nonNullRow)
	if gotNonNull.Current == nil {
		t.Fatal("expected non-nil Current")
	}
	if string(*gotNonNull.Current) != `{"plugin":"tcloud","type":"tencentcloud_vpc"}` {
		t.Fatalf("unexpected Current: %s", *gotNonNull.Current)
	}
}
