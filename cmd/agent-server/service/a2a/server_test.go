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

package a2a

import (
	"testing"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
)

func TestNormalizeBasePath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", constant.A2ABasePathDefault},
		{"/", constant.A2ABasePathDefault},
		{"/api/v1/agent", "/api/v1/agent"},
		{"/api/v1/agent/", "/api/v1/agent"},
		{"api/v1/agent", "/api/v1/agent"},
		{"   /custom/  ", "/custom"},
	}
	for _, c := range cases {
		if got := normalizeBasePath(c.in); got != c.want {
			t.Errorf("normalizeBasePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildSkills_Mapping(t *testing.T) {
	skills := buildSkills(cc.A2ACardConfig{
		Skills: []cc.A2ACardSkillConfig{{
			ID:          "hcm-agent",
			Name:        "HCM Agent",
			Description: "BlueKing Hybrid Cloud Management AI Agent",
			Tags:        []string{"hcm", "agent"},
		}},
	})
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].ID != "hcm-agent" {
		t.Errorf("skill id = %q, want %q", skills[0].ID, "hcm-agent")
	}
	if len(skills[0].Tags) == 0 {
		t.Error("skill tags must not be empty (A2A v0.2.2 requirement)")
	}
}

func TestBuildSkills_FromConfig(t *testing.T) {
	skills := buildSkills(cc.A2ACardConfig{
		Skills: []cc.A2ACardSkillConfig{
			{
				ID:          "cvm",
				Name:        "CVM Skill",
				Description: "Manage CVM instances",
				Tags:        []string{"cvm", "cloud"},
				Examples:    []string{"列出我的 CVM"},
			},
		},
	})
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].ID != "cvm" || skills[0].Name != "CVM Skill" {
		t.Errorf("unexpected skill: %+v", skills[0])
	}
	if skills[0].Description == nil || *skills[0].Description != "Manage CVM instances" {
		t.Errorf("description not propagated: %+v", skills[0].Description)
	}
}

func TestBuildAgentCard_Mapping(t *testing.T) {
	cfg := cc.A2ASetting{
		Card: cc.A2ACardConfig{
			Name:        "HCM Agent",
			Description: "BlueKing Hybrid Cloud Management AI Agent",
			Version:     "1.2.3",
			Skills: []cc.A2ACardSkillConfig{{
				ID:          "hcm-agent",
				Name:        "HCM Agent",
				Description: "BlueKing Hybrid Cloud Management AI Agent",
				Tags:        []string{"hcm", "agent"},
			}},
		},
	}

	card := buildAgentCard(cfg, true)
	if card.Name != cfg.Card.Name {
		t.Errorf("name = %q, want %q", card.Name, cfg.Card.Name)
	}
	if card.Version != cfg.Card.Version {
		t.Errorf("version = %q, want %q", card.Version, cfg.Card.Version)
	}
	if card.Description != cfg.Card.Description {
		t.Errorf("description = %q, want %q", card.Description, cfg.Card.Description)
	}
	if len(card.Skills) == 0 {
		t.Error("Skills must not be empty (A2A v0.2.2 requirement)")
	}
	if card.Capabilities.Streaming == nil || !*card.Capabilities.Streaming {
		t.Error("streaming should follow input flag=true")
	}
	cardNoStream := buildAgentCard(cfg, false)
	if cardNoStream.Capabilities.Streaming == nil || *cardNoStream.Capabilities.Streaming {
		t.Error("streaming should follow input flag=false")
	}
	if len(card.DefaultInputModes) == 0 || len(card.DefaultOutputModes) == 0 {
		t.Error("default input/output modes must not be empty (A2A v0.2.2 requirement)")
	}
}

func TestBuildAgentCard_CustomFields(t *testing.T) {
	card := buildAgentCard(cc.A2ASetting{
		Card: cc.A2ACardConfig{
			Name:        "Custom Agent",
			Description: "Custom Desc",
			Version:     "2.3.4",
			URL:         "http://example.com/api/v1/agent",
			Skills: []cc.A2ACardSkillConfig{{
				ID:   "custom-agent",
				Name: "Custom Agent",
				Tags: []string{"custom", "agent"},
			}},
		},
	}, true)
	if card.Name != "Custom Agent" {
		t.Errorf("name = %q", card.Name)
	}
	if card.Version != "2.3.4" {
		t.Errorf("version = %q", card.Version)
	}
	if card.URL != "http://example.com/api/v1/agent" {
		t.Errorf("url = %q", card.URL)
	}
	if len(card.Skills) != 1 {
		t.Fatalf("skills len = %d, want 1", len(card.Skills))
	}
}

