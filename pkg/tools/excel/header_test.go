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

package excel

import (
	"testing"
)

type headerTestRow struct {
	Name  string `excel:"名称"`
	Count int    `excel:"数量"`
	Skip  string
}

func TestTicketExportRowHeaders(t *testing.T) {
	headers, err := TicketExportRowHeaders(headerTestRow{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(headers) != 2 {
		t.Fatalf("expected 2 headers, got %d", len(headers))
	}
	if headers[0] != "名称" || headers[1] != "数量" {
		t.Fatalf("unexpected headers: %v", headers)
	}

	ptr := &headerTestRow{}
	headers, err = TicketExportRowHeaders(ptr)
	if err != nil {
		t.Fatalf("unexpected error for pointer: %v", err)
	}
	if len(headers) != 2 {
		t.Fatalf("expected 2 headers from pointer, got %d", len(headers))
	}
}

func TestTicketExportRowHeaders_InvalidInput(t *testing.T) {
	cases := []struct {
		name   string
		object any
	}{
		{name: "nil", object: nil},
		{name: "nil pointer", object: (*headerTestRow)(nil)},
		{name: "non-struct", object: "not-a-struct"},
		{name: "int", object: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			headers, err := TicketExportRowHeaders(tc.object)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if headers != nil {
				t.Fatalf("expected nil headers, got %v", headers)
			}
		})
	}
}
