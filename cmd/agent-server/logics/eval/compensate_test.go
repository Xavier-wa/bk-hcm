/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may obtain a copy of the License at http://opensource.org/licenses/MIT
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

package eval

import (
	"testing"

	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"
)

func TestEnsureCompensateKitFillsBackendIdentity(t *testing.T) {
	cc.InitRuntime(cc.TestSetting{})
	got := fillCompensateKit(kit.New())
	if got.User != constant.BackendOperationUserKey {
		t.Fatalf("user = %q, want %q", got.User, constant.BackendOperationUserKey)
	}
	if got.AppCode != constant.BackendOperationAppCodeKey {
		t.Fatalf("app_code = %q, want %q", got.AppCode, constant.BackendOperationAppCodeKey)
	}
	if got.TenantID != constant.DefaultTenantID {
		t.Fatalf("tenant_id = %q, want %q", got.TenantID, constant.DefaultTenantID)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("kit.Validate: %v", err)
	}
}

func TestEnsureCompensateKitKeepsExistingUser(t *testing.T) {
	cc.InitRuntime(cc.TestSetting{})
	kt := kit.New()
	kt.User = "alice"
	kt.AppCode = "keep-me"
	got := fillCompensateKit(kt)
	if got.User != "alice" || got.AppCode != "keep-me" {
		t.Fatalf("must keep caller identity, got user=%q app_code=%q", got.User, got.AppCode)
	}
}
