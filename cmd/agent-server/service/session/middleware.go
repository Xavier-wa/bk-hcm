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
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/metadata"
)

// Resolver resolves sessionCode → threadID with LRU cache.
type Resolver struct {
	cache   *metadata.LRUCache[string]
	dataSvc *dataservice.Client
}

// NewResolver creates a Resolver with LRU cache (capacity 10000, TTL 30 min).
func NewResolver(dataSvc *dataservice.Client) *Resolver {
	return &Resolver{
		cache:   metadata.NewLRUCache[string](constant.SessionCacheResolverCapacity, constant.SessionCacheResolverTTL),
		dataSvc: dataSvc,
	}
}

// Resolve returns the threadID for the given sessionCode.
// It checks the local cache first; on miss it queries data-service and caches the result.
func (r *Resolver) Resolve(kt *kit.Kit, sessionCode string) (string, error) {
	if threadID, ok := r.cache.Get(sessionCode); ok {
		return threadID, nil
	}

	listReq := &dsaiagent.ListAiagentSessionReq{
		Filter: tools.EqualExpression("session_code", sessionCode),
		Page:   &core.BasePage{Start: 0, Limit: 1},
	}

	result, err := r.dataSvc.Aiagent.Session.List(kt, listReq)
	if err != nil {
		logs.Errorf("list session by code %s failed: %v, rid: %s", sessionCode, err, kt.Rid)
		return "", err
	}

	if len(result.Details) == 0 {
		return "", errf.Newf(errf.RecordNotFound, "session not found for code: %s", sessionCode)
	}

	threadID := result.Details[0].ThreadID
	r.cache.Set(sessionCode, threadID)
	return threadID, nil
}
