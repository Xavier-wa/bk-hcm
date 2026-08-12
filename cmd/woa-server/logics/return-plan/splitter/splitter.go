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

// Package splitter 退回计划拆单：按 部门+规划产品+项目类型+资源池 分组，add/cancel 分属不同子单。
package splitter

import (
	"encoding/json"
	"errors"

	rpproto "hcm/pkg/api/data-service/return-plan"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// Splitter 退回计划拆单器。
type Splitter interface {
	// Split 将主单 details 拆分为子单落库，并按明细构成回填主单 type。
	Split(kt *kit.Kit, ticket *tablert.ReturnPlanTicketTable) error
}

// SubTicketSplitter 退回计划拆单器实现。
// 相比资源预测拆单，退回计划无「可转移预测/延期」等复杂分支，仅按拆单维度分组。
type SubTicketSplitter struct {
	client *client.ClientSet
}

// New 创建退回计划拆单器。
func New(cli *client.ClientSet) Splitter {
	return &SubTicketSplitter{client: cli}
}

// Split 拆单主流程：解析明细 → 推导主单类型 → 分组落子单 → 回填主单类型。
func (s *SubTicketSplitter) Split(kt *kit.Kit, ticket *tablert.ReturnPlanTicketTable) error {
	if ticket == nil {
		logs.Errorf("split return plan ticket failed, ticket is nil, rid: %s", kt.Rid)
		return errors.New("return plan ticket is nil")
	}

	logs.Infof("start to split return plan ticket, ticket_id: %s, rid: %s", ticket.ID, kt.Rid)

	// 1. 解析主单明细
	details, err := parseTicketDetails(kt, ticket)
	if err != nil {
		return err
	}
	if len(details) == 0 {
		logs.Errorf("split return plan ticket failed, details is empty, ticket_id: %s, rid: %s",
			ticket.ID, kt.Rid)
		return errors.New("return plan ticket details is empty")
	}

	// 2. 按明细构成推导主单类型（同时校验明细至少含 original 或 updated）
	ticketType, err := details.DeriveTicketType()
	if err != nil {
		logs.Errorf("derive return plan ticket type failed, err: %v, ticket_id: %s, rid: %s",
			err, ticket.ID, kt.Rid)
		return err
	}

	// 3. 按拆单维度分组并落 return_plan_sub_ticket
	groups, err := groupDetails(kt, details)
	if err != nil {
		return err
	}
	if err := s.createSubTickets(kt, ticket, groups); err != nil {
		return err
	}

	// 4. 回填主单类型
	if err := s.backfillTicketType(kt, ticket, ticketType); err != nil {
		return err
	}

	logs.Infof("split return plan ticket success, ticket_id: %s, type: %s, sub_ticket_count: %d, rid: %s",
		ticket.ID, ticketType, len(groups), kt.Rid)
	return nil
}

// parseTicketDetails 解析主单 details JSON。
func parseTicketDetails(kt *kit.Kit, ticket *tablert.ReturnPlanTicketTable) (tablert.ReturnPlanDetails, error) {
	if ticket.Details.IsEmpty() {
		return nil, nil
	}

	var details tablert.ReturnPlanDetails
	if err := json.Unmarshal([]byte(ticket.Details), &details); err != nil {
		logs.Errorf("unmarshal return plan ticket details failed, err: %v, ticket_id: %s, rid: %s",
			err, ticket.ID, kt.Rid)
		return nil, err
	}
	return details, nil
}

// backfillTicketType 回填主单类型（类型未变化时跳过）。
func (s *SubTicketSplitter) backfillTicketType(kt *kit.Kit, ticket *tablert.ReturnPlanTicketTable,
	ticketType enumor.ReturnPlanTicketType) error {

	if ticketType == ticket.Type {
		logs.Infof("return plan ticket type unchanged, skip backfill, ticket_id: %s, type: %s, rid: %s",
			ticket.ID, ticketType, kt.Rid)
		return nil
	}

	// 后台任务，以提单人身份回填
	kt.User = ticket.Applicant
	req := &rpproto.ReturnPlanTicketUpdateReq{
		ID:   ticket.ID,
		Type: ticketType,
	}
	if err := s.client.DataService().Global.ReturnPlan.UpdateReturnPlanTicket(kt, req); err != nil {
		logs.Errorf("backfill return plan ticket type failed, err: %v, ticket_id: %s, type: %s, rid: %s",
			err, ticket.ID, ticketType, kt.Rid)
		return err
	}

	logs.Infof("backfill return plan ticket type success, ticket_id: %s, type: %s, rid: %s",
		ticket.ID, ticketType, kt.Rid)
	return nil
}
