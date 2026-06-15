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
	"strings"

	"hcm/pkg/cc"
	"hcm/pkg/tools/converter"

	a2aserver "trpc.group/trpc-go/trpc-a2a-go/server"
)

// buildAgentCard 根据 cc 配置一次性构建 AgentCard，运行期保持不变。
//
// A2A 默认值由 cc.A2ASetting.trySetDefault() 统一填充，a2a 层仅做字段映射。
// streaming 由上层调用方传入（当前与 AGUI.Model.Stream 保持一致），避免能力声明与实际行为不一致。
func buildAgentCard(cfg cc.A2ASetting, streaming bool) a2aserver.AgentCard {
	return a2aserver.AgentCard{
		Name:        cardName(cfg.Card),
		Description: cardDescription(cfg.Card),
		Version:     cardVersion(cfg.Card),
		URL:         strings.TrimSpace(cfg.Card.URL),
		Capabilities: a2aserver.AgentCapabilities{
			Streaming: &streaming,
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Skills:             buildSkills(cfg.Card),
	}
}

// cardName 返回 AgentCard.name。
func cardName(cfg cc.A2ACardConfig) string {
	return strings.TrimSpace(cfg.Name)
}

// cardDescription 返回 AgentCard.description。
func cardDescription(cfg cc.A2ACardConfig) string {
	return strings.TrimSpace(cfg.Description)
}

// cardVersion 返回 AgentCard.version。
func cardVersion(cfg cc.A2ACardConfig) string {
	return strings.TrimSpace(cfg.Version)
}

// buildSkills 把配置中的 skill 列表映射为 A2A AgentSkill。
func buildSkills(cfg cc.A2ACardConfig) []a2aserver.AgentSkill {
	skills := make([]a2aserver.AgentSkill, 0, len(cfg.Skills))
	for _, sk := range cfg.Skills {
		skills = append(skills, a2aserver.AgentSkill{
			ID:          strings.TrimSpace(sk.ID),
			Name:        strings.TrimSpace(sk.Name),
			Description: converter.ValToPtr(sk.Description),
			Tags:        sk.Tags,
			Examples:    sk.Examples,
			InputModes:  sk.InputModes,
			OutputModes: sk.OutputModes,
		})
	}
	return skills
}
