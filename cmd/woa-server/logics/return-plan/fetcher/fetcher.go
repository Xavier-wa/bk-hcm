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

// Package fetcher provides return plan query operations.
package fetcher

import (
	rptypes "hcm/cmd/woa-server/types/return-plan"
	"hcm/pkg/api/core"
	"hcm/pkg/client"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	"hcm/pkg/kit"
)

// Fetcher fetches return plan ticket and sub ticket data.
type Fetcher interface {
	ListReturnPlanTicket(kt *kit.Kit, opt *core.ListReq) (*rptypes.ListReturnPlanTicketResp, error)
	GetReturnPlanTicket(kt *kit.Kit, bkBizID int64, ticketID string) (*rptypes.GetReturnPlanTicketResp, error)
	GetReturnPlanTicketByID(kt *kit.Kit, ticketID string) (*tablert.ReturnPlanTicketTable, error)
	ListReturnPlanSubTicket(kt *kit.Kit, req *rptypes.ListReturnPlanSubTicketReq) (*rptypes.ListReturnPlanSubTicketResp,
		error)
	GetReturnPlanSubTicketDetail(kt *kit.Kit, bkBizID int64, subTicketID string) (
		*rptypes.GetReturnPlanSubTicketDetailResp, error)
}

// ReturnPlanFetcher implements Fetcher via data-service client.
type ReturnPlanFetcher struct {
	client *client.ClientSet
}

// New creates a ReturnPlanFetcher.
func New(client *client.ClientSet) Fetcher {
	return &ReturnPlanFetcher{client: client}
}
