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

package bkaidev

import (
	"testing"
)

func TestMatchTagNames(t *testing.T) {
	tests := []struct {
		name          string
		skillTagNames [][]string
		tagNameFilter map[string]string
		want          bool
	}{
		{
			name:          "empty filter matches all",
			skillTagNames: [][]string{{"status", "enabled"}},
			tagNameFilter: map[string]string{},
			want:          true,
		},
		{
			name:          "empty skill tags with non-empty filter",
			skillTagNames: [][]string{},
			tagNameFilter: map[string]string{"status": "enabled"},
			want:          false,
		},
		{
			name:          "match first-level tag only (empty value in filter)",
			skillTagNames: [][]string{{"status", "enabled"}},
			tagNameFilter: map[string]string{"status": ""},
			want:          true,
		},
		{
			name:          "match first-level and second-level tag",
			skillTagNames: [][]string{{"status", "enabled"}},
			tagNameFilter: map[string]string{"status": "enabled"},
			want:          true,
		},
		{
			name:          "first-level matches but second-level does not",
			skillTagNames: [][]string{{"status", "disabled"}},
			tagNameFilter: map[string]string{"status": "enabled"},
			want:          false,
		},
		{
			name:          "first-level tag does not match",
			skillTagNames: [][]string{{"category", "test"}},
			tagNameFilter: map[string]string{"status": "enabled"},
			want:          false,
		},
		{
			name: "multiple skill tags, filter matches one",
			skillTagNames: [][]string{
				{"category", "test"},
				{"status", "enabled"},
			},
			tagNameFilter: map[string]string{"status": "enabled"},
			want:          true,
		},
		{
			name: "multiple filter entries, one does not match (AND semantics)",
			skillTagNames: [][]string{{"status", "enabled"}},
			tagNameFilter: map[string]string{
				"category": "test",
				"status":   "enabled",
			},
			want: false, // "category" not in skill tags, so AND fails
		},
		{
			name: "multiple filter entries, all match (AND semantics)",
			skillTagNames: [][]string{
				{"status", "enabled"},
				{"category", "test"},
			},
			tagNameFilter: map[string]string{
				"status":   "enabled",
				"category": "test",
			},
			want: true, // All filter entries match
		},
		{
			name:          "skill tag with only first-level (no second-level)",
			skillTagNames: [][]string{{"status"}},
			tagNameFilter: map[string]string{"status": ""},
			want:          true,
		},
		{
			name:          "skill tag with only first-level, filter has second-level",
			skillTagNames: [][]string{{"status"}},
			tagNameFilter: map[string]string{"status": "enabled"},
			want:          false,
		},
		{
			name: "multiple filter entries, first-level only matches",
			skillTagNames: [][]string{
				{"status", "enabled"},
				{"category", "test"},
			},
			tagNameFilter: map[string]string{
				"status":   "", // only first-level match required
				"category": "test",
			},
			want: true, // Both match: status exists, category=test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchTagNames(tt.skillTagNames, tt.tagNameFilter)
			if got != tt.want {
				t.Errorf("MatchTagNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterSkillsByTagNames(t *testing.T) {
	skills := []SkillListItem{
		{
			BaseSkill: BaseSkill{
				ID:        1,
				SkillName: "skill-1",
				TagNames:  [][]string{{"status", "enabled"}},
			},
		},
		{
			BaseSkill: BaseSkill{
				ID:        2,
				SkillName: "skill-2",
				TagNames:  [][]string{{"status", "disabled"}},
			},
		},
		{
			BaseSkill: BaseSkill{
				ID:        3,
				SkillName: "skill-3",
				TagNames:  [][]string{{"category", "test"}, {"status", "enabled"}},
			},
		},
		{
			BaseSkill: BaseSkill{
				ID:        4,
				SkillName: "skill-4",
				TagNames:  [][]string{},
			},
		},
	}

	tests := []struct {
		name          string
		tagNameFilter map[string]string
		wantCount     int
	}{
		{
			name:          "empty filter returns all",
			tagNameFilter: map[string]string{},
			wantCount:     4,
		},
		{
			name: "filter by status=enabled",
			tagNameFilter: map[string]string{
				"status": "enabled",
			},
			wantCount: 2, // skill-1 and skill-3
		},
		{
			name: "filter by status only (empty value)",
			tagNameFilter: map[string]string{
				"status": "",
			},
			wantCount: 3, // skill-1, skill-2, skill-3
		},
		{
			name: "filter by non-existent tag",
			tagNameFilter: map[string]string{
				"nonexistent": "value",
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterSkillsByTagNames(skills, tt.tagNameFilter)
			if len(got) != tt.wantCount {
				t.Errorf("FilterSkillsByTagNames() returned %d skills, want %d", len(got), tt.wantCount)
			}
		})
	}
}
