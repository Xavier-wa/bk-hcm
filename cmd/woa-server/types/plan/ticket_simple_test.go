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

	devicetype "hcm/pkg/api/core/cloud/device-type"
	"hcm/pkg/criteria/enumor"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestDeriveCvmScaleFromSimple_ByOs(t *testing.T) {
	in := &CvmSimpleInput{
		DeviceType: "SA2.MEDIUM4",
		Os:         decimalPtr("3"),
	}
	meta := devicetype.DistinctDeviceType{
		DeviceType: "SA2.MEDIUM4",
		CpuCore:    4,
		Memory:     8,
	}
	osOut, cpuOut, memOut, err := deriveCvmScaleFromSimple(in, meta)
	require.NoError(t, err)
	require.NotNil(t, osOut)
	require.Equal(t, "3", osOut.String())
	require.NotNil(t, cpuOut)
	require.Equal(t, int64(12), *cpuOut)
	require.NotNil(t, memOut)
	require.Equal(t, int64(24), *memOut)
}

func TestDeriveCvmScaleFromSimple_ByOsAndCpuCore(t *testing.T) {
	in := &CvmSimpleInput{
		DeviceType: "SA2.MEDIUM4",
		Os:         decimalPtr("3"),
		CpuCore:    int64Ptr(8),
	}
	meta := devicetype.DistinctDeviceType{
		DeviceType: "SA2.MEDIUM4",
		CpuCore:    4,
		Memory:     8,
	}
	osOut, cpuOut, memOut, err := deriveCvmScaleFromSimple(in, meta)
	require.NoError(t, err)
	require.NotNil(t, osOut)
	require.True(t, osOut.Equal(decimal.NewFromInt(3)))
	require.NotNil(t, cpuOut)
	require.Equal(t, int64(8), *cpuOut)
	// mem stays derived from os * meta.Memory unless memory override is set.
	require.NotNil(t, memOut)
	require.Equal(t, int64(24), *memOut)
}

func TestDeriveCvmScaleFromSimple_ByOsAndMemoryOverride(t *testing.T) {
	in := &CvmSimpleInput{
		DeviceType: "SA2.MEDIUM4",
		Os:         decimalPtr("3"),
		Memory:     int64Ptr(100),
	}
	meta := devicetype.DistinctDeviceType{
		DeviceType: "SA2.MEDIUM4",
		CpuCore:    4,
		Memory:     8,
	}
	osOut, cpuOut, memOut, err := deriveCvmScaleFromSimple(in, meta)
	require.NoError(t, err)
	require.True(t, osOut.Equal(decimal.NewFromInt(3)))
	require.Equal(t, int64(12), *cpuOut)
	require.Equal(t, int64(100), *memOut)
}

func TestDeriveCvmScaleFromSimple_ByCpuCoreAndMemoryOverride(t *testing.T) {
	in := &CvmSimpleInput{
		DeviceType: "SA2.MEDIUM4",
		CpuCore:    int64Ptr(12),
		Memory:     int64Ptr(30),
	}
	meta := devicetype.DistinctDeviceType{
		DeviceType: "SA2.MEDIUM4",
		CpuCore:    4,
		Memory:     8,
	}
	osOut, cpuOut, memOut, err := deriveCvmScaleFromSimple(in, meta)
	require.NoError(t, err)
	require.True(t, osOut.Equal(decimal.NewFromInt(3)))
	require.Equal(t, int64(12), *cpuOut)
	require.Equal(t, int64(30), *memOut)
}

func TestDeriveCvmScaleFromSimple_ByCpuCore(t *testing.T) {
	in := &CvmSimpleInput{
		DeviceType: "SA2.MEDIUM4",
		CpuCore:    int64Ptr(12),
	}
	meta := devicetype.DistinctDeviceType{
		DeviceType: "SA2.MEDIUM4",
		CpuCore:    4,
		Memory:     8,
	}
	osOut, cpuOut, memOut, err := deriveCvmScaleFromSimple(in, meta)
	require.NoError(t, err)
	require.NotNil(t, osOut)
	require.True(t, osOut.Equal(decimal.NewFromInt(3)))
	require.NotNil(t, cpuOut)
	require.Equal(t, int64(12), *cpuOut)
	require.NotNil(t, memOut)
	require.Equal(t, int64(24), *memOut)
}

func TestDeriveCvmScaleFromSimple_ZeroCpuCoreInMeta(t *testing.T) {
	in := &CvmSimpleInput{
		DeviceType: "BAD",
		CpuCore:    int64Ptr(8),
	}
	meta := devicetype.DistinctDeviceType{
		DeviceType: "BAD",
		CpuCore:    0,
		Memory:     4,
	}
	_, _, _, err := deriveCvmScaleFromSimple(in, meta)
	require.Error(t, err)
}

// TestCreateResPlanDemandSimpleReq_ValidateObsProjectForm 简易提单校验只认形态，不卡当前时间窗口。
func TestCreateResPlanDemandSimpleReq_ValidateObsProjectForm(t *testing.T) {
	future := CreateResPlanDemandSimpleReq{
		ObsProject: "2099春节保障",
		ExpectTime: "2026-09-01",
		RegionID:   "ap-shanghai",
		Cvm:        &CvmSimpleInput{DeviceType: "SA2.LARGE8", Os: decimalPtr("1")},
	}
	require.NoError(t, future.Validate())

	illegal := CreateResPlanDemandSimpleReq{
		ObsProject: "2029春保",
		ExpectTime: "2026-09-01",
		RegionID:   "ap-shanghai",
		Cvm:        &CvmSimpleInput{DeviceType: "SA2.LARGE8", Os: decimalPtr("1")},
	}
	require.Error(t, illegal.Validate())
}

func TestToCreateResPlanTicketReq_DeviceTypeNotFound(t *testing.T) {
	r := &CreateResPlanTicketSimpleReq{
		DemandClass: enumor.DemandClassCVM,
		Demands: []CreateResPlanDemandSimpleReq{
			{
				ObsProject: enumor.ObsProjectNormal,
				ExpectTime: "2025-01-01",
				RegionID:   "ap-shanghai",
				Cvm: &CvmSimpleInput{
					DeviceType: "UNKNOWN.TYPE",
					Os:         decimalPtr("1"),
				},
			},
		},
		Remark: "012345678901234567890",
	}
	dtm := map[string]devicetype.DistinctDeviceType{}
	_, err := r.ToCreateResPlanTicketReq(dtm)
	require.Error(t, err)
}

func decimalPtr(s string) *decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return &d
}

func int64Ptr(v int64) *int64 {
	return &v
}
