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

package math

import "testing"

func TestPercentile(t *testing.T) {
	tests := []struct {
		name   string
		sorted []int
		p      float64
		want   int
	}{
		{name: "empty", want: 0},
		{name: "single", sorted: []int{7}, p: 50, want: 7},
		{name: "p50 of two", sorted: []int{70, 90}, p: 50, want: 70},
		{name: "p95 of two", sorted: []int{70, 90}, p: 95, want: 90},
		{name: "p100", sorted: []int{1, 2, 3, 4}, p: 100, want: 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Percentile(tc.sorted, tc.p); got != tc.want {
				t.Fatalf("Percentile(%v, %v) = %d, want %d", tc.sorted, tc.p, got, tc.want)
			}
		})
	}
}
