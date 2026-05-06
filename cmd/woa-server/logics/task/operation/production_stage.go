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

// GetProductionStageTimeCostOverview aggregates production stage time cost by month within a range
func (op *operation) GetProductionStageTimeCostOverview(kt *kit.Kit, param *types.ProductionStageTimeCostReq) (
	*cvmapplyproto.ProductionStageTimeCostOverviewResult, error) {

	if op.client == nil || op.client.DataService() == nil {
		logs.Errorf("data service client is not initialized, rid: %s", kt.Rid)
		return nil, errf.New(errf.InvalidParameter, "data service client is not initialized")
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

	rules := []filter.RuleFactory{
		tools.RuleGreaterThanEqual("g.created_at", start.Format(constant.TimeStdFormat)),
		tools.RuleLessThan("g.created_at", end.Format(constant.TimeStdFormat)),
		tools.RuleEqual("g.status", types.GenerateStatusSuccess),
		tools.RuleNotEqual("s.source", enumor.ApplyTicketSrcPurchaseToResPool),
	}
	var excludeRules []filter.RuleFactory
	if len(excludeSuborderIDs) > 0 {
		excludeBatches := slice.Split(excludeSuborderIDs, int(core.DefaultMaxPageLimit))
		for _, batch := range excludeBatches {
			excludeRules = append(excludeRules, tools.RuleNotIn("s.suborder_id", batch))
		}
	}
	filterExpr := &filter.Expression{
		Op:    filter.And,
		Rules: append(rules, excludeRules...),
	}

	resp, err := op.client.DataService().TCloudZiyan.ZiyanCvmApplySuborder.GetProductionStageTimeCostOverview(kt.Ctx,
		kt.Header(), filterExpr)
	if err != nil {
		logs.Errorf("query production stage time cost overview failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return resp, nil
}

// GetProductionStageTimeCostCompare implements comparison aggregation per biz across two months
func (op *operation) GetProductionStageTimeCostCompare(kt *kit.Kit, param *types.ProductionStageTimeCostCompareReq) (
	*cvmapplyproto.ProductionStageTimeCostCompareResult, error) {

	if op.client == nil || op.client.DataService() == nil {
		logs.Errorf("data service client is not initialized, rid: %s", kt.Rid)
		return nil, errf.New(errf.InvalidParameter, "data service client is not initialized")
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

	currentExcludeIDs, err := op.getExcludeSuborderIDs(kt, currentStart, currentEnd)
	if err != nil {
		logs.Errorf("get current exclude suborder IDs failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	compareExcludeIDs, err := op.getExcludeSuborderIDs(kt, compareStart, compareEnd)
	if err != nil {
		logs.Errorf("get compare exclude suborder IDs failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	buildAndFetch := func(start, end time.Time, excludeIDs []string) (
		[]cvmapplyproto.ProductionStageTimeCostBizItem, error) {

		rules := []filter.RuleFactory{
			tools.RuleGreaterThanEqual("g.created_at", start.Format(constant.TimeStdFormat)),
			tools.RuleLessThan("g.created_at", end.Format(constant.TimeStdFormat)),
			tools.RuleEqual("g.status", types.GenerateStatusSuccess),
			tools.RuleNotEqual("s.source", enumor.ApplyTicketSrcPurchaseToResPool),
		}
		var excludeRules []filter.RuleFactory
		if len(excludeIDs) > 0 {
			excludeBatches := slice.Split(excludeIDs, int(core.DefaultMaxPageLimit))
			for _, batch := range excludeBatches {
				excludeRules = append(excludeRules, tools.RuleNotIn("s.suborder_id", batch))
			}
		}
		filterExpr := &filter.Expression{
			Op:    filter.And,
			Rules: append(rules, excludeRules...),
		}

		resp, err := op.client.DataService().TCloudZiyan.ZiyanCvmApplySuborder.GetProductionStageTimeCostCompare(kt.Ctx,
			kt.Header(), filterExpr)
		if err != nil {
			logs.Errorf("query production stage time cost compare failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		return resp, nil
	}

	currentItems, err := buildAndFetch(currentStart, currentEnd, currentExcludeIDs)
	if err != nil {
		logs.Errorf("build and fetch current items failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	compareItems, err := buildAndFetch(compareStart, compareEnd, compareExcludeIDs)
	if err != nil {
		logs.Errorf("build and fetch compare items failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	currentBizItems := convertToProductionStageTimeCostBizItems(currentItems)
	compareBizItems := convertToProductionStageTimeCostBizItems(compareItems)

	return &cvmapplyproto.ProductionStageTimeCostCompareResult{
		Current: currentBizItems,
		Compare: compareBizItems,
	}, nil
}

// convertToProductionStageTimeCostBizItems converts []cvmapplyproto.ProductionStageTimeCostBizItem to the same type
func convertToProductionStageTimeCostBizItems(items []cvmapplyproto.ProductionStageTimeCostBizItem,
) []cvmapplyproto.ProductionStageTimeCostBizItem {

	result := make([]cvmapplyproto.ProductionStageTimeCostBizItem, len(items))
	for i, item := range items {
		result[i] = cvmapplyproto.ProductionStageTimeCostBizItem{
			BkBizID:          item.BkBizID,
			YearMonth:        item.YearMonth,
			DoneOrders:       item.DoneOrders,
			AvgDurationHours: item.AvgDurationHours,
		}
	}
	return result
}
