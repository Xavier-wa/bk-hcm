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
	types "hcm/cmd/woa-server/types/task"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// GetDeliveryRateDetail aggregates delivery rate detail by biz and month within a range
func (op *operation) GetDeliveryRateDetail(kt *kit.Kit, param *types.DeliveryRateDetailReq) (
	*types.DeliveryRateDetailResp, error) {

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
	result, err := op.statistics.GetDeliveryRateDetailStatistics(kt, filterExpr)
	if err != nil {
		logs.Errorf("failed to get delivery rate detail statistics, startTime: %s, endTime: %s, err: %v, rid: %s",
			start, end, err, kt.Rid)
		return nil, err
	}

	return convertDeliveryRateDetailResult(result), nil
}

func convertDeliveryRateDetailResult(result *cvmapplyproto.ZiyanCvmApplyDeliveryRateDetailResult,
) *types.DeliveryRateDetailResp {

	if result == nil || len(result.Details) == 0 {
		return &types.DeliveryRateDetailResp{
			Details: make([]types.DeliveryRateDetailItem, 0),
		}
	}

	resp := &types.DeliveryRateDetailResp{
		Details: make([]types.DeliveryRateDetailItem, 0, len(result.Details)),
	}

	for _, item := range result.Details {
		if item == nil {
			continue
		}
		resp.Details = append(resp.Details, types.DeliveryRateDetailItem{
			BkBizID:          item.BkBizID,
			YearMonth:        item.YearMonth,
			TotalOrders:      item.TotalOrders,
			DoneOrders:       item.DoneOrders,
			TotalNumSum:      item.TotalNumSum,
			SuccessNumSum:    item.SuccessNumSum,
			HostDeliveryRate: item.HostDeliveryRate,
		})
	}

	return resp
}
