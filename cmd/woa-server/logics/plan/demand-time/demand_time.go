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
	"errors"
	"math"
	"strconv"
	"time"

	"hcm/pkg/api/core"
	rpproto "hcm/pkg/api/data-service/resource-plan"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/times"
)

// DemandYearMonthWeek is the year, month and week of the month from a demand perspective.
type DemandYearMonthWeek struct {
	Year  int        `json:"year"`
	Month time.Month `json:"month"`
	// Week 需求年月周
	Week int `json:"week"`
	// YearWeek 需求全年周
	YearWeek int `json:"year_week"`
}

// DemandTime is the interface for demand time.
type DemandTime interface {
	// GetDemandYearMonthWeek 返回给定时间所属的需求年月周
	GetDemandYearMonthWeek(kt *kit.Kit, t time.Time) (DemandYearMonthWeek, error)
	// GetDemandYearMonth 返回给定时间的所属需求年月
	GetDemandYearMonth(kt *kit.Kit, t time.Time) (int, time.Month, error)

	// GetDemandDateRangeInMonth 返回给定时间所属需求周期的起始和结束时间
	// 目前需求周期以月为单位，因此该方法返回需求月的起始和结束时间
	GetDemandDateRangeInMonth(kt *kit.Kit, t time.Time) (times.DateRange, error)
	// GetDemandDateRangeByYearMonth 返回给定时间所属需求周期的起始和结束时间，入参为年、月
	GetDemandDateRangeByYearMonth(kt *kit.Kit, year int, month time.Month) (times.DateRange, error)
	// GetDemandDateRangeInWeek 返回给定时间所在周的起始和结束时间
	GetDemandDateRangeInWeek(kt *kit.Kit, t time.Time) times.DateRange

	// IsDayCrossMonth 判断给定日期的所属需求月是否和所属自然月不同
	IsDayCrossMonth(kt *kit.Kit, t time.Time) (bool, error)
	// GetDemandStatusByExpectTime 根据期望交付时间，判断需求状态
	GetDemandStatusByExpectTime(kt *kit.Kit, expectTime string) (enumor.DemandStatus, times.DateRange, error)
}

// DemandTimeFromTable is the implementation of DemandTime.
type DemandTimeFromTable struct {
	client *client.ClientSet
	// testListResPlanWeek is used in unit tests to stub ListResPlanWeek responses.
	testListResPlanWeek func(kt *kit.Kit, req *rpproto.ResPlanWeekListReq) (*rpproto.ResPlanWeekListResult, error)
}

// NewDemandTimeFromTable ...
func NewDemandTimeFromTable(client *client.ClientSet) DemandTime {
	return &DemandTimeFromTable{
		client: client,
	}
}

func (d DemandTimeFromTable) listResPlanWeek(kt *kit.Kit, req *rpproto.ResPlanWeekListReq) (
	*rpproto.ResPlanWeekListResult, error) {

	if d.testListResPlanWeek != nil {
		return d.testListResPlanWeek(kt, req)
	}
	return d.client.DataService().Global.ResourcePlan.ListResPlanWeek(kt, req)
}

// GetDemandYearMonthWeek returns the year, month and week of the month based on the input time from a demand
// perspective.
func (d DemandTimeFromTable) GetDemandYearMonthWeek(kt *kit.Kit, t time.Time) (DemandYearMonthWeek, error) {
	year, month, err := d.GetDemandYearMonth(kt, t)
	if err != nil {
		logs.Errorf("failed to get demand year month, err: %v, demand_time: %s, rid: %s", err, t.String(),
			kt.Rid)
		return DemandYearMonthWeek{}, err
	}

	timeCompactInt := times.ConvTimeToCompactInt(t)
	// 获取该需求月的所有周，计算本周属于第几周
	listReq := &rpproto.ResPlanWeekListReq{
		ListReq: core.ListReq{
			Filter: tools.ExpressionAnd(
				tools.RuleEqual("year", year),
				tools.RuleEqual("month", month),
			),
			Page: &core.BasePage{
				Start: 0,
				Limit: core.DefaultMaxPageLimit,
				Sort:  "year_week",
			},
		},
	}

	rst, err := d.listResPlanWeek(kt, listReq)
	if err != nil {
		logs.Errorf("failed to list res plan week, err: %v, demand_time: %d, rid: %s", err, timeCompactInt,
			kt.Rid)
		return DemandYearMonthWeek{}, err
	}

	if len(rst.Details) == 0 {
		logs.Errorf("no res plan week found, demand_time: %d, rid: %s", timeCompactInt, kt.Rid)
		return DemandYearMonthWeek{}, errors.New("cannot determine which year_month_week the demand belongs to")
	}

	yearWeek := -1
	week := 1
	for _, item := range rst.Details {
		if item.Start <= timeCompactInt && timeCompactInt <= item.End {
			yearWeek = item.YearWeek
			break
		}
		week++
	}

	return DemandYearMonthWeek{
		Year:     year,
		Month:    month,
		Week:     week,
		YearWeek: yearWeek,
	}, nil
}

// GetDemandYearMonth returns the year, month based on the input time from a demand perspective.
func (d DemandTimeFromTable) GetDemandYearMonth(kt *kit.Kit, t time.Time) (int, time.Month, error) {
	timeCompactInt := times.ConvTimeToCompactInt(t)
	listReq := &rpproto.ResPlanWeekListReq{
		ListReq: core.ListReq{
			Filter: tools.ExpressionAnd(
				tools.RuleLessThanEqual("start", timeCompactInt),
				tools.RuleGreaterThanEqual("end", timeCompactInt),
			),
			Page: core.NewDefaultBasePage(),
		},
	}

	rst, err := d.listResPlanWeek(kt, listReq)
	if err != nil {
		logs.Errorf("failed to list res plan week, err: %v, demand_time: %d, rid: %s", err, timeCompactInt,
			kt.Rid)
		return 0, 0, err
	}

	if len(rst.Details) == 0 {
		logs.Errorf("no res plan week found, demand_time: %d, rid: %s", timeCompactInt, kt.Rid)
		return 0, 0, errors.New("cannot determine which month the demand belongs to")
	}

	year := rst.Details[0].Year
	month := rst.Details[0].Month

	return year, time.Month(month), nil
}

// GetDemandDateRangeInMonth get the date range of a month based on the input time from a demand perspective.
func (d DemandTimeFromTable) GetDemandDateRangeInMonth(kt *kit.Kit, t time.Time) (times.DateRange, error) {
	startDate, endDate, err := d.getDemandMonthStartEndByTime(kt, t)
	if err != nil {
		return times.DateRange{}, err
	}

	return times.DateRange{
		Start: startDate.Format(constant.DateLayout),
		End:   endDate.Format(constant.DateLayout),
	}, nil
}

// GetDemandDateRangeByYearMonth get the date range of a month based on the input time from a demand perspective.
func (d DemandTimeFromTable) GetDemandDateRangeByYearMonth(kt *kit.Kit, year int, month time.Month) (
	times.DateRange, error) {

	startDate, endDate, err := d.getDemandMonthStartEnd(kt, year, month)
	if err != nil {
		return times.DateRange{}, err
	}

	return times.DateRange{
		Start: startDate.Format(constant.DateLayout),
		End:   endDate.Format(constant.DateLayout),
	}, nil
}

// GetDemandDateRangeInWeek get the date range of a week based on the input time from a demand perspective.
func (d DemandTimeFromTable) GetDemandDateRangeInWeek(kt *kit.Kit, t time.Time) times.DateRange {
	weekdays := d.weekdays(t)

	return times.DateRange{
		Start: weekdays[0].Format(constant.DateLayout),
		End:   weekdays[6].Format(constant.DateLayout),
	}
}

// getDemandMonthStartEndByTime 获取给定时间的需求年月的第一天和最后一天
// 当输入时间在所在周跨月，则统一将该周周一所在月作为需求月
func (d DemandTimeFromTable) getDemandMonthStartEndByTime(kt *kit.Kit, t time.Time) (time.Time, time.Time, error) {
	// 获取需求所属年月
	year, month, err := d.GetDemandYearMonth(kt, t)
	if err != nil {
		logs.Errorf("failed to get demand year month, err: %v, demand_time: %s, rid: %s", err, t.String(),
			kt.Rid)
		return time.Time{}, time.Time{}, err
	}

	return d.getDemandMonthStartEnd(kt, year, month)
}

// getDemandMonthStartEnd 获取给定需求年月的第一天和最后一天
func (d DemandTimeFromTable) getDemandMonthStartEnd(kt *kit.Kit, year int, month time.Month) (time.Time, time.Time,
	error) {

	// 获取该需求月的范围
	listReq := &rpproto.ResPlanWeekListReq{
		ListReq: core.ListReq{
			Filter: tools.ExpressionAnd(
				tools.RuleEqual("year", year),
				tools.RuleEqual("month", month),
			),
			Page: core.NewDefaultBasePage(),
		},
	}

	rst, err := d.listResPlanWeek(kt, listReq)
	if err != nil {
		logs.Errorf("failed to list res plan week, err: %v, year: %d, month: %d, rid: %s", err, year, month,
			kt.Rid)
		return time.Time{}, time.Time{}, err
	}

	if len(rst.Details) == 0 {
		logs.Errorf("no res plan week found, year: %d, month: %d, rid: %s", year, month, kt.Rid)
		return time.Time{}, time.Time{}, errors.New("cannot determine which month the demand belongs to")
	}

	startDateInt := math.MaxInt
	endDateInt := 0
	for _, demandWeek := range rst.Details {
		if demandWeek.Start < startDateInt {
			startDateInt = demandWeek.Start
		}
		if demandWeek.End > endDateInt {
			endDateInt = demandWeek.End
		}
	}

	startDate := times.ConvCompactIntToTime(startDateInt)
	endDate := times.ConvCompactIntToTime(endDateInt)
	return startDate, endDate, nil
}

// Weekdays returns the weekdays from Monday to Sunday around the input time.
func (d DemandTimeFromTable) weekdays(t time.Time) (week [7]time.Time) {
	offset := int(time.Monday - t.Weekday())
	if offset > 0 {
		offset = -6
	}

	monday := t.AddDate(0, 0, offset)

	for i := 0; i < 7; i++ {
		week[i] = monday.AddDate(0, 0, i)
	}
	return
}

// IsDayCrossMonth 给定日期，判断日期所属需求月和所属自然月是否不同
func (d DemandTimeFromTable) IsDayCrossMonth(kt *kit.Kit, t time.Time) (bool, error) {
	_, month, err := d.GetDemandYearMonth(kt, t)
	if err != nil {
		logs.Errorf("failed to get demand year month, err: %v, demand_time: %s, rid: %s", err, t.String(),
			kt.Rid)
		return false, err
	}

	if month != t.Month() {
		return true, nil
	}

	return false, nil
}

// GetDemandStatusByExpectTime 根据期望交付时间获取需求状态
func (d DemandTimeFromTable) GetDemandStatusByExpectTime(kt *kit.Kit, expectTime string) (enumor.DemandStatus,
	times.DateRange, error) {

	t, err := time.Parse(constant.DateLayout, expectTime)
	if err != nil {
		logs.Errorf("failed to parse expect time, err: %v, expect_time: %s, rid: %s", err, expectTime, kt.Rid)
		return "", times.DateRange{}, err
	}

	monthStart, monthEnd, err := d.getDemandMonthStartEndByTime(kt, time.Now())
	if err != nil {
		logs.Errorf("failed to get demand month start end, err: %v, expect_time: %s, rid: %s", err, expectTime,
			kt.Rid)
		return "", times.DateRange{}, err
	}

	demandStart, demandEnd, err := d.getDemandMonthStartEndByTime(kt, t)
	if err != nil {
		logs.Errorf("failed to get demand start end, err: %v, expect_time: %s, rid: %s", err, expectTime,
			kt.Rid)
		return "", times.DateRange{}, err
	}

	demandRange := times.DateRange{
		Start: demandStart.Format(constant.DateLayout),
		End:   demandEnd.Format(constant.DateLayout),
	}

	// 未到申领时间
	if t.After(monthEnd) {
		return enumor.DemandStatusNotReady, demandRange, nil
	}

	// 已过期
	if t.Before(monthStart) {
		return enumor.DemandStatusExpired, demandRange, nil
	}

	return enumor.DemandStatusCanApply, demandRange, nil
}

// ContainsNonCurrentYearDemand checks whether demands contain expect_time not in the current calendar year.
func ContainsNonCurrentYearDemand(demands rpt.ResPlanDemands) bool {
	currentYear := time.Now().Year()
	for _, demand := range demands {
		if containsNonCurrentYearInDemand(&demand, currentYear) {
			return true
		}
	}
	return false
}

// ContainsNonCurrentYearDemandPtrs checks whether pointer demands contain expect_time not in the current year.
func ContainsNonCurrentYearDemandPtrs(demands []*rpt.ResPlanDemand) bool {
	currentYear := time.Now().Year()
	for _, demand := range demands {
		if demand != nil && containsNonCurrentYearInDemand(demand, currentYear) {
			return true
		}
	}
	return false
}

func containsNonCurrentYearInDemand(demand *rpt.ResPlanDemand, currentYear int) bool {
	if demand.Original != nil && demand.Original.ExpectTime != "" {
		year, err := strconv.Atoi(demand.Original.ExpectTime[:4])
		if err == nil && year != currentYear {
			return true
		}
	}
	if demand.Updated != nil && demand.Updated.ExpectTime != "" {
		year, err := strconv.Atoi(demand.Updated.ExpectTime[:4])
		if err == nil && year != currentYear {
			return true
		}
	}
	return false
}
