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
	"fmt"
	"math"
	"sort"
	"time"

	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	cvmapply "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/slice"
)

// GetAverageTimeConsumptionOverview aggregates average time consumption by month within a range
func (op *operation) GetAverageTimeConsumptionOverview(kt *kit.Kit, param *types.AverageTimeConsumptionReq) (
	[]types.AverageTimeConsumptionItem, error) {

	if op.client == nil || op.client.DataService() == nil {
		return nil, errf.Newf(errf.InvalidParameter, "data service client is not initialized")
	}

	start, err := param.GetStartTime()
	if err != nil {
		logs.Errorf("parse start time failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	end, err := param.GetEndTime()
	if err != nil {
		logs.Errorf("parse end time failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// Get exclude suborder IDs
	excludeSuborderIDs, err := op.getExcludeSuborderIDs(kt, start, end)
	if err != nil {
		logs.Errorf("get exclude suborder IDs failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	rst := make([]types.AverageTimeConsumptionItem, 0)
	rst, err = op.execAverageTimeConsumptionOverviewQuery(kt, start, end, excludeSuborderIDs)
	if err != nil {
		logs.Errorf("execute average time consumption overview query failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	return rst, nil
}

// buildTimeRangeFilter 构建时间范围过滤条件
func (op *operation) buildTimeRangeFilter(start, end time.Time) *filter.Expression {
	return &filter.Expression{
		Op: filter.And,
		Rules: []filter.RuleFactory{
			tools.RuleGreaterThanEqual("created_at", start.Format(constant.TimeStdFormat)),
			tools.RuleLessThanEqual("created_at", end.Format(constant.TimeStdFormat)),
		},
	}
}

// listApplyOrders 获取指定时间范围内的主订单列表
func (op *operation) listApplyOrders(kt *kit.Kit, filterExpr *filter.Expression) (
	*cvmapplyproto.ZiyanCvmApplyOrderListResult, error) {

	allOrders := make([]*cvmapply.ZiyanCvmApplyOrder, 0)
	start := uint32(0)

	for {
		req := &cvmapplyproto.ZiyanCvmApplyOrderListReq{
			Filter: filterExpr,
			Page: &core.BasePage{
				Count: false,
				Start: start,
				Limit: core.DefaultMaxPageLimit,
			},
		}

		orders, err := op.client.DataService().TCloudZiyan.ZiyanCvmApplyOrder.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			logs.Errorf("list apply orders failed, filterExpr: %v, err: %v, rid: %s", req.Filter, err, kt.Rid)
			return nil, fmt.Errorf("list apply orders failed: %w", err)
		}

		allOrders = append(allOrders, orders.Details...)

		if len(orders.Details) < int(core.DefaultMaxPageLimit) {
			break
		}

		start += uint32(core.DefaultMaxPageLimit)
	}

	return &cvmapplyproto.ZiyanCvmApplyOrderListResult{
		Details: allOrders,
	}, nil
}

// calculateAverageDuration 计算平均耗时
func (op *operation) calculateAverageDuration(durations []float64) float64 {
	if len(durations) == 0 {
		return 0
	}

	sum := 0.0
	for _, duration := range durations {
		sum += duration
	}
	avg := sum / float64(len(durations))

	return math.Round(avg*100) / 100
}

// parseTime 解析时间字符串为 time.Time 类型
func (op *operation) parseTime(kt *kit.Kit, timeStr string, fieldName string) (time.Time, error) {
	t, err := time.Parse(constant.TimeStdFormat, timeStr)
	if err != nil {
		logs.Errorf("parse %s failed, err: %v, rid: %s", fieldName, err, kt.Rid)
		return time.Time{}, err
	}
	return t, nil
}

// buildSuborderFilterForMultipleOrders 构建多个订单的子订单过滤条件
func (op *operation) buildSuborderFilterForMultipleOrders(orderIDs []uint64,
	excludeSuborderIDs []string) *filter.Expression {

	baseRules := []filter.RuleFactory{
		tools.RuleEqual("stage", types.TicketStageDone),
		tools.RuleNotEqual("source", enumor.ApplyTicketSrcPurchaseToResPool),
	}

	batches := slice.Split(orderIDs, int(core.DefaultMaxPageLimit))
	var orderIdRules []filter.RuleFactory

	for _, batch := range batches {
		orderIdRules = append(orderIdRules, tools.RuleIn("order_id", batch))
	}

	var excludeRules []filter.RuleFactory
	if len(excludeSuborderIDs) > 0 {
		excludeBatches := slice.Split(excludeSuborderIDs, int(core.DefaultMaxPageLimit))
		for _, batch := range excludeBatches {
			excludeRules = append(excludeRules, tools.RuleNotIn("suborder_id", batch))
		}
	}

	allRules := make([]filter.RuleFactory, 0)
	allRules = append(allRules, baseRules...)

	if len(orderIdRules) > 0 {
		orderIdExpr := &filter.Expression{
			Op:    filter.Or,
			Rules: orderIdRules,
		}
		allRules = append(allRules, orderIdExpr)
	}

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

// execAverageTimeConsumptionOverviewQuery executes the average time consumption overview query
func (op *operation) execAverageTimeConsumptionOverviewQuery(kt *kit.Kit, start, end time.Time,
	excludeSuborderIDs []string) ([]types.AverageTimeConsumptionItem, error) {

	filterExpr := op.buildTimeRangeFilter(start, end)

	// 获取主订单列表
	orders, err := op.listApplyOrders(kt, filterExpr)
	if err != nil {
		logs.Errorf("list apply orders failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	orderIDs := make([]uint64, 0, len(orders.Details))
	for _, order := range orders.Details {
		if order != nil {
			orderIDs = append(orderIDs, order.OrderID)
		}
	}

	// 如果没有主订单，直接返回空结果
	if len(orderIDs) == 0 {
		return []types.AverageTimeConsumptionItem{}, nil
	}

	// 获取最后完成子订单时间
	lastTimeMap, _, err := op.getLastSuborderEndTime(kt, orderIDs, excludeSuborderIDs)
	if err != nil {
		logs.Errorf("get last suborder end time failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 按年月分组统计耗时
	monthMap := make(map[string][]float64)
	for _, order := range orders.Details {
		if order == nil {
			continue
		}

		lastTime, ok := lastTimeMap[order.OrderID]
		if !ok || lastTime.IsZero() {
			continue
		}

		createdAt, err := op.parseTime(kt, string(order.CreatedAt), "created_at")
		if err != nil {
			logs.Errorf("parse created_at failed, err: %v, rid: %s", err, kt.Rid)
			continue
		}

		duration := lastTime.Sub(createdAt).Hours()
		if duration <= 0 {
			continue
		}

		yearMonth := createdAt.Format(constant.YearMonthLayout)
		monthMap[yearMonth] = append(monthMap[yearMonth], duration)
	}

	items := make([]types.AverageTimeConsumptionItem, 0, len(monthMap))
	for yearMonth, durations := range monthMap {
		if len(durations) == 0 {
			continue
		}

		avg := op.calculateAverageDuration(durations)
		items = append(items, types.AverageTimeConsumptionItem{
			YearMonth:        yearMonth,
			AvgDurationHours: avg,
		})
	}

	// 按年月排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].YearMonth < items[j].YearMonth
	})

	return items, nil
}

// getLastSuborderEndTime 获取主订单的最后完成子订单时间
func (op *operation) getLastSuborderEndTime(kt *kit.Kit, orderIDs []uint64, excludeSuborderIDs []string) (
	map[uint64]time.Time, map[uint64]int64, error) {

	// 如果 orderIDs 为空，直接返回空 map
	if len(orderIDs) == 0 {
		return make(map[uint64]time.Time), make(map[uint64]int64), nil
	}

	lastEndTimeMap := make(map[uint64]time.Time)
	completedCountMap := make(map[uint64]int64)
	batches := slice.Split(orderIDs, int(core.DefaultMaxPageLimit))
	for _, batchOrderIDs := range batches {
		// 构建子订单过滤条件
		filterExpr := op.buildSuborderFilterForMultipleOrders(batchOrderIDs, excludeSuborderIDs)

		start := uint32(0)
		for {
			req := &cvmapplyproto.ZiyanCvmApplySuborderListReq{
				Filter: filterExpr,
				Page: &core.BasePage{
					Count: false,
					Start: start,
					Limit: core.DefaultMaxPageLimit,
				},
			}

			suborders, err := op.client.DataService().TCloudZiyan.ZiyanCvmApplySuborder.List(kt.Ctx, kt.Header(), req)
			if err != nil {
				logs.Errorf("list apply suborders failed, err: %v, rid: %s", err, kt.Rid)
				return nil, nil, err
			}

			for _, suborder := range suborders.Details {
				if suborder == nil {
					continue
				}

				updatedAt, err := op.parseTime(kt, string(suborder.UpdatedAt), "updated_at")
				if err != nil {
					logs.Errorf("parse updated_at failed, err: %v, rid: %s", err, kt.Rid)
					continue
				}

				if lastTime, ok := lastEndTimeMap[suborder.OrderID]; !ok || updatedAt.After(lastTime) {
					lastEndTimeMap[suborder.OrderID] = updatedAt
				}
				// 累加已完成子订单计数
				completedCountMap[suborder.OrderID]++
			}

			if len(suborders.Details) < int(core.DefaultMaxPageLimit) {
				break
			}

			start += uint32(core.DefaultMaxPageLimit)
		}
	}

	return lastEndTimeMap, completedCountMap, nil
}

// GetAverageTimeConsumptionCompare aggregates average time consumption compare by biz and month
func (op *operation) GetAverageTimeConsumptionCompare(kt *kit.Kit, param *types.AverageTimeConsumptionCompareReq) (
	*types.AverageTimeConsumptionCompareRst, error) {

	if op.client == nil || op.client.DataService() == nil {
		return nil, errf.Newf(errf.InvalidParameter, "data service client is not initialized")
	}

	currentStart, currentEnd, err := param.GetCurrentRange()
	if err != nil {
		logs.Errorf("parse current range failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	compareStart, compareEnd, err := param.GetCompareRange()
	if err != nil {
		logs.Errorf("parse compare range failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	current, err := op.execAverageTimeConsumptionByRangeQuery(kt, currentStart, currentEnd)
	if err != nil {
		logs.Errorf("execute current range query failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	compare, err := op.execAverageTimeConsumptionByRangeQuery(kt, compareStart, compareEnd)
	if err != nil {
		logs.Errorf("execute compare range query failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return &types.AverageTimeConsumptionCompareRst{Current: current, Compare: compare}, nil
}

// execAverageTimeConsumptionByRangeQuery executes the average time consumption by range query
func (op *operation) execAverageTimeConsumptionByRangeQuery(kt *kit.Kit, start, end time.Time) (
	[]types.AverageTimeConsumptionCompareItem, error) {

	// Get exclude suborder IDs
	excludeSuborderIDs, err := op.getExcludeSuborderIDs(kt, start, end)
	if err != nil {
		logs.Errorf("get exclude suborder IDs failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	filterExpr := op.buildTimeRangeFilter(start, end)

	orders, err := op.listApplyOrders(kt, filterExpr)
	if err != nil {
		logs.Errorf("list apply orders failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	orderIDs := make([]uint64, 0, len(orders.Details))
	for _, order := range orders.Details {
		if order != nil {
			orderIDs = append(orderIDs, order.OrderID)
		}
	}

	// 如果没有主订单，直接返回空结果
	if len(orderIDs) == 0 {
		return []types.AverageTimeConsumptionCompareItem{}, nil
	}

	lastTimeMap, completedCountMap, err := op.getLastSuborderEndTime(kt, orderIDs, excludeSuborderIDs)
	if err != nil {
		logs.Errorf("get last suborder end time failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 按业务和月份分组统计
	bizMonthDurationsMap, bizMonthCompletedCountMap := op.groupByBizAndMonthWithCount(kt, orders, lastTimeMap,
		completedCountMap)

	var items []types.AverageTimeConsumptionCompareItem
	for bkBizID, monthMap := range bizMonthDurationsMap {
		for yearMonth, durations := range monthMap {
			if len(durations) == 0 {
				continue
			}

			avg := op.calculateAverageDuration(durations)
			items = append(items, types.AverageTimeConsumptionCompareItem{
				BkBizID:          bkBizID,
				YearMonth:        yearMonth,
				DoneOrders:       bizMonthCompletedCountMap[bkBizID][yearMonth],
				AvgDurationHours: avg,
			})
		}
	}

	// 按业务 ID 和年月排序
	sort.Slice(items, func(i, j int) bool {
		if items[i].BkBizID != items[j].BkBizID {
			return items[i].BkBizID < items[j].BkBizID
		}
		return items[i].YearMonth < items[j].YearMonth
	})

	return items, nil
}

// groupByBizAndMonth 按业务和月份分组统计耗时
func (op *operation) groupByBizAndMonthWithCount(kt *kit.Kit, orders *cvmapplyproto.ZiyanCvmApplyOrderListResult,
	lastTimeMap map[uint64]time.Time, completedCountMap map[uint64]int64) (
	bizMonthDurationsMap map[int64]map[string][]float64, bizMonthCompletedCountMap map[int64]map[string]int64) {

	bizMonthDurationsMap = make(map[int64]map[string][]float64)
	bizMonthCompletedCountMap = make(map[int64]map[string]int64)

	for _, order := range orders.Details {
		if order == nil {
			continue
		}

		// 获取最后完成子订单时间
		lastTime, ok := lastTimeMap[order.OrderID]
		if !ok || lastTime.IsZero() {
			continue
		}

		createdAt, err := op.parseTime(kt, string(order.CreatedAt), "created_at")
		if err != nil {
			logs.Errorf("parse created_at failed, err: %v, rid: %s", err, kt.Rid)
			continue
		}

		duration := lastTime.Sub(createdAt).Hours()
		if duration <= 0 {
			continue
		}

		// 按业务和月份分组
		yearMonth := createdAt.Format(constant.YearMonthLayout)
		if _, ok := bizMonthDurationsMap[order.BkBizID]; !ok {
			bizMonthDurationsMap[order.BkBizID] = make(map[string][]float64)
			bizMonthCompletedCountMap[order.BkBizID] = make(map[string]int64)
		}
		bizMonthDurationsMap[order.BkBizID][yearMonth] = append(bizMonthDurationsMap[order.BkBizID][yearMonth],
			duration)
		// 累加已完成子订单数
		bizMonthCompletedCountMap[order.BkBizID][yearMonth] += completedCountMap[order.OrderID]
	}
	return
}

// buildFilterExcludedSubordersStage builds a $addFields stage to filter out excluded suborders
func buildFilterExcludedSubordersStage(excludeSuborderIDs []string) map[string]interface{} {
	return map[string]interface{}{
		pkg.BKDBAddFields: map[string]interface{}{
			"suborders": map[string]interface{}{
				"$filter": map[string]interface{}{
					"input": "$suborders",
					"as":    "suborder",
					"cond": map[string]interface{}{
						pkg.BKDBAND: []interface{}{
							// 排除手动配置的订单
							map[string]interface{}{
								pkg.BKDBNot: []interface{}{
									map[string]interface{}{
										pkg.BKDBIN: []interface{}{"$$suborder.suborder_id", excludeSuborderIDs},
									},
								},
							},
							// 排除采购到资源池的订单
							map[string]interface{}{
								pkg.BKDBNE: []interface{}{"$$suborder.source", enumor.ApplyTicketSrcPurchaseToResPool},
							},
						},
					},
				},
			},
		},
	}
}
