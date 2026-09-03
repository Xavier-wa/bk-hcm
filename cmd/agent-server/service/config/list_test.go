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
	"testing"

	"hcm/pkg/criteria/enumor"
)

// TestListConfigTypeWhitelist 守住 GET /config/list 的 config_type 白名单：本期只放行
// agent_feedback_tag，避免今后有人不小心把 auth/access_token 等凭据类配置加进来。
func TestListConfigTypeWhitelist(t *testing.T) {
	tests := []struct {
		name       string
		configType enumor.GlobalConfigType
		wantAllow  bool
	}{
		{name: "agent_feedback_tag is allowed", configType: enumor.GlobalConfigTypeAgentFeedbackTag, wantAllow: true},
		{name: "auth is not allowed", configType: enumor.GlobalConfigTypeAuth, wantAllow: false},
		{name: "unknown type is not allowed", configType: enumor.GlobalConfigType("unknown"), wantAllow: false},
		{name: "empty type is not allowed", configType: enumor.GlobalConfigType(""), wantAllow: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := listConfigTypeWhitelist[tc.configType]
			if ok != tc.wantAllow {
				t.Errorf("listConfigTypeWhitelist[%q] allowed = %v, want %v", tc.configType, ok, tc.wantAllow)
			}
		})
	}
}
