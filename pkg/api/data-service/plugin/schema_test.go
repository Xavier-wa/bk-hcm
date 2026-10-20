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

package dataplugin

import (
	"strings"
	"testing"

	"hcm/pkg/dal/table/types"
)

func TestIacPluginSchemaCreateReqValidate(t *testing.T) {
	req := &IacPluginSchemaCreateReq{
		PluginName:    "tencentcloud",
		PluginVersion: "1.81.5",
		SchemaHash:    "sha256:" + strings.Repeat("b", 64),
		Schema:        types.JsonField(`{"capabilities":{"apply":true}}`),
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected valid IacPluginSchemaCreateReq to pass, got: %v", err)
	}

	invalidHashReq := &IacPluginSchemaCreateReq{
		PluginName:    "tencentcloud",
		PluginVersion: "1.81.5",
		SchemaHash:    "bad",
		Schema:        types.JsonField(`{}`),
	}
	if err := invalidHashReq.Validate(); err == nil {
		t.Fatalf("expected invalid schema_hash to fail validation")
	}
}

func TestIacPluginTempDeleteCreateAndUpdateReqValidate(t *testing.T) {
	createReq := &IacPluginTempDeleteCreateReq{
		PluginName:    "tencentcloud",
		PluginVersion: "1.81.5",
		Protocol:      "http",
		State:         "init",
		Extension:     types.JsonField(`{"endpoint":"http://127.0.0.1:8080"}`),
	}
	if err := createReq.Validate(); err != nil {
		t.Fatalf("expected valid IacPluginTempDeleteCreateReq to pass, got: %v", err)
	}

	updateReq := &IacPluginTempDeleteUpdateReq{
		ID:    "1",
		State: "enabled",
	}
	if err := updateReq.Validate(); err != nil {
		t.Fatalf("expected valid IacPluginTempDeleteUpdateReq to pass, got: %v", err)
	}
}
