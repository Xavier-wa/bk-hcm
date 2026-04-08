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

package ziyan

import (
	"context"
	"net/http"

	"hcm/pkg/api/core"
	dataproto "hcm/pkg/api/data-service"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
)

// ZiyanCvmApplySuborderClient is data service ziyan cvm apply suborder api client.
type ZiyanCvmApplySuborderClient struct {
	client rest.ClientInterface
}

// NewZiyanCvmApplySuborderClient create a new ziyan cvm apply suborder api client.
func NewZiyanCvmApplySuborderClient(client rest.ClientInterface) *ZiyanCvmApplySuborderClient {
	return &ZiyanCvmApplySuborderClient{
		client: client,
	}
}

// BatchCreate batch create ziyan cvm apply suborder.
func (c *ZiyanCvmApplySuborderClient) BatchCreate(ctx context.Context, h http.Header,
	req *cvmapplyproto.BatchCreateZiyanCvmApplySuborderReq) (*core.BatchCreateResult, error) {

	resp := new(core.BatchCreateResp)

	err := c.client.Post().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/suborders/batch/create").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// List list ziyan cvm apply suborder.
func (c *ZiyanCvmApplySuborderClient) List(ctx context.Context, h http.Header,
	req *cvmapplyproto.ZiyanCvmApplySuborderListReq) (*cvmapplyproto.ZiyanCvmApplySuborderListResult, error) {

	resp := new(cvmapplyproto.ZiyanCvmApplySuborderListResp)

	err := c.client.Post().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/suborders/list").
		WithHeaders(h).
		Do().
		Into(resp)

	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// BatchUpdate batch update ziyan cvm apply suborder.
func (c *ZiyanCvmApplySuborderClient) BatchUpdate(ctx context.Context, h http.Header,
	req *cvmapplyproto.BatchUpdateZiyanCvmApplySuborderReq) error {

	resp := new(rest.BaseResp)

	err := c.client.Patch().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/suborders/batch").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return err
	}

	if resp.Code != errf.OK {
		return errf.New(resp.Code, resp.Message)
	}

	return nil
}

// BatchDelete batch delete ziyan cvm apply suborder.
func (c *ZiyanCvmApplySuborderClient) BatchDelete(ctx context.Context, h http.Header,
	req *dataproto.BatchDeleteReq) error {

	resp := new(rest.BaseResp)

	err := c.client.Delete().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/suborders/batch").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return err
	}

	if resp.Code != errf.OK {
		return errf.New(resp.Code, resp.Message)
	}

	return nil
}

// GetOrderTimeCostOverview 按月份统计剔除审批阶段耗时
func (c *ZiyanCvmApplySuborderClient) GetOrderTimeCostOverview(ctx context.Context, h http.Header,
	req *filter.Expression) ([]*cvmapplyproto.OrderTimeCostItem, error) {

	resp := new(cvmapplyproto.OrderTimeCostItemListResp)
	err := c.client.Post().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/analysis/order_time_cost/overview").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Details, nil
}

// GetOrderTimeCostCompare 按业务+月份统计剔除审批阶段耗时详情
func (c *ZiyanCvmApplySuborderClient) GetOrderTimeCostCompare(ctx context.Context, h http.Header,
	req *filter.Expression) ([]*cvmapplyproto.OrderTimeCostCompareItem, error) {

	resp := new(cvmapplyproto.OrderTimeCostCompareItemListResp)
	err := c.client.Post().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/analysis/order_time_cost/compares").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Details, nil
}

// GetPercentileTimeConsumptionOverview get percentile time consumption overview by month.
func (c *ZiyanCvmApplySuborderClient) GetPercentileTimeConsumptionOverview(ctx context.Context, h http.Header,
	filterExpr *filter.Expression) (*cvmapplyproto.ZiyanCvmApplyPercentileTimeOverviewResult, error) {

	resp := new(cvmapplyproto.ZiyanCvmApplyPercentileTimeOverviewResp)

	err := c.client.Post().
		WithContext(ctx).
		Body(filterExpr).
		SubResourcef("/cvm_apply/suborders/statistics/percentile_time/overview").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// GetPercentileTimeConsumptionCompare get percentile time consumption compare by biz.
func (c *ZiyanCvmApplySuborderClient) GetPercentileTimeConsumptionCompare(ctx context.Context, h http.Header,
	filterExpr *filter.Expression) (*cvmapplyproto.ZiyanCvmApplyPercentileTimeCompareResult, error) {

	resp := new(cvmapplyproto.ZiyanCvmApplyPercentileTimeCompareResp)

	err := c.client.Post().
		WithContext(ctx).
		Body(filterExpr).
		SubResourcef("/cvm_apply/suborders/statistics/percentile_time/compare").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// GetProductionStageTimeCostOverview get production stage time cost overview.
func (c *ZiyanCvmApplySuborderClient) GetProductionStageTimeCostOverview(ctx context.Context, h http.Header,
	filterExpr *filter.Expression) (*cvmapplyproto.ProductionStageTimeCostOverviewResult, error) {

	resp := new(cvmapplyproto.ProductionStageTimeCostOverviewResp)
	err := c.client.Post().
		WithContext(ctx).
		Body(filterExpr).
		SubResourcef("/cvm_apply/suborders/statistics/production_stage_time_cost/overview").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// GetProductionStageTimeCostCompare get production stage time cost compare.
func (c *ZiyanCvmApplySuborderClient) GetProductionStageTimeCostCompare(ctx context.Context, h http.Header,
	filterExpr *filter.Expression) ([]cvmapplyproto.ProductionStageTimeCostBizItem, error) {

	resp := new(cvmapplyproto.ProductionStageTimeCostSingleListResp)
	err := c.client.Post().
		WithContext(ctx).
		Body(filterExpr).
		SubResourcef("/cvm_apply/suborders/statistics/production_stage_time_cost/compare").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// GetApplyBizHostsStatistics 按业务统计申请主机数
func (c *ZiyanCvmApplySuborderClient) GetApplyBizHostsStatistics(ctx context.Context, h http.Header,
	req *cvmapplyproto.CvmStatisticsListReq) (*cvmapplyproto.ZiyanCvmApplyBizHostsStatisticsResult, error) {

	resp := new(cvmapplyproto.ZiyanCvmApplyBizHostsStatisticsResp)
	err := c.client.Post().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/suborders/statistics/biz_hosts").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// GetApplyBizCpuCoresStatistics 按业务统计申请CPU核心数
func (c *ZiyanCvmApplySuborderClient) GetApplyBizCpuCoresStatistics(ctx context.Context, h http.Header,
	req *cvmapplyproto.CvmStatisticsListReq) (*cvmapplyproto.ZiyanCvmApplyBizCpuCoresStatisticsResult, error) {

	resp := new(cvmapplyproto.ZiyanCvmApplyBizCpuCoresStatisticsResp)
	err := c.client.Post().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/suborders/statistics/biz_cpu_cores").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// GetCompletionRateStatistics 按月份统计结单率
func (c *ZiyanCvmApplySuborderClient) GetCompletionRateStatistics(ctx context.Context, h http.Header,
	req *cvmapplyproto.CvmStatisticsListReq) (*cvmapplyproto.ZiyanCvmApplyCompletionRateStatisticsResult, error) {

	resp := new(cvmapplyproto.ZiyanCvmApplyCompletionRateStatisticsResp)
	err := c.client.Post().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/suborders/statistics/completion_rate").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// GetCompletionRateDetailStatistics 按业务+月份统计结单率详情
func (c *ZiyanCvmApplySuborderClient) GetCompletionRateDetailStatistics(ctx context.Context, h http.Header,
	req *cvmapplyproto.CvmStatisticsListReq) (*cvmapplyproto.ZiyanCvmApplyCompletionRateDetailResult, error) {

	resp := new(cvmapplyproto.ZiyanCvmApplyCompletionRateDetailResp)
	err := c.client.Post().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/suborders/statistics/completion_rate_detail").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// GetDeliveryRateStatistics 按月份统计主机交付率
func (c *ZiyanCvmApplySuborderClient) GetDeliveryRateStatistics(ctx context.Context, h http.Header,
	req *cvmapplyproto.CvmStatisticsListReq) (*cvmapplyproto.ZiyanCvmApplyDeliveryRateStatisticsResult, error) {

	resp := new(cvmapplyproto.ZiyanCvmApplyDeliveryRateStatisticsResp)
	err := c.client.Post().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/suborders/statistics/delivery_rate").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// GetDeliveryRateDetailStatistics 按业务+月份统计主机交付率详情
func (c *ZiyanCvmApplySuborderClient) GetDeliveryRateDetailStatistics(ctx context.Context, h http.Header,
	req *cvmapplyproto.CvmStatisticsListReq) (*cvmapplyproto.ZiyanCvmApplyDeliveryRateDetailResult, error) {

	resp := new(cvmapplyproto.ZiyanCvmApplyDeliveryRateDetailResp)
	err := c.client.Post().
		WithContext(ctx).
		Body(req).
		SubResourcef("/cvm_apply/suborders/statistics/delivery_rate_detail").
		WithHeaders(h).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}
