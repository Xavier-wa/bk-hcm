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
	"hcm/pkg/tools/times"

	"github.com/stretchr/testify/assert"
)

// validReturnOverwriteFilter 构造一个合法的退回计划覆盖筛选条件。
func validReturnOverwriteFilter() *ReturnPlanOverwriteFilter {
	return &ReturnPlanOverwriteFilter{
		ObsProjects:   []enumor.ObsProject{enumor.ObsProjectNormal},
		PlanTimeRange: &times.DateRange{Start: "2024-09-01", End: "2024-12-31"},
	}
}

// validAppendDetailByModel 构造一个按实例规格的合法追加明细。
func validAppendDetailByModel() AppendReturnPlanDetail {
	return AppendReturnPlanDetail{
		ObsProject:    enumor.ObsProjectNormal,
		PlanTime:      "2024-11-12",
		RegionID:      "ap-shanghai",
		InstanceModel: "SA2.LARGE8",
		CvmAmount:     10,
	}
}

// TestOverwriteAppendReturnPlanTicketReq_Validate 覆盖 overwrite/return_details 组合与筛选条件必选性校验。
func TestOverwriteAppendReturnPlanTicketReq_Validate(t *testing.T) {
	testCases := []struct {
		name    string
		req     *OverwriteAppendReturnPlanTicketReq
		wantErr bool
	}{
		{
			name: "overwrite false and no return_details is invalid",
			req: &OverwriteAppendReturnPlanTicketReq{
				Overwrite: false,
				Applicant: "zhangsan",
			},
			wantErr: true,
		},
		{
			name: "applicant required",
			req: &OverwriteAppendReturnPlanTicketReq{
				Overwrite:       true,
				OverwriteFilter: validReturnOverwriteFilter(),
			},
			wantErr: true,
		},
		{
			name: "overwrite true without filter is invalid",
			req: &OverwriteAppendReturnPlanTicketReq{
				Overwrite: true,
				Applicant: "zhangsan",
			},
			wantErr: true,
		},
		{
			name: "overwrite only with valid filter is ok",
			req: &OverwriteAppendReturnPlanTicketReq{
				Overwrite:       true,
				OverwriteFilter: validReturnOverwriteFilter(),
				Applicant:       "zhangsan",
			},
			wantErr: false,
		},
		{
			name: "append only is ok",
			req: &OverwriteAppendReturnPlanTicketReq{
				Overwrite:     false,
				ReturnDetails: []AppendReturnPlanDetail{validAppendDetailByModel()},
				Applicant:     "zhangsan",
			},
			wantErr: false,
		},
		{
			name: "filter missing plan_time_range is invalid",
			req: &OverwriteAppendReturnPlanTicketReq{
				Overwrite: true,
				OverwriteFilter: &ReturnPlanOverwriteFilter{
					ObsProjects: []enumor.ObsProject{enumor.ObsProjectNormal},
				},
				Applicant: "zhangsan",
			},
			wantErr: true,
		},
		{
			name: "filter future spring form is ok",
			req: &OverwriteAppendReturnPlanTicketReq{
				Overwrite: true,
				OverwriteFilter: &ReturnPlanOverwriteFilter{
					ObsProjects:   []enumor.ObsProject{"2099春节保障"},
					PlanTimeRange: &times.DateRange{Start: "2024-09-01", End: "2024-12-31"},
				},
				Applicant: "zhangsan",
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// TestAppendReturnPlanDetail_Validate 追加明细 instance_model/instance_type 二选一及配套字段校验。
func TestAppendReturnPlanDetail_Validate(t *testing.T) {
	testCases := []struct {
		name    string
		detail  AppendReturnPlanDetail
		wantErr bool
	}{
		{
			name:    "by instance model is ok",
			detail:  validAppendDetailByModel(),
			wantErr: false,
		},
		{
			name: "by instance type is ok",
			detail: AppendReturnPlanDetail{
				ObsProject:   enumor.ObsProjectNormal,
				PlanTime:     "2024-11-12",
				RegionID:     "ap-shanghai",
				InstanceType: "标准型SA2",
				CoreTypeName: "小核心",
				CoreAmount:   80,
			},
			wantErr: false,
		},
		{
			name: "both instance model and type is invalid",
			detail: AppendReturnPlanDetail{
				ObsProject:    enumor.ObsProjectNormal,
				PlanTime:      "2024-11-12",
				RegionID:      "ap-shanghai",
				InstanceModel: "SA2.LARGE8",
				CvmAmount:     10,
				InstanceType:  "标准型SA2",
				CoreTypeName:  "小核心",
				CoreAmount:    80,
			},
			wantErr: true,
		},
		{
			name: "neither instance model nor type is invalid",
			detail: AppendReturnPlanDetail{
				ObsProject: enumor.ObsProjectNormal,
				PlanTime:   "2024-11-12",
				RegionID:   "ap-shanghai",
			},
			wantErr: true,
		},
		{
			name: "instance model without cvm_amount is invalid",
			detail: AppendReturnPlanDetail{
				ObsProject:    enumor.ObsProjectNormal,
				PlanTime:      "2024-11-12",
				RegionID:      "ap-shanghai",
				InstanceModel: "SA2.LARGE8",
			},
			wantErr: true,
		},
		{
			name: "instance type without core_type_name is invalid",
			detail: AppendReturnPlanDetail{
				ObsProject:   enumor.ObsProjectNormal,
				PlanTime:     "2024-11-12",
				RegionID:     "ap-shanghai",
				InstanceType: "标准型SA2",
				CoreAmount:   80,
			},
			wantErr: true,
		},
		{
			name: "future spring form is ok",
			detail: AppendReturnPlanDetail{
				ObsProject:    "2099春节保障",
				PlanTime:      "2024-11-12",
				RegionID:      "ap-shanghai",
				InstanceModel: "SA2.LARGE8",
				CvmAmount:     10,
			},
			wantErr: false,
		},
		{
			name: "future dissolve form is ok",
			detail: AppendReturnPlanDetail{
				ObsProject:    "2098机房裁撤",
				PlanTime:      "2024-11-12",
				RegionID:      "ap-shanghai",
				InstanceModel: "SA2.LARGE8",
				CvmAmount:     10,
			},
			wantErr: false,
		},
		{
			name: "illegal obs project is invalid",
			detail: AppendReturnPlanDetail{
				ObsProject:    "2029春保",
				PlanTime:      "2024-11-12",
				RegionID:      "ap-shanghai",
				InstanceModel: "SA2.LARGE8",
				CvmAmount:     10,
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.detail.Validate()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
