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

	devicetype "hcm/pkg/api/core/cloud/device-type"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/thirdparty/cvmapi"

	"github.com/shopspring/decimal"
)

var baseDeviceType = devicetype.DeviceType{
	DeviceType:      "S5.MEDIUM4",
	DeviceTypeClass: cvmapi.InstanceTypeClass(enumor.CvmTechnicalClassStandard),
	DeviceClass:     "标准型S5",
	DeviceFamily:    "S5",
	CoreType:        "standard",
	CpuCore:         4,
	Memory:          16,
	GpuAmount:       0,
	TechnicalClass:  string(enumor.CvmTechnicalClassStandard),
	TechClassResAmt: decimal.NewFromFloat(4.0), // CPUAmount = 4
	Source:          "sync",
	GenerationType:  "S5",
}

func TestIsDeviceTypeChanged_TechClassResAmtChanged(t *testing.T) {
	tests := []struct {
		name     string
		cloud    devicetype.DeviceType
		db       devicetype.DeviceType
		expected bool
	}{
		{
			name: "tech_class_res_amt变化：内存型返回值不同",
			cloud: devicetype.DeviceType{
				DeviceType:      "M5.LARGE32",
				DeviceTypeClass: cvmapi.InstanceTypeClass(enumor.CvmTechnicalClassMemory),
				DeviceClass:     "内存型M5",
				DeviceFamily:    "M5",
				CoreType:        "memory",
				CpuCore:         8,
				Memory:          256,
				GpuAmount:       0,
				TechnicalClass:  string(enumor.CvmTechnicalClassMemory),
				TechClassResAmt: decimal.NewFromFloat(256.0), // RamAmount = 256
				Source:          "sync",
				GenerationType:  "M5",
			},
			db: devicetype.DeviceType{
				DeviceType:      "M5.LARGE32",
				DeviceTypeClass: cvmapi.InstanceTypeClass(enumor.CvmTechnicalClassMemory),
				DeviceClass:     "内存型M5",
				DeviceFamily:    "M5",
				CoreType:        "memory",
				CpuCore:         8,
				Memory:          256,
				GpuAmount:       0,
				TechnicalClass:  string(enumor.CvmTechnicalClassMemory),
				TechClassResAmt: decimal.NewFromFloat(8.0), // 旧值 = CPUAmount = 8
				Source:          "sync",
				GenerationType:  "M5",
			},
			expected: true,
		},
		{
			name: "tech_class_res_amt变化：高IO型返回值不同",
			cloud: devicetype.DeviceType{
				DeviceType:      "I5.LARGE16",
				DeviceTypeClass: cvmapi.InstanceTypeClass(enumor.CvmTechnicalClassHighIO),
				CpuCore:         16,
				Memory:          64,
				TechnicalClass:  string(enumor.CvmTechnicalClassHighIO),
				TechClassResAmt: decimal.NewFromFloat(2.0), // DiskBlockNum * DiskBlockSize / 1024 = 4 * 512 / 1024 = 2.0
				Source:          "sync",
				GenerationType:  "I5",
			},
			db: devicetype.DeviceType{
				DeviceType:      "I5.LARGE16",
				DeviceTypeClass: cvmapi.InstanceTypeClass(enumor.CvmTechnicalClassHighIO),
				CpuCore:         16,
				Memory:          64,
				TechnicalClass:  string(enumor.CvmTechnicalClassHighIO),
				TechClassResAmt: decimal.NewFromFloat(16.0), // 旧值 = CPUAmount = 16
				Source:          "sync",
				GenerationType:  "I5",
			},
			expected: true,
		},
		{
			name: "tech_class_res_amt变化：从内存型变为高IO型",
			cloud: devicetype.DeviceType{
				DeviceType:      "I3.LARGE8",
				TechnicalClass:  string(enumor.CvmTechnicalClassHighIO),
				TechClassResAmt: decimal.NewFromFloat(1.0),
				Source:          "sync",
			},
			db: devicetype.DeviceType{
				DeviceType:      "I3.LARGE8",
				TechnicalClass:  string(enumor.CvmTechnicalClassMemory),
				TechClassResAmt: decimal.NewFromFloat(64.0), // 内存型旧值
				Source:          "sync",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDeviceTypeChanged(tt.cloud, tt.db)
			if result != tt.expected {
				t.Errorf("isDeviceTypeChanged() = %v, want %v, cloud.TechClassResAmt=%v, db.TechClassResAmt=%v, diff=%v",
					result, tt.expected, tt.cloud.TechClassResAmt, tt.db.TechClassResAmt,
					tt.cloud.TechClassResAmt.Sub(tt.db.TechClassResAmt).Abs())
			}
		})
	}
}

func TestIsDeviceTypeChanged_TechClassResAmtUnchanged(t *testing.T) {
	tests := []struct {
		name     string
		cloud    devicetype.DeviceType
		db       devicetype.DeviceType
		expected bool
	}{
		{
			name:     "所有字段相同：返回false",
			cloud:    baseDeviceType,
			db:       baseDeviceType,
			expected: false,
		},
		{
			name: "tech_class_res_amt未变化：标准型都使用CPUAmount",
			cloud: devicetype.DeviceType{
				DeviceType:      "S5.MEDIUM4",
				DeviceTypeClass: cvmapi.InstanceTypeClass(enumor.CvmTechnicalClassStandard),
				CpuCore:         4,
				Memory:          16,
				TechnicalClass:  string(enumor.CvmTechnicalClassStandard),
				TechClassResAmt: decimal.NewFromFloat(4.0), // CPUAmount = 4
				Source:          "sync",
				GenerationType:  "S5",
			},
			db: devicetype.DeviceType{
				DeviceType:      "S5.MEDIUM4",
				DeviceTypeClass: cvmapi.InstanceTypeClass(enumor.CvmTechnicalClassStandard),
				CpuCore:         4,
				Memory:          16,
				TechnicalClass:  string(enumor.CvmTechnicalClassStandard),
				TechClassResAmt: decimal.NewFromFloat(4.0), // 相同
				Source:          "sync",
				GenerationType:  "S5",
			},
			expected: false,
		},
		{
			name: "tech_class_res_amt未变化：内存型都使用RamAmount",
			cloud: devicetype.DeviceType{
				DeviceType:      "M5.LARGE32",
				TechnicalClass:  string(enumor.CvmTechnicalClassMemory),
				TechClassResAmt: decimal.NewFromFloat(256.0),
				Source:          "sync",
			},
			db: devicetype.DeviceType{
				DeviceType:      "M5.LARGE32",
				TechnicalClass:  string(enumor.CvmTechnicalClassMemory),
				TechClassResAmt: decimal.NewFromFloat(256.0), // 相同
				Source:          "sync",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDeviceTypeChanged(tt.cloud, tt.db)
			if result != tt.expected {
				t.Errorf("isDeviceTypeChanged() = %v, want %v, cloud.TechClassResAmt=%v, db.TechClassResAmt=%v, diff=%v",
					result, tt.expected, tt.cloud.TechClassResAmt, tt.db.TechClassResAmt,
					tt.cloud.TechClassResAmt.Sub(tt.db.TechClassResAmt).Abs())
			}
		})
	}
}

func TestIsDeviceTypeChanged_ExactMatch(t *testing.T) {
	tests := []struct {
		name     string
		cloud    devicetype.DeviceType
		db       devicetype.DeviceType
		expected bool
	}{
		{
			name: "精确匹配测试：完全相同，返回false",
			cloud: devicetype.DeviceType{
				DeviceType:      "S5.MEDIUM4",
				TechClassResAmt: decimal.NewFromFloat(4.0),
				Source:          "sync",
			},
			db: devicetype.DeviceType{
				DeviceType:      "S5.MEDIUM4",
				TechClassResAmt: decimal.NewFromFloat(4.0),
				Source:          "sync",
			},
			expected: false,
		},
		{
			name: "精确匹配测试：差值0.005，返回true",
			cloud: devicetype.DeviceType{
				DeviceType:      "S5.MEDIUM4",
				TechClassResAmt: decimal.NewFromFloat(4.005),
				Source:          "sync",
			},
			db: devicetype.DeviceType{
				DeviceType:      "S5.MEDIUM4",
				TechClassResAmt: decimal.NewFromFloat(4.0),
				Source:          "sync",
			},
			expected: true, // 精确比较，0.005 != 0
		},
		{
			name: "精确匹配测试：差值0.01，返回true",
			cloud: devicetype.DeviceType{
				DeviceType:      "S5.MEDIUM4",
				TechClassResAmt: decimal.NewFromFloat(4.01),
				Source:          "sync",
			},
			db: devicetype.DeviceType{
				DeviceType:      "S5.MEDIUM4",
				TechClassResAmt: decimal.NewFromFloat(4.0),
				Source:          "sync",
			},
			expected: true, // 精确比较，0.01 != 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDeviceTypeChanged(tt.cloud, tt.db)
			if result != tt.expected {
				t.Errorf("isDeviceTypeChanged() = %v, want %v, cloud.TechClassResAmt=%v, db.TechClassResAmt=%v, diff=%v",
					result, tt.expected, tt.cloud.TechClassResAmt, tt.db.TechClassResAmt,
					tt.cloud.TechClassResAmt.Sub(tt.db.TechClassResAmt).Abs())
			}
		})
	}
}
