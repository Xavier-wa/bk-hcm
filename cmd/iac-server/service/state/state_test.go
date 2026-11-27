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
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	logicstate "hcm/cmd/iac-server/logics/state"
	"hcm/pkg/api/core"
	coreiac "hcm/pkg/api/core/iac"
	coreplugin "hcm/pkg/api/core/plugin"
	dataiac "hcm/pkg/api/data-service/iac"
	dataplugin "hcm/pkg/api/data-service/plugin"
	iacproto "hcm/pkg/api/iac-server"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/rest"
	"hcm/pkg/serviced"

	"github.com/emicklei/go-restful/v3"
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

type mockBackend struct {
	repos     map[string]bool
	rows      []coreiac.IacState
	schema    *coreplugin.PluginSchema
	schemaErr error
}

func newMockClientSet(b *mockBackend) *client.ClientSet {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/iac_repository/list"):
			req := new(dataiac.IacRepositoryListReq)
			_ = json.NewDecoder(r.Body).Decode(req)
			raw, _ := json.Marshal(req.Filter)
			var count uint64
			for repoID, ok := range b.repos {
				if ok && strings.Contains(string(raw), `"`+repoID+`"`) {
					count = 1
					break
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    errf.OK,
				"message": "",
				"data":    &dataiac.IacRepositoryListResult{Count: count},
			})

		case strings.HasSuffix(r.URL.Path, "/iac_state/list"):
			req := new(dataiac.IacStateListReq)
			_ = json.NewDecoder(r.Body).Decode(req)
			raw, _ := json.Marshal(req.Filter)
			filterStr := string(raw)
			matched := make([]coreiac.IacState, 0)
			for _, row := range b.rows {
				if !strings.Contains(filterStr, `"`+row.RepositoryID+`"`) || row.ImportFromSpecID != "" {
					continue
				}
				if strings.Contains(filterStr, `"field":"id"`) && !strings.Contains(filterStr, `"`+row.ID+`"`) {
					continue
				}
				matched = append(matched, row)
			}
			res := &dataiac.IacStateListResult{Details: matched}
			if req.Page != nil && req.Page.Count {
				res = &dataiac.IacStateListResult{Count: uint64(len(matched))}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": errf.OK, "message": "", "data": res})

		case strings.HasSuffix(r.URL.Path, "/iac_plugins/list"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    errf.OK,
				"message": "",
				"data": &dataplugin.IaCPluginListResult{
					Details: []coreplugin.IaCPlugin{
						{PluginName: "tcloud", PluginVersion: "1.0.0", State: enumor.IaCPluginStateEnabled},
					},
				},
			})

		case strings.HasSuffix(r.URL.Path, "/iac_plugins/schema"):
			if b.schemaErr != nil {
				_ = json.NewEncoder(w).Encode(map[string]any{"code": errf.Aborted, "message": b.schemaErr.Error()})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": errf.OK, "message": "", "data": b.schema})

		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"code": errf.Aborted, "message": "unexpected path"})
		}
	}
	return client.NewClientSet(&inMemoryHTTPClient{handler: handler}, fakeDiscover{})
}

func setupTestContainer(svc *service) *restful.Container {
	cc.InitService(cc.IacServerName)
	cc.InitRuntime(&cc.IacServerSetting{})

	ws := new(restful.WebService)
	ws.Path("/api/v1/iac").Produces(restful.MIME_JSON)
	h := rest.NewHandler()
	h.Add("ListState", http.MethodPost, "/repository/{repository_id}/state/list", svc.ListState)
	h.Add("GetState", http.MethodGet, "/repository/{repository_id}/state/{state_id}", svc.GetState)
	h.Load(ws)
	return restful.NewContainer().Add(ws)
}

func addTestHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(constant.UserKey, "tester")
	req.Header.Set(constant.RidKey, "test-rid-1234567890")
	req.Header.Set(constant.AppCodeKey, "test-app")
}

func TestListStateAndGetStateHandlers(t *testing.T) {
	curJSON := tabletypes.JsonField(`{"plugin":"tcloud","plugin_instance_id":"inst-2","type":"tencentcloud_instance",` +
		`"state":{"id":"ins-1","password":"secret-pwd"},"depends_on":["tencentcloud_vpc.main"],` +
		`"private":"priv-blob","identity":{"id":"ins-1"},"provider_config":{"az":"gz-1"}}`)
	depJSON := tabletypes.JsonField(`{"a1b2c3d4":{"plugin":"tcloud","plugin_instance_id":"inst-1",` +
		`"type":"tencentcloud_instance","state":{"id":"ins-old","password":"old-pwd"},"private":"old-priv"}}`)

	backend := &mockBackend{
		repos: map[string]bool{"repo-1": true},
		rows: []coreiac.IacState{
			{
				ID:           "st-1",
				RepositoryID: "repo-1",
				BlockID:      "tencentcloud_instance.web",
				Kind:         enumor.IacStateKindResource,
				Current:      &curJSON,
				Deposed:      depJSON,
				LastFlowID:   "flow-1",
				Revision:     &core.Revision{CreatedAt: "2026-11-26 10:00:00", UpdatedAt: "2026-11-26 10:05:00"},
			},
			{
				ID:           "st-deposed-only",
				RepositoryID: "repo-1",
				BlockID:      "tencentcloud_instance.orphan",
				Kind:         enumor.IacStateKindResource,
				Current:      nil,
				Deposed:      depJSON,
				LastFlowID:   "flow-2",
			},
			{
				ID:               "st-staging",
				RepositoryID:     "repo-1",
				BlockID:          "tencentcloud_instance.staging",
				Kind:             enumor.IacStateKindResource,
				Current:          &curJSON,
				Deposed:          tabletypes.JsonField(`{}`),
				ImportFromSpecID: "spec-99",
			},
		},
		schema: &coreplugin.PluginSchema{
			Plugin: coreplugin.PluginInfo{Name: "tcloud", Version: "1.0.0"},
			SchemaDetail: coreplugin.SchemaDetail{
				ProviderSchema: tfjson.ProviderSchema{
					ResourceSchemas: map[string]*tfjson.Schema{
						"tencentcloud_instance": {
							Block: &tfjson.SchemaBlock{
								Attributes: map[string]*tfjson.SchemaAttribute{
									"id":       {Sensitive: false},
									"password": {Sensitive: true},
								},
							},
						},
					},
				},
			},
		},
	}

	cliSet := newMockClientSet(backend)
	resolver, err := logicstate.NewSchemaResolver(cliSet)
	if err != nil {
		t.Fatalf("NewSchemaResolver failed: %v", err)
	}
	svc := &service{
		client:   cliSet,
		resolver: resolver,
	}

	container := setupTestContainer(svc)

	// 1. 仓库不存在返回 RecordNotFound
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/iac/repository/repo-missing/state/list",
		bytes.NewBufferString(`{"page":{"start":0,"limit":20}}`))
	addTestHeaders(req404)
	rec404 := httptest.NewRecorder()
	container.ServeHTTP(rec404, req404)
	var resp404 rest.BaseResp
	_ = json.Unmarshal(rec404.Body.Bytes(), &resp404)
	if resp404.Code != errf.RecordNotFound {
		t.Fatalf("expected RecordNotFound for missing repo, got code=%d body=%s", resp404.Code, rec404.Body.String())
	}

	// 2. ListState 返回摘要（排除暂存态、current==nil 降级从 deposed 提取 plugin/type、不含敏感载荷）
	reqList := httptest.NewRequest(http.MethodPost, "/api/v1/iac/repository/repo-1/state/list",
		bytes.NewBufferString(`{"page":{"start":0,"limit":20}}`))
	addTestHeaders(reqList)
	recList := httptest.NewRecorder()
	container.ServeHTTP(recList, reqList)
	if strings.Contains(recList.Body.String(), "secret-pwd") || strings.Contains(recList.Body.String(), "priv-blob") {
		t.Fatalf("ListState leaked sensitive or private data: %s", recList.Body.String())
	}
	var listResp struct {
		Code int32                  `json:"code"`
		Data iacproto.ListStateResp `json:"data"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal ListState resp failed: %v", err)
	}
	if listResp.Code != errf.OK || len(listResp.Data.Details) != 2 {
		t.Fatalf("expected 2 active items, got %+v", listResp)
	}
	if listResp.Data.Details[0].PluginInstanceID != "inst-2" {
		t.Fatalf("expected active item plugin_instance_id=inst-2, got %+v", listResp.Data.Details[0])
	}
	if listResp.Data.Details[1].HasCurrent || listResp.Data.Details[1].Plugin != "tcloud" ||
		listResp.Data.Details[1].PluginInstanceID != "inst-1" || listResp.Data.Details[1].DeposedCount != 1 {
		t.Fatalf("expected deposed-only item fallback plugin=tcloud, plugin_instance_id=inst-1, got %+v",
			listResp.Data.Details[1])
	}

	// 3. GetState 脱敏详情：password 替换为占位符、private 剔除、provider_config 原样保留
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/iac/repository/repo-1/state/st-1", nil)
	addTestHeaders(reqGet)
	recGet := httptest.NewRecorder()
	container.ServeHTTP(recGet, reqGet)
	bodyStr := recGet.Body.String()
	if strings.Contains(bodyStr, "secret-pwd") || strings.Contains(bodyStr, "old-pwd") ||
		strings.Contains(bodyStr, "priv-blob") || strings.Contains(bodyStr, "old-priv") {
		t.Fatalf("GetState leaked secret or private data: %s", bodyStr)
	}
	var getResp struct {
		Code int32                 `json:"code"`
		Data iacproto.GetStateResp `json:"data"`
	}
	if err := json.Unmarshal(recGet.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("unmarshal GetState resp failed: %v", err)
	}
	if getResp.Code != errf.OK || getResp.Data.Current == nil {
		t.Fatalf("unexpected GetState resp: %+v", getResp)
	}
	if getResp.Data.Current.State["password"] != constant.IacStateSensitiveValueMask {
		t.Fatalf("expected current password masked, got %v", getResp.Data.Current.State["password"])
	}
	if getResp.Data.Deposed["a1b2c3d4"].State["password"] != constant.IacStateSensitiveValueMask {
		t.Fatalf("expected deposed password masked, got %v", getResp.Data.Deposed["a1b2c3d4"].State["password"])
	}
	if getResp.Data.Current.PluginInstanceID != "inst-2" ||
		getResp.Data.Deposed["a1b2c3d4"].PluginInstanceID != "inst-1" {
		t.Fatalf("expected current inst-2 and deposed inst-1, got current=%+v deposed=%+v",
			getResp.Data.Current, getResp.Data.Deposed["a1b2c3d4"])
	}
	if getResp.Data.Current.ProviderConfig["az"] != "gz-1" {
		t.Fatalf("expected provider_config preserved, got %+v", getResp.Data.Current.ProviderConfig)
	}

	// 4. 暂存态记录禁止通过 GetState 访问
	reqStaging := httptest.NewRequest(http.MethodGet, "/api/v1/iac/repository/repo-1/state/st-staging", nil)
	addTestHeaders(reqStaging)
	recStaging := httptest.NewRecorder()
	container.ServeHTTP(recStaging, reqStaging)
	var stagingResp rest.BaseResp
	_ = json.Unmarshal(recStaging.Body.Bytes(), &stagingResp)
	if stagingResp.Code != errf.RecordNotFound {
		t.Fatalf("expected RecordNotFound for staging state, got %+v", stagingResp)
	}

	// 5. Schema 不可用时 Fail-Closed，严禁返回明文
	backend.schemaErr = errors.New("schema missing")
	reqFailClosed := httptest.NewRequest(http.MethodGet, "/api/v1/iac/repository/repo-1/state/st-1", nil)
	addTestHeaders(reqFailClosed)
	recFailClosed := httptest.NewRecorder()
	container.ServeHTTP(recFailClosed, reqFailClosed)
	if strings.Contains(recFailClosed.Body.String(), "secret-pwd") {
		t.Fatalf("Fail-Closed violated, plaintext leaked: %s", recFailClosed.Body.String())
	}
	var failResp rest.BaseResp
	_ = json.Unmarshal(recFailClosed.Body.Bytes(), &failResp)
	if failResp.Code == errf.OK {
		t.Fatalf("expected error on schema failure, got OK")
	}
}
