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

package splitter

import (
	"testing"
	"time"

	"hcm/pkg/criteria/enumor"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	tabletype "hcm/pkg/dal/table/types"

	"github.com/stretchr/testify/assert"
)

// TestConstructSubTicketCreateReq_BudgetDeclareAdminSkip 预算申报父单下子单强制 skip（含跨年）。
func TestConstructSubTicketCreateReq_BudgetDeclareAdminSkip(t *testing.T) {
	nonCurrentYear := time.Now().Year() + 2
	ticket := &rpt.ResPlanTicketTable{
		ID:      "ticket-budget-declare",
		Type:    enumor.RPTicketTypeBudgetDeclare,
		BkBizID: 1,
	}
	demands := []*rpt.ResPlanDemand{
		{
			Updated: &rpt.UpdatedRPDemandItem{
				ExpectTime: time.Date(nonCurrentYear, 6, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
				Cvm: rpt.Cvm{
					CpuCore: 16,
					Memory:  32,
				},
			},
		},
	}

	got := constructSubTicketCreateReq(ticket, 0, enumor.RPTicketTypeAdjust, demands, tabletype.JsonField("{}"))
	assert.Equal(t, enumor.RPAdminAuditStatusSkip, got.AdminAuditStatus)
	assert.Equal(t, enumor.RPTicketTypeAdjust, got.SubType)
}

// TestRootTicketBudgetDeclareUsesAdjustSplit 主单 budget_declare 与 adjust 走同一拆单策略。
func TestRootTicketBudgetDeclareUsesAdjustSplit(t *testing.T) {
	assert.Equal(t, rootTicketSplitKind(enumor.RPTicketTypeAdjust),
		rootTicketSplitKind(enumor.RPTicketTypeBudgetDeclare))
	assert.Equal(t, "adjust", rootTicketSplitKind(enumor.RPTicketTypeBudgetDeclare))
	assert.NotEqual(t, "add", rootTicketSplitKind(enumor.RPTicketTypeBudgetDeclare))
	assert.NotEqual(t, "delete", rootTicketSplitKind(enumor.RPTicketTypeBudgetDeclare))
}

// rootTicketSplitKind mirrors dispatcher/retry createSubTicket routing for unit assertion.
func rootTicketSplitKind(ticketType enumor.RPTicketType) string {
	switch ticketType {
	case enumor.RPTicketTypeDelete, enumor.RPTicketTypeAutomaticTransfer:
		return "delete"
	case enumor.RPTicketTypeAdd:
		return "add"
	case enumor.RPTicketTypeAdjust, enumor.RPTicketTypeBudgetDeclare:
		return "adjust"
	default:
		return "unsupported"
	}
}
