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

package wiring

import (
	"context"
	"net/http"
	"testing"

	"hcm/pkg/cc"
)

func TestRegisterInternalServers_Disabled(t *testing.T) {
	dispatchers, err := RegisterInternalServers(context.Background(), http.NewServeMux(), cc.MCPInternalSetting{})
	if err != nil {
		t.Fatalf("RegisterInternalServers error: %v", err)
	}
	if len(dispatchers) != 0 {
		t.Fatalf("dispatchers = %+v, want empty", dispatchers)
	}
}

func TestRegisterInternalServers_NilMux(t *testing.T) {
	if _, err := RegisterInternalServers(context.Background(), nil, cc.MCPInternalSetting{}); err == nil {
		t.Fatal("expected nil mux error")
	}
}
