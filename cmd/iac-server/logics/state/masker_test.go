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

package state

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	coreplugin "hcm/pkg/api/core/plugin"
	dataplugin "hcm/pkg/api/data-service/plugin"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iac/iacstate"
	"hcm/pkg/kit"
	"hcm/pkg/serviced"

	tfjson "github.com/hashicorp/terraform-json"
)

type fakeDiscover struct{}

func (fakeDiscover) Discover(_ cc.Name) ([]string, error) {
	return []string{"http://127.0.0.1"}, nil
}

func (fakeDiscover) Services() []cc.Name {
	return []cc.Name{cc.DataServiceName, cc.HCServiceName}
}

func (f fakeDiscover) ByLabels(_ []string) serviced.Discover {
	return f
}

func (fakeDiscover) GetServiceAllNodeKeys(_ cc.Name) ([]string, error) {
	return nil, nil
}

type inMemoryHTTPClient struct {
	handler http.HandlerFunc
}

func (c *inMemoryHTTPClient) Do(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	c.handler(rec, req)
	return rec.Result(), nil
}

func newTestClientSet(pluginRes *dataplugin.IaCPluginListResult, schema *coreplugin.PluginSchema,
	schemaErr error) *client.ClientSet {

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/iac_plugins/list"):
			_ = json.NewEncoder(w).Encode(map[string]any{"code": errf.OK, "message": "", "data": pluginRes})
		case strings.HasSuffix(r.URL.Path, "/iac_plugins/schema"):
			if schemaErr != nil {
				_ = json.NewEncoder(w).Encode(map[string]any{"code": errf.Aborted, "message": schemaErr.Error()})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": errf.OK, "message": "", "data": schema})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"code": errf.Aborted, "message": "unexpected path"})
		}
	}
	return client.NewClientSet(&inMemoryHTTPClient{handler: handler}, fakeDiscover{})
}

func TestMaskStateObject_RecursiveNestedTypesAndBlocks(t *testing.T) {
	block := &tfjson.SchemaBlock{
		Attributes: map[string]*tfjson.SchemaAttribute{
			"id":       {Sensitive: false},
			"password": {Sensitive: true},
			"nil_sec":  {Sensitive: true},
			"auth_cfg": {
				AttributeNestedType: &tfjson.SchemaNestedAttributeType{
					NestingMode: tfjson.SchemaNestingModeSingle,
					Attributes: map[string]*tfjson.SchemaAttribute{
						"user":  {Sensitive: false},
						"token": {Sensitive: true},
					},
				},
			},
			"secrets_map": {
				AttributeNestedType: &tfjson.SchemaNestedAttributeType{
					NestingMode: tfjson.SchemaNestingModeMap,
					Attributes: map[string]*tfjson.SchemaAttribute{
						"key":    {Sensitive: false},
						"secret": {Sensitive: true},
					},
				},
			},
		},
		NestedBlocks: map[string]*tfjson.SchemaBlockType{
			"data_disks": {
				NestingMode: tfjson.SchemaNestingModeList,
				Block: &tfjson.SchemaBlock{
					Attributes: map[string]*tfjson.SchemaAttribute{
						"size":        {Sensitive: false},
						"encrypt_key": {Sensitive: true},
					},
				},
			},
		},
	}

	orig := &iacstate.StateObject{
		Plugin:         "tcloud",
		Type:           "tencentcloud_instance",
		DependsOn:      []string{"tencentcloud_vpc.main"},
		Private:        "base64-private-data",
		Identity:       map[string]any{"instance_id": "ins-123"},
		ProviderConfig: map[string]any{"region": "ap-guangzhou"},
		State: map[string]any{
			"id":       "ins-123",
			"password": "plain-password",
			"nil_sec":  nil,
			"auth_cfg": map[string]any{
				"user":  "root",
				"token": "top-secret-token",
			},
			"secrets_map": map[string]any{
				"db": map[string]any{"key": "k1", "secret": "s1"},
			},
			"data_disks": []any{
				map[string]any{"size": 100, "encrypt_key": "kms-key-1"},
			},
		},
	}

	masked, err := MaskStateObject(orig, block)
	if err != nil {
		t.Fatalf("MaskStateObject failed: %v", err)
	}

	// 验证原对象未被修改
	if orig.State["password"] != "plain-password" {
		t.Fatalf("original state map was mutated")
	}

	// 验证脱敏结果
	if masked.State["id"] != "ins-123" {
		t.Fatalf("non-sensitive id changed: %v", masked.State["id"])
	}
	if masked.State["password"] != constant.IacStateSensitiveValueMask {
		t.Fatalf("expected password masked, got %v", masked.State["password"])
	}
	if masked.State["nil_sec"] != nil {
		t.Fatalf("expected nil sensitive field to stay nil, got %v", masked.State["nil_sec"])
	}

	authCfg := masked.State["auth_cfg"].(map[string]any)
	if authCfg["user"] != "root" || authCfg["token"] != constant.IacStateSensitiveValueMask {
		t.Fatalf("unexpected auth_cfg: %+v", authCfg)
	}

	secMap := masked.State["secrets_map"].(map[string]any)["db"].(map[string]any)
	if secMap["key"] != "k1" || secMap["secret"] != constant.IacStateSensitiveValueMask {
		t.Fatalf("unexpected secrets_map: %+v", secMap)
	}

	disks := masked.State["data_disks"].([]any)
	disk0 := disks[0].(map[string]any)
	if disk0["size"] != 100 || disk0["encrypt_key"] != constant.IacStateSensitiveValueMask {
		t.Fatalf("unexpected data_disks: %+v", disk0)
	}

	// 验证 depends_on、identity、provider_config 原样保留且序列化不包含 private 与 schema_version 字段
	if len(masked.DependsOn) != 1 || masked.DependsOn[0] != "tencentcloud_vpc.main" {
		t.Fatalf("expected depends_on preserved, got %+v", masked.DependsOn)
	}
	if masked.Identity["instance_id"] != "ins-123" {
		t.Fatalf("expected identity preserved, got %+v", masked.Identity)
	}
	if masked.ProviderConfig["region"] != "ap-guangzhou" {
		t.Fatalf("expected provider_config preserved, got %+v", masked.ProviderConfig)
	}
	rawJSON, _ := json.Marshal(masked)
	if strings.Contains(string(rawJSON), "private") || strings.Contains(string(rawJSON), "base64-private-data") ||
		strings.Contains(string(rawJSON), "schema_version") {
		t.Fatalf("masked json must not contain private or schema_version field: %s", string(rawJSON))
	}
}

func TestSchemaResolver_ResourceDataAndFailClosed(t *testing.T) {
	kt := kit.New()
	fullSchema := &coreplugin.PluginSchema{
		Plugin: coreplugin.PluginInfo{Name: "tcloud", Version: "1.2.0"},
		SchemaDetail: coreplugin.SchemaDetail{
			ProviderSchema: tfjson.ProviderSchema{
				ResourceSchemas: map[string]*tfjson.Schema{
					"tencentcloud_vpc": {Block: &tfjson.SchemaBlock{}},
				},
				DataSourceSchemas: map[string]*tfjson.Schema{
					"tencentcloud_availability_zones": {Block: &tfjson.SchemaBlock{}},
				},
			},
		},
	}

	enabledRes := &dataplugin.IaCPluginListResult{
		Details: []coreplugin.IaCPlugin{
			{PluginName: "tcloud", PluginVersion: "1.2.0", State: enumor.IaCPluginStateEnabled},
		},
	}

	if _, err := NewSchemaResolver(nil); err == nil {
		t.Fatal("expected error when clientSet is nil")
	}

	resolver, err := NewSchemaResolver(newTestClientSet(enabledRes, fullSchema, nil))
	if err != nil {
		t.Fatalf("NewSchemaResolver failed: %v", err)
	}

	if _, err := resolver.ResolveBlock(kt, enumor.IacStateKindResource, "tcloud", "tencentcloud_vpc"); err != nil {
		t.Fatalf("expected resource block resolved, got err: %v", err)
	}
	if _, err := resolver.ResolveBlock(kt, enumor.IacStateKindData, "tcloud",
		"tencentcloud_availability_zones"); err != nil {
		t.Fatalf("expected data block resolved, got err: %v", err)
	}

	// 未知资源类型 Fail-Closed
	if _, err := resolver.ResolveBlock(kt, enumor.IacStateKindResource, "tcloud", "unknown_type"); err == nil {
		t.Fatal("expected error for unknown resource type")
	}

	// 无 enabled 插件 Fail-Closed
	noPluginResolver, err := NewSchemaResolver(newTestClientSet(&dataplugin.IaCPluginListResult{}, fullSchema, nil))
	if err != nil {
		t.Fatalf("NewSchemaResolver failed: %v", err)
	}
	if _, err := noPluginResolver.ResolveBlock(kt, enumor.IacStateKindResource, "tcloud",
		"tencentcloud_vpc"); err == nil {
		t.Fatal("expected error when no enabled plugin found")
	}

	// hc-service 返回错误 Fail-Closed
	errSchemaResolver, err := NewSchemaResolver(newTestClientSet(enabledRes, nil,
		errors.New("hc-service unavailable")))
	if err != nil {
		t.Fatalf("NewSchemaResolver failed: %v", err)
	}
	if _, err := errSchemaResolver.ResolveBlock(kt, enumor.IacStateKindResource, "tcloud",
		"tencentcloud_vpc"); err == nil {
		t.Fatal("expected error when hc-service GetSchema fails")
	}
}
