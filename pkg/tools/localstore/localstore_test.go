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

package localstore

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

type testEntry struct {
	Version string `json:"version"`
}

func newTempStore(t *testing.T) (*Store[testEntry], string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	return NewStore[testEntry](path), path
}

func TestStore_LoadMissingFile(t *testing.T) {
	s, _ := newTempStore(t)
	if err := s.Load(); err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	_, ok := s.Get("any")
	if ok {
		t.Fatal("expected empty store")
	}
}

func TestStore_SetAndGet(t *testing.T) {
	s, _ := newTempStore(t)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	entry := testEntry{Version: "v1"}
	if err := s.Set("item-a", entry); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, ok := s.Get("item-a")
	if !ok {
		t.Fatal("expected entry to exist")
	}
	if got.Version != "v1" {
		t.Errorf("version: want v1, got %s", got.Version)
	}
}

func TestStore_AtomicWrite(t *testing.T) {
	s, path := newTempStore(t)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	if err := s.Set("item-a", testEntry{Version: "v1"}); err != nil {
		t.Fatal(err)
	}

	s2 := NewStore[testEntry](path)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	if _, ok := s2.Get("item-a"); !ok {
		t.Fatal("entry not persisted to disk")
	}
}

func TestStore_Remove(t *testing.T) {
	s, _ := newTempStore(t)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	if err := s.Set("item-a", testEntry{Version: "v1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("item-a"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := s.Get("item-a"); ok {
		t.Fatal("expected entry to be removed")
	}
}

func TestStore_Keys(t *testing.T) {
	s, _ := newTempStore(t)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	for _, k := range []string{"a", "b", "c"} {
		if err := s.Set(k, testEntry{Version: "v1"}); err != nil {
			t.Fatal(err)
		}
	}

	keys := s.Keys()
	sort.Strings(keys)

	want := []string{"a", "b", "c"}
	if len(keys) != len(want) {
		t.Fatalf("keys: want %v, got %v", want, keys)
	}
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("keys[%d]: want %s, got %s", i, want[i], k)
		}
	}
}

func TestStore_ConcurrentReadWrite(t *testing.T) {
	s, _ := newTempStore(t)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := "item-a"
			_ = s.Set(key, testEntry{Version: "v1"})
			s.Get(key)
		}()
	}
	wg.Wait()
}

func TestStore_LoadAfterManualDelete(t *testing.T) {
	s, path := newTempStore(t)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	if err := s.Set("x", testEntry{Version: "v1"}); err != nil {
		t.Fatal(err)
	}
	os.Remove(path)

	s2 := NewStore[testEntry](path)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load on deleted file: %v", err)
	}
}
