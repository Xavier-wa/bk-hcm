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
	"net/http"

	"hcm/cmd/data-service/service/capability"
	"hcm/pkg/dal/dao"
	"hcm/pkg/rest"
)

// InitService initialize the ziyan cvm apply suborder service
func InitService(cap *capability.Capability) {
	svc := &service{
		dao: cap.Dao,
	}
	h := rest.NewHandler()
	h.Path("/vendors/tcloud-ziyan")

	h.Add("BatchCreateZiyanCvmApplySuborder", http.MethodPost,
		"/cvm_apply/suborders/batch/create", svc.BatchCreateZiyanCvmApplySuborder)
	h.Add("BatchUpdateZiyanCvmApplySuborder", http.MethodPatch,
		"/cvm_apply/suborders/batch", svc.BatchUpdateZiyanCvmApplySuborder)
	h.Add("ListZiyanCvmApplySuborder", http.MethodPost, "/cvm_apply/suborders/list", svc.ListZiyanCvmApplySuborder)
	h.Add("DeleteZiyanCvmApplySuborder", http.MethodDelete,
		"/cvm_apply/suborders/batch", svc.DeleteZiyanCvmApplySuborder)
	h.Add("GetOrderTimeCostOverview", http.MethodPost,
		"/cvm_apply/analysis/order_time_cost/overview", svc.GetOrderTimeCostOverview)
	h.Add("GetOrderTimeCostCompare", http.MethodPost,
		"/cvm_apply/analysis/order_time_cost/compares", svc.GetOrderTimeCostCompare)
	h.Add("GetPercentileTimeConsumptionOverview", http.MethodPost,
		"/cvm_apply/suborders/statistics/percentile_time/overview", svc.GetPercentileTimeConsumptionOverview)
	h.Add("GetPercentileTimeConsumptionCompare", http.MethodPost,
		"/cvm_apply/suborders/statistics/percentile_time/compare", svc.GetPercentileTimeConsumptionCompare)
	h.Add("GetProductionStageTimeCostOverview", http.MethodPost,
		"/cvm_apply/suborders/statistics/production_stage_time_cost/overview", svc.GetProductionStageTimeCostOverview)
	h.Add("GetProductionStageTimeCostCompare", http.MethodPost,
		"/cvm_apply/suborders/statistics/production_stage_time_cost/compare", svc.GetProductionStageTimeCostCompare)
	h.Add("GetApplyBizHostsStatistics", http.MethodPost,
		"/cvm_apply/suborders/statistics/biz_hosts", svc.GetApplyBizHostsStatistics)
	h.Add("GetApplyBizCpuCoresStatistics", http.MethodPost,
		"/cvm_apply/suborders/statistics/biz_cpu_cores", svc.GetApplyBizCpuCoresStatistics)
	h.Add("GetCompletionRateStatistics", http.MethodPost,
		"/cvm_apply/suborders/statistics/completion_rate", svc.GetCompletionRateStatistics)
	h.Add("GetCompletionRateDetailStatistics", http.MethodPost,
		"/cvm_apply/suborders/statistics/completion_rate_detail", svc.GetCompletionRateDetailStatistics)
	h.Add("GetDeliveryRateStatistics", http.MethodPost,
		"/cvm_apply/suborders/statistics/delivery_rate", svc.GetDeliveryRateStatistics)
	h.Add("GetDeliveryRateDetailStatistics", http.MethodPost,
		"/cvm_apply/suborders/statistics/delivery_rate_detail", svc.GetDeliveryRateDetailStatistics)

	h.Load(cap.WebService)
}

type service struct {
	dao dao.Set
}
