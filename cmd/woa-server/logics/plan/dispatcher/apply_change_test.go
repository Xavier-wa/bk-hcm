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

package dispatcher

import (
	"testing"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/thirdparty/cvmapi"

	"github.com/stretchr/testify/assert"
)

// TestConvOrderChangeInfoFromCrpRespItemTrustsFutureObsProject CRP 回包项目类型不再做窗口校验。
func TestConvOrderChangeInfoFromCrpRespItemTrustsFutureObsProject(t *testing.T) {
	item := &cvmapi.PlanOrderChangeItem{
		ProjectName:   "2099春节保障",
		DiskTypeName:  string(enumor.DiskPremium),
		PlanType:      enumor.PlanTypeCrpInPlan,
		ResourceMode:  enumor.ResModeByDeviceType,
		InstanceModel: "SA2.LARGE8",
	}

	got, err := convOrderChangeInfoFromCrpRespItem(testKit(), "order-1", item)
	assert.NoError(t, err)
	assert.Equal(t, enumor.ObsProject("2099春节保障"), got.ObsProject)
	assert.Equal(t, constant.DefaultDiskIO, got.DiskIO)
}

func TestConvOrderChangeInfoFromCrpRespItem_DiskIO(t *testing.T) {
	baseItem := func(instanceIO int64) *cvmapi.PlanOrderChangeItem {
		return &cvmapi.PlanOrderChangeItem{
			ProjectName:   "常规项目",
			DiskTypeName:  string(enumor.DiskPremium),
			PlanType:      enumor.PlanTypeCrpInPlan,
			ResourceMode:  enumor.ResModeByDeviceType,
			InstanceModel: "SA2.LARGE8",
			InstanceIO:    instanceIO,
		}
	}

	testCases := []struct {
		name     string
		item     *cvmapi.PlanOrderChangeItem
		wantDisk int64
	}{
		{
			name:     "crp omitted instance io fallback to default",
			item:     baseItem(0),
			wantDisk: constant.DefaultDiskIO,
		},
		{
			name:     "keep positive instance io from crp",
			item:     baseItem(150),
			wantDisk: 150,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := convOrderChangeInfoFromCrpRespItem(testKit(), "order-1", tc.item)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantDisk, got.DiskIO)
		})
	}
}
