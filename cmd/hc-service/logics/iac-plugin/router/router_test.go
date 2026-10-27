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

package router

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	httpplugin "hcm/cmd/hc-service/logics/iac-plugin/http-plugin"
	pluginschema "hcm/cmd/hc-service/logics/plugin/schema"
	"hcm/pkg/api/core"
	coreplugin "hcm/pkg/api/core/plugin"
	dataplugin "hcm/pkg/api/data-service/plugin"
	proto "hcm/pkg/api/hc-service/iac-plugin"
	"hcm/pkg/cc"
	dataservice "hcm/pkg/client/data-service"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	tableplugin "hcm/pkg/dal/table/plugin"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	restcli "hcm/pkg/rest/client"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/thirdparty/api-gateway/plugin"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

type mockDataStore struct {
	mu        sync.RWMutex
	plugins   map[string][]coreplugin.IaCPlugin
	schemas   map[coreplugin.PluginInfo]*coreplugin.PluginSchema
	instances map[string]*coreplugin.IacPluginInstance
}

func newMockDataStore() *mockDataStore {
	return &mockDataStore{
		plugins:   make(map[string][]coreplugin.IaCPlugin),
		schemas:   make(map[coreplugin.PluginInfo]*coreplugin.PluginSchema),
		instances: make(map[string]*coreplugin.IacPluginInstance),
	}
}

type fakeDiscovery struct{}

func (fakeDiscovery) GetServers() ([]string, error) {
	return []string{"http://127.0.0.1"}, nil
}

type inMemHTTPClient struct {
	handler http.HandlerFunc
}

func (c *inMemHTTPClient) Do(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	c.handler(rec, req)
	return rec.Result(), nil
}

func newMockDataService(store *mockDataStore) *dataservice.Client {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/iac_plugins/list"):
			handlePluginList(w, r, store)
		case strings.HasSuffix(r.URL.Path, "/iac_plugin_schema/list"):
			handleSchemaList(w, r, store)
		case strings.HasSuffix(r.URL.Path, "/iac_plugin_instance/list"):
			handleInstanceList(w, r, store)
		}
	}
	cap := &restcli.Capability{
		Client:   &inMemHTTPClient{handler: handler},
		Discover: fakeDiscovery{},
	}
	return dataservice.NewClient(cap, "v1")
}

func handlePluginList(w http.ResponseWriter, r *http.Request, store *mockDataStore) {
	req := new(core.ListReq)
	_ = json.NewDecoder(r.Body).Decode(req)
	name := extractAtomField(req.Filter, "plugin_name")
	store.mu.RLock()
	list := store.plugins[name]
	store.mu.RUnlock()
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": errf.OK,
		"data": &dataplugin.IaCPluginListResult{Details: list},
	})
}

func handleSchemaList(w http.ResponseWriter, r *http.Request, store *mockDataStore) {
	req := new(dataplugin.IacPluginSchemaListReq)
	_ = json.NewDecoder(r.Body).Decode(req)
	key := coreplugin.PluginInfo{
		Name:    extractAtomField(req.Filter, "plugin_name"),
		Version: extractAtomField(req.Filter, "plugin_version"),
	}
	store.mu.RLock()
	s := store.schemas[key]
	store.mu.RUnlock()

	var details []tableplugin.IacPluginSchemaTable
	if s != nil {
		detailBytes, _ := json.Marshal(s.SchemaDetail)
		details = []tableplugin.IacPluginSchemaTable{{
			ID:            "1",
			PluginName:    key.Name,
			PluginVersion: key.Version,
			SchemaHash:    s.SchemaHash,
			SchemaDetail:  types.JsonField(detailBytes),
		}}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": errf.OK,
		"data": &dataplugin.IacPluginSchemaListResult{Details: details},
	})
}

func handleInstanceList(w http.ResponseWriter, r *http.Request, store *mockDataStore) {
	req := new(dataplugin.IacPluginInstanceListReq)
	_ = json.NewDecoder(r.Body).Decode(req)
	id := extractAtomField(req.Filter, "id")
	store.mu.RLock()
	inst := store.instances[id]
	store.mu.RUnlock()

	var details []coreplugin.IacPluginInstance
	if inst != nil {
		details = []coreplugin.IacPluginInstance{*inst}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": errf.OK,
		"data": &dataplugin.IacPluginInstanceListResult{Details: details},
	})
}

func extractAtomField(expr *filter.Expression, field string) string {
	if expr == nil {
		return ""
	}
	for _, r := range expr.Rules {
		rule, ok := r.(*filter.AtomRule)
		if !ok {
			continue
		}
		if rule.Field == field {
			val, _ := rule.Value.(string)
			return val
		}
	}
	return ""
}

var (
	testKeyOnce sync.Once
	testPubPEM  string
)

func getTestPubPEM(t testing.TB) string {
	t.Helper()
	testKeyOnce.Do(func() {
		priv, err := rsa.GenerateKey(rand.Reader, 3072)
		if err != nil {
			t.Fatalf("generate rsa key failed: %v", err)
		}
		der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		if err != nil {
			t.Fatalf("marshal public key failed: %v", err)
		}
		testPubPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	})
	return testPubPEM
}

func newTestPluginSchema(name, version string, cap coreplugin.Capabilities) *coreplugin.PluginSchema {
	detail := coreplugin.SchemaDetail{
		ProtocolVersion: "1.0",
		Capabilities:    cap,
		ProviderSchema: tfjson.ProviderSchema{
			ConfigSchema: &tfjson.Schema{Block: &tfjson.SchemaBlock{}},
			ResourceSchemas: map[string]*tfjson.Schema{
				name + "_host": {
					Block: &tfjson.SchemaBlock{
						Attributes: map[string]*tfjson.SchemaAttribute{
							"a":          {AttributeType: cty.Number, Optional: true},
							"bk_biz_id":  {AttributeType: cty.Number, Optional: true},
							"bk_host_id": {AttributeType: cty.Number, Computed: true},
						},
					},
				},
			},
			DataSourceSchemas: map[string]*tfjson.Schema{
				name + "_biz": {Block: &tfjson.SchemaBlock{}},
			},
		},
	}
	info := coreplugin.PluginInfo{Name: name, Version: version}
	hash, _ := coreplugin.ComputeSchemaHash(info, &detail)
	return &coreplugin.PluginSchema{
		Plugin:       info,
		SchemaHash:   hash,
		SchemaDetail: detail,
	}
}

type stubSouthboundClient struct {
	mu           sync.RWMutex
	calls        int32
	healthCalls  int32
	schemaCalls  int32
	healthResult *plugin.HealthData
	healthErr    error
}

func newStubSouthboundServer(t testing.TB, sbCli *stubSouthboundClient) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/health"):
			atomic.AddInt32(&sbCli.healthCalls, 1)
			sbCli.mu.RLock()
			hErr := sbCli.healthErr
			hRes := sbCli.healthResult
			sbCli.mu.RUnlock()
			if hErr != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"result": true, "code": 0, "data": hRes})
		case strings.HasSuffix(r.URL.Path, "/schema"):
			atomic.AddInt32(&sbCli.schemaCalls, 1)
			w.WriteHeader(http.StatusInternalServerError)
		case strings.HasSuffix(r.URL.Path, "/apply"):
			atomic.AddInt32(&sbCli.calls, 1)
			_, _ = w.Write([]byte(`{"result":true,"code":0,"data":{"status":"succeeded"}}`))
		case strings.HasSuffix(r.URL.Path, "/validate"), strings.HasSuffix(r.URL.Path, "/read_data"):
			atomic.AddInt32(&sbCli.calls, 1)
			_, _ = w.Write([]byte(`{"result":true,"code":0,"data":{}}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func setupRouterFixture(t testing.TB, cap coreplugin.Capabilities) (*mockDataStore, *stubSouthboundClient, *Service) {
	t.Helper()
	store := newMockDataStore()
	extBytes, _ := json.Marshal(coreplugin.HTTPExtension{
		APIPath:      "/api/bkcmdb",
		CryptoType:   "RSA",
		CryptoConfig: &coreplugin.HTTPCryptoConfig{PublicKey: getTestPubPEM(t)},
	})
	store.plugins["bkcmdb"] = []coreplugin.IaCPlugin{
		{
			ID: "p1", PluginName: "bkcmdb", PluginVersion: "1.0.0",
			Protocol: enumor.IaCPluginProtocolHTTP, State: enumor.IaCPluginStateDisabled, Extension: extBytes,
		},
		{
			ID: "p2", PluginName: "bkcmdb", PluginVersion: "1.2.0",
			Protocol: enumor.IaCPluginProtocolHTTP, State: enumor.IaCPluginStateEnabled, Extension: extBytes,
		},
	}
	store.schemas[coreplugin.PluginInfo{Name: "bkcmdb", Version: "1.2.0"}] = newTestPluginSchema("bkcmdb", "1.2.0", cap)
	store.instances["00000002"] = &coreplugin.IacPluginInstance{
		ID:          "00000002",
		PluginName:  "bkcmdb",
		Attrs:       map[string]any{"bk_paas_host": "https://paas.example.com"},
		SecretAttrs: map[string]any{"token": "tok_123"},
	}
	store.instances["tc-inst"] = &coreplugin.IacPluginInstance{
		ID:          "tc-inst",
		PluginName:  "tencentcloud",
		Attrs:       map[string]any{"region": "ap-guangzhou"},
		SecretAttrs: map[string]any{"secret_key": "tc_secret"},
	}

	dataCli := newMockDataService(store)
	schemaMgr := pluginschema.NewSchemaManager(dataCli, pluginschema.DefaultSchemaCacheTTL)
	sbCli := &stubSouthboundClient{}
	sbSrv := newStubSouthboundServer(t, sbCli)
	gw := &cc.ApiGateway{Endpoints: []string{sbSrv.URL}, AppCode: "hcm", AppSecret: "sec"}
	httpPlugin := httpplugin.New(dataCli, nil, gw, schemaMgr, httpplugin.Options{})
	r := New(dataCli, schemaMgr, httpPlugin)
	return store, sbCli, r
}

func TestRouter_ResolveAndDispatch_PositiveAndOwnershipGate(t *testing.T) {
	fullCap := coreplugin.Capabilities{
		Apply: true, Plan: true, Refresh: true, ReadData: true,
		AsyncApply: true, Stop: true, Validate: true, Import: true, List: true,
	}
	store, sbCli, r := setupRouterFixture(t, fullCap)
	kt := kit.New()

	// 1. Positive resolve & dispatch: picks latest enabled 1.2.0 and merges properties
	routeCtx, err := r.Resolve(kt, "bkcmdb", "00000002", "bkcmdb_host", enumor.IaCPluginOperationValidate)
	assert.NoError(t, err)
	assert.Equal(t, "1.2.0", routeCtx.Plugin.PluginVersion)
	assert.Equal(t, map[string]any{"bk_paas_host": "https://paas.example.com", "token": "tok_123"}, routeCtx.Properties)

	_, err = r.Validate(kt, &proto.ValidateReq{
		CommonInstanceReq: proto.CommonInstanceReq{
			Plugin: "bkcmdb", PluginInstanceID: "00000002", TypeName: "bkcmdb_host",
		},
		Config: map[string]any{},
	})
	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&sbCli.calls))

	// 2. AC-S02: Cross-plugin instance misuse rejected with InvalidParameter, 0 downstream calls
	_, err = r.Execute(kt, &proto.ExecuteReq{
		CommonInstanceReq: proto.CommonInstanceReq{
			Plugin: "bkcmdb", PluginInstanceID: "tc-inst", TypeName: "bkcmdb_host",
		},
		Config: map[string]any{"bk_biz_id": 213},
	})
	assert.Error(t, err)
	assert.Equal(t, errf.InvalidParameter, errf.Error(err).Code)
	assert.Equal(t, int32(1), atomic.LoadInt32(&sbCli.calls))

	// 3. Instance not found -> RecordNotFound
	_, err = r.Resolve(kt, "bkcmdb", "not-exist", "bkcmdb_host", enumor.IaCPluginOperationExecute)
	assert.Equal(t, errf.RecordNotFound, errf.Error(err).Code)

	// 4. Plugin not found -> RecordNotFound
	_, err = r.Resolve(kt, "unknown_plugin", "00000002", "bkcmdb_host", enumor.IaCPluginOperationExecute)
	assert.Equal(t, errf.RecordNotFound, errf.Error(err).Code)

	// 5. Latest plugin not enabled -> Aborted
	store.mu.Lock()
	store.plugins["bkcmdb"][1].State = enumor.IaCPluginStateDisabled
	store.mu.Unlock()
	_, err = r.Resolve(kt, "bkcmdb", "00000002", "bkcmdb_host", enumor.IaCPluginOperationExecute)
	assert.Equal(t, errf.Aborted, errf.Error(err).Code)
}

func TestRouter_SchemaTypeAndCapabilitiesGating(t *testing.T) {
	partialCap := coreplugin.Capabilities{
		Apply: true, Plan: true, Refresh: true, ReadData: true, Stop: true, AsyncApply: false,
	}
	_, sbCli, r := setupRouterFixture(t, partialCap)
	kt := kit.New()

	// Positive resource and data_source routing
	_, err := r.Execute(kt, &proto.ExecuteReq{
		CommonInstanceReq: proto.CommonInstanceReq{
			Plugin: "bkcmdb", PluginInstanceID: "00000002", TypeName: "bkcmdb_host",
		},
		Config: map[string]any{"a": 1},
	})
	assert.NoError(t, err)

	_, err = r.ReadData(kt, &proto.ReadDataReq{
		CommonInstanceReq: proto.CommonInstanceReq{
			Plugin: "bkcmdb", PluginInstanceID: "00000002", TypeName: "bkcmdb_biz",
		},
		Config: map[string]any{},
	})
	assert.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&sbCli.calls))

	// Negative type_name not in schema
	_, err = r.Resolve(kt, "bkcmdb", "00000002", "bkcmdb_unknown", enumor.IaCPluginOperationExecute)
	assert.Equal(t, errf.InvalidParameter, errf.Error(err).Code)

	// Negative ReadData on resource type
	_, err = r.Resolve(kt, "bkcmdb", "00000002", "bkcmdb_host", enumor.IaCPluginOperationReadData)
	assert.Equal(t, errf.InvalidParameter, errf.Error(err).Code)

	// Negative Import when capability.import == false
	_, err = r.Resolve(kt, "bkcmdb", "00000002", "bkcmdb_host", enumor.IaCPluginOperationImport)
	assert.Equal(t, errf.Aborted, errf.Error(err).Code)

	// Negative Stop when stop == true but async_apply == false
	_, err = r.Resolve(kt, "bkcmdb", "00000002", "bkcmdb_host", enumor.IaCPluginOperationStop)
	assert.Equal(t, errf.Aborted, errf.Error(err).Code)
	assert.Equal(t, int32(2), atomic.LoadInt32(&sbCli.calls))
}

func TestRouter_GetSchemaZeroSouthboundAndHealth(t *testing.T) {
	fullCap := coreplugin.Capabilities{Apply: true, Plan: true, Refresh: true, ReadData: true}
	store, sbCli, r := setupRouterFixture(t, fullCap)
	store.schemas[coreplugin.PluginInfo{Name: "bkcmdb", Version: "1.0.0"}] = newTestPluginSchema("bkcmdb", "1.0.0", fullCap)
	sbCli.healthResult = &plugin.HealthData{PluginVersion: "1.2.0"}
	kt := kit.New()

	// AC-P02: Call GetSchema 100 times (both default enabled version and explicit historical version)
	for i := 0; i < 100; i++ {
		s, err := r.GetSchema(kt, &proto.GetSchemaReq{Plugin: "bkcmdb"})
		assert.NoError(t, err)
		assert.Equal(t, "1.2.0", s.Plugin.Version)
	}
	histSchema, err := r.GetSchema(kt, &proto.GetSchemaReq{Plugin: "bkcmdb", PluginVersion: "1.0.0"})
	assert.NoError(t, err)
	assert.Equal(t, "1.0.0", histSchema.Plugin.Version)
	assert.Equal(t, int32(0), atomic.LoadInt32(&sbCli.schemaCalls))

	// GetSchema non-existent version -> RecordNotFound
	_, err = r.GetSchema(kt, &proto.GetSchemaReq{Plugin: "bkcmdb", PluginVersion: "9.9.9"})
	assert.Equal(t, errf.RecordNotFound, errf.Error(err).Code)

	// Health positive -> 1.2.0
	hRes, err := r.Health(kt, &proto.HealthReq{Plugin: "bkcmdb"})
	assert.NoError(t, err)
	assert.Equal(t, "1.2.0", hRes.PluginVersion)

	// Health negative when southbound fails -> Aborted (2000006)
	sbCli.healthErr = errors.New("gateway 503")
	_, err = r.Health(kt, &proto.HealthReq{Plugin: "bkcmdb"})
	assert.Equal(t, errf.Aborted, errf.Error(err).Code)
}
