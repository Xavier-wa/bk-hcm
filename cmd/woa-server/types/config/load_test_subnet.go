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

package config

import (
	"fmt"
)

// LoadTestSubnetConfig 压测子网配置，三层映射结构：
// 外层 key 为 region，中层 key 为 vpc_id，内层 value 为 subnet_id 列表
type LoadTestSubnetConfig map[string]map[string][]string

// UpsertLoadTestSubnetReq upsert 压测子网配置请求，请求体即为完整的三层映射
type UpsertLoadTestSubnetReq LoadTestSubnetConfig

// Validate 校验压测子网配置请求，仅校验格式，不校验 vpc_id/subnet_id 是否真实存在
// 传入空 map 视为合法请求，效果为清空配置
func (req UpsertLoadTestSubnetReq) Validate() error {
	for region, vpcMap := range req {
		if region == "" {
			return fmt.Errorf("region can not be empty")
		}
		for vpcID, subnetIDs := range vpcMap {
			if vpcID == "" {
				return fmt.Errorf("vpc_id can not be empty, region: %s", region)
			}
			for _, subnetID := range subnetIDs {
				if subnetID == "" {
					return fmt.Errorf("subnet_id can not be empty, region: %s, vpc_id: %s", region, vpcID)
				}
			}
		}
	}

	return nil
}
