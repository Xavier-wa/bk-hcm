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

// Package operation define the operation interface
package operation

import (
	"context"
	"fmt"
	"sort"
	"time"

	"hcm/cmd/woa-server/logics/task/statistics"
	model "hcm/cmd/woa-server/model/task"
	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/mapstr"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/language"
	"hcm/pkg/tools/metadata"
	"hcm/pkg/tools/slice"
	"hcm/pkg/tools/util"
)

// Interface operation interface
type Interface interface {
	// GetApplyStatistics get resource apply operation statistics
	GetApplyStatistics(kt *kit.Kit, param *types.GetApplyStatReq) (*types.GetApplyStatRst, error)
	// GetAverageTimeConsumptionOverview get average time consumption overview
	GetAverageTimeConsumptionOverview(kt *kit.Kit, param *types.AverageTimeConsumptionReq) (
		[]types.AverageTimeConsumptionItem, error)
	// GetAverageTimeConsumptionCompare get average time consumption compare
	GetAverageTimeConsumptionCompare(kt *kit.Kit, param *types.AverageTimeConsumptionCompareReq) (
		*types.AverageTimeConsumptionCompareRst, error)
	// GetOrderTimeCostOverview get order time cost overview
	GetOrderTimeCostOverview(kt *kit.Kit, param *types.OrderTimeCostReq) (*types.OrderTimeCostOverviewResp, error)
	// GetOrderTimeCostCompare get order time cost compare
	GetOrderTimeCostCompare(kt *kit.Kit, param *types.OrderTimeCostCompareReq) (*types.OrderTimeCostCompareRst, error)
	// GetProductionStageTimeCostOverview get production stage time cost overview
	GetProductionStageTimeCostOverview(kt *kit.Kit, param *types.ProductionStageTimeCostReq) (
		*cvmapplyproto.ProductionStageTimeCostOverviewResult, error)
	// GetProductionStageTimeCostCompare get production stage time cost compare
	GetProductionStageTimeCostCompare(kt *kit.Kit, param *types.ProductionStageTimeCostCompareReq) (
		*cvmapplyproto.ProductionStageTimeCostCompareResult, error)
	// GetPercentileTimeConsumptionOverview get percentile time consumption overview
	GetPercentileTimeConsumptionOverview(kt *kit.Kit, startDate, endDate time.Time) (
		*cvmapplyproto.ZiyanCvmApplyPercentileTimeOverviewResult, error)
	// GetPercentileTimeConsumptionCompare get percentile time consumption compare
	GetPercentileTimeConsumptionCompare(kt *kit.Kit, currentStart, compareStart string) (
		*cvmapplyproto.ZiyanCvmApplyPercentileTimeCompareResult, error)
	// GetDeliveryRateStatistics get delivery rate statistics
	GetDeliveryRateStatistics(kt *kit.Kit, param *types.DeliveryRateStatisticsReq) (
		[]types.DeliveryRateStatisticsItem, error)
	// GetDeliveryRateDetail get delivery rate detail
	GetDeliveryRateDetail(kt *kit.Kit, param *types.DeliveryRateDetailReq) (*types.DeliveryRateDetailResp, error)
	// GetCompletionRateStatistics get completion rate statistics
	GetCompletionRateStatistics(kt *kit.Kit,
		param *types.GetCompletionRateStatReq) (*types.GetCompletionRateStatRst, error)
	// GetCompletionRateDetail 获取结单率详情统计
	GetCompletionRateDetail(kt *kit.Kit,
		param *types.GetCompletionRateDetailReq) (*types.GetCompletionRateDetailRst, error)
	// GetApplyBizHostsStatistics get apply biz hosts statistics
	GetApplyBizHostsStatistics(kt *kit.Kit, startDate, endDate time.Time) (
		*cvmapplyproto.ZiyanCvmApplyBizHostsStatisticsResult, error)
	// GetApplyBizCpuCoresStatistics get apply biz cpu cores statistics
	GetApplyBizCpuCoresStatistics(kt *kit.Kit, startDate, endDate time.Time) (
		*cvmapplyproto.ZiyanCvmApplyBizCpuCoresStatisticsResult, error)
}

// operation provides operation statistics service
type operation struct {
	lang       language.CCLanguageIf
	statistics statistics.Interface
	client     *client.ClientSet
}

// New create a operation instance
func New(_ context.Context, clientSet *client.ClientSet) (*operation, error) {
	op := &operation{
		lang:   language.NewFromCtx(language.EmptyLanguageSetting),
		client: clientSet,
	}

	if clientSet != nil {
		op.statistics = statistics.New(clientSet)
	}

	return op, nil
}

// GetApplyStatistics get resource apply operation statistics
func (op *operation) GetApplyStatistics(kt *kit.Kit, param *types.GetApplyStatReq) (*types.GetApplyStatRst, error) {
	filter, err := param.GetFilter()
	if err != nil {
		logs.Errorf("failed to get resource apply operation statistics, for get filter err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	orderTotalStats, err := op.getOrderStats(filter, param.Dimension)
	if err != nil {
		logs.Errorf("failed to get resource apply total order statistics, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	succOrderFilter := util.CopyMap(filter, nil, nil)
	succOrderFilter["status"] = types.ApplyStatusDone
	orderSuccStats, err := op.getOrderStats(succOrderFilter, param.Dimension)
	if err != nil {
		logs.Errorf("failed to get resource apply success order statistics, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	osTotalStats, err := op.getDeviceStats(filter, param.Dimension)
	if err != nil {
		logs.Errorf("failed to get resource apply total os statistics, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	manualOrderList, err := op.getManualOrderList(filter)
	if err != nil {
		logs.Errorf("failed to get manual order list, err: %v", err)
		return nil, err
	}
	manualOrderFilter := util.CopyMap(filter, nil, nil)
	manualOrderFilter["suborder_id"] = mapstr.MapStr{
		pkg.BKDBIN: manualOrderList,
	}
	orderManualStats, err := op.getOrderStats(manualOrderFilter, param.Dimension)
	if err != nil {
		logs.Errorf("failed to get resource apply manual order statistics, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	succOsFilter := util.CopyMap(filter, nil, nil)
	succOsFilter["is_delivered"] = true
	succOsFilter["deliverer"] = "icr"
	osSuccStats, err := op.getDeviceStats(succOsFilter, param.Dimension)
	if err != nil {
		logs.Errorf("failed to get resource apply success os statistics, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	rst := getApplyStatRst(orderTotalStats, orderSuccStats, orderManualStats, osTotalStats, osSuccStats)
	return rst, nil
}

func getApplyStatRst(orderTotalStats map[string]metadata.StringIDCount,
	orderSuccStats map[string]metadata.StringIDCount, orderManualStats map[string]metadata.StringIDCount,
	osTotalStats map[string]metadata.StringIDCount,
	osSuccStats map[string]metadata.StringIDCount) *types.GetApplyStatRst {

	// sort date keys
	dateKeys := make([]string, 0)
	for k := range orderTotalStats {
		dateKeys = append(dateKeys, k)
	}
	sort.Strings(dateKeys)

	rst := new(types.GetApplyStatRst)
	for _, date := range dateKeys {
		orderTotalCnt := uint(0)
		if orderTotalStat, ok := orderTotalStats[date]; ok {
			orderTotalCnt = uint(orderTotalStat.Count)
		}

		orderSuccCnt := uint(0)
		if orderSuccStat, ok := orderSuccStats[date]; ok {
			orderSuccCnt = uint(orderSuccStat.Count)
		}

		orderSuccRate := float64(0)
		if orderTotalCnt > 0 {
			orderSuccRate = float64(orderSuccCnt) / float64(orderTotalCnt)
		}

		orderManualCnt := uint(0)
		if orderManualStat, ok := orderManualStats[date]; ok {
			orderManualCnt = uint(orderManualStat.Count)
		}

		orderManualRate := float64(0)
		if orderManualCnt > 0 {
			orderManualRate = float64(orderManualCnt) / float64(orderTotalCnt)
		}

		osTotalCnt := uint(0)
		if osTotalStat, ok := osTotalStats[date]; ok {
			osTotalCnt = uint(osTotalStat.Count)
		}

		osSuccCnt := uint(0)
		if osSuccStat, ok := osSuccStats[date]; ok {
			osSuccCnt = uint(osSuccStat.Count)
		}

		osSuccRate := float64(0)
		if osTotalCnt > 0 {
			osSuccRate = float64(osSuccCnt) / float64(osTotalCnt)
		}

		applyStat := &types.ApplyStat{
			Date:            date,
			OrderTotal:      orderTotalCnt,
			OrderSucc:       orderSuccCnt,
			OrderSuccRate:   orderSuccRate,
			OrderManual:     orderManualCnt,
			OrderManualRate: orderManualRate,
			OsTotal:         osTotalCnt,
			OsSucc:          osSuccCnt,
			OsSuccRate:      osSuccRate,
		}
		rst.Info = append(rst.Info, applyStat)
	}
	return rst
}

// getOrderStats get resource apply order operation statistics
func (op *operation) getOrderStats(filter map[string]interface{}, dimension types.TimeDimension) (
	map[string]metadata.StringIDCount, error) {

	format := op.getDateFormat(dimension)
	pipeline := []map[string]interface{}{
		{pkg.BKDBMatch: filter},
		{pkg.BKDBGroup: map[string]interface{}{
			"_id": map[string]interface{}{
				"$dateToString": map[string]interface{}{
					"format": format,
					"date":   "$create_at"}},
			"count": map[string]interface{}{pkg.BKDBSum: 1}},
		},
		{pkg.BKDBSort: map[string]interface{}{"_id": 1}},
	}

	aggRst := make([]metadata.StringIDCount, 0)
	if err := model.Operation().ApplyOrder().AggregateAll(context.Background(), pipeline, &aggRst); err != nil {
		logs.Errorf("failed to get resource apply order operation statistics, err: %v", err)
		return nil, err
	}

	mapDateStat := make(map[string]metadata.StringIDCount)
	for _, stat := range aggRst {
		mapDateStat[stat.ID] = stat
	}

	return mapDateStat, nil
}

// getDeviceStats get resource apply delivered device operation statistics
func (op *operation) getDeviceStats(filter map[string]interface{}, dimension types.TimeDimension) (
	map[string]metadata.StringIDCount, error) {

	format := op.getDateFormat(dimension)
	pipeline := []map[string]interface{}{
		{pkg.BKDBMatch: filter},
		{pkg.BKDBGroup: map[string]interface{}{
			"_id": map[string]interface{}{
				"$dateToString": map[string]interface{}{
					"format": format,
					"date":   "$create_at"}},
			"count": map[string]interface{}{pkg.BKDBSum: 1}},
		},
		{pkg.BKDBSort: map[string]interface{}{"_id": 1}},
	}

	aggRst := make([]metadata.StringIDCount, 0)
	if err := model.Operation().DeviceInfo().AggregateAll(context.Background(), pipeline, &aggRst); err != nil {
		logs.Errorf("failed to get resource apply delivered device operation statistics, err: %v", err)
		return nil, err
	}

	mapDateStat := make(map[string]metadata.StringIDCount)
	for _, stat := range aggRst {
		mapDateStat[stat.ID] = stat
	}

	return mapDateStat, nil
}

func (op *operation) getManualOrderList(filter map[string]interface{}) ([]interface{}, error) {
	manualFilter := util.CopyMap(filter, nil, nil)
	manualFilter["deliverer"] = mapstr.MapStr{
		pkg.BKDBNE: "icr",
	}

	orderList, err := model.Operation().DeviceInfo().Distinct(context.Background(), "suborder_id", manualFilter)
	if err != nil {
		return nil, err
	}

	return orderList, nil
}

func (op *operation) getDateFormat(dimension types.TimeDimension) string {
	format := ""
	switch dimension {
	case types.DimensionDay:
		format = "%Y-%m-%d"
	case types.DimensionMonth:
		format = "%Y-%m"
	case types.DimensionYear:
		format = "%Y"
	default:
		// treat dimension as day by default
		format = "%Y-%m-%d"
	}

	return format
}

// parseTimeRange 解析时间范围
func parseTimeRange(startTimeStr, endTimeStr string) (time.Time, time.Time, error) {
	startTime, err := time.Parse(constant.DateLayout, startTimeStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to parse start_time: %w", err)
	}

	endTime, err := time.Parse(constant.DateLayout, endTimeStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to parse end_time: %w", err)
	}

	return startTime, endTime, nil
}

// getExcludeSuborderIDs 获取排除的子单号列表
func (op *operation) getExcludeSuborderIDs(kt *kit.Kit, startTime, endTime time.Time) ([]string, error) {
	if op.statistics == nil {
		return nil, nil
	}

	excludeSuborderIDs, err := op.statistics.ListExcludedSubOrderIDs(kt, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get exclude suborder ids: %w", err)
	}

	return excludeSuborderIDs, nil
}

// convertCompletionRateStatisticsResult 转换 MySQL 统计结果
func convertCompletionRateStatisticsResult(result *cvmapplyproto.ZiyanCvmApplyCompletionRateStatisticsResult,
) *types.GetCompletionRateStatRst {

	rst := &types.GetCompletionRateStatRst{
		Details: make([]*types.CompletionRateStat, 0),
	}
	if result == nil || len(result.Details) == 0 {
		return rst
	}

	for _, stat := range result.Details {
		if stat == nil {
			continue
		}
		rst.Details = append(rst.Details, &types.CompletionRateStat{
			YearMonth:      stat.YearMonth,
			CompletionRate: stat.CompletionRate,
		})
	}

	return rst
}

// GetCompletionRateStatistics get completion rate statistics
func (op *operation) GetCompletionRateStatistics(kt *kit.Kit,
	param *types.GetCompletionRateStatReq) (*types.GetCompletionRateStatRst, error) {
	startTime, endTime, err := parseTimeRange(param.StartTime, param.EndTime)
	if err != nil {
		logs.Errorf("failed to parse time range, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	excludeSuborderIDs, err := op.getExcludeSuborderIDs(kt, startTime, endTime)
	if err != nil {
		logs.Errorf("failed to get exclude suborder ids for completion rate statistics, err: %v, rid: %s",
			err, kt.Rid)
		return nil, err
	}

	// 结束时间需要加1天
	endTime = endTime.AddDate(0, 0, 1)

	filterExpr, err := buildCompletionRateFilterExpression(startTime, endTime, excludeSuborderIDs)
	if err != nil {
		logs.Errorf("failed to build completion rate filter expression, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	result, err := op.statistics.GetCompletionRateStatistics(kt, filterExpr)
	if err != nil {
		logs.Errorf("failed to get completion rate statistics, startTime: %s, endTime: %s, err: %v, rid: %s",
			startTime, endTime, err, kt.Rid)
		return nil, err
	}

	return convertCompletionRateStatisticsResult(result), nil
}

// buildCompletionRateFilterExpression 构建结单率统计过滤条件（MySQL）
func buildCompletionRateFilterExpression(startTime, endTime time.Time, excludeSuborderIDs []string,
) (*filter.Expression, error) {

	rules := []filter.RuleFactory{
		tools.RuleGreaterThanEqual("created_at", startTime.Format(constant.TimeStdFormat)),
		tools.RuleLessThan("created_at", endTime.Format(constant.TimeStdFormat)),
		tools.RuleNotEqual("source", enumor.ApplyTicketSrcPurchaseToResPool),
	}

	if len(excludeSuborderIDs) > 0 {
		appendChunkedNotInRules(&rules, "suborder_id", excludeSuborderIDs)
	}

	return &filter.Expression{
		Op:    filter.And,
		Rules: rules,
	}, nil
}

func appendChunkedNotInRules(rules *[]filter.RuleFactory, field string, values []string) {
	if len(values) == 0 {
		return
	}

	chunkSize := int(filter.DefaultMaxInLimit)
	for i := 0; i < len(values); i += chunkSize {
		end := i + chunkSize
		if end > len(values) {
			end = len(values)
		}
		*rules = append(*rules, tools.RuleNotIn(field, values[i:end]))
	}
}

// GetCompletionRateDetail 获取结单率详情统计
func (op *operation) GetCompletionRateDetail(kt *kit.Kit,
	param *types.GetCompletionRateDetailReq) (*types.GetCompletionRateDetailRst, error) {
	startTime, endTime, err := parseTimeRange(param.StartTime, param.EndTime)
	if err != nil {
		logs.Errorf("failed to parse time range, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 结束时间需要加1天，因为查询条件是 $lt（小于）不包含当天
	endTime = endTime.AddDate(0, 0, 1)

	excludeSuborderIDs, err := op.getExcludeSuborderIDs(kt, startTime, endTime)
	if err != nil {
		logs.Errorf("failed to get exclude suborder ids for completion rate detail, err: %v, rid: %s",
			err, kt.Rid)
		return nil, err
	}

	filterExpr, err := buildCompletionRateFilterExpression(startTime, endTime, excludeSuborderIDs)
	if err != nil {
		logs.Errorf("failed to build completion rate detail filter expression, err: %v, rid: %s",
			err, kt.Rid)
		return nil, err
	}

	result, err := op.statistics.GetCompletionRateDetailStatistics(kt, filterExpr)
	if err != nil {
		logs.Errorf("failed to get completion rate detail statistics, startTime: %s, endTime: %s, "+
			"err: %v, rid: %s", startTime, endTime, err, kt.Rid)
		return nil, err
	}

	return convertCompletionRateDetailResult(result), nil
}

// convertCompletionRateDetailResult 转换 MySQL 结单率详情结果
func convertCompletionRateDetailResult(
	result *cvmapplyproto.ZiyanCvmApplyCompletionRateDetailResult,
) *types.GetCompletionRateDetailRst {
	rst := &types.GetCompletionRateDetailRst{
		Details: make([]*types.CompletionRateDetailItem, 0),
	}
	if result == nil || len(result.Details) == 0 {
		return rst
	}

	for _, stat := range result.Details {
		if stat == nil {
			continue
		}
		rst.Details = append(rst.Details, &types.CompletionRateDetailItem{
			BkBizID:        stat.BkBizID,
			YearMonth:      stat.YearMonth,
			TotalOrders:    stat.TotalOrders,
			DoneOrders:     stat.DoneOrders,
			CompletionRate: stat.CompletionRate,
		})
	}

	return rst
}

// GetApplyBizHostsStatistics 按日期范围统计业务维度的申请主机数据
// startDate: 开始日期字符串，格式：2025-11-01
// endDate: 结束日期字符串，格式：2025-11-30
func (op *operation) GetApplyBizHostsStatistics(kt *kit.Kit, startDate, endDate time.Time) (
	*cvmapplyproto.ZiyanCvmApplyBizHostsStatisticsResult, error) {

	// 获取需要排除的suborder_id列表（例如：手动处理的订单）
	excludeSuborderIDs, err := op.getExcludeSuborderIDs(kt, startDate, endDate)
	if err != nil {
		logs.Errorf("failed to get exclude suborder ids, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if len(excludeSuborderIDs) > 0 {
		logs.Infof("query biz host statistics exclude [%d] suborder_ids, rid: %s", len(excludeSuborderIDs), kt.Rid)
	}

	// 构建过滤条件
	filterExpr, err := op.buildSuborderFilterExpression(startDate, endDate, excludeSuborderIDs)
	if err != nil {
		logs.Errorf("failed to build filter expression, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 调用 DAO 方法进行统计查询
	result, err := op.statistics.GetApplyBizHostsStatistics(kt, filterExpr)
	if err != nil {
		logs.Errorf("failed to get apply biz hosts statistics, startDate: %s, endDate: %s, "+
			"err: %v, rid: %s", startDate, endDate, err, kt.Rid)
		return nil, err
	}

	return result, nil
}

// GetApplyBizCpuCoresStatistics 按日期范围统计业务维度的申请核心数数据
// startDate: 开始日期字符串，格式：2025-11-01
// endDate: 结束日期字符串，格式：2025-11-30
func (op *operation) GetApplyBizCpuCoresStatistics(kt *kit.Kit, startDate, endDate time.Time) (
	*cvmapplyproto.ZiyanCvmApplyBizCpuCoresStatisticsResult, error) {

	// 获取需要排除的suborder_id列表（例如：手动处理的订单）
	excludeSuborderIDs, err := op.getExcludeSuborderIDs(kt, startDate, endDate)
	if err != nil {
		logs.Errorf("failed to get exclude suborder ids, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if len(excludeSuborderIDs) > 0 {
		logs.Infof("query biz cpu cores statistics exclude [%d] suborder_ids, rid: %s", len(excludeSuborderIDs), kt.Rid)
	}

	// 构建过滤条件
	filterExpr, err := op.buildSuborderFilterExpression(startDate, endDate, excludeSuborderIDs)
	if err != nil {
		logs.Errorf("failed to build filter expression, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 调用 DAO 方法进行统计查询
	result, err := op.statistics.GetApplyBizCpuCoresStatistics(kt, filterExpr)
	if err != nil {
		logs.Errorf("failed to get apply biz cpu cores statistics, startDate: %s, endDate: %s, "+
			"err: %v, rid: %s", startDate, endDate, err, kt.Rid)
		return nil, err
	}

	return result, nil
}

// buildSuborderFilterExpression 构建子单查询的过滤表达式
func (op *operation) buildSuborderFilterExpression(startDate, endDate time.Time,
	excludeSuborderIDs []string) (*filter.Expression, error) {
	baseRules := []filter.RuleFactory{
		tools.RuleGreaterThanEqual("created_at", startDate.Format(constant.TimeStdFormat)),
		tools.RuleLessThanEqual("created_at", endDate.Format(constant.TimeStdFormat)),
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
	allRules = append(allRules, baseRules...)

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
	}, nil
}
