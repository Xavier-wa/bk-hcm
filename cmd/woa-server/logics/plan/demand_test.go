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

package plan

import (
	"testing"
	"time"

	"hcm/pkg/criteria/constant"
)

func TestDeduplicateBudgetOperatorCandidates(t *testing.T) {
	tests := []struct {
		name     string
		input    [][]string
		expected []string
	}{
		{
			name:     "空列表",
			input:    [][]string{},
			expected: []string{},
		},
		{
			name:     "单个空列表",
			input:    [][]string{{}},
			expected: []string{},
		},
		{
			name:     "过滤空字符串",
			input:    [][]string{{"", "user1", "", "user2"}},
			expected: []string{"user1", "user2"},
		},
		{
			name:     "过滤backend用户",
			input:    [][]string{{"user1", constant.BackendOperationUserKey, "user2"}},
			expected: []string{"user1", "user2"},
		},
		{
			name:     "去重",
			input:    [][]string{{"user1", "user2", "user1", "user3", "user2"}},
			expected: []string{"user1", "user2", "user3"},
		},
		{
			name:     "去除前后空格",
			input:    [][]string{{"  user1  ", "user2", " user1"}},
			expected: []string{"user1", "user2"},
		},
		{
			name:     "多个列表合并去重",
			input:    [][]string{{"user1", "user2"}, {"user2", "user3"}, {"user1", "user4"}},
			expected: []string{"user1", "user2", "user3", "user4"},
		},
		{
			name:     "综合场景",
			input:    [][]string{{"", "user1", constant.BackendOperationUserKey}, {"  user1  ", "user2", ""}},
			expected: []string{"user1", "user2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateBudgetOperatorCandidates(tt.input...)
			if len(result) != len(tt.expected) {
				t.Errorf("deduplicateBudgetOperatorCandidates() 长度不匹配, got %d, want %d", len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("deduplicateBudgetOperatorCandidates()[%d] = %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestGroupBudgetDemands(t *testing.T) {
	tests := []struct {
		name           string
		demands        []budgetDemand
		expectedGroups int
		checkFunc      func(t *testing.T, groups map[string]*demandGroup)
	}{
		{
			name:           "空列表",
			demands:        []budgetDemand{},
			expectedGroups: 0,
		},
		{
			name: "单个需求",
			demands: []budgetDemand{
				{ID: "1", OpProductID: 100, ExpectTime: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
			},
			expectedGroups: 1,
			checkFunc: func(t *testing.T, groups map[string]*demandGroup) {
				key := "100-2026-01"
				if _, exists := groups[key]; !exists {
					t.Errorf("期望存在分组 %s", key)
				}
				if len(groups[key].Items) != 1 {
					t.Errorf("分组 %s 应有 1 个元素, 实际有 %d 个", key, len(groups[key].Items))
				}
			},
		},
		{
			name: "同一OpProductID同一月份",
			demands: []budgetDemand{
				{ID: "1", OpProductID: 100, ExpectTime: time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)},
				{ID: "2", OpProductID: 100, ExpectTime: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)},
			},
			expectedGroups: 1,
			checkFunc: func(t *testing.T, groups map[string]*demandGroup) {
				key := "100-2026-01"
				if len(groups[key].Items) != 2 {
					t.Errorf("分组 %s 应有 2 个元素, 实际有 %d 个", key, len(groups[key].Items))
				}
			},
		},
		{
			name: "同一OpProductID跨月份",
			demands: []budgetDemand{
				{ID: "1", OpProductID: 100, ExpectTime: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
				{ID: "2", OpProductID: 100, ExpectTime: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)},
			},
			expectedGroups: 2,
			checkFunc: func(t *testing.T, groups map[string]*demandGroup) {
				if _, exists := groups["100-2026-01"]; !exists {
					t.Error("期望存在分组 100-2026-01")
				}
				if _, exists := groups["100-2026-02"]; !exists {
					t.Error("期望存在分组 100-2026-02")
				}
			},
		},
		{
			name: "不同OpProductID同一月份",
			demands: []budgetDemand{
				{ID: "1", OpProductID: 100, ExpectTime: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
				{ID: "2", OpProductID: 200, ExpectTime: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
			},
			expectedGroups: 2,
			checkFunc: func(t *testing.T, groups map[string]*demandGroup) {
				if _, exists := groups["100-2026-01"]; !exists {
					t.Error("期望存在分组 100-2026-01")
				}
				if _, exists := groups["200-2026-01"]; !exists {
					t.Error("期望存在分组 200-2026-01")
				}
			},
		},
		{
			name: "跨年场景",
			demands: []budgetDemand{
				{ID: "1", OpProductID: 100, ExpectTime: time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC)},
				{ID: "2", OpProductID: 100, ExpectTime: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
			},
			expectedGroups: 2,
			checkFunc: func(t *testing.T, groups map[string]*demandGroup) {
				if _, exists := groups["100-2025-12"]; !exists {
					t.Error("期望存在分组 100-2025-12")
				}
				if _, exists := groups["100-2026-01"]; !exists {
					t.Error("期望存在分组 100-2026-01")
				}
				// 验证年份字段
				if groups["100-2025-12"].Year != 2025 {
					t.Errorf("分组 100-2025-12 的 Year 应为 2025, 实际为 %d", groups["100-2025-12"].Year)
				}
				if groups["100-2026-01"].Year != 2026 {
					t.Errorf("分组 100-2026-01 的 Year 应为 2026, 实际为 %d", groups["100-2026-01"].Year)
				}
			},
		},
		{
			name: "复杂场景：多产品多月份",
			demands: []budgetDemand{
				{ID: "1", OpProductID: 100, ExpectTime: time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)},
				{ID: "2", OpProductID: 100, ExpectTime: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)},
				{ID: "3", OpProductID: 100, ExpectTime: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)},
				{ID: "4", OpProductID: 200, ExpectTime: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
				{ID: "5", OpProductID: 200, ExpectTime: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)},
			},
			expectedGroups: 4,
			checkFunc: func(t *testing.T, groups map[string]*demandGroup) {
				if len(groups["100-2026-01"].Items) != 2 {
					t.Errorf("分组 100-2026-01 应有 2 个元素")
				}
				if len(groups["100-2026-02"].Items) != 1 {
					t.Errorf("分组 100-2026-02 应有 1 个元素")
				}
				if len(groups["200-2026-01"].Items) != 1 {
					t.Errorf("分组 200-2026-01 应有 1 个元素")
				}
				if len(groups["200-2026-03"].Items) != 1 {
					t.Errorf("分组 200-2026-03 应有 1 个元素")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := groupBudgetDemands(tt.demands)
			if len(groups) != tt.expectedGroups {
				t.Errorf("groupBudgetDemands() 分组数量 = %d, 期望 %d", len(groups), tt.expectedGroups)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, groups)
			}
		})
	}
}

func TestGroupBudgetDemands_PointerStability(t *testing.T) {
	demands := []budgetDemand{
		{ID: "1", OpProductID: 100, ExpectTime: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
		{ID: "2", OpProductID: 100, ExpectTime: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)},
	}

	groups := groupBudgetDemands(demands)
	key := "100-2026-01"

	// 验证指针指向原数组元素
	for i, item := range groups[key].Items {
		if item != &demands[i] {
			t.Errorf("Items[%d] 应指向原数组 demands[%d]", i, i)
		}
	}

	// 修改原数组，验证分组中的数据也变化
	demands[0].Creator = "modified_user"
	if groups[key].Items[0].Creator != "modified_user" {
		t.Error("修改原数组后，分组中的数据应同步变化")
	}
}
