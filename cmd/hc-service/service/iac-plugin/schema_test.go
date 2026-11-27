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

package iacplugin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hcm/cmd/hc-service/logics/plugin/schema"
	"hcm/pkg/api/core"
	coreplugin "hcm/pkg/api/core/plugin"
	dataplugin "hcm/pkg/api/data-service/plugin"
	"hcm/pkg/cc"
	dataservice "hcm/pkg/client/data-service"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	tableplugin "hcm/pkg/dal/table/plugin"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/rest"
	restcli "hcm/pkg/rest/client"

	"github.com/emicklei/go-restful/v3"
)

type fakeDiscovery struct{}

func (fakeDiscovery) GetServers() ([]string, error) {
	return []string{"http://127.0.0.1"}, nil
}

type inMemoryHTTPClient struct {
	handler http.HandlerFunc
}

func (c *inMemoryHTTPClient) Do(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	c.handler(rec, req)
	return rec.Result(), nil
}

func newMockDataServiceClient() *dataservice.Client {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/iac_plugins/list"):
			req := new(core.ListReq)
			_ = json.NewDecoder(r.Body).Decode(req)
			raw, _ := json.Marshal(req.Filter)
			var details []coreplugin.IaCPlugin
			if strings.Contains(string(raw), `"1.0.0"`) {
				details = []coreplugin.IaCPlugin{
					{PluginName: "tcloud", PluginVersion: "1.0.0", State: enumor.IaCPluginStateEnabled},
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    errf.OK,
				"message": "",
				"data":    &dataplugin.IaCPluginListResult{Details: details},
			})

		case strings.HasSuffix(r.URL.Path, "/iac_plugin_schema/list"):
			req := new(dataplugin.IacPluginSchemaListReq)
			_ = json.NewDecoder(r.Body).Decode(req)
			raw, _ := json.Marshal(req.Filter)
			var details []tableplugin.IacPluginSchemaTable
			if strings.Contains(string(raw), `"1.0.0"`) {
				details = []tableplugin.IacPluginSchemaTable{
					{
						ID:            "1",
						PluginName:    "tcloud",
						PluginVersion: "1.0.0",
						SchemaHash:    "sha256:abc",
						SchemaDetail:  types.JsonField(`{}`),
					},
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    errf.OK,
				"message": "",
				"data":    &dataplugin.IacPluginSchemaListResult{Details: details},
			})
		}
	}
	return dataservice.NewClient(&restcli.Capability{
		Client:   &inMemoryHTTPClient{handler: handler},
		Discover: fakeDiscovery{},
	}, "v1")
}

func TestGetPluginSchemaHandler(t *testing.T) {
	cc.InitService(cc.HCServiceName)
	cc.InitRuntime(&cc.HCServiceSetting{})

	mgr := schema.NewSchemaManager(newMockDataServiceClient(), time.Minute)
	defer mgr.Close()

	ws := new(restful.WebService)
	ws.Path("/api/v1/hc").Produces(restful.MIME_JSON)
	svc := &service{schemaMgr: mgr}
	h := rest.NewHandler()
	h.Add("NorthboundGetSchemaIaCPlugin", http.MethodGet, "/iac_plugins/schema", svc.GetSchema)
	h.Load(ws)
	container := restful.NewContainer().Add(ws)

	// 正常请求
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hc/iac_plugins/schema?plugin=tcloud&plugin_version=1.0.0", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(constant.UserKey, "tester")
	req.Header.Set(constant.RidKey, "test-rid-1234567890")
	req.Header.Set(constant.AppCodeKey, "test-app")
	rec := httptest.NewRecorder()
	container.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Code int32                    `json:"code"`
		Data *coreplugin.PluginSchema `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.Code != errf.OK || resp.Data == nil || resp.Data.Plugin.Version != "1.0.0" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	// 缺少参数返回 InvalidParameter
	badReq := httptest.NewRequest(http.MethodGet, "/api/v1/hc/iac_plugins/schema?plugin=tcloud", nil)
	badReq.Header.Set("Content-Type", "application/json")
	badReq.Header.Set(constant.UserKey, "tester")
	badReq.Header.Set(constant.RidKey, "test-rid-1234567891")
	badReq.Header.Set(constant.AppCodeKey, "test-app")
	badRec := httptest.NewRecorder()
	container.ServeHTTP(badRec, badReq)
	var badResp rest.BaseResp
	_ = json.Unmarshal(badRec.Body.Bytes(), &badResp)
	if badResp.Code != errf.InvalidParameter {
		t.Fatalf("expected InvalidParameter, got code=%d msg=%s", badResp.Code, badResp.Message)
	}

	// 不存在的版本返回错误
	errReq := httptest.NewRequest(http.MethodGet,
		"/api/v1/hc/iac_plugins/schema?plugin=tcloud&plugin_version=9.9.9", nil)
	errReq.Header.Set("Content-Type", "application/json")
	errReq.Header.Set(constant.UserKey, "tester")
	errReq.Header.Set(constant.RidKey, "test-rid-1234567892")
	errReq.Header.Set(constant.AppCodeKey, "test-app")
	errRec := httptest.NewRecorder()
	container.ServeHTTP(errRec, errReq)
	var errResp rest.BaseResp
	_ = json.Unmarshal(errRec.Body.Bytes(), &errResp)
	if errResp.Code == errf.OK {
		t.Fatalf("expected error response when schema missing, got OK")
	}
}
