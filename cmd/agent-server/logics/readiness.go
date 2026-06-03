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

package logics

import (
	"sync/atomic"
)

// Readiness tracks whether agent-server is ready to serve AGUI traffic.
// Two independent conditions must both be satisfied:
//   - skillReady:  BKAIDev skill initial sync has completed (or sync is disabled).
//   - promptReady: Remote prompt initial sync has completed (reserved for future use;
//     defaults to true when prompt remote sync is not configured).
type Readiness struct {
	skillReady  atomic.Bool
	promptReady atomic.Bool
}

// NewReadiness creates a Readiness instance.
// Both skillReady and promptReady start as false; each is set to true once
// its corresponding initial sync completes (or is skipped when sync is disabled).
func NewReadiness() *Readiness {
	return &Readiness{}
}

// MarkSkillReady records that the initial skill sync has completed.
func (r *Readiness) MarkSkillReady() {
	r.skillReady.Store(true)
}

// MarkPromptReady records that the initial prompt sync has completed.
func (r *Readiness) MarkPromptReady() {
	r.promptReady.Store(true)
}

// SkillReady reports whether the skill initial sync has finished.
func (r *Readiness) SkillReady() bool {
	return r.skillReady.Load()
}

// PromptReady reports whether the prompt initial sync has finished.
func (r *Readiness) PromptReady() bool {
	return r.promptReady.Load()
}

// IsReady reports whether both skill and prompt syncs are complete.
func (r *Readiness) IsReady() bool {
	return r.skillReady.Load() && r.promptReady.Load()
}
