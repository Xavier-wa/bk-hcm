/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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

package ziyan

import (
	"testing"

	"hcm/pkg/thirdparty/cvmapi"

	"github.com/shopspring/decimal"
)

func TestCalcTechClassResAmt_MemoryType(t *testing.T) {
	tests := []struct {
		name     string
		item     cvmapi.QueryCvmInstanceTypeItem
		expected decimal.Decimal
	}{
		// 8.1.1 测试内存型机型资源量计算：验证返回值 = RamAmount
		{
			name: "内存型：返回RamAmount",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "内存型",
				CPUAmount:            8,
				RamAmount:            64,
				DiskBlockNum:         1,
				DiskBlockSize:        512,
			},
			expected: decimal.NewFromFloat(64),
		},
		{
			name: "内存型：RamAmount为小数",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "内存型",
				CPUAmount:            16,
				RamAmount:            128.5,
				DiskBlockNum:         2,
				DiskBlockSize:        1024,
			},
			expected: decimal.NewFromFloat(128.5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.item.CalcTechClassResAmt()
			if err != nil {
				t.Errorf("CalcTechClassResAmt() unexpected err: %v, item: %+v", err, tt.item)
			}
			if result.Cmp(tt.expected) != 0 {
				t.Errorf("CalcTechClassResAmt() = %v, want %v, item: %+v", result, tt.expected, tt.item)
			}
		})
	}
}

func TestCalcTechClassResAmt_DiskType(t *testing.T) {
	tests := []struct {
		name     string
		item     cvmapi.QueryCvmInstanceTypeItem
		expected decimal.Decimal
	}{
		// 8.1.2 测试高IO型机型资源量计算：验证返回值 = DiskBlockNum（单位：块），不受 DiskBlockSize 影响
		{
			name: "高IO型：返回盘块数",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "高IO型",
				CPUAmount:            16,
				RamAmount:            64,
				DiskBlockNum:         4,
				DiskBlockSize:        512,
			},
			expected: decimal.NewFromFloat(4),
		},
		{
			name: "高IO型：盘块数不受单盘容量影响",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "高IO型",
				CPUAmount:            32,
				RamAmount:            128,
				DiskBlockNum:         2,
				DiskBlockSize:        4096,
			},
			expected: decimal.NewFromFloat(2),
		},
		// 8.1.3 测试大数据型机型资源量计算：验证返回值 = DiskBlockNum * DiskBlockSize / 1024
		{
			name: "大数据：返回磁盘总容量(TB)",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "大数据",
				CPUAmount:            32,
				RamAmount:            128,
				DiskBlockNum:         8,
				DiskBlockSize:        1024,
			},
			expected: decimal.NewFromFloat(8.0), // 8 * 1024 / 1024 = 8.0 TB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.item.CalcTechClassResAmt()
			if err != nil {
				t.Errorf("CalcTechClassResAmt() unexpected err: %v, item: %+v", err, tt.item)
			}
			if result.Cmp(tt.expected) != 0 {
				t.Errorf("CalcTechClassResAmt() = %v, want %v, item: %+v", result, tt.expected, tt.item)
			}
		})
	}
}

func TestCalcTechClassResAmt_OtherTypes(t *testing.T) {
	tests := []struct {
		name     string
		item     cvmapi.QueryCvmInstanceTypeItem
		expected decimal.Decimal
	}{
		// 8.1.4 测试其他类型机型资源量计算：验证返回值 = CPUAmount（fallback）
		{
			name: "标准型：fallback到CPUAmount",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "标准型",
				CPUAmount:            16,
				RamAmount:            64,
				DiskBlockNum:         2,
				DiskBlockSize:        512,
			},
			expected: decimal.NewFromFloat(16),
		},
		{
			name: "高主频：fallback到CPUAmount",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "高主频",
				CPUAmount:            32,
				RamAmount:            256,
				DiskBlockNum:         4,
				DiskBlockSize:        1024,
			},
			expected: decimal.NewFromFloat(32),
		},
		{
			name: "推理GPU：fallback到CPUAmount（TODO: 等效L20卡数）",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "推理GPU",
				CPUAmount:            64,
				RamAmount:            128,
				DiskBlockNum:         0,
				DiskBlockSize:        0,
			},
			expected: decimal.NewFromFloat(64),
		},
		{
			name: "训练GPU：fallback到CPUAmount（TODO: 等效L20卡数）",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "训练GPU",
				CPUAmount:            128,
				RamAmount:            256,
				DiskBlockNum:         0,
				DiskBlockSize:        0,
			},
			expected: decimal.NewFromFloat(128),
		},
		{
			name: "GPU-其他：fallback到CPUAmount（TODO: 等效L20卡数）",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "GPU-其他",
				CPUAmount:            32,
				RamAmount:            64,
				DiskBlockNum:         0,
				DiskBlockSize:        0,
			},
			expected: decimal.NewFromFloat(32),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.item.CalcTechClassResAmt()
			if err != nil {
				t.Errorf("CalcTechClassResAmt() unexpected err: %v, item: %+v", err, tt.item)
			}
			if result.Cmp(tt.expected) != 0 {
				t.Errorf("CalcTechClassResAmt() = %v, want %v, item: %+v", result, tt.expected, tt.item)
			}
		})
	}
}

func TestCalcTechClassResAmt_UnknownType(t *testing.T) {
	tests := []struct {
		name string
		item cvmapi.QueryCvmInstanceTypeItem
	}{
		// 8.1.5 测试未知分类机型资源量计算：验证返回 error，不再 fallback 到 CPUAmount
		{
			name: "未知分类：直接报错",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "未知类型",
				CPUAmount:            8,
				RamAmount:            32,
				DiskBlockNum:         1,
				DiskBlockSize:        256,
			},
		},
		{
			name: "空分类：直接报错",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "",
				CPUAmount:            4,
				RamAmount:            16,
				DiskBlockNum:         1,
				DiskBlockSize:        128,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.item.CalcTechClassResAmt(); err == nil {
				t.Errorf("CalcTechClassResAmt() want err, got nil, item: %+v", tt.item)
			}
		})
	}
}

func TestCalcTechClassResAmt_DiskFallback(t *testing.T) {
	tests := []struct {
		name string
		item cvmapi.QueryCvmInstanceTypeItem
	}{
		// 8.1.6 测试磁盘信息异常时：验证返回 error，不再 fallback 到 CPUAmount
		{
			name: "高IO型但DiskBlockNum为0：直接报错",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "高IO型",
				CPUAmount:            16,
				RamAmount:            64,
				DiskBlockNum:         0,
				DiskBlockSize:        512,
			},
		},
		{
			name: "大数据型但DiskBlockNum为0：直接报错",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "大数据",
				CPUAmount:            32,
				RamAmount:            128,
				DiskBlockNum:         0,
				DiskBlockSize:        512,
			},
		},
		{
			name: "大数据型但DiskBlockSize为0：直接报错",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "大数据",
				CPUAmount:            32,
				RamAmount:            128,
				DiskBlockNum:         4,
				DiskBlockSize:        0,
			},
		},
		{
			name: "大数据型但磁盘信息都为负数：直接报错",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "大数据",
				CPUAmount:            32,
				RamAmount:            128,
				DiskBlockNum:         -1,
				DiskBlockSize:        -512,
			},
		},

		// 8.1.7 测试磁盘信息缺失时：验证返回 error，不再 fallback 到 CPUAmount
		{
			name: "高IO型但磁盘信息为默认值：直接报错",
			item: cvmapi.QueryCvmInstanceTypeItem{
				CvmInstanceTypeClass: "高IO型",
				CPUAmount:            16,
				RamAmount:            64,
				// DiskBlockNum和DiskBlockSize为默认零值
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.item.CalcTechClassResAmt(); err == nil {
				t.Errorf("CalcTechClassResAmt() want err, got nil, item: %+v", tt.item)
			}
		})
	}
}
