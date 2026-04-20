/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
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

	demandtime "hcm/cmd/woa-server/logics/plan/demand-time"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/tools/times"
)

// mockDemandTime 是 DemandTime 接口的 mock 实现
type mockDemandTime struct {
	dateRange times.DateRange
	err       error
}

func (m *mockDemandTime) GetDemandYearMonthWeek(kt *kit.Kit, t time.Time) (demandtime.DemandYearMonthWeek, error) {
	return demandtime.DemandYearMonthWeek{}, nil
}

func (m *mockDemandTime) GetDemandYearMonth(kt *kit.Kit, t time.Time) (int, time.Month, error) {
	return 0, 0, nil
}

func (m *mockDemandTime) GetDemandDateRangeInMonth(kt *kit.Kit, t time.Time) (times.DateRange, error) {
	return m.dateRange, m.err
}

func (m *mockDemandTime) GetDemandDateRangeByYearMonth(kt *kit.Kit, year int, month time.Month) (times.DateRange, error) {
	return m.dateRange, m.err
}

func (m *mockDemandTime) GetDemandDateRangeInWeek(kt *kit.Kit, t time.Time) times.DateRange {
	return m.dateRange
}

func (m *mockDemandTime) IsDayCrossMonth(kt *kit.Kit, t time.Time) (bool, error) {
	return false, nil
}

func (m *mockDemandTime) GetDemandStatusByExpectTime(kt *kit.Kit, expectTime string) (enumor.DemandStatus,
	times.DateRange, error) {
	return "", times.DateRange{}, nil
}

func TestNeedToTransferDemand(t *testing.T) {
	loc := time.Local

	tests := []struct {
		name      string
		dateRange times.DateRange
		checkTime time.Time
		want      bool
		wantErr   bool
	}{
		{
			name:      "需求月=自然月：月末当天应触发",
			dateRange: times.DateRange{Start: "2026-04-01", End: "2026-04-30"},
			checkTime: time.Date(2026, 4, 30, 22, 0, 0, 0, loc),
			want:      true,
		},
		{
			name:      "需求月=自然月：非月末不触发",
			dateRange: times.DateRange{Start: "2026-04-01", End: "2026-04-30"},
			checkTime: time.Date(2026, 4, 15, 22, 0, 0, 0, loc),
			want:      false,
		},
		{
			name:      "需求月跨月(03.30-05.03)：截止日05.03当天应触发",
			dateRange: times.DateRange{Start: "2026-03-30", End: "2026-05-03"},
			checkTime: time.Date(2026, 5, 3, 22, 0, 0, 0, loc),
			want:      true,
		},
		{
			name:      "需求月跨月(03.30-05.03)：04.03的day也是3但不应触发(原BUG场景)",
			dateRange: times.DateRange{Start: "2026-03-30", End: "2026-05-03"},
			checkTime: time.Date(2026, 4, 3, 22, 0, 0, 0, loc),
			want:      false,
		},
		{
			name:      "需求月跨月(03.30-05.03)：04.30(自然月末)不应触发",
			dateRange: times.DateRange{Start: "2026-03-30", End: "2026-05-03"},
			checkTime: time.Date(2026, 4, 30, 22, 0, 0, 0, loc),
			want:      false,
		},
		{
			name:      "需求月跨月(03.30-05.03)：月中某天不应触发",
			dateRange: times.DateRange{Start: "2026-03-30", End: "2026-05-03"},
			checkTime: time.Date(2026, 4, 15, 22, 0, 0, 0, loc),
			want:      false,
		},
		{
			name:      "需求月跨月(03.30-05.03)：起始日不应触发",
			dateRange: times.DateRange{Start: "2026-03-30", End: "2026-05-03"},
			checkTime: time.Date(2026, 3, 30, 22, 0, 0, 0, loc),
			want:      false,
		},
		{
			name:      "2月截止日28号：28号应触发",
			dateRange: times.DateRange{Start: "2026-02-02", End: "2026-02-28"},
			checkTime: time.Date(2026, 2, 28, 22, 0, 0, 0, loc),
			want:      true,
		},
		{
			name:      "2月截止日28号：3月28号不应触发(跨月同day)",
			dateRange: times.DateRange{Start: "2026-02-02", End: "2026-02-28"},
			checkTime: time.Date(2026, 3, 28, 22, 0, 0, 0, loc),
			want:      false,
		},
		{
			name:      "闰年2月截止日29号：29号应触发",
			dateRange: times.DateRange{Start: "2024-02-05", End: "2024-02-29"},
			checkTime: time.Date(2024, 2, 29, 22, 0, 0, 0, loc),
			want:      true,
		},
		{
			name:      "跨年需求月(12.30-01.05)：1月5号应触发",
			dateRange: times.DateRange{Start: "2025-12-30", End: "2026-01-05"},
			checkTime: time.Date(2026, 1, 5, 22, 0, 0, 0, loc),
			want:      true,
		},
		{
			name:      "跨年需求月(12.01-01.05)：12月5号不应触发",
			dateRange: times.DateRange{Start: "2025-12-01", End: "2026-01-05"},
			checkTime: time.Date(2025, 12, 5, 22, 0, 0, 0, loc),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := &Controller{
				demandTime: &mockDemandTime{dateRange: tt.dateRange},
			}
			kt := &kit.Kit{
				Rid: "test-rid",
			}

			got, err := ctrl.needToTransferDemand(kt, tt.checkTime)
			if (err != nil) != tt.wantErr {
				t.Errorf("needToTransferDemand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("needToTransferDemand() = %v, want %v", got, tt.want)
			}
		})
	}
}
