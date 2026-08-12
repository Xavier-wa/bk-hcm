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

package plan

import (
	"testing"

	"hcm/pkg/api/core"
	"hcm/pkg/criteria/enumor"

	"github.com/stretchr/testify/assert"
)

// TestListResPlanTicketReq_ValidateBudgetDeclareType 列表筛选 ticket_types 接受 budget_declare。
func TestListResPlanTicketReq_ValidateBudgetDeclareType(t *testing.T) {
	req := &ListResPlanTicketReq{
		TicketTypes: []enumor.RPTicketType{enumor.RPTicketTypeBudgetDeclare},
		Page:        &core.BasePage{Start: 0, Limit: 10},
	}
	assert.NoError(t, req.Validate())
}

// TestListBizResPlanTicketReq_ValidateBudgetDeclareType 业务列表筛选接受 budget_declare。
func TestListBizResPlanTicketReq_ValidateBudgetDeclareType(t *testing.T) {
	req := &ListBizResPlanTicketReq{
		TicketTypes: []enumor.RPTicketType{enumor.RPTicketTypeBudgetDeclare},
		Page:        &core.BasePage{Start: 0, Limit: 10},
	}
	assert.NoError(t, req.Validate())
}

// TestListResPlanTicketReq_RejectSubOnlyType 列表筛选拒绝仅子单类型。
func TestListResPlanTicketReq_RejectSubOnlyType(t *testing.T) {
	req := &ListResPlanTicketReq{
		TicketTypes: []enumor.RPTicketType{enumor.RPTicketTypeDelay},
		Page:        &core.BasePage{Start: 0, Limit: 10},
	}
	assert.Error(t, req.Validate())
}
