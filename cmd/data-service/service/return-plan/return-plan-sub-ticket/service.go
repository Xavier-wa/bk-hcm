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

// Package returnplansubticket 退回计划子单 data-service 服务
package returnplansubticket

import (
	"net/http"

	"hcm/cmd/data-service/service/capability"
	"hcm/pkg/dal/dao"
	"hcm/pkg/rest"
)

// InitService initialize the return plan sub ticket service.
func InitService(cap *capability.Capability) {
	svc := &service{
		dao: cap.Dao,
	}
	h := rest.NewHandler()
	h.Add("BatchCreateReturnPlanSubTicket", http.MethodPost,
		"/return_plans/return_plan_sub_tickets/batch/create", svc.BatchCreateReturnPlanSubTicket)
	h.Add("DeleteReturnPlanSubTicket", http.MethodDelete, "/return_plans/return_plan_sub_tickets/batch",
		svc.DeleteReturnPlanSubTicket)
	h.Add("ListReturnPlanSubTicket", http.MethodPost, "/return_plans/return_plan_sub_tickets/list",
		svc.ListReturnPlanSubTicket)
	h.Add("BatchUpdateReturnPlanSubTicket", http.MethodPatch, "/return_plans/return_plan_sub_tickets/batch",
		svc.BatchUpdateReturnPlanSubTicket)
	h.Add("UpdateReturnPlanSubTicketStatusCAS", http.MethodPatch,
		"/return_plans/return_plan_sub_tickets/status/cas", svc.UpdateReturnPlanSubTicketStatusCAS)

	h.Load(cap.WebService)
}

type service struct {
	dao dao.Set
}
