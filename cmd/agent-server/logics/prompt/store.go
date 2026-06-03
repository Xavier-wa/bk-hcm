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

// Package prompt provides the prompt Store, BKAIDev syncer, and BeforeModel callbacks
// for agent-server prompt hosting. Entry point: NewManager.
package prompt

import (
	"time"

	"hcm/pkg/tools/localstore"
)

// PromptEntry holds the content and sync metadata for a single named prompt.
// All fields are exported and tagged for JSON serialisation so the store can
// persist and restore entries across restarts.
type PromptEntry struct {
	Content   string    `json:"content"`
	MD5       string    `json:"md5"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store is the prompt store type used throughout agent-server.
// It is backed by a JSON file on disk so content survives restarts without
// waiting for a BKAIDev sync cycle. When created with an empty path (local-file
// mode), all writes are memory-only.
type Store = localstore.Store[PromptEntry]

// NewStore creates a prompt Store backed by the given file path.
// When path is empty the store operates in memory-only mode (no persistence).
func NewStore(path string) *Store {
	return localstore.NewStore[PromptEntry](path)
}
