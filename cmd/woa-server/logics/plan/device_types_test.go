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
	"testing"

	dt "hcm/pkg/api/core/cloud/device-type"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
)

func TestCvmTechnicalClassIsGPUClass(t *testing.T) {
	tests := []struct {
		name           string
		technicalClass enumor.CvmTechnicalClass
		want           bool
	}{
		{name: "推理GPU", technicalClass: enumor.CvmTechnicalClassInferGPU, want: true},
		{name: "训练GPU", technicalClass: enumor.CvmTechnicalClassTrainGPU, want: true},
		{name: "GPU-其他", technicalClass: enumor.CvmTechnicalClassGPUDeprecated, want: true},
		{name: "标准型", technicalClass: enumor.CvmTechnicalClassStandard, want: false},
		{name: "空", technicalClass: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.technicalClass.IsGPUClass(); got != tt.want {
				t.Errorf("IsGPUClass() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyGpuType(t *testing.T) {
	tests := []struct {
		name    string
		gpuType string
		want    enumor.GpuTypeKind
	}{
		{name: "空字符串", gpuType: "", want: enumor.GpuTypeKindMissing},
		{name: "空白", gpuType: "  ", want: enumor.GpuTypeKindMissing},
		{name: "无", gpuType: constant.GpuTypeNoneValue, want: enumor.GpuTypeKindNone},
		{name: "无带空格", gpuType: " 无 ", want: enumor.GpuTypeKindNone},
		{name: "真实卡", gpuType: "NVIDIA A100", want: enumor.GpuTypeKindReal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := enumor.ClassifyGpuType(tt.gpuType); got != tt.want {
				t.Errorf("ClassifyGpuType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchDevicePair(t *testing.T) {
	gpuA100 := dt.DistinctDeviceType{
		DeviceType:     "GN10Xp.2XLARGE40",
		TechnicalClass: string(enumor.CvmTechnicalClassInferGPU),
		DeviceFamily:   constant.GpuInstanceClassValue,
		CoreType:       enumor.CoreTypeBig,
		GpuType:        "NVIDIA A100",
	}
	gpuA100OtherFamily := dt.DistinctDeviceType{
		DeviceType:     "GT4.2XLARGE",
		TechnicalClass: string(enumor.CvmTechnicalClassTrainGPU),
		DeviceFamily:   constant.GpuHighFreqInstanceClassValue,
		CoreType:       enumor.CoreTypeSmall,
		GpuType:        "NVIDIA A100",
	}
	gpuV100 := dt.DistinctDeviceType{
		DeviceType:     "GN10X.2XLARGE40",
		TechnicalClass: string(enumor.CvmTechnicalClassInferGPU),
		DeviceFamily:   constant.GpuInstanceClassValue,
		CoreType:       enumor.CoreTypeBig,
		GpuType:        "V100",
	}
	gpuA100Alias := dt.DistinctDeviceType{
		DeviceType:     "GN10Xp.ALIAS",
		TechnicalClass: string(enumor.CvmTechnicalClassInferGPU),
		DeviceFamily:   constant.GpuInstanceClassValue,
		CoreType:       enumor.CoreTypeBig,
		GpuType:        "A100",
	}
	gpuNone := dt.DistinctDeviceType{
		DeviceType:     "GN10Xp.NONE",
		TechnicalClass: string(enumor.CvmTechnicalClassInferGPU),
		DeviceFamily:   constant.GpuInstanceClassValue,
		CoreType:       enumor.CoreTypeBig,
		GpuType:        "无",
	}
	gpuEmpty := dt.DistinctDeviceType{
		DeviceType:     "GN10Xp.EMPTY",
		TechnicalClass: string(enumor.CvmTechnicalClassInferGPU),
		DeviceFamily:   constant.GpuInstanceClassValue,
		CoreType:       enumor.CoreTypeBig,
		GpuType:        "",
	}
	stdSA2 := dt.DistinctDeviceType{
		DeviceType:     "SA2.2XLARGE16",
		TechnicalClass: string(enumor.CvmTechnicalClassStandard),
		DeviceFamily:   "SA2",
		CoreType:       enumor.CoreTypeBig,
	}
	stdSA3 := dt.DistinctDeviceType{
		DeviceType:     "SA3.2XLARGE16",
		TechnicalClass: string(enumor.CvmTechnicalClassStandard),
		DeviceFamily:   "SA3",
		CoreType:       enumor.CoreTypeBig,
	}
	stdSmallCore := dt.DistinctDeviceType{
		DeviceType:     "SA2.SMALL",
		TechnicalClass: string(enumor.CvmTechnicalClassStandard),
		DeviceFamily:   "SA2",
		CoreType:       enumor.CoreTypeSmall,
	}
	gpuFamilyButStd := dt.DistinctDeviceType{
		DeviceType:     "GN.FAMILY.ONLY",
		TechnicalClass: string(enumor.CvmTechnicalClassStandard),
		DeviceFamily:   constant.GpuInstanceClassValue,
		CoreType:       enumor.CoreTypeBig,
		GpuType:        "NVIDIA A100",
	}

	tests := []struct {
		name  string
		left  dt.DistinctDeviceType
		right dt.DistinctDeviceType
		want  bool
	}{
		{name: "同卡跨族可通配", left: gpuA100, right: gpuA100OtherFamily, want: true},
		{name: "不同卡同族不可通配", left: gpuA100, right: gpuV100, want: false},
		{name: "A100不等于NVIDIA A100", left: gpuA100, right: gpuA100Alias, want: false},
		{name: "有卡与无不可通配", left: gpuA100, right: gpuNone, want: false},
		{name: "有卡与空不可通配", left: gpuA100, right: gpuEmpty, want: false},
		{name: "双方无不得互配", left: gpuNone, right: gpuNone, want: false},
		{name: "双方空不得互配", left: gpuEmpty, right: gpuEmpty, want: false},
		{name: "GPU与非GPU不可通配", left: gpuA100, right: stdSA2, want: false},
		{name: "非GPU同技术分类同核心可通配", left: stdSA2, right: stdSA3, want: true},
		{name: "非GPU核心类型不同不可通配", left: stdSA2, right: stdSmallCore, want: false},
		{name: "仅机型族为GPU不按GPU类", left: gpuFamilyButStd, right: stdSA2, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchDevicePair(tt.left, tt.right); got != tt.want {
				t.Errorf("matchDevicePair() = %v, want %v", got, tt.want)
			}
			if got := matchDevicePair(tt.right, tt.left); got != tt.want {
				t.Errorf("matchDevicePair() reverse = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateDeviceGpuCard(t *testing.T) {
	gpuEmpty := dt.DistinctDeviceType{
		DeviceType:     "GN10Xp.EMPTY",
		TechnicalClass: string(enumor.CvmTechnicalClassInferGPU),
		GpuType:        "",
	}
	gpuNone := dt.DistinctDeviceType{
		DeviceType:     "GN10Xp.NONE",
		TechnicalClass: string(enumor.CvmTechnicalClassInferGPU),
		GpuType:        "无",
	}
	gpuReal := dt.DistinctDeviceType{
		DeviceType:     "GN10Xp.2XLARGE40",
		TechnicalClass: string(enumor.CvmTechnicalClassInferGPU),
		GpuType:        "NVIDIA A100",
	}
	std := dt.DistinctDeviceType{
		DeviceType:     "SA2.2XLARGE16",
		TechnicalClass: string(enumor.CvmTechnicalClassStandard),
	}

	tests := []struct {
		name       string
		deviceType string
		spec       dt.DistinctDeviceType
		exists     bool
		wantErr    bool
	}{
		{name: "缓存不存在不通配不报错", deviceType: "UNKNOWN", exists: false, wantErr: false},
		{name: "非GPU不报错", deviceType: std.DeviceType, spec: std, exists: true, wantErr: false},
		{name: "真实卡不报错", deviceType: gpuReal.DeviceType, spec: gpuReal, exists: true, wantErr: false},
		{name: "空卡类型报错", deviceType: gpuEmpty.DeviceType, spec: gpuEmpty, exists: true, wantErr: true},
		{name: "无为非真实卡报错", deviceType: gpuNone.DeviceType, spec: gpuNone, exists: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDeviceGpuCard(tt.deviceType, tt.spec, tt.exists)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDeviceGpuCard() err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "GPU 类机型缺少真实卡类型") {
				t.Errorf("error message %q missing required text", err.Error())
			}
		})
	}
}
