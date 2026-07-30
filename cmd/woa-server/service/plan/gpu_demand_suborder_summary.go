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

package plan

import (
	"fmt"
	"sort"

	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/api/core"
	rpproto "hcm/pkg/api/data-service/resource-plan"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	gpusuborder "hcm/pkg/dal/table/resource-plan/res-plan-demand-gpu-suborder"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
)

// ListBizResPlanDemandGpuSubOrderSummary lists GPU demand summary by demand_type for MCP / agent.
func (s *service) ListBizResPlanDemandGpuSubOrderSummary(cts *rest.Contexts) (interface{}, error) {
	bizID, err := cts.PathParameter("bk_biz_id").Int64()
	if err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	req := new(ptypes.ResPlanDemandGpuSubOrderSummaryReq)
	if err = cts.DecodeInto(req); err != nil {
		logs.Errorf("decode gpu demand suborder summary request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err = req.Validate(); err != nil {
		logs.Errorf("validate gpu demand suborder summary request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	authRes := meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.Biz, Action: meta.Access}, BizID: bizID}
	if err = s.authorizer.AuthorizeWithPerm(cts.Kit, authRes); err != nil {
		return nil, err
	}

	expr := buildGpuSubOrderSummaryFilter(bizID, req)

	subOrders, err := s.listAllGpuDemandSubOrders(cts.Kit, expr)
	if err != nil {
		logs.Errorf("list gpu demand suborders for summary failed, err: %v, biz: %d, order_id: %s, rid: %s",
			err, bizID, req.OrderID, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.Aborted, err)
	}

	return &ptypes.ResPlanDemandGpuSubOrderSummaryResp{
		OrderID: req.OrderID,
		Details: aggregateGpuSubOrderSummary(subOrders),
	}, nil
}

func buildGpuSubOrderSummaryFilter(bizID int64, req *ptypes.ResPlanDemandGpuSubOrderSummaryReq) *filter.Expression {
	rules := []*filter.AtomRule{
		tools.RuleEqual("bk_biz_id", bizID),
		tools.RuleEqual("order_id", req.OrderID),
	}

	if len(req.Statuses) > 0 {
		statuses := make([]string, 0, len(req.Statuses))
		for _, status := range req.Statuses {
			statuses = append(statuses, string(status))
		}
		rules = append(rules, tools.RuleIn("status", statuses))
	}

	if req.DemandYear != nil {
		rules = append(rules, tools.RuleEqual("demand_year", cvt.PtrToVal(req.DemandYear)))
	}

	if req.DemandMonth != nil {
		rules = append(rules, tools.RuleEqual("demand_month", cvt.PtrToVal(req.DemandMonth)))
	}

	if req.DemandType != "" {
		rules = append(rules, tools.RuleEqual("demand_type", req.DemandType))
	}

	return tools.ExpressionAnd(rules...)
}

func (s *service) listAllGpuDemandSubOrders(kt *kit.Kit, expr *filter.Expression) (
	[]gpusuborder.ResPlanDemandGpuSubOrderTable, error) {

	listReq := &rpproto.ResPlanDemandGpuSubOrderListReq{
		ListReq: core.ListReq{
			Filter: expr,
			Page:   core.NewDefaultBasePage(),
			Fields: []string{
				"id", "order_id", "demand_type", "demand_year", "demand_month", "gpu_num", "qpm_max", "status",
			},
		},
	}

	details := make([]gpusuborder.ResPlanDemandGpuSubOrderTable, 0)
	for {
		result, err := s.client.DataService().Global.ResourcePlan.ListResPlanDemandGpuSubOrder(kt, listReq)
		if err != nil {
			return nil, err
		}

		details = append(details, result.Details...)
		if len(result.Details) < int(listReq.Page.Limit) {
			break
		}
		listReq.Page.Start += uint32(listReq.Page.Limit)
	}

	return details, nil
}

type gpuSubOrderSummaryAgg struct {
	gpuNum int64
	qpmMax int64
	months map[string]int64
}

// aggregateGpuSubOrderSummary aggregates suborders by demand_type (aligned with FE summaryRows).
func aggregateGpuSubOrderSummary(subOrders []gpusuborder.ResPlanDemandGpuSubOrderTable) []ptypes.ResPlanDemandGpuSubOrderSummaryElem {

	aggMap := make(map[string]*gpuSubOrderSummaryAgg)
	order := make([]string, 0)

	for _, sub := range subOrders {
		agg, exists := aggMap[sub.DemandType]
		if !exists {
			agg = &gpuSubOrderSummaryAgg{months: make(map[string]int64)}
			aggMap[sub.DemandType] = agg
			order = append(order, sub.DemandType)
		}

		agg.gpuNum += sub.GPUNum
		agg.qpmMax += sub.QpmMax

		ym := fmt.Sprintf("%d-%02d", sub.DemandYear, sub.DemandMonth)
		// 卡数与 QPM 通常互斥，按月列累加二者之和即可对齐前端单度量列
		agg.months[ym] += sub.GPUNum + sub.QpmMax
	}

	sort.Strings(order)

	details := make([]ptypes.ResPlanDemandGpuSubOrderSummaryElem, 0, len(order))
	for _, demandType := range order {
		agg := aggMap[demandType]
		details = append(details, ptypes.ResPlanDemandGpuSubOrderSummaryElem{
			DemandType: demandType,
			GPUNum:     agg.gpuNum,
			QpmMax:     agg.qpmMax,
			Months:     agg.months,
		})
	}

	return details
}
