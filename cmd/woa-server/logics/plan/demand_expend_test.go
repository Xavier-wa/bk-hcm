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

	ptypes "hcm/cmd/woa-server/types/plan"
	dt "hcm/pkg/api/core/cloud/device-type"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
)

// TestConvResConsumePoolToExpendMap_UsesOriginalDeviceFamily 验证 expend key 的 DeviceFamily
// 取自消耗池中的原始申领机型，而不会因为同 TechnicalClass 的代表机型（如 NPU）而漂移。
func TestConvResConsumePoolToExpendMap_UsesOriginalDeviceFamily(t *testing.T) {
	kt := kit.New()

	const (
		appliedType  = "GC50.192XLARGE2304"
		remappedType = "NPU.FAKE.REPRESENTATIVE"
		gpuFamily    = "GPU型"
		npuFamily    = "NPU计算型"
		techClass    = "GPU/NPU-Shared-TechClass"
		appliedCores = int64(2304)
	)

	deviceTypes := map[string]dt.DistinctDeviceType{
		appliedType: {
			DeviceType:     appliedType,
			DeviceFamily:   gpuFamily,
			CoreType:       enumor.CoreTypeBig,
			TechnicalClass: techClass,
		},
		remappedType: {
			DeviceType:     remappedType,
			DeviceFamily:   npuFamily,
			CoreType:       enumor.CoreTypeBig,
			TechnicalClass: techClass,
		},
	}

	// 列表/罚金路径：消耗池保留原始申领机型
	pool := ResPlanConsumePool{
		ResPlanPoolKeyV2{
			PlanType:      enumor.PlanTypeCodeOutPlan,
			AvailableTime: ptypes.NewAvailableMonth(2026, 7),
			DeviceType:    appliedType,
			ObsProject:    enumor.ObsProjectNormal,
			BkBizID:       5017152,
			DemandClass:   enumor.DemandClassCVM,
			RegionID:      "ap-guangzhou",
		}: appliedCores,
	}

	got := convResConsumePoolToExpendMap(kt, pool, deviceTypes)
	if len(got) != 1 {
		t.Fatalf("expected 1 expend key, got %d: %+v", len(got), got)
	}

	for key, cores := range got {
		if key.DeviceFamily != gpuFamily {
			t.Errorf("DeviceFamily = %q, want %q (must not drift to remapped NPU family)",
				key.DeviceFamily, gpuFamily)
		}
		if key.CoreType != string(enumor.CoreTypeBig) {
			t.Errorf("CoreType = %q, want %q", key.CoreType, enumor.CoreTypeBig)
		}
		if cores != appliedCores {
			t.Errorf("cores = %d, want %d", cores, appliedCores)
		}
	}

	// 对照：若错误地使用并查代表机型建池，Family 会漂到 NPU（回归锁）
	driftedPool := ResPlanConsumePool{
		ResPlanPoolKeyV2{
			PlanType:      enumor.PlanTypeCodeOutPlan,
			AvailableTime: ptypes.NewAvailableMonth(2026, 7),
			DeviceType:    remappedType,
			ObsProject:    enumor.ObsProjectNormal,
			BkBizID:       5017152,
			DemandClass:   enumor.DemandClassCVM,
			RegionID:      "ap-guangzhou",
		}: appliedCores,
	}
	drifted := convResConsumePoolToExpendMap(kt, driftedPool, deviceTypes)
	for key := range drifted {
		if key.DeviceFamily != npuFamily {
			t.Fatalf("control case: remapped DeviceType should map to %q, got %q",
				npuFamily, key.DeviceFamily)
		}
	}
}
