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
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/cron/core"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/bkaidev"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/util"
)

const (
	// syncPromptURL is the HTTP path for the manual prompt sync trigger endpoint.
	syncPromptURL = "/prompts/sync"
)

// ReadinessNotifier is implemented by the agent-server Readiness state machine.
// Using an interface keeps the prompt package decoupled from the logics package.
type ReadinessNotifier interface {
	// MarkPromptReady signals that the initial prompt sync has completed.
	MarkPromptReady()
}

// Syncer orchestrates initial and incremental prompt synchronisation from BKAIDev.
// It iterates over the configured prompts array, fetches each by ID, and updates
// the Store when content changes (detected via MD5 comparison).
type Syncer struct {
	client    bkaidev.Client
	store     *Store
	readiness ReadinessNotifier
	cfg       *cc.AgentPromptConfig
}

// newSyncer constructs a Syncer. All arguments are required.
func newSyncer(cli bkaidev.Client, store *Store, readiness ReadinessNotifier,
	cfg *cc.AgentPromptConfig) *Syncer {

	return &Syncer{
		client:    cli,
		store:     store,
		readiness: readiness,
		cfg:       cfg,
	}
}

// InitialSync starts the first full sync in a background goroutine and returns
// immediately. readiness.MarkPromptReady is called only when all required prompts
// synchronise successfully.
func (s *Syncer) InitialSync() {
	go func() {
		kt := kit.New()

		if err := s.SyncPrompts(kt); err != nil {
			logs.Errorf("prompt initial sync failed: %v, rid: %s", err, kt.Rid)
			return
		}

		logs.Infof("prompt initial sync done, rid: %s", kt.Rid)
	}()
}

// SyncPrompts fetches all configured prompts from BKAIDev, compares content MD5
// with what is currently in the store, and persists any that changed.
// Returns a joined error if any prompt (required or not) fails.
func (s *Syncer) SyncPrompts(kt *kit.Kit) error {
	updated, allOK := s.fetchAllPrompts(kt)
	if updated > 0 {
		logs.Infof("prompt store updated: count=%d, rid: %s", updated, kt.Rid)
	}

	if !allOK {
		return errors.New("one or more prompts failed to sync, see previous error logs")
	}

	s.readiness.MarkPromptReady()
	logs.Infof("sync prompts done make prompt ready, updated=%d, rid: %s", updated, kt.Rid)

	return nil
}

// fetchAllPrompts fetches all space prompts via paginated ListPrompts, matches them
// against cfg.Entries by ID, and writes changed entries directly into the store.
// Returns:
//   - updated: the count of prompts whose content changed and were written to the store.
//   - allRequiredOK: true when every required prompt was found and processed without error.
func (s *Syncer) fetchAllPrompts(kt *kit.Kit) (updated int, allRequiredOK bool) {
	allRequiredOK = true

	items, err := s.listAllSpacePrompts(kt)
	if err != nil {
		logs.Errorf("list prompts failed, err: %v, rid: %s", err, kt.Rid)
		for _, p := range s.cfg.Entries {
			if p.Required {
				allRequiredOK = false
				break
			}
		}
		return 0, allRequiredOK
	}

	// Index returned prompts by ID for O(1) lookup.
	remotePrompts := converter.SliceToMap(items, func(item bkaidev.PromptListItem) (int, bkaidev.PromptListItem) {
		return item.PromptID, item
	})

	for _, p := range s.cfg.Entries {
		item, ok := remotePrompts[p.ID]
		if !ok {
			logs.Errorf("prompt not found in space list: name=%s id=%d, rid: %s", p.Code, p.ID, kt.Rid)
			if p.Required {
				allRequiredOK = false
			}
			continue
		}

		newMD5 := contentMD5(item.Content)
		e, ok := s.store.Get(p.Code)
		if !ok {
			logs.Infof("prompt not found in store: name=%s id=%d, rid: %s", p.Code, p.ID, kt.Rid)
		}
		// Skip only when MD5 is unchanged AND content is already loaded in the store.
		// On startup the store may be empty for a new prompt; in that case we must fetch
		// and apply remote content even if MD5 matches a stale local record.
		if newMD5 == e.MD5 && e.Content != "" {
			logs.Infof("prompt content unchanged: name=%s id=%d md5=%s, rid: %s", p.Code, p.ID, newMD5, kt.Rid)
			continue
		}

		entry := PromptEntry{Content: item.Content, MD5: newMD5, UpdatedAt: time.Now()}
		if err = s.store.Set(p.Code, entry); err != nil {
			logs.Errorf("save prompt failed: name=%s, err: %v, rid: %s", p.Code, err, kt.Rid)
			if p.Required {
				allRequiredOK = false
			}
			continue
		}

		updated++
		logs.Infof("prompt content changed: name=%s id=%d md5=%s preview=%q, rid: %s",
			p.Code, p.ID, newMD5, util.TruncateRune(item.Content, PromptContentMaxRuneLength), kt.Rid)
	}

	return updated, allRequiredOK
}

// listAllSpacePrompts fetches all prompts in the configured space via paginated ListPrompts.
func (s *Syncer) listAllSpacePrompts(kt *kit.Kit) ([]bkaidev.PromptListItem, error) {
	var allItems []bkaidev.PromptListItem

	for page := 1; ; page++ {
		resp, err := s.client.ListPrompts(kt, &bkaidev.ListPromptsReq{
			SpaceID: s.cfg.SpaceID,
			// space 代表仅拉取空间下的 prompt
			GroupType: constant.BKAIDEVGroupTypeSpace,
			Page:      page,
		})
		if err != nil {
			logs.Errorf("list prompts from bkaidev failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		allItems = append(allItems, resp.Results...)
		if page >= resp.NumPages {
			break
		}
	}

	return allItems, nil
}

// contentMD5 computes the hex-encoded MD5 of the given string.
func contentMD5(content string) string {
	sum := md5.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}

// SyncCronTask is the cron task that triggers incremental prompt sync.
type SyncCronTask struct {
	syncer   *Syncer
	interval time.Duration
}

// Name returns the cron task identifier.
func (t *SyncCronTask) Name() string {
	return string(enumor.CronTaskSyncAgentPrompts)
}

// Next returns the next scheduled execution time.
func (t *SyncCronTask) Next() (time.Time, error) {
	return time.Now().Add(t.interval), nil
}

// Do runs a single incremental sync cycle.
func (t *SyncCronTask) Do(kt *kit.Kit) error {
	return t.syncer.SyncPrompts(kt)
}

// GetURL returns the HTTP path for the manual prompt sync trigger endpoint.
func (t *SyncCronTask) GetURL() string {
	return syncPromptURL
}

// parseSyncInterval parses cfg.SyncInterval into a time.Duration.
func parseSyncInterval(cfg *cc.AgentPromptConfig) (time.Duration, error) {
	d, err := time.ParseDuration(cfg.SyncInterval)
	if err != nil {
		return 0, fmt.Errorf("invalid prompt sync interval %q: %w", cfg.SyncInterval, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("prompt sync interval must be positive, got %q", cfg.SyncInterval)
	}
	return d, nil
}

// NewSyncCronTask registers the prompt sync cron task when BKAIDev sync is enabled.
// Returns nil when syncer is nil.
func NewSyncCronTask(syncer *Syncer) (core.Task, error) {
	if syncer == nil {
		return nil, nil
	}

	interval, err := parseSyncInterval(syncer.cfg)
	if err != nil {
		logs.Errorf("parse prompt sync task interval failed, err: %v", err)
		return nil, errf.Newf(errf.Aborted, "parse prompt sync interval: %v", err)
	}

	return &SyncCronTask{syncer: syncer, interval: interval}, nil
}
