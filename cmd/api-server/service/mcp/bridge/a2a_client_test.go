/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package bridge

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
)

type fakeServerDiscovery struct {
	servers []string
	err     error
}

func (f *fakeServerDiscovery) GetServers() ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.servers, nil
}

func TestNewA2AClientProvider_NilDiscover(t *testing.T) {
	if _, err := NewA2AClientProvider(cc.MCPBridgeSetting{}, nil); err == nil {
		t.Fatal("expected error when serviced discover is nil")
	}
}

func TestA2AClientProvider_NewClient(t *testing.T) {
	t.Run("nil provider", func(t *testing.T) {
		var provider *A2AClientProvider
		if _, _, err := provider.NewClient(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("discovery error", func(t *testing.T) {
		provider := newA2AClientProviderWithDiscovery(cc.MCPBridgeSetting{},
			&fakeServerDiscovery{err: errors.New("etcd down")})
		if _, _, err := provider.NewClient(context.Background()); err == nil {
			t.Fatal("expected discovery error")
		}
	})

	t.Run("empty servers", func(t *testing.T) {
		provider := newA2AClientProviderWithDiscovery(cc.MCPBridgeSetting{}, &fakeServerDiscovery{})
		if _, _, err := provider.NewClient(context.Background()); err == nil {
			t.Fatal("expected empty server error")
		}
	})

	t.Run("invalid server url", func(t *testing.T) {
		provider := newA2AClientProviderWithDiscovery(cc.MCPBridgeSetting{},
			&fakeServerDiscovery{servers: []string{"http://[::1"}})
		if _, _, err := provider.NewClient(context.Background()); err == nil {
			t.Fatal("expected parse error")
		}
	})

	t.Run("success", func(t *testing.T) {
		agent := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		defer agent.Close()

		provider := newA2AClientProviderWithDiscovery(cc.MCPBridgeSetting{},
			&fakeServerDiscovery{servers: []string{agent.URL}})
		cli, endpoint, err := provider.NewClient(context.Background())
		if err != nil {
			t.Fatalf("NewClient error: %v", err)
		}
		if cli == nil {
			t.Fatal("client is nil")
		}
		want := agent.URL + constant.A2ABasePathDefault + constant.A2AJSONRPCSubPath
		if endpoint != want {
			t.Fatalf("endpoint = %q, want %q", endpoint, want)
		}
	})
}

func TestEndpointSlashTrimmingHandler(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+constant.A2ABasePathDefault+
		constant.A2AJSONRPCSubPath+"/", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	handler := endpointSlashTrimmingHandler{}
	resp, err := handler.Handle(context.Background(), http.DefaultClient, req)
	if err != nil {
		t.Fatalf("Handle error: %v", err)
	}
	defer resp.Body.Close()

	if gotPath != constant.A2ABasePathDefault+constant.A2AJSONRPCSubPath {
		t.Fatalf("path = %q, want trimmed A2A path", gotPath)
	}
	if _, err := handler.Handle(context.Background(), nil, req); err == nil {
		t.Fatal("expected nil http client error")
	}
}
