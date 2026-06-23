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

package cc

import (
	"strings"
	"testing"
	"time"

	"hcm/pkg/criteria/constant"
)

func TestMCPIngressSetting_TrySetDefault(t *testing.T) {
	cfg := MCPIngressSetting{}
	cfg.trySetDefault()

	if cfg.BasePath != constant.MCPIngressBasePathDefault {
		t.Errorf("BasePath = %q, want %q", cfg.BasePath, constant.MCPIngressBasePathDefault)
	}
	if cfg.AggregatedToolName != constant.MCPIngressAggregatedToolName {
		t.Errorf("AggregatedToolName = %q, want %q", cfg.AggregatedToolName, constant.MCPIngressAggregatedToolName)
	}
	if cfg.TextMaxLength != 8000 {
		t.Errorf("TextMaxLength = %d, want 8000", cfg.TextMaxLength)
	}
	if cfg.ProgressTaskMapMaxSize != 10000 {
		t.Errorf("ProgressTaskMapMaxSize = %d, want 10000", cfg.ProgressTaskMapMaxSize)
	}
	if cfg.ProgressTaskMapEntryTTL != 30*time.Minute {
		t.Errorf("ProgressTaskMapEntryTTL = %v, want 30m", cfg.ProgressTaskMapEntryTTL)
	}
}

func TestMCPIngressSetting_Validate_DisabledSkipsCheck(t *testing.T) {
	cfg := MCPIngressSetting{Enable: false, BasePath: ""}
	if err := cfg.Validate(); err != nil {
		t.Errorf("disabled ingress should pass validate, got %v", err)
	}
}

func TestMCPIngressSetting_Validate_RequiresLeadingSlash(t *testing.T) {
	cfg := MCPIngressSetting{Enable: true, BasePath: "api/v1/mcp/servers", AggregatedToolName: "send_message"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error on basePath without leading slash")
	}
}

func TestMCPBridgeSetting_TrySetDefault(t *testing.T) {
	cfg := MCPBridgeSetting{}
	cfg.trySetDefault()
	if cfg.ConnectTimeout != 5*time.Second {
		t.Errorf("ConnectTimeout = %v, want 5s", cfg.ConnectTimeout)
	}
	if cfg.ReadTimeout != 300*time.Second {
		t.Errorf("ReadTimeout = %v, want 300s", cfg.ReadTimeout)
	}
	if cfg.MaxIdleConnsPerHost != 100 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 100", cfg.MaxIdleConnsPerHost)
	}
}

func TestMCPBridgeSetting_Validate_NoStrictRequirement(t *testing.T) {
	cfg := MCPBridgeSetting{}
	cfg.trySetDefault()
	// agent-server 地址走 etcd 服务发现，配置层无强校验。
	if err := cfg.Validate(); err != nil {
		t.Errorf("default bridge setting should pass validate, got %v", err)
	}
}

func TestMCPInternalSetting_TrySetDefault(t *testing.T) {
	cfg := MCPInternalSetting{}
	cfg.trySetDefault()

	if cfg.BasePath != constant.MCPInternalBasePathDefault {
		t.Errorf("BasePath = %q, want %q", cfg.BasePath, constant.MCPInternalBasePathDefault)
	}
	if cfg.SchemaSync.Interval != 5*time.Minute {
		t.Errorf("SchemaSync.Interval = %v, want 5m", cfg.SchemaSync.Interval)
	}
	if cfg.SchemaSync.Timeout != 30*time.Second {
		t.Errorf("SchemaSync.Timeout = %v, want 30s", cfg.SchemaSync.Timeout)
	}
	if !strings.HasSuffix(cfg.SchemaSync.CachePath, ".json") {
		t.Errorf("SchemaSync.CachePath = %q, want *.json suffix", cfg.SchemaSync.CachePath)
	}
}

func TestMCPInternalSetting_Validate_RequiresGatewayURL(t *testing.T) {
	cfg := MCPInternalSetting{Enable: true}
	cfg.trySetDefault()
	if err := cfg.Validate(); err == nil {
		t.Error("expected error on empty schemaSync.gatewayURL when internal.enable=true")
	}
}

func TestMCPServerSetting_Validate_IngressOnlyOK(t *testing.T) {
	// 启用 ingress 但 bridge 走 etcd 服务发现，不需要额外 URL 配置。
	cfg := MCPServerSetting{
		Ingress: MCPIngressSetting{Enable: true},
	}
	cfg.trySetDefault()
	if err := cfg.Validate(); err != nil {
		t.Errorf("ingress-only enabled setting should pass validate, got %v", err)
	}
}

func TestMCPServerSetting_Validate_AllDisabledOK(t *testing.T) {
	cfg := MCPServerSetting{}
	cfg.trySetDefault()
	if err := cfg.Validate(); err != nil {
		t.Errorf("all-disabled MCP setting should pass validate, got %v", err)
	}
}

func TestA2APassthroughSetting_TrySetDefault(t *testing.T) {
	cfg := A2APassthroughSetting{}
	cfg.trySetDefault()

	if cfg.BasePath != constant.A2ABasePathDefault {
		t.Errorf("BasePath = %q, want %q", cfg.BasePath, constant.A2ABasePathDefault)
	}
	if cfg.FlushIntervalMS != -1 {
		t.Errorf("FlushIntervalMS = %d, want -1", cfg.FlushIntervalMS)
	}
	if cfg.IdleTimeout != 600*time.Second {
		t.Errorf("IdleTimeout = %v, want 600s", cfg.IdleTimeout)
	}
}

func TestA2APassthroughSetting_Validate_DisabledSkipsCheck(t *testing.T) {
	cfg := A2APassthroughSetting{Enable: false}
	if err := cfg.Validate(); err != nil {
		t.Errorf("disabled passthrough should pass validate, got %v", err)
	}
}

func TestA2APassthroughSetting_Validate_EnabledOK(t *testing.T) {
	// agent-server 地址走 etcd 服务发现，启用透传不依赖手填 URL。
	cfg := A2APassthroughSetting{Enable: true}
	cfg.trySetDefault()
	if err := cfg.Validate(); err != nil {
		t.Errorf("enabled passthrough without URL should pass validate, got %v", err)
	}
}

func TestA2APassthroughSetting_Validate_BasePathLeadingSlash(t *testing.T) {
	cfg := A2APassthroughSetting{Enable: true, BasePath: "api/v1/agent"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error on basePath without leading slash")
	}
}

func TestApiServerSetting_Validate_AllDefaults(t *testing.T) {
	cfg := ApiServerSetting{
		Network: Network{BindIP: "127.0.0.1", Port: 8080},
		Service: Service{Etcd: Etcd{Endpoints: []string{"127.0.0.1:2379"}}},
	}
	cfg.trySetDefault()
	if err := cfg.Validate(); err != nil {
		t.Errorf("default ApiServerSetting should pass validate, got %v", err)
	}
}
