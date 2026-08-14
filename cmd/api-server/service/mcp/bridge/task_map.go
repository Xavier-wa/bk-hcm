/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package bridge

import (
	"container/list"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mcpmetrics "hcm/cmd/api-server/service/mcp/metrics"
	"hcm/pkg/logs"
)

const (
	defaultProgressTaskMapMaxSize  = 10000
	defaultProgressTaskMapEntryTTL = 30 * time.Minute
	unknownRid                     = "unknown"
)

type progressTaskEntry struct {
	key      string
	taskID   string
	expireAt time.Time
}

// ProgressTaskMap 维护 MCP progressToken 到 A2A taskId 的并发安全映射。
type ProgressTaskMap struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	lru     *list.List
	maxSize int
	ttl     time.Duration
}

// NewProgressTaskMap 构造 progressToken 映射表。
func NewProgressTaskMap(maxSize int, ttl time.Duration) *ProgressTaskMap {
	if maxSize <= 0 {
		maxSize = defaultProgressTaskMapMaxSize
	}
	if ttl <= 0 {
		ttl = defaultProgressTaskMapEntryTTL
	}
	return &ProgressTaskMap{
		entries: make(map[string]*list.Element),
		lru:     list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Set 写入或更新一个 progressToken→taskId 映射。
func (m *ProgressTaskMap) Set(progressToken interface{}, taskID string) {
	if m == nil || progressToken == nil || taskID == "" {
		return
	}

	key := progressTokenKey(progressToken)
	if key == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.cleanupExpiredLocked(now)
	if elem, ok := m.entries[key]; ok {
		entry := elem.Value.(*progressTaskEntry)
		entry.taskID = taskID
		entry.expireAt = now.Add(m.ttl)
		m.lru.MoveToBack(elem)
		mcpmetrics.SetBridgeActiveTasks(float64(len(m.entries)))
		return
	}

	entry := &progressTaskEntry{
		key:      key,
		taskID:   taskID,
		expireAt: now.Add(m.ttl),
	}
	m.entries[key] = m.lru.PushBack(entry)
	for len(m.entries) > m.maxSize {
		m.removeOldestLocked("overflow")
	}
	mcpmetrics.SetBridgeActiveTasks(float64(len(m.entries)))
}

// Get 查找 progressToken 对应的 taskId，并刷新 LRU 顺序。
func (m *ProgressTaskMap) Get(progressToken interface{}) (string, bool) {
	if m == nil || progressToken == nil {
		return "", false
	}

	key := progressTokenKey(progressToken)
	if key == "" {
		return "", false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	elem, ok := m.entries[key]
	if !ok {
		return "", false
	}
	entry := elem.Value.(*progressTaskEntry)
	if !entry.expireAt.After(time.Now()) {
		m.removeLocked(elem)
		mcpmetrics.SetBridgeActiveTasks(float64(len(m.entries)))
		return "", false
	}
	m.lru.MoveToBack(elem)
	return entry.taskID, true
}

// Delete 删除一个 progressToken 映射。
func (m *ProgressTaskMap) Delete(progressToken interface{}) {
	if m == nil || progressToken == nil {
		return
	}

	key := progressTokenKey(progressToken)
	if key == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if elem, ok := m.entries[key]; ok {
		m.removeLocked(elem)
		mcpmetrics.SetBridgeActiveTasks(float64(len(m.entries)))
	}
}

// Len 返回当前映射表大小。
func (m *ProgressTaskMap) Len() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

func (m *ProgressTaskMap) cleanupExpiredLocked(now time.Time) {
	for elem := m.lru.Front(); elem != nil; {
		next := elem.Next()
		entry := elem.Value.(*progressTaskEntry)
		if entry.expireAt.After(now) {
			break
		}
		m.removeLocked(elem)
		elem = next
	}
}

func (m *ProgressTaskMap) removeOldestLocked(reason string) {
	elem := m.lru.Front()
	if elem == nil {
		return
	}
	entry := elem.Value.(*progressTaskEntry)
	logs.Warnf("bridge: evict progress task map entry, reason=%s, progressToken=%s, taskId=%s, rid: %s",
		reason, entry.key, entry.taskID, unknownRid)
	m.removeLocked(elem)
}

func (m *ProgressTaskMap) removeLocked(elem *list.Element) {
	entry := elem.Value.(*progressTaskEntry)
	delete(m.entries, entry.key)
	m.lru.Remove(elem)
}

func progressTokenKey(progressToken interface{}) string {
	switch v := progressToken.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(b)
	}
}
