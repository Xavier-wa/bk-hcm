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

// Package localstore provides a thread-safe, atomically-written local JSON
// key-value file store. Each Store[T] is bound to a single JSON file on disk
// and maps string keys to caller-defined values of type T.
//
// Typical usage: one Store per sync domain (e.g. skills, prompts), each
// backed by its own dedicated file so they never interfere with each other.
package localstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store is a generic, thread-safe local file store that persists a map of
// string keys to values of type T as a JSON file on disk.
// Writes are atomic: data is written to a temp file then renamed into place.
type Store[T any] struct {
	path string
	mu   sync.RWMutex
	data map[string]T
}

// NewStore creates a Store bound to the given file path.
// The parent directory must exist before Save/Set/Remove are called.
func NewStore[T any](path string) *Store[T] {
	return &Store[T]{
		path: path,
		data: make(map[string]T),
	}
}

// Load reads the JSON file from disk and replaces the in-memory state.
// If the file does not exist, Load returns nil (empty store) without error.
func (s *Store[T]) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		s.data = make(map[string]T)
		return nil
	}
	if err != nil {
		return fmt.Errorf("read localstore %s: %w", s.path, err)
	}

	var m map[string]T
	if err = json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("parse localstore %s: %w", s.path, err)
	}
	s.data = m
	return nil
}

// Save persists the current in-memory state to disk atomically
// (write temp → rename).
func (s *Store[T]) Save() error {
	s.mu.RLock()
	snap := s.snapshot()
	s.mu.RUnlock()
	return s.writeToDisk(snap)
}

// Get returns the value for key and reports whether it was found.
func (s *Store[T]) Get(key string) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

// Set updates the in-memory entry for key and persists the store atomically.
func (s *Store[T]) Set(key string, val T) error {
	s.mu.Lock()
	s.data[key] = val
	snap := s.snapshot()
	s.mu.Unlock()
	return s.writeToDisk(snap)
}

// Remove deletes the entry for key and persists the store atomically.
func (s *Store[T]) Remove(key string) error {
	s.mu.Lock()
	delete(s.data, key)
	snap := s.snapshot()
	s.mu.Unlock()
	return s.writeToDisk(snap)
}

// Keys returns a snapshot of all keys currently in the store.
func (s *Store[T]) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// snapshot returns a shallow copy of the in-memory map.
// The caller must hold at least a read lock before calling.
func (s *Store[T]) snapshot() map[string]T {
	cp := make(map[string]T, len(s.data))
	for k, v := range s.data {
		cp[k] = v
	}
	return cp
}

// writeToDisk atomically persists the given map to s.path.
func (s *Store[T]) writeToDisk(m map[string]T) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal localstore: %w", err)
	}

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".localstore-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp localstore: %w", err)
	}
	tmpName := tmp.Name()

	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp localstore: %w", err)
	}
	if err = tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp localstore: %w", err)
	}
	if err = os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename localstore: %w", err)
	}
	return nil
}
