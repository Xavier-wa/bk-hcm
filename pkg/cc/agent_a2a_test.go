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
)

func TestA2ASetting_TrySetDefault(t *testing.T) {
	cfg := A2ASetting{}
	cfg.trySetDefault()
	if cfg.BasePath != constant.A2ABasePathDefault {
		t.Errorf("BasePath = %q, want %q", cfg.BasePath, constant.A2ABasePathDefault)
	}
}

func TestA2ASetting_Validate_DisabledSkipsCheck(t *testing.T) {
	cfg := A2ASetting{Enable: false}
	if err := cfg.Validate(); err != nil {
		t.Errorf("disabled A2A should pass validate, got %v", err)
	}
}

func TestA2ASetting_Validate_RequiresBasePath(t *testing.T) {
	cfg := A2ASetting{Enable: true, BasePath: ""}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error on empty basePath")
	}
}

func TestA2ASetting_Validate_BasePathLeadingSlash(t *testing.T) {
	cfg := A2ASetting{Enable: true, BasePath: "api/v1/agent"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error on basePath without leading slash")
	}
}

func TestA2ASetting_Validate_SkillMissingTagsRejected(t *testing.T) {
	cfg := A2ASetting{
		Enable:   true,
		BasePath: "/api/v1/agent",
		Card: A2ACardConfig{
			Skills: []A2ACardSkillConfig{
				{ID: "x", Name: "X"},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error on skill without tags")
	}
}

func TestA2ASetting_Validate_ValidConfig(t *testing.T) {
	cfg := A2ASetting{
		Enable:   true,
		BasePath: "/api/v1/agent",
		Card: A2ACardConfig{
			Name:    "HCM",
			Version: "1.0.0",
			Skills: []A2ACardSkillConfig{
				{ID: "hcm", Name: "HCM", Tags: []string{"cloud"}},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid config should pass, got %v", err)
	}
}

func TestAgentToolsConfig_Validate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     AgentToolsConfig
		wantErr bool
	}{
		{
			name: "empty type is allowed",
			cfg:  AgentToolsConfig{MCPToolSets: []AgentMCPToolSet{{Name: "a", Type: ""}}},
		},
		{
			name: "bkaidev allowed",
			cfg: AgentToolsConfig{
				MCPToolSets: []AgentMCPToolSet{{Name: "a", Type: constant.MCPTypeBKAIDev}},
			},
		},
		{
			name: "internal allowed",
			cfg: AgentToolsConfig{
				MCPToolSets: []AgentMCPToolSet{{Name: "a", Type: constant.MCPTypeInternal}},
			},
		},
		{
			name: "case-insensitive match",
			cfg: AgentToolsConfig{
				MCPToolSets: []AgentMCPToolSet{{Name: "a", Type: "Internal"}},
			},
		},
		{
			name: "unknown type rejected",
			cfg: AgentToolsConfig{
				MCPToolSets: []AgentMCPToolSet{{Name: "a", Type: "foo"}},
			},
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.cfg.Validate()
			if (err != nil) != c.wantErr {
				t.Errorf("Validate() err = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}
