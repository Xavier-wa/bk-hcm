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

// Package skill provides the skill repository Manager, BKAIDev syncer, and
// installer for the agent-server. Entry point: NewManager.
package skill

import (
	"fmt"
	"strings"

	"hcm/pkg/cc"
	"hcm/pkg/logs"

	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
)

// BuildFSRepo constructs an FSRepository from the configured roots.
// Returns nil when no roots are configured.
func BuildFSRepo(cfg cc.AgentBKAIDevSyncSkillsConfig) (skillpkg.RefreshableRepository, error) {
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

	if err := ensureDir(roots[0]); err != nil {
		logs.Errorf("ensure root dir %s failed, err: %v", roots[0], err)
		return nil, fmt.Errorf("ensure root dir %s: %w", roots[0], err)
	}

	if err := ensureDir(cfg.ArchiveDir); err != nil {
		logs.Errorf("ensure skill archive dir %s failed, err: %v", cfg.ArchiveDir, err)
		return nil, fmt.Errorf("ensure skill archive dir %s: %w", cfg.ArchiveDir, err)
	}

	repo, err := skillpkg.NewFSRepository(roots...)
	if err != nil {
		return nil, fmt.Errorf("new fs repository: %w", err)
	}
	logs.Infof("skill fs repository loaded: roots=%v skills=%d", roots, len(repo.Summaries()))
	return repo, nil
}
