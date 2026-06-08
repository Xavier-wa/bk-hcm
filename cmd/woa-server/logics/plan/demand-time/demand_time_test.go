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

package demandtime

import (
	"strconv"
	"testing"
	"time"

	rpproto "hcm/pkg/api/data-service/resource-plan"
	"hcm/pkg/criteria/enumor"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	tablers "hcm/pkg/dal/table/resource-plan/res-plan-week"
	"hcm/pkg/kit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKit() *kit.Kit {
	return &kit.Kit{Rid: "test-rid"}
}

func weekListResult(weeks []tablers.ResPlanWeekTable) *rpproto.ResPlanWeekListResult {
	return &rpproto.ResPlanWeekListResult{
		Count:   uint64(len(weeks)),
		Details: weeks,
	}
}

func TestGetDemandYearMonthWeek(t *testing.T) {
	kt := testKit()
	in := []time.Time{
		time.Date(2024, 9, 1, 0, 0, 0, 0, time.Local),
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local),
		time.Date(2024, 1, 7, 0, 0, 0, 0, time.Local),
		time.Date(2024, 1, 8, 0, 0, 0, 0, time.Local),
		time.Date(2024, 12, 31, 0, 0, 0, 0, time.Local),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}
	expect := []DemandYearMonthWeek{
		{
			Year: 2024, Month: 8, Week: 4, YearWeek: 35,
		},
		{
			Year: 2024, Month: 1, Week: 1, YearWeek: 1,
		},
		{
			Year: 2024, Month: 1, Week: 1, YearWeek: 1,
		},
		{
			Year: 2024, Month: 1, Week: 2, YearWeek: 2,
		},
		{
			Year: 2024, Month: 12, Week: 5, YearWeek: 53,
		},
		{
			Year: 2024, Month: 12, Week: 5, YearWeek: 53,
		},
	}

	stubs := []func(kt *kit.Kit, req *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error){
		// 2024-09-01
		func(_ *kit.Kit, req *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error) {
			if len(req.Filter.Rules) == 2 {
				return weekListResult([]tablers.ResPlanWeekTable{
					{Year: 2024, Month: 8, YearWeek: 35, Start: 20240826, End: 20240901},
				}), nil
			}
			return weekListResult([]tablers.ResPlanWeekTable{
				{Year: 2024, Month: 8, YearWeek: 32, Start: 20240729, End: 20240804},
				{Year: 2024, Month: 8, YearWeek: 33, Start: 20240805, End: 20240811},
				{Year: 2024, Month: 8, YearWeek: 34, Start: 20240812, End: 20240818},
				{Year: 2024, Month: 8, YearWeek: 35, Start: 20240826, End: 20240901},
			}), nil
		},
		// 2024-01-01
		func(_ *kit.Kit, req *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error) {
			if len(req.Filter.Rules) == 2 {
				return weekListResult([]tablers.ResPlanWeekTable{
					{Year: 2024, Month: 1, YearWeek: 1, Start: 20240101, End: 20240107},
				}), nil
			}
			return weekListResult([]tablers.ResPlanWeekTable{
				{Year: 2024, Month: 1, YearWeek: 1, Start: 20240101, End: 20240107},
			}), nil
		},
		// 2024-01-07
		func(_ *kit.Kit, req *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error) {
			if len(req.Filter.Rules) == 2 {
				return weekListResult([]tablers.ResPlanWeekTable{
					{Year: 2024, Month: 1, YearWeek: 1, Start: 20240101, End: 20240107},
				}), nil
			}
			return weekListResult([]tablers.ResPlanWeekTable{
				{Year: 2024, Month: 1, YearWeek: 1, Start: 20240101, End: 20240107},
			}), nil
		},
		// 2024-01-08
		func(_ *kit.Kit, req *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error) {
			if len(req.Filter.Rules) == 2 {
				return weekListResult([]tablers.ResPlanWeekTable{
					{Year: 2024, Month: 1, YearWeek: 2, Start: 20240108, End: 20240114},
				}), nil
			}
			return weekListResult([]tablers.ResPlanWeekTable{
				{Year: 2024, Month: 1, YearWeek: 1, Start: 20240101, End: 20240107},
				{Year: 2024, Month: 1, YearWeek: 2, Start: 20240108, End: 20240114},
			}), nil
		},
		// 2024-12-31
		func(_ *kit.Kit, req *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error) {
			if len(req.Filter.Rules) == 2 {
				return weekListResult([]tablers.ResPlanWeekTable{
					{Year: 2024, Month: 12, YearWeek: 53, Start: 20241230, End: 20250105},
				}), nil
			}
			return weekListResult([]tablers.ResPlanWeekTable{
				{Year: 2024, Month: 12, YearWeek: 49, Start: 20241202, End: 20241208},
				{Year: 2024, Month: 12, YearWeek: 50, Start: 20241209, End: 20241215},
				{Year: 2024, Month: 12, YearWeek: 51, Start: 20241216, End: 20241222},
				{Year: 2024, Month: 12, YearWeek: 52, Start: 20241223, End: 20241229},
				{Year: 2024, Month: 12, YearWeek: 53, Start: 20241230, End: 20250105},
			}), nil
		},
		// 2025-01-01
		func(_ *kit.Kit, req *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error) {
			if len(req.Filter.Rules) == 2 {
				return weekListResult([]tablers.ResPlanWeekTable{
					{Year: 2024, Month: 12, YearWeek: 53, Start: 20241230, End: 20250105},
				}), nil
			}
			return weekListResult([]tablers.ResPlanWeekTable{
				{Year: 2024, Month: 12, YearWeek: 49, Start: 20241202, End: 20241208},
				{Year: 2024, Month: 12, YearWeek: 50, Start: 20241209, End: 20241215},
				{Year: 2024, Month: 12, YearWeek: 51, Start: 20241216, End: 20241222},
				{Year: 2024, Month: 12, YearWeek: 52, Start: 20241223, End: 20241229},
				{Year: 2024, Month: 12, YearWeek: 53, Start: 20241230, End: 20250105},
			}), nil
		},
	}

	for i, d := range in {
		t.Run(d.Format("2006-01-02"), func(t *testing.T) {
			dt := DemandTimeFromTable{testListResPlanWeek: stubs[i]}
			ymw, err := dt.GetDemandYearMonthWeek(kt, d)
			require.NoError(t, err)
			assert.Equal(t, expect[i], ymw)
		})
	}
}

func TestGetDemandDateRangeInWeek(t *testing.T) {
	kt := testKit()
	date := time.Date(2024, 9, 1, 0, 0, 0, 0, time.Local)
	dt := DemandTimeFromTable{}
	dr := dt.GetDemandDateRangeInWeek(kt, date)
	assert.Equal(t, "2024-08-26", dr.Start)
	assert.Equal(t, "2024-09-01", dr.End)
}

func TestGetDemandDateRangeInMonth(t *testing.T) {
	kt := testKit()
	date := time.Date(2024, 9, 1, 0, 0, 0, 0, time.Local)
	call := 0
	dt := DemandTimeFromTable{
		testListResPlanWeek: func(_ *kit.Kit, req *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error) {
			call++
			if call == 1 {
				return weekListResult([]tablers.ResPlanWeekTable{
					{Year: 2024, Month: 8, YearWeek: 35, Start: 20240826, End: 20240901},
				}), nil
			}
			return weekListResult([]tablers.ResPlanWeekTable{
				{Year: 2024, Month: 8, YearWeek: 32, Start: 20240729, End: 20240804},
				{Year: 2024, Month: 8, YearWeek: 33, Start: 20240805, End: 20240811},
				{Year: 2024, Month: 8, YearWeek: 34, Start: 20240812, End: 20240818},
				{Year: 2024, Month: 8, YearWeek: 35, Start: 20240826, End: 20240901},
			}), nil
		},
	}

	dr, err := dt.GetDemandDateRangeInMonth(kt, date)
	require.NoError(t, err)
	assert.Equal(t, "2024-07-29", dr.Start)
	assert.Equal(t, "2024-09-01", dr.End)
}

func TestIsDayCrossMonth(t *testing.T) {
	kt := testKit()
	in := []time.Time{
		time.Date(2024, 12, 31, 0, 0, 0, 0, time.Local),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
		time.Date(2025, 2, 28, 0, 0, 0, 0, time.Local),
		time.Date(2025, 3, 2, 0, 0, 0, 0, time.Local),
	}
	expect := []bool{
		false,
		true,
		false,
		true,
	}

	stubs := []func(_ *kit.Kit, _ *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error){
		func(_ *kit.Kit, _ *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error) {
			return weekListResult([]tablers.ResPlanWeekTable{
				{Year: 2024, Month: 12, YearWeek: 53, Start: 20241230, End: 20250105},
			}), nil
		},
		func(_ *kit.Kit, _ *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error) {
			return weekListResult([]tablers.ResPlanWeekTable{
				{Year: 2024, Month: 12, YearWeek: 53, Start: 20241230, End: 20250105},
			}), nil
		},
		func(_ *kit.Kit, _ *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error) {
			return weekListResult([]tablers.ResPlanWeekTable{
				{Year: 2025, Month: 2, YearWeek: 9, Start: 20250224, End: 20250302},
			}), nil
		},
		func(_ *kit.Kit, _ *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error) {
			return weekListResult([]tablers.ResPlanWeekTable{
				{Year: 2025, Month: 2, YearWeek: 9, Start: 20250224, End: 20250302},
			}), nil
		},
	}

	for i, d := range in {
		t.Run(d.Format("2006-01-02"), func(t *testing.T) {
			dt := DemandTimeFromTable{testListResPlanWeek: stubs[i]}
			got, err := dt.IsDayCrossMonth(kt, d)
			require.NoError(t, err)
			assert.Equal(t, expect[i], got)
		})
	}
}

func TestContainsNonCurrentYearDemand(t *testing.T) {
	currentYear := time.Now().Year()
	nextYear := strconv.Itoa(currentYear + 1)
	prevYear := strconv.Itoa(currentYear - 1)

	testCases := []struct {
		name     string
		demands  rpt.ResPlanDemands
		wantTrue bool
	}{
		{
			name:     "empty demands",
			demands:  rpt.ResPlanDemands{},
			wantTrue: false,
		},
		{
			name: "updated is next year",
			demands: rpt.ResPlanDemands{
				{Updated: &rpt.UpdatedRPDemandItem{ExpectTime: nextYear + "-01-01"}},
			},
			wantTrue: true,
		},
		{
			name: "updated is previous year",
			demands: rpt.ResPlanDemands{
				{Updated: &rpt.UpdatedRPDemandItem{ExpectTime: prevYear + "-01-01"}},
			},
			wantTrue: true,
		},
		{
			name: "original is next year",
			demands: rpt.ResPlanDemands{
				{
					Original: &rpt.OriginalRPDemandItem{
						ExpectTime: nextYear + "-06-01",
						Cvm:        rpt.Cvm{DeviceFamily: string(enumor.DeviceFamilyStandard)},
					},
					Updated: &rpt.UpdatedRPDemandItem{
						ExpectTime: strconv.Itoa(currentYear) + "-06-01",
					},
				},
			},
			wantTrue: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ContainsNonCurrentYearDemand(tc.demands)
			assert.Equal(t, tc.wantTrue, got)
		})
	}
}

func TestContainsNonCurrentYearDemandPtrs(t *testing.T) {
	currentYear := time.Now().Year()
	nextYear := strconv.Itoa(currentYear + 1)

	demands := []*rpt.ResPlanDemand{
		{Updated: &rpt.UpdatedRPDemandItem{ExpectTime: nextYear + "-03-01"}},
		nil,
	}
	assert.True(t, ContainsNonCurrentYearDemandPtrs(demands))
}
