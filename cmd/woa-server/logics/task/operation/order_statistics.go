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
	"hcm/pkg"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

var (
	bizOrderMap = map[string]interface{}{pkg.BKDBSort: map[string]interface{}{
		"bk_biz_id":  pkg.BKDBAsc,
		"year_month": pkg.BKDBAsc,
	}}
)

// GetOrderTimeCostOverview aggregates order time cost by month within a range
func (op *operation) GetOrderTimeCostOverview(kt *kit.Kit, param *types.OrderTimeCostReq) (
	*types.OrderTimeCostOverviewResp, error) {

	if op.client == nil || op.client.DataService() == nil {
		logs.Errorf("data service client is not initialized, rid: %s", kt.Rid)
		return nil, errf.New(errf.Aborted, "data service client is not initialized")
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

	filterExpr, err := op.buildSuborderFilterExpression(start, end, excludeSuborderIDs)
	if err != nil {
		logs.Errorf("build suborder filter expression failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	result, err := op.client.DataService().TCloudZiyan.ZiyanCvmApplySuborder.GetOrderTimeCostOverview(kt.Ctx,
		kt.Header(), filterExpr)
	if err != nil {
		logs.Errorf("get order time cost overview from data service failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	convertedDetails := make([]*types.OrderTimeCostItem, 0, len(result))
	for _, item := range result {
		convertedDetails = append(convertedDetails, &types.OrderTimeCostItem{
			YearMonth:        item.YearMonth,
			AvgDurationHours: item.AvgDurationHours,
		})
	}

	return &types.OrderTimeCostOverviewResp{Details: convertedDetails}, nil
}

// GetOrderTimeCostCompare implements comparison aggregation per biz across two months
func (op *operation) GetOrderTimeCostCompare(kt *kit.Kit, param *types.OrderTimeCostCompareReq) (
	*types.OrderTimeCostCompareRst, error) {

	if op.client == nil || op.client.DataService() == nil {
		logs.Errorf("data service client is not initialized, rid: %s", kt.Rid)
		return nil, errf.New(errf.Aborted, "data service client is not initialized")
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

	currentItems, err := op.buildAndFetchOrderTimeCostCompare(kt, currentStart, currentEnd, currentExcludeIDs)
	if err != nil {
		logs.Errorf("build and fetch current items failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	compareItems, err := op.buildAndFetchOrderTimeCostCompare(kt, compareStart, compareEnd, compareExcludeIDs)
	if err != nil {
		logs.Errorf("build and fetch compare items failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	return &types.OrderTimeCostCompareRst{Current: currentItems, Compare: compareItems}, nil
}

// buildAndFetchOrderTimeCostCompare builds filter and fetches order time cost compare data
func (op *operation) buildAndFetchOrderTimeCostCompare(kt *kit.Kit, start, end time.Time, excludeIDs []string) (
	[]*types.OrderTimeCostCompareItem, error) {

	filterExpr, err := op.buildSuborderFilterExpression(start, end, excludeIDs)
	if err != nil {
		logs.Errorf("build suborder filter expression failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	result, err := op.client.DataService().TCloudZiyan.ZiyanCvmApplySuborder.GetOrderTimeCostCompare(kt.Ctx,
		kt.Header(), filterExpr)

	if err != nil {
		logs.Errorf("get order time cost compare from data service failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	convertedItems := make([]*types.OrderTimeCostCompareItem, 0, len(result))
	for _, item := range result {
		convertedItems = append(convertedItems, &types.OrderTimeCostCompareItem{
			BkBizID:          item.BkBizID,
			YearMonth:        item.YearMonth,
			DoneOrders:       item.DoneOrders,
			AvgDurationHours: item.AvgDurationHours,
		})
	}

	return convertedItems, nil
}
