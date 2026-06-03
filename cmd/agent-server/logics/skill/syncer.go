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
	"path/filepath"
	"time"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/cron/core"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/bkaidev"
	"hcm/pkg/tools/concurrence"
	"hcm/pkg/tools/localstore"

	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
)

// ReadinessNotifier is implemented by the agent-server Readiness state machine.
// Using an interface keeps the skill package decoupled from the logics package.
type ReadinessNotifier interface {
	// MarkSkillReady signals that the initial skill sync has completed.
	MarkSkillReady()
}

// Syncer orchestrates initial and incremental skill synchronisation from BKAIDev.
type Syncer struct {
	client    bkaidev.Client
	installer *Installer
	store     *localstore.Store[skillRecord]
	repo      skillpkg.RefreshableRepository
	readiness ReadinessNotifier
	cfg       *cc.AgentBKAIDevSyncSkillsConfig
	skillRoot string
}

// newSyncer constructs a Syncer. All arguments are required.
func newSyncer(cli bkaidev.Client, inst *Installer, store *localstore.Store[skillRecord],
	repo skillpkg.RefreshableRepository, readiness ReadinessNotifier, cfg *cc.AgentBKAIDevSyncSkillsConfig,
	skillRoot string) *Syncer {

	return &Syncer{
		client:    cli,
		installer: inst,
		store:     store,
		repo:      repo,
		readiness: readiness,
		cfg:       cfg,
		skillRoot: skillRoot,
	}
}

// InitialSyncSkill starts the first full sync in a background goroutine and returns
// immediately. The caller must not block on its completion; readiness.MarkSkillReady
// is invoked by the goroutine when sync finishes.
func (s *Syncer) InitialSyncSkill() {
	go func() {
		kt := kit.New()
		if err := s.SyncSkills(kt); err != nil {
			logs.Errorf("skill initial sync failed: %v", err)
			return
		}
		logs.Infof("skill initial sync done, repo refreshed")
	}()
}

// listSkillsReq builds the list request from sync configuration.
func (s *Syncer) listSkillsReq() *bkaidev.ListSkillsReq {
	return &bkaidev.ListSkillsReq{
		SpaceID: s.cfg.SpaceID,
		// TODO:AIDEV接口暂时不支持多个tag查询，所以策略是拉下来以后再过滤
		// 代表只拉取该空间下的skill
		GroupType: constant.BKAIDEVGroupTypeSpace,
	}
}

// SyncSkills performs an incremental diff-and-sync cycle. It is called by the
// cron task on every scheduled tick.
func (s *Syncer) SyncSkills(kt *kit.Kit) error {
	items, err := s.client.ListSkills(kt, s.listSkillsReq())
	if err != nil {
		return fmt.Errorf("list skills: %w", err)
	}

	// TODO: bkaidev接口目前不支持多tag和二级tag查询，这里先拉取全部后再进行过滤
	items = filterSkillsByTagNames(items, s.cfg.TagName)
	logs.Infof("filtered skills by tagName %v: %d skills matched, rid: %s", s.cfg.TagName, len(items), kt.Rid)

	remoteItems := make(map[string]bkaidev.SkillListItem, len(items))
	for _, item := range items {
		remoteItems[item.InstallKey()] = item
	}

	added, removed, updated := s.diffSkills(remoteItems)

	if len(added)+len(updated) > 0 {
		toInstall := make([]bkaidev.SkillListItem, 0, len(added)+len(updated))
		for _, key := range append(added, updated...) {
			toInstall = append(toInstall, remoteItems[key])
		}
		if installErr := s.installParallel(kt, toInstall); installErr != nil {
			logs.Errorf("parallel install failed, err: %v, rid: %s", installErr, kt.Rid)
			return errf.Newf(errf.Aborted, "parallel install failed, err: %v", installErr)
		}
	}

	for _, name := range removed {
		if err = s.removeSkill(kt, name); err != nil {
			logs.Errorf("remove skill %s failed, err: %v, rid: %s", name, err, kt.Rid)
			return errf.Newf(errf.Aborted, "remove skill %s failed, err: %v", name, err)
		}
	}

	if err = s.repo.Refresh(); err != nil {
		logs.Errorf("repo refresh after sync failed, err: %v, rid: %s", err, kt.Rid)
		return fmt.Errorf("repo refresh after sync: %w", err)
	}

	logs.Infof("skill incremental sync done: added=%d updated=%d removed=%d, rid: %s",
		len(added), len(updated), len(removed), kt.Rid)

	s.readiness.MarkSkillReady()

	return nil
}

// installParallel installs skills concurrently; each install uses its own sub-kit
// and target directory so failures do not affect other skills.
func (s *Syncer) installParallel(kt *kit.Kit, items []bkaidev.SkillListItem) error {
	if len(items) == 0 {
		return nil
	}

	// 限制并发数在[1, 10]之间
	maxP := max(1, min(s.cfg.MaxParallel, 10))

	return concurrence.BaseExec(maxP, items, func(item bkaidev.SkillListItem) error {
		subKt := kt.NewSubKitWithCtx(kt.Ctx)
		if err := s.installer.Install(subKt, item); err != nil {
			logs.Errorf("install skill %s: %v, rid: %s", item.InstallKey(), err, subKt.Rid)
			return fmt.Errorf("install skill %s: %v", item.InstallKey(), err)
		}
		return nil
	})
}

// removeSkill deletes the skill directory and local store entry for name.
func (s *Syncer) removeSkill(kt *kit.Kit, name string) error {
	dir := filepath.Join(s.skillRoot, name)
	if err := os.RemoveAll(dir); err != nil {
		logs.Errorf("remove skill dir %s failed, err: %v, rid: %s", dir, err, kt.Rid)
		return fmt.Errorf("remove skill dir %s: %w", dir, err)
	}
	if err := s.store.Remove(name); err != nil {
		logs.Errorf("remove local store entry %s failed, err: %v, rid: %s", name, err, kt.Rid)
		return fmt.Errorf("remove local store entry %s: %w", name, err)
	}
	logs.Infof("skill removed: name=%s, rid: %s", name, kt.Rid)
	return nil
}

// diffSkills compares the remote skill list against the local store and returns
// three disjoint name slices: added (in remote only), removed (in local only),
// updated (in both but version differs).
func (s *Syncer) diffSkills(remote map[string]bkaidev.SkillListItem) (added, removed, updated []string) {
	localKeys := make(map[string]struct{}, len(s.store.Keys()))
	for _, k := range s.store.Keys() {
		localKeys[k] = struct{}{}
	}

	for name, item := range remote {
		local, ok := s.store.Get(name)
		if !ok {
			added = append(added, name)
		} else if local.Version != item.Version {
			updated = append(updated, name)
		}
	}

	for name := range localKeys {
		if _, ok := remote[name]; !ok {
			removed = append(removed, name)
		}
	}
	return added, removed, updated
}

// ParseSyncInterval parses cfg.SyncInterval into a time.Duration.
func ParseSyncInterval(cfg *cc.AgentBKAIDevSyncSkillsConfig) (time.Duration, error) {
	d, err := time.ParseDuration(cfg.SyncInterval)
	if err != nil {
		return 0, fmt.Errorf("invalid skill sync interval %q: %w", cfg.SyncInterval, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("skill sync interval must be positive, got %q", cfg.SyncInterval)
	}
	return d, nil
}

// SyncCronTask is the cron task that triggers incremental skill sync.
type SyncCronTask struct {
	syncer   *Syncer
	interval time.Duration
}

// Name returns the cron task identifier.
func (t *SyncCronTask) Name() string {
	return string(enumor.CronTaskSyncAgentSkills)
}

// Next returns the next scheduled execution time.
func (t *SyncCronTask) Next() (time.Time, error) {
	return time.Now().Add(t.interval), nil
}

// Do runs a single incremental sync cycle.
func (t *SyncCronTask) Do(kt *kit.Kit) error {
	return t.syncer.SyncSkills(kt)
}

// SyncSkillsURL is the HTTP path for the manual skill sync trigger endpoint.
const SyncSkillsURL = "/skills/sync"

// GetURL returns the HTTP path for the manual skill sync trigger endpoint.
func (t *SyncCronTask) GetURL() string {
	return SyncSkillsURL
}

// NewSyncCronTask registers the skill sync cron task when BKAIDev sync is enabled.
// Returns nil when syncer is nil or sync is not configured.
func NewSyncCronTask(syncer *Syncer) (core.Task, error) {
	if syncer == nil {
		logs.Warnf("syncer is nil, skip register skill sync cron task")
		return nil, nil
	}

	srvCfg := cc.AgentServer()
	if !srvCfg.SkillSyncEnabled() {
		return nil, nil
	}

	interval, err := ParseSyncInterval(srvCfg.Skills)
	if err != nil {
		return nil, fmt.Errorf("parse skill sync interval: %v", err)
	}

	return &SyncCronTask{syncer: syncer, interval: interval}, nil

}

// convertTagNamesToMap converts skillTagNames from [][]string to map[string]string for easy comparison.
func convertTagNamesToMap(skillTagNames [][]string) map[string]string {
	// bkaidev list skill 接口拉到的是二级数组的形式展示二级标签，这里转换为map
	result := make(map[string]string)
	for _, tagPair := range skillTagNames {
		if len(tagPair) == 0 {
			continue
		}

		// 只有一级标签则value默认为""
		key := tagPair[0]
		value := ""

		if len(tagPair) == 2 {
			value = tagPair[1]
		}
		result[key] = value
	}
	return result
}

// MatchTagNames checks if the skill's TagNames matches the configured tagName filter.
// The tagNameFilter is a map where key is the first-level tag name and value is the second-level tag name.
// A skill matches only if ALL filter entries match (AND semantics):
//   - If filter value is empty, only first-level tag name needs to match
//   - If filter value is not empty, both first-level and second-level tag names must match
func matchTagNames(skillTagNames [][]string, tagNameFilter map[string]string) bool {
	if len(tagNameFilter) == 0 {
		return true
	}
	if len(skillTagNames) == 0 {
		return false
	}

	// Convert skillTagNames to map[string]string for easy comparison
	tagMap := convertTagNamesToMap(skillTagNames)

	// Check if ALL filter entries match (AND semantics)
	for filterKey, filterValue := range tagNameFilter {
		tagValue, ok := tagMap[filterKey]
		if !ok {
			return false // filterKey not found in skill tags
		}

		if filterValue != tagValue {
			return false
		}
	}
	return true
}

// FilterSkillsByTagNames filters a list of SkillListItem by the given tagName filter.
// The tagNameFilter is a map where key is the first-level tag name and value is the second-level tag name.
// Returns a new slice containing only the skills whose TagNames match the filter.
func filterSkillsByTagNames(skills []bkaidev.SkillListItem, tagNameFilter map[string]string) []bkaidev.SkillListItem {
	if len(tagNameFilter) == 0 {
		return skills
	}

	result := make([]bkaidev.SkillListItem, 0, len(skills))
	for _, skill := range skills {
		if matchTagNames(skill.TagNames, tagNameFilter) {
			result = append(result, skill)
		}
	}
	return result
}
