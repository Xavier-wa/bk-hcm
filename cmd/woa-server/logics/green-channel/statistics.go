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

// Package greenchannel ...
package greenchannel

import (
	"fmt"
	"sort"
	"strings"
	"time"

	model "hcm/cmd/woa-server/model/task"
	gctypes "hcm/cmd/woa-server/types/green-channel"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/times"
)

// buildSuborderFilter 构建子单过滤条件：created_at在日期范围内，且require_type为小额绿通。
func buildSuborderFilter(dateRange gctypes.DateRange, bkBizIDs []int64) (*filter.Expression, error) {
	rules := []filter.RuleFactory{
		&filter.AtomRule{
			Field: "created_at",
			Op:    filter.GreaterThanEqual.Factory(),
			Value: dateRange.Start.GetTime().Format(constant.TimeStdFormat),
		},
		&filter.AtomRule{
			Field: "created_at",
			Op:    filter.LessThanEqual.Factory(),
			Value: dateRange.End.GetTime().Format(constant.TimeStdFormat),
		},
		tools.RuleEqual("require_type", enumor.RequireTypeGreenChannel),
	}

	if len(bkBizIDs) != 0 {
		rules = append(rules, tools.RuleIn("bk_biz_id", bkBizIDs))
	}

	return tools.And(rules...)
}

// GetCpuCoreSummary get cpu core summary.
func (l *logics) GetCpuCoreSummary(kt *kit.Kit, req *gctypes.CpuCoreSummaryReq) (*gctypes.CpuCoreSummaryResp, error) {
	if err := req.Validate(); err != nil {
		logs.Errorf("failed to validate cpu core summary request, err: %v, req: %+v, rid: %s", err, *req, kt.Rid)
		return nil, err
	}

	filterExpr, err := buildSuborderFilter(req.DateRange, req.BkBizIDs)
	if err != nil {
		logs.Errorf("failed to build suborder filter, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	items, err := model.Operation().ApplyOrder().FindManyApplyOrder(kt, filterExpr, nil)
	if err != nil {
		logs.Errorf("failed to list suborders for cpu core summary, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 汇总所有子单的 delivered_core
	var sumDeliveredCore uint64
	for _, item := range items {
		sumDeliveredCore += uint64(item.DeliveredCore)
	}

	return &gctypes.CpuCoreSummaryResp{SumDeliveredCore: sumDeliveredCore}, nil
}

// ListStatisticalRecord list statistical record.
func (l *logics) ListStatisticalRecord(kt *kit.Kit, req *gctypes.StatisticalRecordReq) (*gctypes.StatisticalRecordResp,
	error) {

	if err := req.Validate(); err != nil {
		logs.Errorf("failed to validate request, err: %v, req: %+v, rid: %s", err, *req, kt.Rid)
		return nil, err
	}

	filterExpr, err := buildSuborderFilter(req.DateRange, req.BkBizIDs)
	if err != nil {
		logs.Errorf("failed to build suborder filter, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	items, err := model.Operation().ApplyOrder().FindManyApplyOrder(kt, filterExpr, nil)
	if err != nil {
		logs.Errorf("failed to list suborders for statistical record, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 按 bk_biz_id 聚合
	type bizAgg struct {
		sumDeliveredCore uint64
		sumAppliedCore   uint64
		uniqueOrderIDs   map[uint64]struct{}
	}
	bizMap := make(map[int64]*bizAgg)
	for _, item := range items {
		agg, ok := bizMap[item.BkBizId]
		if !ok {
			agg = &bizAgg{uniqueOrderIDs: make(map[uint64]struct{})}
			bizMap[item.BkBizId] = agg
		}
		agg.sumDeliveredCore += uint64(item.DeliveredCore)
		agg.sumAppliedCore += uint64(item.AppliedCore)
		agg.uniqueOrderIDs[item.OrderId] = struct{}{}
	}

	// 如果是 count 请求，返回去重后的 biz 数量
	if req.Page.Count {
		return &gctypes.StatisticalRecordResp{Count: uint64(len(bizMap))}, nil
	}

	// 转换为结果列表
	details := make([]gctypes.StatisticalRecordItem, 0, len(bizMap))
	for bizID, agg := range bizMap {
		details = append(details, gctypes.StatisticalRecordItem{
			BizID:            bizID,
			OrderCount:       uint64(len(agg.uniqueOrderIDs)),
			SumDeliveredCore: agg.sumDeliveredCore,
			SumAppliedCore:   agg.sumAppliedCore,
		})
	}

	// 排序
	if req.Page.Sort != "" {
		sortFields := strings.Split(req.Page.Sort, ",")
		sort.Slice(details, func(i, j int) bool {
			for _, field := range sortFields {
				switch field {
				case "order_count":
					if details[i].OrderCount != details[j].OrderCount {
						return details[i].OrderCount < details[j].OrderCount
					}
				case "sum_applied_core":
					if details[i].SumAppliedCore != details[j].SumAppliedCore {
						return details[i].SumAppliedCore < details[j].SumAppliedCore
					}
				case "sum_delivered_core":
					if details[i].SumDeliveredCore != details[j].SumDeliveredCore {
						return details[i].SumDeliveredCore < details[j].SumDeliveredCore
					}
				}
			}
			return false
		})
	}

	// 分页
	start := int(req.Page.Start)
	limit := int(req.Page.Limit)
	if start >= len(details) {
		return &gctypes.StatisticalRecordResp{Details: []gctypes.StatisticalRecordItem{}}, nil
	}
	end := start + limit
	if end > len(details) {
		end = len(details)
	}

	return &gctypes.StatisticalRecordResp{Details: details[start:end]}, nil
}

// CanApplyHost check if can apply host
func (l *logics) CanApplyHost(kt *kit.Kit, bizID int64, appliedCount uint) (bool, string, error) {
	now := time.Now()
	monday := times.GetMondayOfWeek(now)

	summaryReq := &gctypes.CpuCoreSummaryReq{
		DateRange: gctypes.DateRange{
			Start: times.DateTimeItem{Year: monday.Year(), Month: int(monday.Month()), Day: monday.Day()},
			End:   times.DateTimeItem{Year: now.Year(), Month: int(now.Month()), Day: now.Day()},
		},
		BkBizIDs: []int64{bizID},
	}
	summary, err := l.GetCpuCoreSummary(kt, summaryReq)
	if err != nil {
		logs.Errorf("failed to get cpu core summary, err: %v, req: %v, rid: %s", err, summaryReq, kt.Rid)
		return false, "", err
	}

	config, err := l.GetConfigs(kt)
	if err != nil {
		logs.Errorf("failed to get green channel configs, err: %v, rid: %s", err, kt.Rid)
		return false, "", err
	}

	if uint(summary.SumDeliveredCore)+appliedCount > uint(config.BizQuota) {
		return false, fmt.Sprintf("业务(%d)本周小额绿通已交付%d核心，本次申请%d核心，超过本周总额度限制:%d核心", bizID,
			summary.SumDeliveredCore, appliedCount, config.BizQuota), nil
	}

	return true, "", nil
}
