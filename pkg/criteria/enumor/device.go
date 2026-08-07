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

package enumor

import (
	"fmt"

	"hcm/pkg/criteria/constant"
)

// DiskType is disk type.
type DiskType string

const (
	// DiskPremium disk cloud premium.
	DiskPremium DiskType = "CLOUD_PREMIUM"
	// DiskSSD disk cloud ssd.
	DiskSSD DiskType = "CLOUD_SSD"
	// DiskLocalBasic disk local basic.
	DiskLocalBasic DiskType = "LOCAL_BASIC"
	// DiskUnknown 当CRP出现拆单时，会出现云盘类型为空的情况，此时需在某些场景中特殊处理
	DiskUnknown DiskType = "UNKNOWN"
)

// Validate DiskType.
func (t DiskType) Validate() error {
	switch t {
	case DiskPremium:
	case DiskSSD:
	case DiskLocalBasic:
	default:
		return fmt.Errorf("unsupported disk type: %s", t)
	}

	return nil
}

// GetWithDefault get DiskType, return default value if DiskType Validate is error.
func (t DiskType) GetWithDefault() DiskType {
	if err := t.Validate(); err != nil {
		return DiskPremium
	}

	return t
}

// diskTypeNameMap records disk type corresponding name.
var diskTypeNameMap = map[DiskType]string{
	DiskPremium:    "高性能云硬盘",
	DiskSSD:        "SSD云硬盘",
	DiskLocalBasic: "本地盘",
	DiskUnknown:    "",
}

// Name return disk type name.
func (t DiskType) Name() string {
	return diskTypeNameMap[t]
}

// GetDiskTypeMembers get DiskType's members.
func GetDiskTypeMembers() []DiskType {
	return []DiskType{DiskPremium, DiskSSD}
}

// GetDiskTypeFromCrpName get DiskType from crp disk type name.
func GetDiskTypeFromCrpName(name string) (DiskType, error) {
	for typ, n := range diskTypeNameMap {
		if string(typ) == name {
			return typ, nil
		}

		// 兼容crpName返回为中文的情况
		if n == name {
			return typ, nil
		}
	}
	return "", fmt.Errorf("unsupported disk type name: %s", name)
}

// DiskSpec disk specifications
type DiskSpec struct {
	DiskType DiskType `json:"disk_type"`
	DiskSize uint     `json:"disk_size"`
	DiskNum  uint     `json:"disk_num"`
}

// Validate DiskType.
func (t DiskSpec) Validate() error {
	if err := t.DiskType.Validate(); err != nil {
		return err
	}

	if t.DiskSize < 0 || t.DiskSize > constant.DataDiskMaxSize {
		return fmt.Errorf("invalid disk size: %d", t.DiskSize)
	}

	if t.DiskNum < 0 || t.DiskNum > constant.DataDiskTotalNum {
		return fmt.Errorf("invalid disk num: %d", t.DiskNum)
	}

	return nil
}

// DeviceFamily CVM机型族
type DeviceFamily string

const (
	// DeviceFamilyStandard 标准型
	DeviceFamilyStandard DeviceFamily = "标准型"
)

// DeviceTypeSource 机型来源
type DeviceTypeSource string

const (
	// DeviceTypeSourceSync 自动同步
	DeviceTypeSourceSync DeviceTypeSource = "sync"
	// DeviceTypeSourceManually 手动添加
	DeviceTypeSourceManually DeviceTypeSource = "manually"
)

// CvmTechnicalClass CVM机型技术分类
type CvmTechnicalClass string

const (
	// CvmTechnicalClassStandard 标准型
	CvmTechnicalClassStandard CvmTechnicalClass = "标准型"
	// CvmTechnicalClassMemory 内存型
	CvmTechnicalClassMemory CvmTechnicalClass = "内存型"
	// CvmTechnicalClassHighIO 高IO型
	CvmTechnicalClassHighIO CvmTechnicalClass = "高IO型"
	// CvmTechnicalClassBigData 大数据
	CvmTechnicalClassBigData CvmTechnicalClass = "大数据"
	// CvmTechnicalClassHighFreq 高主频
	CvmTechnicalClassHighFreq CvmTechnicalClass = "高主频"
	// TODO: 推理GPU、训练GPU、GPU-其他的技术分类资源量是等效L20卡数，目前暂时没数据，等后续接口对接完再实现
	// CvmTechnicalClassInferGPU 推理GPU
	CvmTechnicalClassInferGPU CvmTechnicalClass = "推理GPU"
	// CvmTechnicalClassTrainGPU 训练GPU
	CvmTechnicalClassTrainGPU CvmTechnicalClass = "训练GPU"
	// CvmTechnicalClassGPUDeprecated GPU-其他
	CvmTechnicalClassGPUDeprecated CvmTechnicalClass = "GPU-其他"
)

// Validate 校验CvmTechnicalClass
func (c CvmTechnicalClass) Validate() error {
	switch c {
	case CvmTechnicalClassStandard,
		CvmTechnicalClassMemory,
		CvmTechnicalClassHighIO,
		CvmTechnicalClassBigData,
		CvmTechnicalClassHighFreq,
		CvmTechnicalClassInferGPU,
		CvmTechnicalClassTrainGPU,
		CvmTechnicalClassGPUDeprecated:
		return nil
	default:
		return fmt.Errorf("unsupported technical class: %s", c)
	}
}

// Validate DeviceTypeSource.
func (s DeviceTypeSource) Validate() error {
	switch s {
	case DeviceTypeSourceSync, DeviceTypeSourceManually:
	default:
		return fmt.Errorf("unsupported device type source: %s", s)
	}
	return nil
}
