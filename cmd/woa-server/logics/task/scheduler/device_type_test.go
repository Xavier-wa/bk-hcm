/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package scheduler

import (
	"testing"

	gctypes "hcm/cmd/woa-server/types/green-channel"
	coredevicetype "hcm/pkg/api/core/cloud/device-type"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/thirdparty/cvmapi"

	"github.com/stretchr/testify/assert"
)

func TestCheckDeviceTypeReason(t *testing.T) {
	enabledCfg := gctypes.CvmApplyConfig{
		Enabled: true, DeviceGroups: []string{"标准型"}, CpuMaxLimit: 16,
	}
	ita5 := coredevicetype.DistinctDeviceType{
		DeviceType: "ITA5", DeviceFamily: "ITA", CpuCore: 8, DeviceTypeClass: cvmapi.CommonType,
	}
	standardOK := coredevicetype.DistinctDeviceType{
		DeviceType: "SA2.MEDIUM4", DeviceFamily: "标准型", CpuCore: 16, DeviceTypeClass: cvmapi.CommonType,
	}
	standardOverCPU := coredevicetype.DistinctDeviceType{
		DeviceType: "SA2.LARGE32", DeviceFamily: "标准型", CpuCore: 32, DeviceTypeClass: cvmapi.CommonType,
	}
	specialType := coredevicetype.DistinctDeviceType{
		DeviceType: "SPECIAL.1", DeviceFamily: "标准型", CpuCore: 8, DeviceTypeClass: cvmapi.SpecialType,
	}

	tests := []struct {
		name        string
		requireType enumor.RequireType
		info        coredevicetype.DistinctDeviceType
		cfg         gctypes.CvmApplyConfig
		wantAllow   bool
		wantReason  string
	}{
		{name: "green family mismatch", requireType: enumor.RequireTypeGreenChannel, info: ita5, cfg: enabledCfg,
			wantReason: "机型族 ITA 不在允许范围"},
		{name: "green standard allow", requireType: enumor.RequireTypeGreenChannel, info: standardOK, cfg: enabledCfg,
			wantAllow: true},
		{name: "green special deny", requireType: enumor.RequireTypeGreenChannel, info: specialType, cfg: enabledCfg,
			wantReason: "不支持特殊机型"},
		{name: "green policy off allow", requireType: enumor.RequireTypeGreenChannel, info: ita5, wantAllow: true},
		{name: "green cpu exceed", requireType: enumor.RequireTypeGreenChannel, info: standardOverCPU, cfg: enabledCfg,
			wantReason: "CPU核数 32 超过上限 16"},
		{name: "roll ignore family", requireType: enumor.RequireTypeRollServer, info: ita5, cfg: enabledCfg,
			wantAllow: true},
		{name: "roll special deny", requireType: enumor.RequireTypeRollServer, info: specialType, cfg: enabledCfg,
			wantReason: "不支持特殊机型"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			allowed, reason := checkDeviceType(tc.requireType, tc.info, tc.cfg)
			assert.Equal(t, tc.wantAllow, allowed)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}

func TestCheckDeviceType(t *testing.T) {
	specialType := coredevicetype.DistinctDeviceType{
		DeviceType: "SPECIAL.1", DeviceTypeClass: cvmapi.SpecialType,
	}
	ita5 := coredevicetype.DistinctDeviceType{
		DeviceType: "ITA5", DeviceTypeClass: cvmapi.CommonType,
	}
	svc := &scheduler{}
	kt := &kit.Kit{Rid: "test-rid"}

	denied, err := svc.CheckDeviceType(kt, enumor.RequireTypeRollServer,
		[]coredevicetype.DistinctDeviceType{specialType, ita5, specialType})
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"SPECIAL.1": "不支持特殊机型"}, denied)

	denied, err = svc.CheckDeviceType(kt, enumor.RequireTypeRegular,
		[]coredevicetype.DistinctDeviceType{specialType})
	assert.NoError(t, err)
	assert.Empty(t, denied)
}
