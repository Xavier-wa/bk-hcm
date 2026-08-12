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

	"hcm/pkg/criteria/enumor"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	"hcm/pkg/kit"

	"github.com/stretchr/testify/assert"
)

// addDetail 构造仅含 updated 的新增条目。
func addDetail(resPool, city string) tablert.ReturnPlanDetail {
	return tablert.ReturnPlanDetail{
		Updated: &tablert.ReturnPlanItem{
			ObsProject:       enumor.ObsProjectNormal,
			ResourcePoolName: resPool,
			City:             city,
		},
	}
}

// cancelDetail 构造仅含 original 的取消条目。
func cancelDetail(crpPlanID int64, resPool string) tablert.ReturnPlanDetail {
	return tablert.ReturnPlanDetail{
		Original: &tablert.ReturnPlanItem{
			CrpPlanID:        crpPlanID,
			ObsProject:       enumor.ObsProjectNormal,
			ResourcePoolName: resPool,
		},
	}
}

// adjustDetail 构造同时含 original 与 updated 的调整条目。
func adjustDetail(crpPlanID int64, resPool string) tablert.ReturnPlanDetail {
	return tablert.ReturnPlanDetail{
		Original: &tablert.ReturnPlanItem{CrpPlanID: crpPlanID, ObsProject: enumor.ObsProjectNormal, ResourcePoolName: resPool},
		Updated:  &tablert.ReturnPlanItem{ObsProject: enumor.ObsProjectNormal, ResourcePoolName: resPool},
	}
}

func TestDeriveTicketType(t *testing.T) {
	testCases := []struct {
		name     string
		details  tablert.ReturnPlanDetails
		expected enumor.ReturnPlanTicketType
		wantErr  bool
	}{
		{
			name:     "only add -> add",
			details:  tablert.ReturnPlanDetails{addDetail("自研池", "北京"), addDetail("公有池", "")},
			expected: enumor.ReturnPlanTicketTypeAdd,
		},
		{
			name:     "only cancel -> cancel",
			details:  tablert.ReturnPlanDetails{cancelDetail(1001, "自研池")},
			expected: enumor.ReturnPlanTicketTypeCancel,
		},
		{
			name:     "only adjust -> adjust",
			details:  tablert.ReturnPlanDetails{adjustDetail(1001, "自研池")},
			expected: enumor.ReturnPlanTicketTypeAdjust,
		},
		{
			name:     "add and cancel -> adjust",
			details:  tablert.ReturnPlanDetails{addDetail("自研池", ""), cancelDetail(1001, "自研池")},
			expected: enumor.ReturnPlanTicketTypeAdjust,
		},
		{
			name:    "empty origin and update -> error",
			details: tablert.ReturnPlanDetails{{}},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.details.DeriveTicketType()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestGroupDetails(t *testing.T) {
	details := tablert.ReturnPlanDetails{
		addDetail("自研池", "北京"),
		addDetail("自研池", "上海"),
		cancelDetail(1001, "自研池"),
		addDetail("公有池", ""),
	}

	groups, err := groupDetails(kit.New(), details)
	assert.NoError(t, err)

	// 期望分为 3 组：add+公有池(1条) / add+自研池(2条) / cancel+自研池(1条)
	// 所有类型均按资源池维度拆分，保证一个子单对应一个 CRP 单据
	assert.Len(t, groups, 3)

	// 排序稳定：add < cancel；同类型内按资源池排序(公有池 < 自研池)
	assert.Equal(t, enumor.ReturnPlanTicketTypeAdd, groups[0].key.subType)
	assert.Equal(t, "公有池", groups[0].key.resPoolName)
	assert.Len(t, groups[0].details, 1)

	assert.Equal(t, enumor.ReturnPlanTicketTypeAdd, groups[1].key.subType)
	assert.Equal(t, "自研池", groups[1].key.resPoolName)
	assert.Len(t, groups[1].details, 2)

	assert.Equal(t, enumor.ReturnPlanTicketTypeCancel, groups[2].key.subType)
	assert.Equal(t, "自研池", groups[2].key.resPoolName)
	assert.Len(t, groups[2].details, 1)
}
