/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package scheduler

import (
	"testing"

	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/thirdparty/cvmapi"

	"github.com/stretchr/testify/assert"
)

func TestGetSpecConcreteZones(t *testing.T) {
	testCases := []struct {
		name     string
		spec     *types.ResourceSpec
		expected []string
	}{
		{
			name:     "nil spec",
			spec:     nil,
			expected: []string{},
		},
		{
			name:     "zone and zones are inconsistent",
			spec:     &types.ResourceSpec{Zone: "ap-guangzhou-9", Zones: []string{"ap-guangzhou-10"}},
			expected: []string{"ap-guangzhou-10"},
		},
		{
			name:     "multiple zones",
			spec:     &types.ResourceSpec{Zones: []string{"ap-guangzhou-9", "ap-guangzhou-10"}},
			expected: []string{"ap-guangzhou-9", "ap-guangzhou-10"},
		},
		{
			name:     "duplicated zones",
			spec:     &types.ResourceSpec{Zones: []string{"ap-guangzhou-9", "ap-guangzhou-9"}},
			expected: []string{"ap-guangzhou-9"},
		},
		{
			name:     "all zones",
			spec:     &types.ResourceSpec{Zones: []string{cvmapi.CvmZoneAll}},
			expected: []string{},
		},
		{
			name:     "separate campus without zones",
			spec:     &types.ResourceSpec{Zone: cvmapi.CvmSeparateCampus},
			expected: []string{},
		},
		{
			name:     "fallback to zone while zones is empty",
			spec:     &types.ResourceSpec{Zone: "ap-guangzhou-9"},
			expected: []string{"ap-guangzhou-9"},
		},
		{
			name:     "no zone and no zones",
			spec:     &types.ResourceSpec{},
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, getSpecConcreteZones(tc.spec))
		})
	}
}

func TestDescribeZones(t *testing.T) {
	testCases := []struct {
		name     string
		zones    []string
		expected string
	}{
		{
			name:     "no concrete zone",
			zones:    []string{},
			expected: "全部可用区",
		},
		{
			name:     "single zone",
			zones:    []string{"ap-guangzhou-10"},
			expected: "可用区ap-guangzhou-10",
		},
		{
			name:     "multiple zones",
			zones:    []string{"ap-guangzhou-10", "ap-guangzhou-11"},
			expected: "可用区ap-guangzhou-10、ap-guangzhou-11",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, describeZones(tc.zones))
		})
	}
}
