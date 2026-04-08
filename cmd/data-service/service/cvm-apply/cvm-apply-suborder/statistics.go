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

// Package cvmapplysuborder ...
package cvmapplysuborder

import (
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
)

// GetOrderTimeCostOverview 按月份统计剔除审批阶段耗时
func (svc *service) GetOrderTimeCostOverview(cts *rest.Contexts) (interface{}, error) {
	req := new(filter.Expression)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	exprOpt := filter.NewExprOption(
		filter.RuleFields(cvmapplytable.ZiyanCvmApplySuborderColumns.ColumnTypes()),
	)
	if err := req.Validate(exprOpt); err != nil {
		logs.Errorf("invalid order time cost overview request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := svc.dao.ZiyanCvmApplySuborder().GetOrderTimeCostOverview(cts.Kit, req)
	if err != nil {
		logs.Errorf("get order time cost overview failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return result, nil
}

// GetOrderTimeCostCompare 按业务+月份统计剔除审批阶段耗时详情
func (svc *service) GetOrderTimeCostCompare(cts *rest.Contexts) (interface{}, error) {
	req := new(filter.Expression)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	exprOpt := filter.NewExprOption(
		filter.RuleFields(cvmapplytable.ZiyanCvmApplySuborderColumns.ColumnTypes()),
	)
	if err := req.Validate(exprOpt); err != nil {
		logs.Errorf("invalid order time cost compare request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := svc.dao.ZiyanCvmApplySuborder().GetOrderTimeCostCompare(cts.Kit, req)
	if err != nil {
		logs.Errorf("get order time cost compare failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	return result, nil
}

// GetPercentileTimeConsumptionOverview get percentile time consumption overview by month.
func (svc *service) GetPercentileTimeConsumptionOverview(cts *rest.Contexts) (interface{}, error) {
	req := new(filter.Expression)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	exprOpt := filter.NewExprOption(
		filter.RuleFields(cvmapplytable.ZiyanCvmApplySuborderColumns.ColumnTypes()),
	)
	if err := req.Validate(exprOpt); err != nil {
		logs.Errorf("invalid percentile time consumption overview request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := svc.dao.ZiyanCvmApplySuborder().GetPercentileTimeConsumptionOverview(cts.Kit, req)
	if err != nil {
		logs.Errorf("get percentile time consumption overview failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyPercentileTimeOverviewResult{Details: result.Details}, nil
}

// GetPercentileTimeConsumptionCompare get percentile time consumption compare by biz.
func (svc *service) GetPercentileTimeConsumptionCompare(cts *rest.Contexts) (interface{}, error) {
	req := new(filter.Expression)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	exprOpt := filter.NewExprOption(
		filter.RuleFields(cvmapplytable.ZiyanCvmApplySuborderColumns.ColumnTypes()),
	)
	if err := req.Validate(exprOpt); err != nil {
		logs.Errorf("invalid percentile time consumption compare request, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := svc.dao.ZiyanCvmApplySuborder().GetPercentileTimeConsumptionCompare(cts.Kit, req)
	if err != nil {
		logs.Errorf("get percentile time consumption compare failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyPercentileTimeCompareResult{
		Current: result.Current,
		Compare: result.Compare,
	}, nil
}

// GetProductionStageTimeCostOverview 获取生产阶段平均耗时
func (svc *service) GetProductionStageTimeCostOverview(cts *rest.Contexts) (interface{}, error) {
	req := new(filter.Expression)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	items, err := svc.dao.ZiyanCvmApplySuborder().GetProductionStageTimeCostOverview(cts.Kit, req)
	if err != nil {
		logs.Errorf("get order time cost overview failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &cvmapplyproto.ProductionStageTimeCostOverviewResult{
		Details: items,
	}, nil
}

// GetProductionStageTimeCostCompare 按业务获取生产阶段平均耗时
func (svc *service) GetProductionStageTimeCostCompare(cts *rest.Contexts) (interface{}, error) {
	req := new(filter.Expression)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	items, err := svc.dao.ZiyanCvmApplySuborder().GetProductionStageTimeCostCompare(cts.Kit, req)
	if err != nil {
		logs.Errorf("get order time cost compare failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	bizItems := make([]cvmapplyproto.ProductionStageTimeCostBizItem, 0, len(items))
	for _, item := range items {
		if item != nil {
			bizItems = append(bizItems, *item)
		}
	}

	return bizItems, nil
}

// GetApplyBizHostsStatistics 按业务统计申请主机数
func (svc *service) GetApplyBizHostsStatistics(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.CvmStatisticsListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("get apply biz hosts statistics req is invalid, err: %v, req: %+v, rid: %s", err, req, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := svc.dao.ZiyanCvmApplySuborder().GetApplyBizHostsStatistics(cts.Kit, req.Filter)
	if err != nil {
		logs.Errorf("get apply biz hosts statistics failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyBizHostsStatisticsResult{Details: result}, nil
}

// GetApplyBizCpuCoresStatistics 按业务统计申请CPU核心数
func (svc *service) GetApplyBizCpuCoresStatistics(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.CvmStatisticsListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("get apply biz core statistics req is invalid, err: %v, req: %+v, rid: %s", err, req, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := svc.dao.ZiyanCvmApplySuborder().GetApplyBizCpuCoresStatistics(cts.Kit, req.Filter)
	if err != nil {
		logs.Errorf("get apply biz cpu cores statistics failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyBizCpuCoresStatisticsResult{Details: result}, nil
}

// GetCompletionRateStatistics 按月份统计结单率
func (svc *service) GetCompletionRateStatistics(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.CvmStatisticsListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("get completion rate statistics req is invalid, err: %v, req: %+v, rid: %s", err, req, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := svc.dao.ZiyanCvmApplySuborder().GetCompletionRateStatistics(cts.Kit, req.Filter)
	if err != nil {
		logs.Errorf("get completion rate statistics failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyCompletionRateStatisticsResult{Details: result}, nil
}

// GetCompletionRateDetailStatistics 按业务+月份统计结单率详情
func (svc *service) GetCompletionRateDetailStatistics(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.CvmStatisticsListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("get completion rate detail statistics req is invalid, err: %v, req: %+v, rid: %s",
			err, req, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := svc.dao.ZiyanCvmApplySuborder().GetCompletionRateDetailStatistics(cts.Kit, req.Filter)
	if err != nil {
		logs.Errorf("get completion rate detail statistics failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyCompletionRateDetailResult{Details: result}, nil
}

// GetDeliveryRateStatistics 按月份统计主机交付率
func (svc *service) GetDeliveryRateStatistics(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.CvmStatisticsListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("get delivery rate statistics req is invalid, err: %v, req: %+v, rid: %s", err, req, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := svc.dao.ZiyanCvmApplySuborder().GetDeliveryRateStatistics(cts.Kit, req.Filter)
	if err != nil {
		logs.Errorf("get delivery rate statistics failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyDeliveryRateStatisticsResult{Details: result}, nil
}

// GetDeliveryRateDetailStatistics 按业务+月份统计主机交付率详情
func (svc *service) GetDeliveryRateDetailStatistics(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.CvmStatisticsListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		logs.Errorf("get delivery rate detail statistics req is invalid, err: %v, req: %+v, rid: %s",
			err, req, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	result, err := svc.dao.ZiyanCvmApplySuborder().GetDeliveryRateDetailStatistics(cts.Kit, req.Filter)
	if err != nil {
		logs.Errorf("get delivery rate detail statistics failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyDeliveryRateDetailResult{Details: result}, nil
}