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

package operation

import (
	"time"

	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/slice"
)

// GetPercentileTimeConsumptionOverview aggregates percentile time consumption by month within a range
func (op *operation) GetPercentileTimeConsumptionOverview(kt *kit.Kit, startDate, endDate time.Time) (
	*cvmapplyproto.ZiyanCvmApplyPercentileTimeOverviewResult, error) {

	if op.client == nil {
		logs.Errorf("operation client is nil when getting percentile time consumption overview, rid: %s", kt.Rid)
		return nil, errf.Newf(errf.InvalidParameter, "operation client is nil, cannot access data service"+
			" for percentile time overview")
	}

	// Get exclude suborder IDs
	excludeSuborderIDs, err := op.getExcludeSuborderIDs(kt, startDate, endDate)
	if err != nil {
		logs.Errorf("get exclude suborder IDs failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	filterExpr := op.buildSuborderFilterForStatistics(startDate, endDate, excludeSuborderIDs)

	result, err := op.client.DataService().TCloudZiyan.ZiyanCvmApplySuborder.GetPercentileTimeConsumptionOverview(
		kt.Ctx, kt.Header(), filterExpr)
	if err != nil {
		logs.Errorf("get percentile time consumption overview failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return result, nil
}

// GetPercentileTimeConsumptionCompare implements comparison aggregation per biz across two months
func (op *operation) GetPercentileTimeConsumptionCompare(kt *kit.Kit, currentMonth, compareMonth string) (
	*cvmapplyproto.ZiyanCvmApplyPercentileTimeCompareResult, error) {

	if op.client == nil {
		logs.Errorf("operation client is nil when getting percentile time consumption compare, rid: %s", kt.Rid)
		return nil, errf.Newf(errf.InvalidParameter, "operation client is nil, cannot access data service "+
			"for percentile time compare")
	}

	currentStart, currentEnd, err := parseMonthRange(currentMonth)
	if err != nil {
		logs.Errorf("parse current month range failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	currentExcludeSuborderIDs, err := op.getExcludeSuborderIDs(kt, currentStart, currentEnd)
	if err != nil {
		logs.Errorf("get current exclude suborder IDs failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	currentFilter := op.buildSuborderFilterForStatistics(currentStart, currentEnd, currentExcludeSuborderIDs)
	compareStart, compareEnd, err := parseMonthRange(compareMonth)
	if err != nil {
		logs.Errorf("parse compare month range failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	compareExcludeSuborderIDs, err := op.getExcludeSuborderIDs(kt, compareStart, compareEnd)
	if err != nil {
		logs.Errorf("get compare exclude suborder IDs failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	compareFilter := op.buildSuborderFilterForStatistics(compareStart, compareEnd, compareExcludeSuborderIDs)

	currentResult, err := op.client.DataService().TCloudZiyan.ZiyanCvmApplySuborder.GetPercentileTimeConsumptionCompare(
		kt.Ctx, kt.Header(), currentFilter)
	if err != nil {
		logs.Errorf("get percentile time consumption compare failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	compareResult, err := op.client.DataService().TCloudZiyan.ZiyanCvmApplySuborder.GetPercentileTimeConsumptionCompare(
		kt.Ctx, kt.Header(), compareFilter)
	if err != nil {
		logs.Errorf("get percentile time consumption compare failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	rst := &cvmapplyproto.ZiyanCvmApplyPercentileTimeCompareResult{}
	if currentResult != nil {
		rst.Current = currentResult.Current
	}
	if compareResult != nil {
		rst.Compare = compareResult.Current
	}

	return rst, nil
}

// parseMonthRange parses month string (e.g., "2026-01") to start and end time
func parseMonthRange(month string) (time.Time, time.Time, error) {
	start, err := time.Parse(constant.YearMonthLayout, month)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	end := start.AddDate(0, 1, 0)
	return start, end, nil
}

// buildSuborderFilterForStatistics 构建统计用的子订单过滤条件
func (op *operation) buildSuborderFilterForStatistics(start, end time.Time,
	excludeSuborderIDs []string) *filter.Expression {

	rules := []filter.RuleFactory{
		tools.RuleGreaterThanEqual("created_at", start.Format(constant.TimeStdFormat)),
		tools.RuleLessThan("created_at", end.Format(constant.TimeStdFormat)),
		tools.RuleEqual("stage", types.TicketStageDone),
		tools.RuleEqual("status", types.ApplyStatusDone),
		tools.RuleNotEqual("source", enumor.ApplyTicketSrcPurchaseToResPool),
	}

	var excludeRules []filter.RuleFactory
	if len(excludeSuborderIDs) > 0 {
		excludeBatches := slice.Split(excludeSuborderIDs, int(core.DefaultMaxPageLimit))
		for _, batch := range excludeBatches {
			excludeRules = append(excludeRules, tools.RuleNotIn("suborder_id", batch))
		}
	}

	allRules := make([]filter.RuleFactory, 0)
	allRules = append(allRules, rules...)

	if len(excludeRules) > 0 {
		excludeExpr := &filter.Expression{
			Op:    filter.And,
			Rules: excludeRules,
		}
		allRules = append(allRules, excludeExpr)
	}

	return &filter.Expression{
		Op:    filter.And,
		Rules: allRules,
	}
}
