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

// Package returnplan provides return plan HTTP handlers.
package returnplan

import (
	"net/http"

	returnplanlogic "hcm/cmd/woa-server/logics/return-plan"
	"hcm/cmd/woa-server/service/capability"
	"hcm/pkg/iam/auth"
	"hcm/pkg/rest"
)

// InitService initializes return plan service.
func InitService(c *capability.Capability) {
	s := &service{
		returnPlanController: c.ReturnPlanController,
		authorizer:           c.Authorizer,
	}

	bizH := rest.NewHandler()
	bizH.Path("/bizs/{bk_biz_id}")
	s.initBizReturnPlanService(bizH)
	bizH.Load(c.WebService)
}

type service struct {
	returnPlanController returnplanlogic.Logics
	authorizer           auth.Authorizer
}

func (s *service) initBizReturnPlanService(h *rest.Handler) {
	h.Add("ListBizReturnPlanTicket", http.MethodPost, "/plans/returns/tickets/list", s.ListBizReturnPlanTicket)
	h.Add("GetBizReturnPlanTicket", http.MethodGet, "/plans/returns/tickets/{id}", s.GetBizReturnPlanTicket)
	h.Add("ListBizReturnPlanSubTicket", http.MethodPost, "/plans/returns/sub_tickets/list", s.ListBizReturnPlanSubTicket)
	h.Add("GetBizReturnPlanSubTicket", http.MethodGet, "/plans/returns/sub_tickets/{id}", s.GetBizReturnPlanSubTicket)
	h.Add("RetryBizReturnPlanTicket", http.MethodPost, "/plans/returns/tickets/{ticket_id}/retry",
		s.RetryBizReturnPlanTicket)
	h.Add("TerminateBizReturnPlanTicket", http.MethodPost, "/plans/returns/tickets/{ticket_id}/terminate",
		s.TerminateBizReturnPlanTicket)
	h.Add("ListBizReturnReasonClass", http.MethodPost, "/plans/returns/reason_classes/list", s.ListBizReturnReasonClass)
	h.Add("OverwriteAppendBizReturnPlanTicket", http.MethodPost, "/plans/returns/tickets/overwrite_append",
		s.OverwriteAppendBizReturnPlanTicket)
}
