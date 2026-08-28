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
	"strings"

	dt "hcm/pkg/api/core/cloud/device-type"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// IsDeviceMatched return whether each device type in deviceTypeSlice can use deviceType's resource plan.
func (c *Controller) IsDeviceMatched(kt *kit.Kit, deviceTypeSlice []string, deviceType string) ([]bool, error) {
	// get device type map.
	deviceTypeMap, err := c.deviceTypesMap.GetDeviceTypes(kt)
	if err != nil {
		logs.Errorf("failed to get device type map, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	result := make([]bool, len(deviceTypeSlice))
	for idx, ele := range deviceTypeSlice {
		// if ele and device type are equal, then they are matched.
		if ele == deviceType {
			result[idx] = true
			continue
		}

		left, ok := deviceTypeMap[ele]
		if !ok {
			continue
		}

		right, ok := deviceTypeMap[deviceType]
		if !ok {
			continue
		}

		if isGpuWithoutRealCard(left) {
			logs.Warnf("gpu device type missing real gpu type, device_type: %s, gpu_type: %s, rid: %s",
				ele, left.GpuType, kt.Rid)
		}
		if isGpuWithoutRealCard(right) {
			logs.Warnf("gpu device type missing real gpu type, device_type: %s, gpu_type: %s, rid: %s",
				deviceType, right.GpuType, kt.Rid)
		}

		result[idx] = matchDevicePair(left, right)
	}

	return result, nil
}

// validateOperateDeviceGpuCard checks the operation target before apply / recycle / resize verify.
// GPU class without a real card type must fail the current operation.
func (c *Controller) validateOperateDeviceGpuCard(kt *kit.Kit, deviceType string) error {
	deviceTypeMap, err := c.deviceTypesMap.GetDeviceTypes(kt)
	if err != nil {
		logs.Errorf("failed to get device type map, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	spec, ok := deviceTypeMap[deviceType]
	if err := validateDeviceGpuCard(deviceType, spec, ok); err != nil {
		logs.Errorf("validate operate device gpu card failed, err: %v, device_type: %s, rid: %s",
			err, deviceType, kt.Rid)
		return err
	}
	return nil
}

// validateDeviceGpuCard returns an error when the device exists, is GPU class, and has no real card type.
func validateDeviceGpuCard(deviceType string, spec dt.DistinctDeviceType, exists bool) error {
	if !exists {
		return nil
	}
	if isGpuWithoutRealCard(spec) {
		return errf.Newf(errf.InvalidParameter, "GPU 类机型缺少真实卡类型, device_type: %s", deviceType)
	}
	return nil
}

// isGpuWithoutRealCard reports whether the spec is GPU class but gpu_type is empty or 「无」.
func isGpuWithoutRealCard(spec dt.DistinctDeviceType) bool {
	return enumor.CvmTechnicalClass(spec.TechnicalClass).IsGPUClass() &&
		enumor.ClassifyGpuType(spec.GpuType) != enumor.GpuTypeKindReal
}

// matchDevicePair decides whether two cached specs can wildcard. Same-name is handled by the caller.
func matchDevicePair(left, right dt.DistinctDeviceType) bool {
	leftGPU := enumor.CvmTechnicalClass(left.TechnicalClass).IsGPUClass()
	rightGPU := enumor.CvmTechnicalClass(right.TechnicalClass).IsGPUClass()

	// 匹配机型中有 GPU 机型，若要通配，需两侧均为 GPU 机型，且卡类型均为真实卡类型且相等
	if leftGPU || rightGPU {
		if !leftGPU || !rightGPU {
			return false
		}
		if enumor.ClassifyGpuType(left.GpuType) != enumor.GpuTypeKindReal ||
			enumor.ClassifyGpuType(right.GpuType) != enumor.GpuTypeKindReal {
			return false
		}
		return strings.TrimSpace(left.GpuType) == strings.TrimSpace(right.GpuType)
	}

	// 非 GPU 类机型，仅按 TechnicalClass 和 CoreType 匹配
	return left.TechnicalClass == right.TechnicalClass && left.CoreType == right.CoreType
}
