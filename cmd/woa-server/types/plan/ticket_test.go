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

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func validCreateDemand(obs enumor.ObsProject) CreateResPlanDemandReq {
	os := decimal.NewFromInt(1)
	cpu := int64(8)
	mem := int64(16)
	return CreateResPlanDemandReq{
		ObsProject:     obs,
		ExpectTime:     "2026-09-01",
		RegionID:       "ap-shanghai",
		DemandResTypes: []enumor.DemandResType{enumor.DemandResTypeCVM},
		Cvm: &struct {
			ResMode    enumor.ResMode   `json:"res_mode"`
			DeviceType string           `json:"device_type"`
			Os         *decimal.Decimal `json:"os"`
			CpuCore    *int64           `json:"cpu_core"`
			Memory     *int64           `json:"memory"`
		}{
			ResMode:    enumor.ResModeByDeviceType,
			DeviceType: "SA2.LARGE8",
			Os:         &os,
			CpuCore:    &cpu,
			Memory:     &mem,
		},
	}
}

// TestCreateResPlanDemandReq_ValidateObsProjectForm 提单校验只认形态，不卡当前时间窗口。
func TestCreateResPlanDemandReq_ValidateObsProjectForm(t *testing.T) {
	normal := validCreateDemand(enumor.ObsProjectNormal)
	futureSpring := validCreateDemand("2099春节保障")
	futureDissolve := validCreateDemand("2098机房裁撤")
	illegal := validCreateDemand("2029春保")
	assert.NoError(t, normal.Validate())
	assert.NoError(t, futureSpring.Validate())
	assert.NoError(t, futureDissolve.Validate())
	assert.Error(t, illegal.Validate())
}

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
