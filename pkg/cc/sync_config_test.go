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

package cc

import (
	"testing"

	"hcm/pkg/criteria/constant"

	"github.com/stretchr/testify/assert"
)

// TestSyncConfigTrySetDefault 缺省配置经 trySetDefault 后取默认值。
func TestSyncConfigTrySetDefault(t *testing.T) {
	cfg := new(SyncConfig)
	cfg.trySetDefault()

	assert.Equal(t, uint(1), cfg.DefaultConcurrent)
	assert.Equal(t, uint(constant.DefaultTargetsPrefetchMaxListeners), cfg.TargetsPrefetchMaxListeners)
}

// TestSyncConfigTrySetDefaultKeepUserValue 用户已配置的预取阈值不被默认值覆盖。
func TestSyncConfigTrySetDefaultKeepUserValue(t *testing.T) {
	cfg := &SyncConfig{TargetsPrefetchMaxListeners: 5000}
	cfg.trySetDefault()

	assert.Equal(t, uint(5000), cfg.TargetsPrefetchMaxListeners)
}
