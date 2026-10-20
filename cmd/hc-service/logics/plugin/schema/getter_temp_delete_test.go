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
	"testing"

	coreplugin "hcm/pkg/api/core/plugin"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/tools/json"
)

// TODO(temp_delete): 临时 SchemaGetter 自测用例，提交 MR 前与 getter_temp_delete.go 一并删除。

func TestStaticAndJSONSchemaGetterTempDeleteEndToEnd(t *testing.T) {
	store := newMockSchemaStore()
	regKey := cacheKey{PluginName: "tencentcloud", PluginVersion: "1.81.5"}
	store.plugins[regKey] = &coreplugin.IacPluginTempDelete{
		ID:            "p-1",
		PluginName:    "tencentcloud",
		PluginVersion: "1.81.5",
		Protocol:      coreplugin.ProtocolHTTP,
		State:         coreplugin.PluginStateInitTempDelete,
	}

	cache := NewSchemaCacheWithStore(store, DefaultSchemaCacheTTL)
	defer cache.Close()
	mgr := NewSchemaManagerWithStore(store, cache)
	kt := kit.New()

	defaultGetter := NewDefaultStaticSchemaGetterTempDelete(false)
	defaultSyncRes, err := mgr.Sync(kt, defaultGetter)
	if err != nil {
		t.Fatalf("Sync with NewDefaultStaticSchemaGetterTempDelete failed: %v", err)
	}
	if defaultSyncRes.SyncResult != enumor.SchemaSyncSuccess || defaultSyncRes.Schema == nil {
		t.Fatalf("expected default static schema sync_result=success, got: %+v", defaultSyncRes)
	}
	if _, hasSecretKey := defaultSyncRes.Schema.Provider.Block.Attributes["secret_key"]; !hasSecretKey {
		t.Fatalf("expected tencentcloud 1.81.5 provider block to include secret_key")
	}
	if _, hasAssumeRole := defaultSyncRes.Schema.Provider.Block.NestedBlocks["assume_role"]; !hasAssumeRole {
		t.Fatalf("expected tencentcloud 1.81.5 provider block to include assume_role nested block")
	}
	if len(defaultSyncRes.Schema.ResourceSchemas) != 614 {
		t.Fatalf("expected 614 resource_schemas in full tencentcloud 1.81.5 schema, got %d",
			len(defaultSyncRes.Schema.ResourceSchemas))
	}
	if len(defaultSyncRes.Schema.DataSourceSchemas) != 433 {
		t.Fatalf("expected 433 data_source_schemas in full tencentcloud 1.81.5 schema, got %d",
			len(defaultSyncRes.Schema.DataSourceSchemas))
	}

	delete(store.schemas, regKey)

	baseSchema := buildTestPluginSchema(t, "tencentcloud", "1.81.5", "self-test-provider")
	baseBytes, err := json.Marshal(baseSchema)
	if err != nil {
		t.Fatalf("marshal baseSchema failed: %v", err)
	}

	getter := NewJSONSchemaGetterTempDelete(baseBytes, true)
	syncRes, err := mgr.Sync(kt, getter)
	if err != nil {
		t.Fatalf("Sync with JSONSchemaGetterTempDelete failed: %v", err)
	}
	if syncRes.SyncResult != enumor.SchemaSyncSuccess || syncRes.SyncMessage != "" || syncRes.Schema == nil {
		t.Fatalf("expected sync_result=success, got: %+v", syncRes)
	}

	staticGetter := NewStaticSchemaGetterTempDelete(baseSchema, false)
	syncRes2, err := mgr.Sync(kt, staticGetter)
	if err != nil {
		t.Fatalf("Sync with StaticSchemaGetterTempDelete failed: %v", err)
	}
	if syncRes2.SyncResult != enumor.SchemaSyncSuccess || syncRes2.SyncMessage != "" || syncRes2.Schema == nil {
		t.Fatalf("expected sync_result=success on second idempotent sync, got: %+v", syncRes2)
	}

	loaded, err := mgr.GetSchema(kt, "tencentcloud", "1.81.5")
	if err != nil {
		t.Fatalf("GetSchema failed: %v", err)
	}
	if loaded.SchemaHash != baseSchema.SchemaHash {
		t.Fatalf("expected hash %s, got %s", baseSchema.SchemaHash, loaded.SchemaHash)
	}
}
