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

package session

import (
	"hcm/pkg/api/core"
	dsaiagent "hcm/pkg/api/data-service/aiagent"
	dataservice "hcm/pkg/client/data-service"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/metadata"
)

// SessionMeta is the cached session metadata keyed by sessionCode.
type SessionMeta struct {
	// ThreadID 框架内部 thread id（与会话 id 相同）。
	ThreadID string
	// SessionTag is the session-level scene tag.
	SessionTag enumor.IntentType
}

// Resolver resolves sessionCode → session metadata with LRU cache.
type Resolver struct {
	cache   *metadata.LRUCache[SessionMeta]
	dataSvc *dataservice.Client
}

// NewResolver creates a Resolver with LRU cache (capacity 10000, TTL 30 min).
func NewResolver(dataSvc *dataservice.Client) *Resolver {
	return &Resolver{
		cache: metadata.NewLRUCache[SessionMeta](constant.SessionCacheResolverCapacity,
			constant.SessionCacheResolverTTL),
		dataSvc: dataSvc,
	}
}

// Resolve returns the threadID for the given sessionCode.
func (r *Resolver) Resolve(kt *kit.Kit, sessionCode string) (string, error) {
	meta, err := r.ResolveMeta(kt, sessionCode)
	if err != nil {
		return "", err
	}
	return meta.ThreadID, nil
}

// ResolveMeta returns the cached session metadata (threadID + sessionTag) for the given sessionCode.
// It checks the local cache first; on miss it queries data-service once and caches the result.
func (r *Resolver) ResolveMeta(kt *kit.Kit, sessionCode string) (SessionMeta, error) {
	if meta, ok := r.cache.Get(sessionCode); ok {
		return meta, nil
	}

	listReq := &dsaiagent.ListAiagentSessionReq{
		Filter: tools.EqualExpression("session_code", sessionCode),
		Page:   &core.BasePage{Start: 0, Limit: 1},
	}

	result, err := r.dataSvc.Aiagent.Session.List(kt, listReq)
	if err != nil {
		logs.Errorf("list session by code %s failed: %v, rid: %s", sessionCode, err, kt.Rid)
		return SessionMeta{}, err
	}

	if len(result.Details) == 0 {
		return SessionMeta{}, errf.Newf(errf.RecordNotFound, "session not found for code: %s", sessionCode)
	}

	meta := SessionMeta{
		ThreadID:   result.Details[0].ThreadID,
		SessionTag: result.Details[0].SessionTag,
	}
	r.cache.Set(sessionCode, meta)
	return meta, nil
}

// UpdateCachedSessionTag refreshes the cached session tag after a runtime write-back.
// 仅当缓存中已有该会话条目时更新，避免无谓地预热缓存。
func (r *Resolver) UpdateCachedSessionTag(kt *kit.Kit, sessionCode string, tag enumor.IntentType) {
	if meta, ok := r.cache.Get(sessionCode); ok {
		meta.SessionTag = tag
		r.cache.Set(sessionCode, meta)
		logs.Infof("update cached session tag: success, session_code: %s, tag: %s, rid: %s", sessionCode, tag, kt.Rid)
		return
	}

	logs.Infof("update cached session tag: session not found, session_code: %s, tag: %s, rid: %s",
		sessionCode, tag, kt.Rid)
}
