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

// Package skill provides functionality for loading and managing skill repositories.
package skill

import (
	"fmt"
	"strings"

	"hcm/pkg/cc"
	"hcm/pkg/logs"

	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
)

// BuildSkillRepo constructs a filesystem-backed skill repository from the given config.
// Returns nil when cfg is nil or no root directories are configured.
func BuildSkillRepo() (skillpkg.Repository, error) {
	cfg := cc.AgentServer().Tools.Skills
	if cfg == nil {
		return nil, nil
	}

	roots := make([]string, 0, 1+len(cfg.ExtraDirs))
	if r := strings.TrimSpace(cfg.Root); r != "" {
		roots = append(roots, r)
	}
	for _, d := range cfg.ExtraDirs {
		if d = strings.TrimSpace(d); d != "" {
			roots = append(roots, d)
		}
	}
	if len(roots) == 0 {
		return nil, nil
	}

	repo, err := skillpkg.NewFSRepository(roots...)
	if err != nil {
		return nil, fmt.Errorf("create skill repository: %w", err)
	}

	logs.Infof("AGUI skill repo loaded: root=%q extraDirs=%v skills=%d",
		cfg.Root, cfg.ExtraDirs, len(repo.Summaries()))
	return repo, nil
}
