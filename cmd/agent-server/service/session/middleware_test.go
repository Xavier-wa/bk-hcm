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
	"testing"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
)

func TestResolverUpdateCachedSessionTag(t *testing.T) {
	r := NewResolver(nil)

	// 缓存中无该会话时为 no-op，不应预热缓存
	r.UpdateCachedSessionTag(kit.New(), "missing", enumor.IntentTypeHostApply)
	if _, ok := r.cache.Get("missing"); ok {
		t.Errorf("UpdateCachedSessionTag should not populate cache for absent key")
	}

	// 缓存命中时仅更新 SessionTag，保留 ThreadID
	r.cache.Set("c1", SessionMeta{ThreadID: "t1"})
	r.UpdateCachedSessionTag(kit.New(), "c1", enumor.IntentTypeHostApply)

	meta, ok := r.cache.Get("c1")
	if !ok {
		t.Fatalf("cached entry missing after update")
	}
	if meta.ThreadID != "t1" {
		t.Errorf("ThreadID = %q, want t1", meta.ThreadID)
	}
	if meta.SessionTag != enumor.IntentTypeHostApply {
		t.Errorf("SessionTag = %q, want %q", meta.SessionTag, enumor.IntentTypeHostApply)
	}
}
