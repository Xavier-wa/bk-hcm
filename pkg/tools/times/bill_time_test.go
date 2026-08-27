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

package times

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldAutoPullBillPeriod(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	cst := func(year int, month time.Month, day, hour, min int) time.Time {
		return time.Date(year, month, day, hour, min, 0, 0, loc)
	}

	testCases := []struct {
		name      string
		now       time.Time
		billYear  int
		billMonth int
		want      bool
	}{
		{
			name:      "current month mid-month is always false",
			now:       cst(2026, time.August, 15, 10, 0),
			billYear:  2026,
			billMonth: 8,
			want:      false,
		},
		{
			name:      "current month last day still false",
			now:       cst(2026, time.August, 31, 23, 59),
			billYear:  2026,
			billMonth: 8,
			want:      false,
		},
		{
			name:      "last month before 16:00 on the 1st is false",
			now:       cst(2026, time.August, 1, 15, 59),
			billYear:  2026,
			billMonth: 7,
			want:      false,
		},
		{
			name:      "last month at 16:00 on the 1st is true",
			now:       cst(2026, time.August, 1, 16, 0),
			billYear:  2026,
			billMonth: 7,
			want:      true,
		},
		{
			name:      "last month at 16:01 on the 1st is true",
			now:       cst(2026, time.August, 1, 16, 1),
			billYear:  2026,
			billMonth: 7,
			want:      true,
		},
		{
			name:      "UTC month cut before Beijing 16:00 last month still false",
			now:       time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
			billYear:  2026,
			billMonth: 7,
			want:      false,
		},
		{
			name:      "cross year last month at 16:00 is true",
			now:       cst(2026, time.January, 1, 16, 0),
			billYear:  2025,
			billMonth: 12,
			want:      true,
		},
		{
			name:      "cross year last month before 16:00 is false",
			now:       cst(2026, time.January, 1, 15, 59),
			billYear:  2025,
			billMonth: 12,
			want:      false,
		},
		{
			name:      "weekend 1st does not postpone 16:00",
			now:       cst(2026, time.August, 1, 16, 0),
			billYear:  2026,
			billMonth: 7,
			want:      true,
		},
		{
			name:      "national day 1st does not postpone 16:00",
			now:       cst(2026, time.October, 1, 16, 0),
			billYear:  2026,
			billMonth: 9,
			want:      true,
		},
		{
			name:      "older than last month is true",
			now:       cst(2026, time.August, 15, 10, 0),
			billYear:  2026,
			billMonth: 5,
			want:      true,
		},
		{
			name:      "last month after window remains true mid-month",
			now:       cst(2026, time.August, 15, 10, 0),
			billYear:  2026,
			billMonth: 7,
			want:      true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldAutoPullBillPeriod(tc.now, loc, tc.billYear, tc.billMonth)
			assert.Equal(t, tc.want, got)
		})
	}
}
