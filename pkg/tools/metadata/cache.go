/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package metadata ...
package metadata

import (
	"container/list"
	"sync"
	"time"

	"hcm/pkg/dal/watch"
)

// LRUEntry is a single entry in the LRU cache.
type LRUEntry[V any] struct {
	key       string
	value     V
	expiresAt time.Time
}

// LRUCache is a thread-safe LRU cache with TTL.
type LRUCache[V any] struct {
	mu       sync.Mutex
	capacity int
	ttl      time.Duration
	ll       *list.List
	items    map[string]*list.Element
}

// NewLRUCache creates an LRU cache with given capacity and TTL.
func NewLRUCache[V any](capacity int, ttl time.Duration) *LRUCache[V] {
	return &LRUCache[V]{
		capacity: capacity,
		ttl:      ttl,
		ll:       list.New(),
		items:    make(map[string]*list.Element, capacity),
	}
}

// Get retrieves a value, returning (zero value, false) if not found or expired.
func (c *LRUCache[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var zero V
	elem, ok := c.items[key]
	if !ok {
		return zero, false
	}

	entry := elem.Value.(*LRUEntry[V])
	if time.Now().After(entry.expiresAt) {
		c.ll.Remove(elem)
		delete(c.items, key)
		return zero, false
	}

	c.ll.MoveToFront(elem)
	return entry.value, true
}

// Set inserts or updates a key-value pair, evicting the LRU entry if at capacity.
func (c *LRUCache[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.ll.MoveToFront(elem)
		elem.Value.(*LRUEntry[V]).value = value
		elem.Value.(*LRUEntry[V]).expiresAt = time.Now().Add(c.ttl)
		return
	}

	if c.ll.Len() >= c.capacity {
		oldest := c.ll.Back()
		if oldest != nil {
			c.ll.Remove(oldest)
			delete(c.items, oldest.Value.(*LRUEntry[V]).key)
		}
	}

	entry := &LRUEntry[V]{key: key, value: value, expiresAt: time.Now().Add(c.ttl)}
	elem := c.ll.PushFront(entry)
	c.items[key] = elem
}

// SearchHostWithInnerIPOption search host with inner ip
type SearchHostWithInnerIPOption struct {
	InnerIP string `json:"bk_host_innerip"`
	CloudID int64  `json:"bk_cloud_id"`
	// only return these fields in hosts.
	Fields []string `json:"fields"`
}

// SearchHostWithIDOption search host with id
type SearchHostWithIDOption struct {
	HostID int64 `json:"bk_host_id"`
	// only return these fields in hosts.
	Fields []string `json:"fields"`
}

// ListWithIDOption list hosts with ids
type ListWithIDOption struct {
	// length range is [1,500]
	IDs []int64 `json:"ids"`
	// only return these fields in hosts.
	Fields []string `json:"fields"`
}

// DeleteArchive delete archive
type DeleteArchive struct {
	Oid    string      `json:"oid" bson:"oid"`
	Coll   string      `json:"coll" bson:"coll"`
	Detail interface{} `json:"detail" bson:"detail"`
}

// ListHostWithPage list hosts with page in cache, which page info is in redis cache.
// store in a zset.
type ListHostWithPage struct {
	// length range is [1,1000]
	HostIDs []int64 `json:"bk_host_ids"`
	// only return these fields in hosts.
	Fields []string `json:"fields"`
	// sort field is not used.
	// max page limit is 1000
	Page BasePage `json:"page"`
}

// WatchEventResp watch event response
type WatchEventResp struct {
	BaseResp `json:",inline"`
	Data     *watch.WatchResp `json:"data"`
}
