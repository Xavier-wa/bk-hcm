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

package rollingserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsInheritedHostRecommended(t *testing.T) {
	now := time.Date(2026, 8, 14, 10, 30, 0, 0, time.UTC)
	tests := []struct {
		name             string
		billingStartTime time.Time
		want             bool
	}{
		{
			name:             "计费刚好满 36 个月",
			billingStartTime: time.Date(2023, 8, 14, 10, 30, 0, 0, time.UTC),
			want:             true,
		},
		{
			name:             "计费超过 36 个月",
			billingStartTime: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
			want:             true,
		},
		{
			name:             "计费不足 36 个月：差一天",
			billingStartTime: time.Date(2023, 8, 15, 10, 30, 0, 0, time.UTC),
			want:             false,
		},
		{
			name:             "计费不足 36 个月",
			billingStartTime: time.Date(2025, 3, 5, 9, 0, 0, 0, time.UTC),
			want:             false,
		},
		{
			// 计费起始时间缺失时无法判断已计费月数，不作为推荐项
			name:             "计费起始时间为零值",
			billingStartTime: time.Time{},
			want:             false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isInheritedHostRecommended(now, tt.billingStartTime))
		})
	}
}
