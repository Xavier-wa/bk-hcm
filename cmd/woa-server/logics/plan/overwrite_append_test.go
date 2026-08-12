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

	dt "hcm/pkg/api/core/cloud/device-type"
	"hcm/pkg/criteria/enumor"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"

	"github.com/stretchr/testify/assert"
)

// TestCheckDemandsDeviceTypesExist 覆盖追加提单前置机型校验。
func TestCheckDemandsDeviceTypesExist(t *testing.T) {
	deviceTypeMap := map[string]dt.DistinctDeviceType{
		"S5.2XLARGE16": {DeviceType: "S5.2XLARGE16", CpuCore: 8, Memory: 16},
		"ZERO.CORE":    {DeviceType: "ZERO.CORE", CpuCore: 0, Memory: 0},
	}

	tests := []struct {
		name    string
		demands rpt.ResPlanDemands
		wantErr bool
	}{
		{
			name: "updated known device type ok",
			demands: rpt.ResPlanDemands{
				{Updated: &rpt.UpdatedRPDemandItem{Cvm: rpt.Cvm{DeviceType: "S5.2XLARGE16"}}},
			},
			wantErr: false,
		},
		{
			name: "updated unknown device type failed",
			demands: rpt.ResPlanDemands{
				{Updated: &rpt.UpdatedRPDemandItem{Cvm: rpt.Cvm{DeviceType: "PDi1.24XLARGE384"}}},
			},
			wantErr: true,
		},
		{
			name: "original unknown device type failed",
			demands: rpt.ResPlanDemands{
				{Original: &rpt.OriginalRPDemandItem{Cvm: rpt.Cvm{DeviceType: "PDi1.24XLARGE384"}}},
			},
			wantErr: true,
		},
		{
			name: "cpu core zero treated as not found",
			demands: rpt.ResPlanDemands{
				{Updated: &rpt.UpdatedRPDemandItem{Cvm: rpt.Cvm{DeviceType: "ZERO.CORE"}}},
			},
			wantErr: true,
		},
		{
			name: "empty device type skipped",
			demands: rpt.ResPlanDemands{
				{Updated: &rpt.UpdatedRPDemandItem{Cvm: rpt.Cvm{DeviceType: ""}}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDemandsDeviceTypesExist(tt.demands, deviceTypeMap)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "cannot found device type")
				return
			}
			assert.NoError(t, err)
		})
	}
}

// TestValidateOverwriteAppendTicketType 校验主单 type 与 cancel/add 明细组成一致。
func TestValidateOverwriteAppendTicketType(t *testing.T) {
	testCases := []struct {
		name       string
		ticketType enumor.RPTicketType
		hasCancel  bool
		hasAdd     bool
		wantErr    bool
	}{
		{
			name:       "cancel+add with adjust ok",
			ticketType: enumor.RPTicketTypeAdjust,
			hasCancel:  true,
			hasAdd:     true,
			wantErr:    false,
		},
		{
			name:       "cancel+add with budget_declare ok",
			ticketType: enumor.RPTicketTypeBudgetDeclare,
			hasCancel:  true,
			hasAdd:     true,
			wantErr:    false,
		},
		{
			name:       "cancel+add with add failed",
			ticketType: enumor.RPTicketTypeAdd,
			hasCancel:  true,
			hasAdd:     true,
			wantErr:    true,
		},
		{
			name:       "cancel+add with delete failed",
			ticketType: enumor.RPTicketTypeDelete,
			hasCancel:  true,
			hasAdd:     true,
			wantErr:    true,
		},
		{
			name:       "add-only with add ok",
			ticketType: enumor.RPTicketTypeAdd,
			hasCancel:  false,
			hasAdd:     true,
			wantErr:    false,
		},
		{
			name:       "add-only with budget_declare ok",
			ticketType: enumor.RPTicketTypeBudgetDeclare,
			hasCancel:  false,
			hasAdd:     true,
			wantErr:    false,
		},
		{
			name:       "add-only with adjust failed",
			ticketType: enumor.RPTicketTypeAdjust,
			hasCancel:  false,
			hasAdd:     true,
			wantErr:    true,
		},
		{
			name:       "cancel-only with delete ok",
			ticketType: enumor.RPTicketTypeDelete,
			hasCancel:  true,
			hasAdd:     false,
			wantErr:    false,
		},
		{
			name:       "cancel-only with budget_declare ok",
			ticketType: enumor.RPTicketTypeBudgetDeclare,
			hasCancel:  true,
			hasAdd:     false,
			wantErr:    false,
		},
		{
			name:       "cancel-only with adjust failed",
			ticketType: enumor.RPTicketTypeAdjust,
			hasCancel:  true,
			hasAdd:     false,
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOverwriteAppendTicketType(tc.ticketType, tc.hasCancel, tc.hasAdd)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "inconsistent")
				return
			}
			assert.NoError(t, err)
		})
	}
}
