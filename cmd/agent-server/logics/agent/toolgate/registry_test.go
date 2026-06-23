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

package toolgate

import (
	"testing"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
)

func TestIsGateEnabled(t *testing.T) {
	tool := constant.ToolNameCreateBizApply

	tests := []struct {
		name string
		tool string
		cfg  cc.AgentConfirmGateConfig
		want bool
	}{
		{name: "enabled empty list", tool: tool, cfg: cc.AgentConfirmGateConfig{Enabled: true}, want: true},
		{
			name: "enabled list contains",
			tool: tool,
			cfg:  cc.AgentConfirmGateConfig{Enabled: true, Tools: []string{tool}},
			want: true,
		},
		{
			name: "enabled list excludes",
			tool: tool,
			cfg:  cc.AgentConfirmGateConfig{Enabled: true, Tools: []string{"other_tool"}},
			want: false,
		},
		{name: "disabled", tool: tool, cfg: cc.AgentConfirmGateConfig{Enabled: false}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isGateEnabled(tc.tool, tc.cfg); got != tc.want {
				t.Errorf("isGateEnabled(%q) = %v, want %v", tc.tool, got, tc.want)
			}
		})
	}
}

func TestEnabledGateHandlers(t *testing.T) {
	if hs := GetEnabledGateHandlers(cc.AgentConfirmGateConfig{Enabled: false}, nil); len(hs) != 0 {
		t.Errorf("disabled gate enabled handlers = %d, want 0", len(hs))
	}

	hs := GetEnabledGateHandlers(cc.AgentConfirmGateConfig{Enabled: true}, nil)
	found := false
	for _, h := range hs {
		if h.ToolName() == constant.ToolNameCreateBizApply {
			found = true
		}
	}
	if !found {
		t.Errorf("enabled gate handlers missing %q", constant.ToolNameCreateBizApply)
	}
}

func TestEnabledGateToolNames(t *testing.T) {
	if names := GetEnabledGateToolNames(cc.AgentConfirmGateConfig{Enabled: false}); len(names) != 0 {
		t.Errorf("disabled gate tool names = %v, want empty", names)
	}

	names := GetEnabledGateToolNames(cc.AgentConfirmGateConfig{Enabled: true})
	if _, ok := names[constant.ToolNameCreateBizApply]; !ok {
		t.Errorf("enabled gate tool names missing %q, got %v", constant.ToolNameCreateBizApply, names)
	}
}
