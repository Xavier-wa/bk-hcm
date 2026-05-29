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

package skill

import (
	"fmt"
	"os"

	"hcm/pkg/cc"
	"hcm/pkg/logs"
	"hcm/pkg/metrics"
	"hcm/pkg/thirdparty/api-gateway/bkaidev"
	"hcm/pkg/tools/localstore"

	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
)

// Manager holds the skill repository and the optional BKAIDev syncer.
// Readiness tracking lives in the logics package (logics.Readiness), not here.
type Manager struct {
	// Repository is the FSRepository exposed to skill tools and prompt injection.
	Repository skillpkg.RefreshableRepository
	// Syncer is non-nil only when BKAIDev skill sync is enabled.
	Syncer *Syncer
}

// NewManager constructs a skill Manager from the global cc.AgentServer configuration.
// readiness is notified (MarkSkillReady) once the initial sync completes; when
// BKAIDev sync is disabled it is called immediately so the server can serve traffic.
func NewManager(readiness ReadinessNotifier) (*Manager, error) {
	agentCfg := cc.AgentServer()
	skillsCfg := agentCfg.Skills
	if skillsCfg == nil {
		logs.Warnf("skill config is empty")
		return nil, nil
	}

	repo, err := BuildFSRepo(skillsCfg)
	if err != nil {
		logs.Errorf("build skill fs repository failed, err: %v", err)
		return nil, fmt.Errorf("build skill fs repository: %w", err)
	}

	mgr := &Manager{Repository: repo}

	if !agentCfg.SkillSyncEnabled() {
		if repo != nil {
			// Local-only mode: skills are already present in root dir, ready immediately.
			readiness.MarkSkillReady()
		}
		return mgr, nil
	}

	store := localstore.NewStore[skillRecord](skillsCfg.StoreFile())
	if err = store.Load(); err != nil {
		logs.Errorf("load skill local store failed, err: %v", err)
		return nil, fmt.Errorf("load skill local store: %w", err)
	}

	gw := agentCfg.BKAIDevSyncAPIGateway
	skillCli, err := bkaidev.NewClient(&gw, metrics.Register())
	if err != nil {
		logs.Errorf("create bkaidev skill client failed, err: %v", err)
		return nil, fmt.Errorf("create bkaidev skill client: %w", err)
	}

	inst := newInstaller(skillCli, store, skillsCfg.Root, skillsCfg.ArchiveDir)
	syncer := newSyncer(skillCli, inst, store, repo, readiness, skillsCfg, skillsCfg.Root)
	syncer.InitialSyncSkill()
	mgr.Syncer = syncer

	logs.Infof("skill manager built with BKAIDev sync: spaceID=%s interval=%s tagName=%q maxParallel=%d",
		skillsCfg.SpaceID, skillsCfg.SyncInterval, skillsCfg.TagName, skillsCfg.MaxParallel)
	return mgr, nil
}

// ensureDir creates dir if it does not already exist.
func ensureDir(dir string) error {
	if dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}
