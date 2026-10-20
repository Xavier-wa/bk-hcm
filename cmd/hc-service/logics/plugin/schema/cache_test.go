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

package schema

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	coreplugin "hcm/pkg/api/core/plugin"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"

	tfjson "github.com/hashicorp/terraform-json"
)

type mockSchemaStore struct {
	mu            sync.RWMutex
	plugins       map[cacheKey]*coreplugin.IacPluginTempDelete
	schemas       map[cacheKey]*coreplugin.PluginSchema
	hashCallCount int32
	fullCallCount int32
	fullQueryHook func()
}

func newMockSchemaStore() *mockSchemaStore {
	return &mockSchemaStore{
		plugins: make(map[cacheKey]*coreplugin.IacPluginTempDelete),
		schemas: make(map[cacheKey]*coreplugin.PluginSchema),
	}
}

func (m *mockSchemaStore) GetPlugin(kt *kit.Kit, pluginName, pluginVersion string) (
	*coreplugin.IacPluginTempDelete, bool, error) {

	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[cacheKey{PluginName: pluginName, PluginVersion: pluginVersion}]
	return p, ok, nil
}

func (m *mockSchemaStore) GetSchemaHash(kt *kit.Kit, pluginName, pluginVersion string) (string, bool, error) {
	atomic.AddInt32(&m.hashCallCount, 1)
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.schemas[cacheKey{PluginName: pluginName, PluginVersion: pluginVersion}]
	if !ok {
		return "", false, nil
	}
	return s.SchemaHash, true, nil
}

func (m *mockSchemaStore) GetFullSchema(kt *kit.Kit, pluginName, pluginVersion string) (
	*coreplugin.PluginSchema, bool, error) {

	atomic.AddInt32(&m.fullCallCount, 1)
	if m.fullQueryHook != nil {
		m.fullQueryHook()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.schemas[cacheKey{PluginName: pluginName, PluginVersion: pluginVersion}]
	return s, ok, nil
}

func (m *mockSchemaStore) CreateSchema(kt *kit.Kit, schema *coreplugin.PluginSchema) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := cacheKey{PluginName: schema.PluginName, PluginVersion: schema.PluginVersion}
	if _, exists := m.schemas[key]; exists {
		return "", errf.New(errf.Aborted, "duplicate key")
	}
	copied := *schema
	m.schemas[key] = &copied
	return "1", nil
}

func buildTestPluginSchema(t *testing.T, name, version, desc string) *coreplugin.PluginSchema {
	t.Helper()
	raw := coreplugin.RawSchema{
		ProtocolVersion: "1.0",
		Capabilities: map[string]bool{
			coreplugin.CapabilityApply:   true,
			coreplugin.CapabilityPlan:    true,
			coreplugin.CapabilityRefresh: true,
		},
		Provider: &tfjson.Schema{
			Version: 0,
			Block: &tfjson.SchemaBlock{
				Description: desc,
			},
		},
		ResourceSchemas: map[string]*tfjson.Schema{
			"tencentcloud_vpc": {
				Version: 0,
				Block:   &tfjson.SchemaBlock{},
			},
		},
	}
	hash, err := coreplugin.ComputeRawSchemaHash(&raw)
	if err != nil {
		t.Fatalf("compute raw schema hash failed: %v", err)
	}
	return &coreplugin.PluginSchema{
		PluginName:    name,
		PluginVersion: version,
		SchemaHash:    hash,
		RawSchema:     raw,
	}
}

// waitSchemaCached 轮询同包可见的缓存，直到异步写入到达期望的 schema_hash 且尚未过期。
func waitSchemaCached(t *testing.T, cache *SchemaCache, key cacheKey, hash string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		cache.mu.RLock()
		cached, ok := cache.schemaCache[key]
		expireAt, hasExpire := cache.expireMap[key]
		cache.mu.RUnlock()
		if ok && cached != nil && cached.SchemaHash == hash && hasExpire && cache.nowFunc().Before(expireAt) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("schema cache was not updated for %s@%s hash %s", key.PluginName, key.PluginVersion, hash)
}

func TestSchemaCacheHitMissExpireAndMismatch(t *testing.T) {
	store := newMockSchemaStore()
	s1 := buildTestPluginSchema(t, "tencentcloud", "1.81.5", "v1")
	key := cacheKey{PluginName: "tencentcloud", PluginVersion: "1.81.5"}
	store.schemas[key] = s1

	now := time.Now()
	var nowMu sync.RWMutex
	cache := NewSchemaCacheWithStore(store, 7*24*time.Hour)
	defer cache.Close()
	cache.nowFunc = func() time.Time {
		nowMu.RLock()
		defer nowMu.RUnlock()
		return now
	}

	kt := kit.New()

	// 1. Cold start cache miss -> loads full schema from DB and asynchronously writes to cache
	got, err := cache.Get(kt, "tencentcloud", "1.81.5")
	if err != nil {
		t.Fatalf("Get on cold start failed: %v", err)
	}
	if got.SchemaHash != s1.SchemaHash {
		t.Fatalf("expected hash %s, got %s", s1.SchemaHash, got.SchemaHash)
	}
	waitSchemaCached(t, cache, key, s1.SchemaHash)
	if atomic.LoadInt32(&store.fullCallCount) != 1 {
		t.Fatalf("expected 1 full DB query on miss, got %d", store.fullCallCount)
	}

	// 2. Cache hit & not expired & hash matches -> only queries schema_hash, refreshes TTL
	nowMu.Lock()
	now = now.Add(24 * time.Hour)
	nowMu.Unlock()

	got2, err := cache.Get(kt, "tencentcloud", "1.81.5")
	if err != nil {
		t.Fatalf("Get on cache hit failed: %v", err)
	}
	if got2.SchemaHash != s1.SchemaHash {
		t.Fatalf("expected hash %s, got %s", s1.SchemaHash, got2.SchemaHash)
	}
	if atomic.LoadInt32(&store.hashCallCount) != 1 || atomic.LoadInt32(&store.fullCallCount) != 1 {
		t.Fatalf("expected 1 hash call and still 1 full call, got hash=%d full=%d",
			store.hashCallCount, store.fullCallCount)
	}

	// 3. Cache hit & not expired, but DB schema_hash changed -> reloads full schema from DB & updates cache
	s2 := buildTestPluginSchema(t, "tencentcloud", "1.81.5", "v2-updated")
	store.mu.Lock()
	store.schemas[key] = s2
	store.mu.Unlock()

	got3, err := cache.Get(kt, "tencentcloud", "1.81.5")
	if err != nil {
		t.Fatalf("Get on hash mismatch failed: %v", err)
	}
	if got3.SchemaHash != s2.SchemaHash {
		t.Fatalf("expected updated schema hash %s, got %s", s2.SchemaHash, got3.SchemaHash)
	}
	waitSchemaCached(t, cache, key, s2.SchemaHash)
	if atomic.LoadInt32(&store.fullCallCount) != 2 {
		t.Fatalf("expected 2 full DB queries after hash mismatch, got %d", store.fullCallCount)
	}

	// 4. Cache expired -> triggers eviction, reloads full schema from DB & updates cache
	nowMu.Lock()
	now = now.Add(8 * 24 * time.Hour)
	nowMu.Unlock()

	got4, err := cache.Get(kt, "tencentcloud", "1.81.5")
	if err != nil {
		t.Fatalf("Get after expiration failed: %v", err)
	}
	if got4.SchemaHash != s2.SchemaHash {
		t.Fatalf("expected hash %s, got %s", s2.SchemaHash, got4.SchemaHash)
	}
	waitSchemaCached(t, cache, key, s2.SchemaHash)
	if atomic.LoadInt32(&store.fullCallCount) != 3 {
		t.Fatalf("expected 3 full DB queries after expiration, got %d", store.fullCallCount)
	}

	// 5. DB record deleted -> returns RecordNotFound error and evicts cache entry
	store.mu.Lock()
	delete(store.schemas, key)
	store.mu.Unlock()

	if _, err = cache.Get(kt, "tencentcloud", "1.81.5"); err == nil {
		t.Fatalf("expected error when schema record is missing in DB")
	}
}

func TestSchemaCacheSingleflightConcurrency(t *testing.T) {
	store := newMockSchemaStore()
	s1 := buildTestPluginSchema(t, "tencentcloud", "1.81.5", strings.Repeat("x", 16))
	key := cacheKey{PluginName: "tencentcloud", PluginVersion: "1.81.5"}
	store.schemas[key] = s1

	store.fullQueryHook = func() {
		time.Sleep(30 * time.Millisecond)
	}

	cache := NewSchemaCacheWithStore(store, DefaultSchemaCacheTTL)
	defer cache.Close()

	const workers = 10
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			res, err := cache.Get(kit.New(), "tencentcloud", "1.81.5")
			if err != nil {
				t.Errorf("concurrent Get failed: %v", err)
				return
			}
			if res.SchemaHash != s1.SchemaHash {
				t.Errorf("unexpected hash: %s", res.SchemaHash)
			}
		}()
	}
	wg.Wait()

	if calls := atomic.LoadInt32(&store.fullCallCount); calls != 1 {
		t.Fatalf("expected singleflight to coalesce 10 concurrent misses into 1 DB query, got %d", calls)
	}
}
