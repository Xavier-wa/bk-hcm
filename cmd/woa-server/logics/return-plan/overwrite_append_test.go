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

package returnplan

import (
	"testing"

	"hcm/pkg/criteria/enumor"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	"hcm/pkg/thirdparty/cvmapi"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// TestDeriveReturnPlanTicketType 主单类型推导：仅 add→add / 仅 cancel→cancel / 混合→adjust / 空→err。
func TestDeriveReturnPlanTicketType(t *testing.T) {
	addDetail := tablert.ReturnPlanDetail{Updated: &tablert.ReturnPlanItem{}}
	cancelDetail := tablert.ReturnPlanDetail{Original: &tablert.ReturnPlanItem{}}

	testCases := []struct {
		name     string
		details  tablert.ReturnPlanDetails
		expected enumor.ReturnPlanTicketType
		wantErr  bool
	}{
		{
			name:     "only add derives add",
			details:  tablert.ReturnPlanDetails{addDetail, addDetail},
			expected: enumor.ReturnPlanTicketTypeAdd,
		},
		{
			name:     "only cancel derives cancel",
			details:  tablert.ReturnPlanDetails{cancelDetail},
			expected: enumor.ReturnPlanTicketTypeCancel,
		},
		{
			name:     "mixed derives adjust",
			details:  tablert.ReturnPlanDetails{addDetail, cancelDetail},
			expected: enumor.ReturnPlanTicketTypeAdjust,
		},
		{
			name:    "empty details is invalid",
			details: tablert.ReturnPlanDetails{},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.details.DeriveTicketType()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestConvCrpReturnPlanItem CRP 退回计划项转 Original 明细：核数/实例数类型转换与关键字段映射。
func TestConvCrpReturnPlanItem(t *testing.T) {
	item := &cvmapi.ReturnPlanItem{
		ID:            1024,
		ProjectName:   enumor.ObsProjectNormal,
		PlanTime:      "2024-11-12",
		CityName:      "上海",
		ZoneName:      "上海二区",
		InstanceModel: "SA2.LARGE8",
		InstanceType:  "标准型SA2",
		CoreTypeName:  "小核心",
		CvmAmount:     10.9,
		CoreAmount:    decimal.NewFromFloat(80.6),
	}

	got := convCrpReturnPlanItem(item)

	assert.Equal(t, int64(1024), got.CrpPlanID)
	assert.Equal(t, enumor.ObsProjectNormal, got.ObsProject)
	assert.Equal(t, "2024-11-12", got.PlanTime)
	assert.Equal(t, "上海", got.City)
	assert.Equal(t, "上海二区", got.Zone)
	assert.Equal(t, "SA2.LARGE8", got.InstanceModel)
	assert.Equal(t, "标准型SA2", got.InstanceType)
	assert.Equal(t, "小核心", got.CoreTypeName)
	// float/decimal 转 int64 采用截断。
	assert.Equal(t, int64(10), got.CvmAmount)
	assert.Equal(t, int64(80), got.CoreAmount)
}
