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
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"hcm/pkg/criteria/enumor"
)

func TestBudgetOperatorSyncReqValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     BudgetOperatorSyncReq
		wantErr bool
	}{
		{
			name: "有效请求",
			req: BudgetOperatorSyncReq{
				StartTime: "2026-01-01",
				EndTime:   "2026-12-31",
			},
			wantErr: false,
		},
		{
			name: "相同日期",
			req: BudgetOperatorSyncReq{
				StartTime: "2026-06-15",
				EndTime:   "2026-06-15",
			},
			wantErr: false,
		},
		{
			name: "缺少start_time",
			req: BudgetOperatorSyncReq{
				StartTime: "",
				EndTime:   "2026-12-31",
			},
			wantErr: true,
		},
		{
			name: "缺少end_time",
			req: BudgetOperatorSyncReq{
				StartTime: "2026-01-01",
				EndTime:   "",
			},
			wantErr: true,
		},
		{
			name: "start_time格式错误",
			req: BudgetOperatorSyncReq{
				StartTime: "2026/01/01",
				EndTime:   "2026-12-31",
			},
			wantErr: true,
		},
		{
			name: "end_time格式错误",
			req: BudgetOperatorSyncReq{
				StartTime: "2026-01-01",
				EndTime:   "20261231",
			},
			wantErr: true,
		},
		{
			name: "start_time晚于end_time",
			req: BudgetOperatorSyncReq{
				StartTime: "2026-12-31",
				EndTime:   "2026-01-01",
			},
			wantErr: true,
		},
		{
			name: "无效日期-2月30日",
			req: BudgetOperatorSyncReq{
				StartTime: "2026-02-30",
				EndTime:   "2026-12-31",
			},
			wantErr: true,
		},
		{
			name: "无效日期-13月",
			req: BudgetOperatorSyncReq{
				StartTime: "2026-13-01",
				EndTime:   "2026-12-31",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("BudgetOperatorSyncReq.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBudgetOperatorSyncReqTimeRange(t *testing.T) {
	tests := []struct {
		name      string
		req       BudgetOperatorSyncReq
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name: "正常时间范围",
			req: BudgetOperatorSyncReq{
				StartTime: "2026-01-01",
				EndTime:   "2026-12-31",
			},
			wantStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name: "跨年时间范围",
			req: BudgetOperatorSyncReq{
				StartTime: "2025-06-01",
				EndTime:   "2026-06-30",
			},
			wantStart: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name: "无效start_time格式",
			req: BudgetOperatorSyncReq{
				StartTime: "invalid",
				EndTime:   "2026-12-31",
			},
			wantErr: true,
		},
		{
			name: "无效end_time格式",
			req: BudgetOperatorSyncReq{
				StartTime: "2026-01-01",
				EndTime:   "invalid",
			},
			wantErr: true,
		},
		{
			name: "start_time晚于end_time",
			req: BudgetOperatorSyncReq{
				StartTime: "2026-12-31",
				EndTime:   "2026-01-01",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := tt.req.TimeRange()
			if (err != nil) != tt.wantErr {
				t.Errorf("BudgetOperatorSyncReq.TimeRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !start.Equal(tt.wantStart) {
					t.Errorf("BudgetOperatorSyncReq.TimeRange() start = %v, want %v", start, tt.wantStart)
				}
				if !end.Equal(tt.wantEnd) {
					t.Errorf("BudgetOperatorSyncReq.TimeRange() end = %v, want %v", end, tt.wantEnd)
				}
			}
		})
	}
}

func testValidCreateResPlanDemandReq() *CreateResPlanDemandReq {
	osVal := decimal.NewFromInt(2)
	cpuCore := int64(8)
	memory := int64(16)
	return &CreateResPlanDemandReq{
		ObsProject:     enumor.ObsProjectNormal,
		ExpectTime:     "2026-07-07",
		RegionID:       "ap-guangzhou",
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
			Os:         &osVal,
			CpuCore:    &cpuCore,
			Memory:     &memory,
		},
	}
}

func TestAdjustRPDemandReqElemValidate(t *testing.T) {
	validUpdated := testValidCreateResPlanDemandReq()
	validOriginal := testValidCreateResPlanDemandReq()

	tests := []struct {
		name    string
		elem    AdjustRPDemandReqElem
		wantErr bool
	}{
		{
			name: "add valid",
			elem: AdjustRPDemandReqElem{
				AdjustType:   enumor.RPDemandAdjustTypeAdd,
				DemandSource: enumor.DemandSourceIndChg,
				UpdatedInfo:  validUpdated,
			},
			wantErr: false,
		},
		{
			name: "add without demand_source",
			elem: AdjustRPDemandReqElem{
				AdjustType:  enumor.RPDemandAdjustTypeAdd,
				UpdatedInfo: validUpdated,
			},
			wantErr: true,
		},
		{
			name: "add with demand_id",
			elem: AdjustRPDemandReqElem{
				DemandID:    "demand-1",
				AdjustType:  enumor.RPDemandAdjustTypeAdd,
				UpdatedInfo: validUpdated,
			},
			wantErr: true,
		},
		{
			name: "add with original_info",
			elem: AdjustRPDemandReqElem{
				AdjustType:   enumor.RPDemandAdjustTypeAdd,
				OriginalInfo: validOriginal,
				UpdatedInfo:  validUpdated,
			},
			wantErr: true,
		},
		{
			name: "update valid",
			elem: AdjustRPDemandReqElem{
				DemandID:     "demand-1",
				AdjustType:   enumor.RPDemandAdjustTypeUpdate,
				DemandSource: enumor.DemandSourceIndChg,
				OriginalInfo: validOriginal,
				UpdatedInfo:  validUpdated,
			},
			wantErr: false,
		},
		{
			name: "update without demand_id",
			elem: AdjustRPDemandReqElem{
				AdjustType:   enumor.RPDemandAdjustTypeUpdate,
				OriginalInfo: validOriginal,
				UpdatedInfo:  validUpdated,
			},
			wantErr: true,
		},
		{
			name: "delay with original_info rejected",
			elem: AdjustRPDemandReqElem{
				DemandID:     "demand-1",
				AdjustType:   enumor.RPDemandAdjustTypeDelay,
				ExpectTime:   "2026-08-07",
				OriginalInfo: validOriginal,
			},
			wantErr: true,
		},
		{
			name: "delay without original_info",
			elem: AdjustRPDemandReqElem{
				DemandID:   "demand-1",
				AdjustType: enumor.RPDemandAdjustTypeDelay,
				ExpectTime: "2026-08-07",
			},
			wantErr: false,
		},
		{
			name: "delay without expect_time",
			elem: AdjustRPDemandReqElem{
				DemandID:   "demand-1",
				AdjustType: enumor.RPDemandAdjustTypeDelay,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.elem.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAdjustRPDemandReqElemValidateUpdateOnlyTimeChangeRejected(t *testing.T) {
	original := testValidCreateResPlanDemandReq()
	updated := testValidCreateResPlanDemandReq()
	updated.ExpectTime = "2026-08-07"

	elem := AdjustRPDemandReqElem{
		DemandID:     "demand-1",
		AdjustType:   enumor.RPDemandAdjustTypeUpdate,
		DemandSource: enumor.DemandSourceIndChg,
		OriginalInfo: original,
		UpdatedInfo:  updated,
	}

	err := elem.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "adjust_type should be delay")
}

func TestAdjustRPDemandReqElemValidateUpdateCombinedChangeAccepted(t *testing.T) {
	original := testValidCreateResPlanDemandReq()
	updated := testValidCreateResPlanDemandReq()
	updated.ExpectTime = "2026-08-07"
	cpuCore := int64(4)
	updated.Cvm.CpuCore = &cpuCore

	elem := AdjustRPDemandReqElem{
		DemandID:     "demand-1",
		AdjustType:   enumor.RPDemandAdjustTypeUpdate,
		DemandSource: enumor.DemandSourceIndChg,
		OriginalInfo: original,
		UpdatedInfo:  updated,
	}

	err := elem.Validate()
	assert.NoError(t, err)
}

func TestAdjustRPDemandReqElemValidateUpdateNoChangeRejected(t *testing.T) {
	original := testValidCreateResPlanDemandReq()
	updated := testValidCreateResPlanDemandReq()

	elem := AdjustRPDemandReqElem{
		DemandID:     "demand-1",
		AdjustType:   enumor.RPDemandAdjustTypeUpdate,
		DemandSource: enumor.DemandSourceIndChg,
		OriginalInfo: original,
		UpdatedInfo:  updated,
	}

	err := elem.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no change detected")
}

func TestAdjustRPDemandReqElemValidateAddDemandSourceInUpdatedInfoRejected(t *testing.T) {
	updated := testValidCreateResPlanDemandReq()
	updated.DemandSource = enumor.DemandSourceIndChg

	elem := AdjustRPDemandReqElem{
		AdjustType:  enumor.RPDemandAdjustTypeAdd,
		UpdatedInfo: updated,
	}

	err := elem.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "demand_source is required")
}

func TestAdjustRPDemandReqElemValidateAddDemandSourceAtElemLevel(t *testing.T) {
	elem := AdjustRPDemandReqElem{
		AdjustType:   enumor.RPDemandAdjustTypeAdd,
		DemandSource: enumor.DemandSourceIndChg,
		UpdatedInfo:  testValidCreateResPlanDemandReq(),
	}

	err := elem.Validate()
	assert.NoError(t, err)
}

func TestAdjustRPDemandReqElemValidateDelayWithUpdatedInfoRejected(t *testing.T) {
	elem := AdjustRPDemandReqElem{
		DemandID:    "demand-1",
		AdjustType:  enumor.RPDemandAdjustTypeDelay,
		ExpectTime:  "2026-08-07",
		UpdatedInfo: testValidCreateResPlanDemandReq(),
	}

	err := elem.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updated_info must be empty")
}

func TestAdjustRPDemandReqElemValidateDelayWithDelayOsRejected(t *testing.T) {
	delayOS := "1"
	elem := AdjustRPDemandReqElem{
		DemandID:   "demand-1",
		AdjustType: enumor.RPDemandAdjustTypeDelay,
		ExpectTime: "2026-08-07",
		DelayOs:    &delayOS,
	}

	err := elem.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delay_os is not supported")
}

func TestAdjustRPDemandReqValidateDemandClassRequiredForAllAdd(t *testing.T) {
	req := &AdjustRPDemandReq{
		Adjusts: []AdjustRPDemandReqElem{
			{
				AdjustType:   enumor.RPDemandAdjustTypeAdd,
				DemandSource: enumor.DemandSourceIndChg,
				UpdatedInfo:  testValidCreateResPlanDemandReq(),
			},
		},
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "demand_class is required")
}

func TestHasCreateReqResourceChange(t *testing.T) {
	original := testValidCreateResPlanDemandReq()
	updated := testValidCreateResPlanDemandReq()
	osVal := decimal.NewFromInt(1)
	updated.Cvm.Os = &osVal

	assert.False(t, hasCreateReqResourceChange(original, original))
	assert.True(t, hasCreateReqResourceChange(original, updated))
}

func TestHasCreateReqExpectTimeChange(t *testing.T) {
	original := testValidCreateResPlanDemandReq()
	updated := testValidCreateResPlanDemandReq()
	updated.ExpectTime = "2026-08-07"

	assert.True(t, hasCreateReqExpectTimeChange(original, updated))
	assert.False(t, hasCreateReqExpectTimeChange(original, original))
}
