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

package constant

const (
	// UnsetRecycleTime defines default value for unset recycle time
	UnsetRecycleTime = -1
	// RecycleHostQueryArrayLimit is max length of array filter fields in recycle host list query.
	RecycleHostQueryArrayLimit = 20
)

// GlobalConfigTypeRecycle 回收相关配置类型
const GlobalConfigTypeRecycle = "recycle"

// RecycleDetectConcurrenceConfigKey 回收预检并发配置
const RecycleDetectConcurrenceConfigKey = "detect_concurrence"

// RecycleTransitHost2CRBatchSizeConfigKey 回收转CR模块单次请求批量大小
const RecycleTransitHost2CRBatchSizeConfigKey = "transit_host2cr_batch_size"

// DetectDefaultConcurrence 预检单个单据默认并发数
const DetectDefaultConcurrence = 10
