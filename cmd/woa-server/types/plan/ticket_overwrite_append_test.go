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

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/tools/times"

	"github.com/stretchr/testify/assert"
)

// validOverwriteFilter 构造一个合法的覆盖筛选条件。
func validOverwriteFilter() *ResPlanOverwriteFilter {
	return &ResPlanOverwriteFilter{
		ObsProjects:     []enumor.ObsProject{enumor.ObsProjectNormal},
		ExpectTimeRange: &times.DateRange{Start: "2024-09-01", End: "2024-12-31"},
	}
}

// TestOverwriteAppendResPlanTicketReq_Validate 覆盖 overwrite/demands 组合与筛选条件必选性校验。
func TestOverwriteAppendResPlanTicketReq_Validate(t *testing.T) {
	testCases := []struct {
		name    string
		req     *OverwriteAppendResPlanTicketReq
		wantErr bool
	}{
		{
			name: "overwrite false and no demands is invalid",
			req: &OverwriteAppendResPlanTicketReq{
				Type:      enumor.RPTicketTypeAdjust,
				Overwrite: false,
				Applicant: "zhangsan",
			},
			wantErr: true,
		},
		{
			name: "applicant required",
			req: &OverwriteAppendResPlanTicketReq{
				Type:            enumor.RPTicketTypeAdjust,
				Overwrite:       true,
				OverwriteFilter: validOverwriteFilter(),
			},
			wantErr: true,
		},
		{
			name: "overwrite true without filter is invalid",
			req: &OverwriteAppendResPlanTicketReq{
				Type:      enumor.RPTicketTypeAdjust,
				Overwrite: true,
				Applicant: "zhangsan",
			},
			wantErr: true,
		},
		{
			name: "overwrite only with valid filter is ok",
			req: &OverwriteAppendResPlanTicketReq{
				Type:            enumor.RPTicketTypeAdjust,
				Overwrite:       true,
				OverwriteFilter: validOverwriteFilter(),
				Applicant:       "zhangsan",
			},
			wantErr: false,
		},
		{
			name: "filter missing obs_projects is invalid",
			req: &OverwriteAppendResPlanTicketReq{
				Type:      enumor.RPTicketTypeAdjust,
				Overwrite: true,
				OverwriteFilter: &ResPlanOverwriteFilter{
					ExpectTimeRange: &times.DateRange{Start: "2024-09-01", End: "2024-12-31"},
				},
				Applicant: "zhangsan",
			},
			wantErr: true,
		},
		{
			name: "filter missing expect_time_range is invalid",
			req: &OverwriteAppendResPlanTicketReq{
				Type:      enumor.RPTicketTypeAdjust,
				Overwrite: true,
				OverwriteFilter: &ResPlanOverwriteFilter{
					ObsProjects: []enumor.ObsProject{enumor.ObsProjectNormal},
				},
				Applicant: "zhangsan",
			},
			wantErr: true,
		},
		{
			name: "filter end earlier than start is invalid",
			req: &OverwriteAppendResPlanTicketReq{
				Type:      enumor.RPTicketTypeAdjust,
				Overwrite: true,
				OverwriteFilter: &ResPlanOverwriteFilter{
					ObsProjects:     []enumor.ObsProject{enumor.ObsProjectNormal},
					ExpectTimeRange: &times.DateRange{Start: "2024-12-31", End: "2024-09-01"},
				},
				Applicant: "zhangsan",
			},
			wantErr: true,
		},
		{
			name: "type required",
			req: &OverwriteAppendResPlanTicketReq{
				Overwrite:       true,
				OverwriteFilter: validOverwriteFilter(),
				Applicant:       "zhangsan",
			},
			wantErr: true,
		},
		{
			name: "type invalid",
			req: &OverwriteAppendResPlanTicketReq{
				Type:            enumor.RPTicketTypeDelay,
				Overwrite:       true,
				OverwriteFilter: validOverwriteFilter(),
				Applicant:       "zhangsan",
			},
			wantErr: true,
		},
		{
			name: "type budget_declare ok",
			req: &OverwriteAppendResPlanTicketReq{
				Type:            enumor.RPTicketTypeBudgetDeclare,
				Overwrite:       true,
				OverwriteFilter: validOverwriteFilter(),
				Applicant:       "zhangsan",
			},
			wantErr: false,
		},
		{
			name: "type add ok",
			req: &OverwriteAppendResPlanTicketReq{
				Type:            enumor.RPTicketTypeAdd,
				Overwrite:       true,
				OverwriteFilter: validOverwriteFilter(),
				Applicant:       "zhangsan",
			},
			wantErr: false,
		},
		{
			name: "type delete ok",
			req: &OverwriteAppendResPlanTicketReq{
				Type:            enumor.RPTicketTypeDelete,
				Overwrite:       true,
				OverwriteFilter: validOverwriteFilter(),
				Applicant:       "zhangsan",
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
