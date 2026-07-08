/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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

package task

import (
	"testing"
	"time"

	"hcm/pkg/api/core"
	coretask "hcm/pkg/api/core/task"
	"hcm/pkg/criteria/enumor"
)

func TestValidManagementMetricDims(t *testing.T) {
	data := coretask.Management{
		BkBizID:  213,
		Revision: core.Revision{CreatedAt: "2026-05-21T07:20:19Z"},
	}
	if !validManagementMetricDims(data, string(enumor.TCloudZiyan), string(enumor.TaskTargetGroupModifyWeight)) {
		t.Fatal("expected valid management metric dimensions")
	}

	cases := []struct {
		name      string
		data      coretask.Management
		vendor    string
		operation string
	}{
		{
			name:      "empty biz id",
			data:      coretask.Management{Revision: core.Revision{CreatedAt: data.CreatedAt}},
			vendor:    string(enumor.TCloudZiyan),
			operation: string(enumor.TaskTargetGroupModifyWeight),
		},
		{
			name:      "empty vendor",
			data:      data,
			vendor:    "",
			operation: string(enumor.TaskTargetGroupModifyWeight),
		},
		{
			name:      "empty operation",
			data:      data,
			vendor:    string(enumor.TCloudZiyan),
			operation: "",
		},
		{
			name:      "empty created at",
			data:      coretask.Management{BkBizID: data.BkBizID},
			vendor:    string(enumor.TCloudZiyan),
			operation: string(enumor.TaskTargetGroupModifyWeight),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if validManagementMetricDims(tc.data, tc.vendor, tc.operation) {
				t.Fatal("expected invalid management metric dimensions")
			}
		})
	}
}

func TestValidDetailMetricDims(t *testing.T) {
	detail := coretask.Detail{
		BkBizID:   213,
		Operation: enumor.TaskTargetGroupModifyWeight,
		Revision: core.Revision{
			CreatedAt: "2026-05-21T07:20:19Z",
			UpdatedAt: "2026-05-21T07:20:47Z",
		},
	}
	if !validDetailMetricDims(detail) {
		t.Fatal("expected valid detail metric dimensions")
	}

	cases := []struct {
		name   string
		detail coretask.Detail
	}{
		{name: "empty biz id", detail: coretask.Detail{
			Operation: detail.Operation,
			Revision:  core.Revision{CreatedAt: detail.CreatedAt, UpdatedAt: detail.UpdatedAt}}},
		{name: "empty operation", detail: coretask.Detail{
			BkBizID:  detail.BkBizID,
			Revision: core.Revision{CreatedAt: detail.CreatedAt, UpdatedAt: detail.UpdatedAt}}},
		{name: "empty created at", detail: coretask.Detail{
			BkBizID:   detail.BkBizID,
			Operation: detail.Operation,
			Revision:  core.Revision{UpdatedAt: detail.UpdatedAt}}},
		{name: "empty updated at", detail: coretask.Detail{
			BkBizID:   detail.BkBizID,
			Operation: detail.Operation,
			Revision:  core.Revision{CreatedAt: detail.CreatedAt}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if validDetailMetricDims(tc.detail) {
				t.Fatal("expected invalid detail metric dimensions")
			}
		})
	}
}

func TestTerminalCost(t *testing.T) {
	cost, ok := terminalCost("2026-05-21T07:20:19Z", "2026-05-21T07:20:47Z")
	if !ok {
		t.Fatal("expected terminal cost to be valid")
	}
	if cost != 28*time.Second {
		t.Fatalf("expected cost 28s, got %s", cost)
	}

	cases := []struct {
		name  string
		start string
		end   string
	}{
		{name: "empty start", start: "", end: "2026-05-21T07:20:47Z"},
		{name: "invalid end", start: "2026-05-21T07:20:19Z", end: "invalid"},
		{name: "end before start", start: "2026-05-21T07:20:47Z", end: "2026-05-21T07:20:19Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := terminalCost(tc.start, tc.end); ok {
				t.Fatal("expected terminal cost to be invalid")
			}
		})
	}
}
