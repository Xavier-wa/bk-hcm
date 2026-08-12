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

package global

import (
	"hcm/pkg/api/core"
	dataproto "hcm/pkg/api/data-service"
	rpproto "hcm/pkg/api/data-service/return-plan"
	"hcm/pkg/client/common"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
)

// ReturnPlanClient is data service return plan api client.
type ReturnPlanClient struct {
	client rest.ClientInterface
}

// NewReturnPlanClient create a new return plan api client.
func NewReturnPlanClient(client rest.ClientInterface) *ReturnPlanClient {
	return &ReturnPlanClient{
		client: client,
	}
}

// --- return plan ticket ---

// CreateReturnPlanTicket create return plan ticket.
func (b *ReturnPlanClient) CreateReturnPlanTicket(kt *kit.Kit, req *rpproto.ReturnPlanTicketCreateReq) (
	*core.CreateResult, error) {

	return common.Request[rpproto.ReturnPlanTicketCreateReq, core.CreateResult](
		b.client, rest.POST, kt, req, "/return_plans/return_plan_tickets/create")
}

// UpdateReturnPlanTicket update return plan ticket.
func (b *ReturnPlanClient) UpdateReturnPlanTicket(kt *kit.Kit, req *rpproto.ReturnPlanTicketUpdateReq) error {
	return common.RequestNoResp[rpproto.ReturnPlanTicketUpdateReq](
		b.client, rest.PATCH, kt, req, "/return_plans/return_plan_tickets")
}

// ListReturnPlanTicket list return plan ticket.
func (b *ReturnPlanClient) ListReturnPlanTicket(kt *kit.Kit, req *rpproto.ReturnPlanTicketListReq) (
	*rpproto.ReturnPlanTicketListResult, error) {

	return common.Request[rpproto.ReturnPlanTicketListReq, rpproto.ReturnPlanTicketListResult](
		b.client, rest.POST, kt, req, "/return_plans/return_plan_tickets/list")
}

// DeleteReturnPlanTicket delete return plan ticket.
func (b *ReturnPlanClient) DeleteReturnPlanTicket(kt *kit.Kit, req *dataproto.BatchDeleteReq) error {
	return common.RequestNoResp[dataproto.BatchDeleteReq](
		b.client, rest.DELETE, kt, req, "/return_plans/return_plan_tickets/batch")
}

// --- return plan sub ticket ---

// BatchCreateReturnPlanSubTicket batch create return plan sub ticket.
func (b *ReturnPlanClient) BatchCreateReturnPlanSubTicket(kt *kit.Kit,
	req *rpproto.ReturnPlanSubTicketBatchCreateReq) (*core.BatchCreateResult, error) {

	return common.Request[rpproto.ReturnPlanSubTicketBatchCreateReq, core.BatchCreateResult](
		b.client, rest.POST, kt, req, "/return_plans/return_plan_sub_tickets/batch/create")
}

// BatchUpdateReturnPlanSubTicket batch update return plan sub ticket.
func (b *ReturnPlanClient) BatchUpdateReturnPlanSubTicket(kt *kit.Kit,
	req *rpproto.ReturnPlanSubTicketBatchUpdateReq) error {

	return common.RequestNoResp[rpproto.ReturnPlanSubTicketBatchUpdateReq](
		b.client, rest.PATCH, kt, req, "/return_plans/return_plan_sub_tickets/batch")
}

// UpdateReturnPlanSubTicketStatusCAS update return plan sub ticket status with cas.
func (b *ReturnPlanClient) UpdateReturnPlanSubTicketStatusCAS(kt *kit.Kit,
	req *rpproto.ReturnPlanSubTicketStatusUpdateReq) error {

	return common.RequestNoResp[rpproto.ReturnPlanSubTicketStatusUpdateReq](
		b.client, rest.PATCH, kt, req, "/return_plans/return_plan_sub_tickets/status/cas")
}

// ListReturnPlanSubTicket list return plan sub ticket.
func (b *ReturnPlanClient) ListReturnPlanSubTicket(kt *kit.Kit, req *rpproto.ReturnPlanSubTicketListReq) (
	*rpproto.ReturnPlanSubTicketListResult, error) {

	return common.Request[rpproto.ReturnPlanSubTicketListReq, rpproto.ReturnPlanSubTicketListResult](
		b.client, rest.POST, kt, req, "/return_plans/return_plan_sub_tickets/list")
}

// DeleteReturnPlanSubTicket delete return plan sub ticket.
func (b *ReturnPlanClient) DeleteReturnPlanSubTicket(kt *kit.Kit, req *dataproto.BatchDeleteReq) error {
	return common.RequestNoResp[dataproto.BatchDeleteReq](
		b.client, rest.DELETE, kt, req, "/return_plans/return_plan_sub_tickets/batch")
}
