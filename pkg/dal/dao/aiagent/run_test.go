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
	"strings"
	"testing"

	"hcm/pkg/criteria/errf"
)

func TestErrIfCASZeroRows(t *testing.T) {
	err := errIfCASZeroRows(0)
	if errf.Error(err).Code != errf.RecordNotUpdate {
		t.Fatalf("zero rows must map to RecordNotUpdate, got %v", err)
	}
	if err := errIfCASZeroRows(1); err != nil {
		t.Fatalf("one row must succeed, err: %v", err)
	}
}

func TestCASUpdateSQLRequiresRunning(t *testing.T) {
	sql := casUpdateSQL()
	if !strings.Contains(sql, "status=:from_status") {
		t.Fatalf("CAS must filter from_status, sql: %s", sql)
	}
	if strings.Contains(sql, "JSON_EXTRACT") {
		t.Fatal("CAS must not use JSON_EXTRACT")
	}
	if !strings.Contains(sql, "IF(:query='', query, :query)") {
		t.Fatalf("empty query must keep existing column, sql: %s", sql)
	}
}

func TestPatchTranscriptSQLKeepsUpdatedAt(t *testing.T) {
	sql := patchTranscriptSQL()
	if !strings.Contains(sql, "updated_at=updated_at") {
		t.Fatalf("patch must keep updated_at, sql: %s", sql)
	}
}

func TestPatchOccurredAtSQLSetsBothTimestamps(t *testing.T) {
	sql := patchOccurredAtSQL()
	if !strings.Contains(sql, "created_at=:created_at") {
		t.Fatalf("patch occurred_at must set created_at, sql: %s", sql)
	}
	if !strings.Contains(sql, "updated_at=:updated_at") {
		t.Fatalf("patch occurred_at must set updated_at, sql: %s", sql)
	}
}
