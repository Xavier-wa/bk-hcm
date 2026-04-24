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
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
)

// GetDeliveryRateStatistics aggregates delivery rate statistics by month within a range
func (op *operation) GetDeliveryRateStatistics(kt *kit.Kit, param *types.DeliveryRateStatisticsReq) (
	[]types.DeliveryRateStatisticsItem, error) {

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

	filterExpr := buildDeliveryRateFilterExpression(start, end)
	result, err := op.statistics.GetDeliveryRateStatistics(kt, filterExpr)
	if err != nil {
		logs.Errorf("failed to get delivery rate statistics, startTime: %s, endTime: %s, err: %v, rid: %s",
			start, end, err, kt.Rid)
		return nil, err
	}

	return convertDeliveryRateStatisticsResult(result), nil
}

func buildDeliveryRateFilterExpression(start, end time.Time) *filter.Expression {
	rules := []filter.RuleFactory{
		tools.RuleGreaterThanEqual("created_at", start.Format(constant.TimeStdFormat)),
		tools.RuleLessThan("created_at", end.Format(constant.TimeStdFormat)),
		tools.RuleNotEqual("source", enumor.ApplyTicketSrcPurchaseToResPool),
	}

	return &filter.Expression{Op: filter.And, Rules: rules}
}

func convertDeliveryRateStatisticsResult(
	result *cvmapplyproto.ZiyanCvmApplyDeliveryRateStatisticsResult,
) []types.DeliveryRateStatisticsItem {
	if result == nil || len(result.Details) == 0 {
		return []types.DeliveryRateStatisticsItem{}
	}

	out := make([]types.DeliveryRateStatisticsItem, 0, len(result.Details))
	for _, item := range result.Details {
		if item == nil {
			continue
		}
		out = append(out, types.DeliveryRateStatisticsItem{
			YearMonth:    item.YearMonth,
			DeliveryRate: item.DeliveryRate,
		})
	}

	return out
}
