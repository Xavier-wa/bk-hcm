/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
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
	"hcm/pkg/tools/times"
)

func TestListResPlanDemandWithDeviceTypesReqValidate(t *testing.T) {
	// BkBizID 由 service 层从 URL path 参数解析后注入，不参与 body Validate 校验
	validReq := ListResPlanDemandWithDeviceTypesReq{
		BkBizID: 100,
		ExpectTimeRange: &times.DateRange{
			Start: "2026-06-01",
			End:   "2026-06-30",
		},
		Page: &core.BasePage{
			Start: 0,
			Limit: 10,
		},
	}

	tests := []struct {
		name    string
		modify  func(req *ListResPlanDemandWithDeviceTypesReq)
		wantErr bool
	}{
		{
			name:    "有效请求-最小参数",
			modify:  func(req *ListResPlanDemandWithDeviceTypesReq) {},
			wantErr: false,
		},
		{
			name:    "缺少expect_time_range",
			modify:  func(req *ListResPlanDemandWithDeviceTypesReq) { req.ExpectTimeRange = nil },
			wantErr: true,
		},
		{
			name: "expect_time_range格式错误",
			modify: func(req *ListResPlanDemandWithDeviceTypesReq) {
				req.ExpectTimeRange = &times.DateRange{Start: "invalid", End: "2026-06-30"}
			},
			wantErr: true,
		},
		{
			name:    "缺少page",
			modify:  func(req *ListResPlanDemandWithDeviceTypesReq) { req.Page = nil },
			wantErr: true,
		},
		{
			name: "带cpu_cores筛选",
			modify: func(req *ListResPlanDemandWithDeviceTypesReq) {
				req.CpuCores = []int64{60, 120}
			},
			wantErr: false,
		},
		{
			name: "带memories筛选",
			modify: func(req *ListResPlanDemandWithDeviceTypesReq) {
				req.Memories = []int64{64, 128}
			},
			wantErr: false,
		},
		{
			name: "带device_types筛选",
			modify: func(req *ListResPlanDemandWithDeviceTypesReq) {
				req.DeviceTypes = []string{"SA3.8XLARGE128"}
			},
			wantErr: false,
		},
		{
			name: "带plan_types筛选",
			modify: func(req *ListResPlanDemandWithDeviceTypesReq) {
				req.PlanTypes = []enumor.PlanType{enumor.PlanTypeHcmInPlan}
			},
			wantErr: false,
		},
		{
			name: "带statuses筛选",
			modify: func(req *ListResPlanDemandWithDeviceTypesReq) {
				req.Statuses = []enumor.DemandStatus{enumor.DemandStatusCanApply}
			},
			wantErr: false,
		},
		{
			name: "完整参数",
			modify: func(req *ListResPlanDemandWithDeviceTypesReq) {
				req.CpuCores = []int64{60, 120}
				req.Memories = []int64{64, 128}
				req.DeviceTypes = []string{"SA3.8XLARGE128"}
				req.RegionIDs = []string{"gz"}
				req.PlanTypes = []enumor.PlanType{enumor.PlanTypeHcmInPlan}
				req.Statuses = []enumor.DemandStatus{enumor.DemandStatusCanApply}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validReq // 浅拷贝
			tt.modify(&req)
			err := req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
