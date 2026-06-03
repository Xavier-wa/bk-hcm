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

package dissolve

import (
	"strings"
	"testing"

	"hcm/pkg/criteria/enumor"
	cvt "hcm/pkg/tools/converter"
)

// TestUpsertConfigReqValidate_QuotaCoefficient 测试 UpsertConfigReq.Validate() 中配额系数校验
func TestUpsertConfigReqValidate_QuotaCoefficient(t *testing.T) {
	tests := []struct {
		name    string
		req     UpsertConfigReq
		wantErr bool
		errMsg  string
	}{
		{
			name: "配额系数最小边界有效 1",
			req: UpsertConfigReq{
				QuotaCoefficient: cvt.ValToPtr(1.0),
			},
			wantErr: false,
		},
		{
			name: "配额系数最大边界有效 100",
			req: UpsertConfigReq{
				QuotaCoefficient: cvt.ValToPtr(100.0),
			},
			wantErr: false,
		},
		{
			name: "配额系数中间值有效 65",
			req: UpsertConfigReq{
				QuotaCoefficient: cvt.ValToPtr(65.0),
			},
			wantErr: false,
		},
		{
			name: "配额系数低于最小值无效 0.5",
			req: UpsertConfigReq{
				QuotaCoefficient: cvt.ValToPtr(0.5),
			},
			wantErr: true,
			errMsg:  "quota_coefficient must between 1 and 100",
		},
		{
			name: "配额系数超过最大值无效 101",
			req: UpsertConfigReq{
				QuotaCoefficient: cvt.ValToPtr(101.0),
			},
			wantErr: true,
			errMsg:  "quota_coefficient must between 1 and 100",
		},
		{
			name: "配额系数为0无效",
			req: UpsertConfigReq{
				QuotaCoefficient: cvt.ValToPtr(0.0),
			},
			wantErr: true,
			errMsg:  "quota_coefficient must between 1 and 100",
		},
		{
			name: "配额系数为负数无效",
			req: UpsertConfigReq{
				QuotaCoefficient: cvt.ValToPtr(-5.0),
			},
			wantErr: true,
			errMsg:  "quota_coefficient must between 1 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("UpsertConfigReq.Validate() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("UpsertConfigReq.Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("UpsertConfigReq.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestUpsertConfigReqValidate_QuotaOffsets 测试 UpsertConfigReq.Validate() 中偏移配置校验
func TestUpsertConfigReqValidate_QuotaOffsets(t *testing.T) {
	tests := []struct {
		name    string
		req     UpsertConfigReq
		wantErr bool
		errMsg  string
	}{
		{
			name: "偏移配置有效",
			req: UpsertConfigReq{
				QuotaOffsets: []QuotaOffsetItem{
					{BkBizID: 100, Offset: 50, Type: enumor.DissolveQuotaOffsetTypeIncrease, Memo: "测试调增"},
					{BkBizID: 200, Offset: 30, Type: enumor.DissolveQuotaOffsetTypeDecrease, Memo: "测试调减"},
				},
			},
			wantErr: false,
		},
		{
			name: "偏移配置 bizID 为0无效",
			req: UpsertConfigReq{
				QuotaOffsets: []QuotaOffsetItem{
					{BkBizID: 0, Offset: 50, Type: enumor.DissolveQuotaOffsetTypeIncrease, Memo: "测试"},
				},
			},
			wantErr: true,
			errMsg:  "invalid bk_biz_id",
		},
		{
			name: "偏移配置 bizID 为负数无效",
			req: UpsertConfigReq{
				QuotaOffsets: []QuotaOffsetItem{
					{BkBizID: -1, Offset: 50, Type: enumor.DissolveQuotaOffsetTypeIncrease, Memo: "测试"},
				},
			},
			wantErr: true,
			errMsg:  "invalid bk_biz_id",
		},
		{
			name: "偏移配置 bizID 重复无效",
			req: UpsertConfigReq{
				QuotaOffsets: []QuotaOffsetItem{
					{BkBizID: 100, Offset: 50, Type: enumor.DissolveQuotaOffsetTypeIncrease, Memo: "测试1"},
					{BkBizID: 100, Offset: 30, Type: enumor.DissolveQuotaOffsetTypeDecrease, Memo: "测试2"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate bk_biz_id",
		},
		{
			name: "偏移配置 type 无效",
			req: UpsertConfigReq{
				QuotaOffsets: []QuotaOffsetItem{
					{BkBizID: 100, Offset: 50, Type: "invalid", Memo: "测试"},
				},
			},
			wantErr: true,
			errMsg:  "unsupported dissolve quota offset type",
		},
		{
			name: "偏移配置 offset 为负数无效",
			req: UpsertConfigReq{
				QuotaOffsets: []QuotaOffsetItem{
					{BkBizID: 100, Offset: -50, Type: enumor.DissolveQuotaOffsetTypeIncrease, Memo: "测试"},
				},
			},
			wantErr: true,
			errMsg:  "offset must be non-negative",
		},
		{
			name: "偏移配置 memo 超过 512 字符无效",
			req: UpsertConfigReq{
				QuotaOffsets: []QuotaOffsetItem{
					{BkBizID: 100, Offset: 50, Type: enumor.DissolveQuotaOffsetTypeIncrease, Memo: strings.Repeat("a", 513)},
				},
			},
			wantErr: true,
			errMsg:  "memo exceeds 512 characters",
		},
		{
			name: "偏移配置 memo 正好 512 字符有效",
			req: UpsertConfigReq{
				QuotaOffsets: []QuotaOffsetItem{
					{BkBizID: 100, Offset: 50, Type: enumor.DissolveQuotaOffsetTypeIncrease, Memo: strings.Repeat("a", 512)},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("UpsertConfigReq.Validate() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("UpsertConfigReq.Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("UpsertConfigReq.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestUpdateDissolveQuotaOffsetReqValidate 测试 UpdateDissolveQuotaOffsetReq.Validate() 方法
func TestUpdateDissolveQuotaOffsetReqValidate(t *testing.T) {
	int64Ptr := func(v int64) *int64 { return &v }

	tests := []struct {
		name    string
		req     UpdateDissolveQuotaOffsetReq
		wantErr bool
		errMsg  string
	}{
		{
			name: "有效的调增请求",
			req: UpdateDissolveQuotaOffsetReq{
				Offset: int64Ptr(50),
				Type:   enumor.DissolveQuotaOffsetTypeIncrease,
				Memo:   "测试调增",
			},
			wantErr: false,
		},
		{
			name: "有效的调减请求",
			req: UpdateDissolveQuotaOffsetReq{
				Offset: int64Ptr(30),
				Type:   enumor.DissolveQuotaOffsetTypeDecrease,
				Memo:   "测试调减",
			},
			wantErr: false,
		},
		{
			name: "空 memo 有效",
			req: UpdateDissolveQuotaOffsetReq{
				Offset: int64Ptr(50),
				Type:   enumor.DissolveQuotaOffsetTypeIncrease,
				Memo:   "",
			},
			wantErr: false,
		},
		{
			name: "type 无效值",
			req: UpdateDissolveQuotaOffsetReq{
				Offset: int64Ptr(50),
				Type:   "invalid",
				Memo:   "测试",
			},
			wantErr: true,
			errMsg:  "unsupported dissolve quota offset type",
		},
		{
			name: "type 为空无效",
			req: UpdateDissolveQuotaOffsetReq{
				Offset: int64Ptr(50),
				Type:   "",
				Memo:   "测试",
			},
			wantErr: true,
			errMsg:  "type",
		},
		{
			name: "memo 超过512字符无效",
			req: UpdateDissolveQuotaOffsetReq{
				Offset: int64Ptr(50),
				Type:   enumor.DissolveQuotaOffsetTypeIncrease,
				Memo:   strings.Repeat("a", 513),
			},
			wantErr: true,
			errMsg:  "memo",
		},
		{
			name: "memo 正好512字符有效",
			req: UpdateDissolveQuotaOffsetReq{
				Offset: int64Ptr(50),
				Type:   enumor.DissolveQuotaOffsetTypeIncrease,
				Memo:   strings.Repeat("a", 512),
			},
			wantErr: false,
		},
		{
			name: "offset 为0有效",
			req: UpdateDissolveQuotaOffsetReq{
				Offset: int64Ptr(0),
				Type:   enumor.DissolveQuotaOffsetTypeIncrease,
				Memo:   "测试",
			},
			wantErr: false,
		},
		{
			name: "offset 为 nil 无效",
			req: UpdateDissolveQuotaOffsetReq{
				Offset: nil,
				Type:   enumor.DissolveQuotaOffsetTypeIncrease,
				Memo:   "测试",
			},
			wantErr: true,
			errMsg:  "Offset",
		},
		{
			name: "offset 为负数无效",
			req: UpdateDissolveQuotaOffsetReq{
				Offset: int64Ptr(-1),
				Type:   enumor.DissolveQuotaOffsetTypeIncrease,
				Memo:   "测试",
			},
			wantErr: true,
			errMsg:  "Offset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateDissolveQuotaOffsetReq.Validate() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("UpdateDissolveQuotaOffsetReq.Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("UpdateDissolveQuotaOffsetReq.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}
