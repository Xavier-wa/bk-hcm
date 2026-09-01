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
	"fmt"
	"slices"

	gctypes "hcm/cmd/woa-server/types/green-channel"
	coredevicetype "hcm/pkg/api/core/cloud/device-type"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/cvmapi"
)

// CheckDeviceType 按需求类型校验机型是否允许申领，返回不允许的机型及其原因。
func (s *scheduler) CheckDeviceType(kt *kit.Kit, requireType enumor.RequireType,
	infos []coredevicetype.DistinctDeviceType) (map[string]string, error) {

	denied := make(map[string]string)
	if requireType != enumor.RequireTypeGreenChannel && requireType != enumor.RequireTypeRollServer {
		return denied, nil
	}

	needCfg := requireType == enumor.RequireTypeGreenChannel && len(infos) > 0
	cfg, err := s.loadCvmApplyConfig(kt, needCfg)
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		allowed, reason := checkDeviceType(requireType, info, cfg)
		if allowed {
			continue
		}
		denied[info.DeviceType] = reason
	}
	return denied, nil
}

func (s *scheduler) loadCvmApplyConfig(kt *kit.Kit, need bool) (gctypes.CvmApplyConfig, error) {
	if !need {
		return gctypes.CvmApplyConfig{}, nil
	}

	configs, err := s.gcLogics.GetConfigs(kt)
	if err != nil {
		logs.Errorf("get green channel configs failed, err: %v, rid: %s", err, kt.Rid)
		return gctypes.CvmApplyConfig{}, err
	}
	return configs.CvmApplyConfig, nil
}

func checkDeviceType(requireType enumor.RequireType, info coredevicetype.DistinctDeviceType,
	cfg gctypes.CvmApplyConfig) (bool, string) {

	if info.DeviceTypeClass == cvmapi.SpecialType {
		return false, "不支持特殊机型"
	}
	if requireType != enumor.RequireTypeGreenChannel || !cfg.Enabled {
		return true, ""
	}
	if !slices.Contains(cfg.DeviceGroups, info.DeviceFamily) {
		return false, fmt.Sprintf("机型族 %s 不在允许范围", info.DeviceFamily)
	}
	if info.CpuCore > cfg.CpuMaxLimit {
		return false, fmt.Sprintf("CPU核数 %d 超过上限 %d", info.CpuCore, cfg.CpuMaxLimit)
	}
	return true, ""
}
