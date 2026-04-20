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

// Package memory ...
package memory

import "errors"

// AddMemoryReq is the request body for adding a memory entry.
type AddMemoryReq struct {
	// Memory is the content to remember.
	Memory string `json:"memory"`
	// Topics is an optional list of topic tags.
	Topics []string `json:"topics,omitempty"`
}

// Validate validates the request body.
func (r *AddMemoryReq) Validate() error {
	if r.Memory == "" {
		return errors.New("memory is required")
	}
	return nil
}

// MemoryEntry is the response representation of a single memory entry.
type MemoryEntry struct {
	ID        string   `json:"id"`
	Memory    string   `json:"memory"`
	Topics    []string `json:"topics,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
	UpdatedAt string   `json:"updated_at,omitempty"`
}

// ListMemoriesResp is the response body for listing memory entries.
type ListMemoriesResp struct {
	Memories []MemoryEntry `json:"memories"`
	Total    int           `json:"total"`
}
