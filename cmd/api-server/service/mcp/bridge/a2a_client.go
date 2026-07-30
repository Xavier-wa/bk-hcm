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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"

	"hcm/pkg/cc"
	"hcm/pkg/client/discovery"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/serviced"

	a2aclient "trpc.group/trpc-go/trpc-a2a-go/client"
	"trpc.group/trpc-go/trpc-a2a-go/protocol"
)

// a2aStreamClient 是 bridge 对 trpc-a2a-go client 的最小依赖面，便于单元测试注入 mock。
type a2aStreamClient interface {
	StreamMessage(ctx context.Context, params protocol.SendMessageParams,
		opts ...a2aclient.RequestOption) (<-chan protocol.StreamingMessageEvent, error)
	CancelTasks(ctx context.Context, params protocol.TaskIDParams,
		opts ...a2aclient.RequestOption) (*protocol.Task, error)
}

// a2aClientProvider 按请求创建面向当前 agent-server 实例的 A2A client。
type a2aClientProvider interface {
	NewClient(ctx context.Context) (a2aStreamClient, string, error)
}

type serverDiscovery interface {
	GetServers() ([]string, error)
}

// A2AClientProvider 通过服务发现获取 agent-server 实例，并复用共享 HTTP transport。
type A2AClientProvider struct {
	discovery  serverDiscovery
	httpClient *http.Client
}

// NewA2AClientProvider 构造 A2A client provider。
func NewA2AClientProvider(cfg cc.MCPBridgeSetting, dis serviced.Discover) (*A2AClientProvider, error) {
	if dis == nil {
		return nil, fmt.Errorf("bridge: serviced discover is nil")
	}
	return newA2AClientProviderWithDiscovery(cfg, discovery.NewAPIDiscovery(cc.AgentServerName, dis)), nil
}

func newA2AClientProviderWithDiscovery(cfg cc.MCPBridgeSetting, dis serverDiscovery) *A2AClientProvider {
	return &A2AClientProvider{
		discovery:  dis,
		httpClient: newSharedHTTPClient(cfg),
	}
}

func newSharedHTTPClient(cfg cc.MCPBridgeSetting) *http.Client {
	return &http.Client{
		Timeout: cfg.ReadTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   cfg.ConnectTimeout,
				KeepAlive: constant.MCPBridgeDefaultKeepAlive,
			}).DialContext,
			MaxIdleConns:          cfg.MaxIdleConnsPerHost,
			MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
			IdleConnTimeout:       constant.MCPBridgeDefaultIdleConnTimeout,
			TLSHandshakeTimeout:   constant.MCPBridgeDefaultTLSHandshakeTimeout,
			ExpectContinueTimeout: constant.MCPBridgeDefaultExpectContinueTimeout,
		},
	}
}

// NewClient 返回一个绑定到当前轮询选中 agent-server 实例的 A2A client。
func (p *A2AClientProvider) NewClient(_ context.Context) (a2aStreamClient, string, error) {
	if p == nil || p.discovery == nil {
		return nil, "", fmt.Errorf("bridge: a2a client provider is not initialized")
	}
	servers, err := p.discovery.GetServers()
	if err != nil {
		return nil, "", fmt.Errorf("bridge: get agent-server instances failed: %w", err)
	}
	if len(servers) == 0 {
		return nil, "", fmt.Errorf("bridge: no agent-server instance available")
	}

	endpoint, err := buildA2AEndpointURL(servers[0])
	if err != nil {
		return nil, "", err
	}
	cli, err := a2aclient.NewA2AClient(endpoint,
		a2aclient.WithHTTPClient(p.httpClient),
		a2aclient.WithHTTPReqHandler(endpointSlashTrimmingHandler{}),
	)
	if err != nil {
		return nil, "", fmt.Errorf("bridge: new a2a client failed: %w", err)
	}
	return cli, endpoint, nil
}

func buildA2AEndpointURL(server string) (string, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return "", fmt.Errorf("bridge: empty agent-server address")
	}
	if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
		server = "http://" + server
	}

	u, err := url.Parse(server)
	if err != nil {
		return "", fmt.Errorf("bridge: parse agent-server address %q failed: %w", server, err)
	}
	u.Path = path.Join(u.Path, constant.A2ABasePathDefault, constant.A2AJSONRPCSubPath)
	return u.String(), nil
}

type endpointSlashTrimmingHandler struct{}

func (endpointSlashTrimmingHandler) Handle(_ context.Context, cli *http.Client, req *http.Request) (*http.Response, error) {
	if cli == nil {
		return nil, fmt.Errorf("bridge: http client is nil")
	}
	if req != nil && strings.HasSuffix(req.URL.Path, constant.A2AJSONRPCSubPath+"/") {
		req.URL.Path = strings.TrimSuffix(req.URL.Path, "/")
	}
	return cli.Do(req)
}
