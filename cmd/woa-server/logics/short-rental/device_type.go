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

package shortrental

import (
	"hcm/pkg/api/core"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/slice"
)

// ListDeviceTypeFamily 根据用户退回机器的机型，在本地表中查询对应的物理机机型族
func (l *logics) ListDeviceTypeFamily(kt *kit.Kit, deviceTypes []string) (map[string]string, error) {
	deviceToPhysFamilyMap := make(map[string]string)
	for _, batch := range slice.Split(deviceTypes, int(core.DefaultMaxPageLimit)) {
		listReq := &core.ListReq{
			Filter: tools.ContainersExpression("device_type", batch),
			Page:   core.NewDefaultBasePage(),
		}

		rst, err := l.client.DataService().Global.ResourcePlan.ListWoaDeviceTypePhysicalRel(kt, listReq)
		if err != nil {
			logs.Errorf("list device type physical rel failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		for _, item := range rst.Details {
			deviceToPhysFamilyMap[item.DeviceType] = item.PhysicalDeviceFamily
		}
	}

	return deviceToPhysFamilyMap, nil
}
