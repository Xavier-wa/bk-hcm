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
	"hcm/pkg/criteria/enumor"

	skillpkg "trpc.group/trpc-go/trpc-agent-go/skill"
)

// SkillRepos maps a scene to its skill repository, mirroring toolproxy.ToolProxies.
// All scenes currently share the same underlying repository; the map structure allows
// per-scene repositories to be configured independently in the future.
type SkillRepos struct {
	// Scene maps each IntentType to its scoped skill repository.
	Scene map[enumor.IntentType]skillpkg.Repository
}

// NewSkillRepos builds a SkillRepos where all provided scenes map to the same repository.
// Pass a nil repo to create an empty mapping (all SceneRepo calls return nil).
func NewSkillRepos(repo skillpkg.Repository, scenes ...enumor.IntentType) *SkillRepos {
	m := make(map[enumor.IntentType]skillpkg.Repository, len(scenes))
	for _, s := range scenes {
		m[s] = repo
	}
	return &SkillRepos{Scene: m}
}

// SceneRepo returns the skill repository for the given scene.
// Returns nil if the scene is not registered or the repository is nil.
func (r *SkillRepos) SceneRepo(scene enumor.IntentType) skillpkg.Repository {
	if r == nil {
		return nil
	}
	return r.Scene[scene]
}
