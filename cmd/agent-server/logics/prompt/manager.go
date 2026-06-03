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

package prompt

import (
	"fmt"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/logs"
	"hcm/pkg/metrics"
	"hcm/pkg/thirdparty/api-gateway/bkaidev"
)

// Manager holds the prompt Store and the optional BKAIDev syncer.
type Manager struct {
	// Store is the prompt store exposed to agents via BeforeModel callbacks.
	// It is backed by a JSON file on disk (BKAIDev mode) or memory-only (local-file mode).
	Store *Store
	// Syncer is non-nil only when BKAIDev prompt sync is enabled.
	Syncer *Syncer
}

// NewManager constructs a prompt Manager from the global cc.AgentServer configuration.
// readiness is notified (MarkPromptReady) once the initial sync completes; when
// BKAIDev sync is disabled it is called immediately so the server can serve traffic.
func NewManager(readiness ReadinessNotifier) (*Manager, error) {
	cfg := cc.AgentServer().Prompt

	if !cfg.BKAIDevSyncEnabled() {
		return newLocalFileManager(cfg, readiness), nil
	}

	return newBKAIDevManager(cfg, readiness)
}

// newLocalFileManager creates a Manager from locally configured prompt strings.
// The store operates in memory-only mode (no file backing needed since content
// comes directly from config). Readiness is marked immediately.
func newLocalFileManager(cfg cc.AgentPromptConfig, readiness ReadinessNotifier) *Manager {
	store := NewStore("") // memory-only: path="" skips all disk writes
	if cfg.SystemPrompt != "" {
		_ = store.Set(constant.SystemPromptKey, PromptEntry{Content: cfg.SystemPrompt})
	}
	if cfg.Instruction != "" {
		_ = store.Set(constant.InstructionKey, PromptEntry{Content: cfg.Instruction})
	}

	readiness.MarkPromptReady()
	logs.Infof("prompt manager built in local-file mode")
	return &Manager{Store: store}
}

// newBKAIDevManager creates a Manager that syncs prompts from BKAIDev.
// The store is backed by a JSON file on disk so content is immediately available
// on restart from the previous sync cycle. The initial sync runs in a background
// goroutine; readiness is gated on all required prompts completing successfully.
func newBKAIDevManager(cfg cc.AgentPromptConfig, readiness ReadinessNotifier) (*Manager, error) {
	gw := cc.AgentServer().BKAIDevSyncAPIGateway
	promptCli, err := bkaidev.NewClient(&gw, metrics.Register())
	if err != nil {
		logs.Errorf("create bkaidev prompt client failed, err: %v", err)
		return nil, fmt.Errorf("create bkaidev prompt client: %w", err)
	}

	store := NewStore(cfg.StorePath)
	if err = store.Load(); err != nil {
		logs.Errorf("load prompt store failed, err: %v", err)
		return nil, fmt.Errorf("load prompt store: %w", err)
	}

	syncer := newSyncer(promptCli, store, readiness, &cfg)
	syncer.InitialSync()

	logs.Infof("prompt manager built with BKAIDev sync: spaceID=%s interval=%s entries=%d",
		cfg.SpaceID, cfg.SyncInterval, len(cfg.Entries))

	return &Manager{Store: store, Syncer: syncer}, nil
}
