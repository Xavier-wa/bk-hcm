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

package runobserve

import (
	"context"
	"testing"

	"hcm/pkg/kit"
)

func TestLedgerAsyncRidStaysWithinKitLimit(t *testing.T) {
	// 蓝鲸入站 rid 常见为 UUID(36)。create/cas 各自从 RunMeta.Kit 派生一次，
	// 必须仍落在 data-service kit.Validate 的 16~50 之间；评估复用 CAS 的 kit。
	const incoming = "2f35e1a4-5444-482a-926b-91286fdc79fd"
	if len(incoming) != 36 {
		t.Fatalf("fixture rid len = %d, want 36", len(incoming))
	}

	parent := &kit.Kit{Rid: incoming, Ctx: context.Background()}
	meta := NewRunMeta("run-1", "sess", "user", "chat", 1, parent)
	if meta.Kit == nil || meta.Kit.Rid != incoming {
		t.Fatalf("RunMeta must keep request rid, got %v", meta.Kit)
	}

	createKt := meta.Kit.NewSubKit()
	if n := len(createKt.Rid); n < 16 || n > 50 {
		t.Fatalf("create/cas rid %q len %d not in 16~50", createKt.Rid, n)
	}

	// 旧路径中间件+CAS+submitEval 各叠一次：36+7+7+7=57，会被 data-service 拒。
	stacked := createKt.NewSubKit().NewSubKit()
	if len(stacked.Rid) <= 50 {
		t.Fatalf("three NewSubKit on UUID should exceed 50, got %d (%s)",
			len(stacked.Rid), stacked.Rid)
	}
}
