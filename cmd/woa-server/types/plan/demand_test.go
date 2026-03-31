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
