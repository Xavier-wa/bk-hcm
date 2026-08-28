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

package plan

import (
	"context"
	"testing"

	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/criteria/enumor"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	"hcm/pkg/kit"
)

func TestCollectNonEmptyAdjustDemandIDs(t *testing.T) {
	adjusts := []ptypes.AdjustRPDemandReqElem{
		{DemandID: "d1", AdjustType: enumor.RPDemandAdjustTypeUpdate},
		{AdjustType: enumor.RPDemandAdjustTypeAdd},
		{DemandID: "d2", AdjustType: enumor.RPDemandAdjustTypeDelay},
	}

	got := collectNonEmptyAdjustDemandIDs(adjusts)
	if len(got) != 2 || got[0] != "d1" || got[1] != "d2" {
		t.Fatalf("collectNonEmptyAdjustDemandIDs() = %v, want [d1 d2]", got)
	}
}

func TestBuildAdjustLockItems(t *testing.T) {
	demands := rpt.ResPlanDemands{
		{
			Original: &rpt.OriginalRPDemandItem{
				DemandID: "d1",
				Cvm:      rpt.Cvm{CpuCore: 8},
			},
			Updated: &rpt.UpdatedRPDemandItem{},
		},
		{
			Updated: &rpt.UpdatedRPDemandItem{},
		},
	}

	lockedItems := buildAdjustLockItems(demands)
	if len(lockedItems) != 1 {
		t.Fatalf("buildAdjustLockItems() len = %d, want 1", len(lockedItems))
	}
	if lockedItems[0].ID != "d1" || lockedItems[0].LockedCPUCore != 8 {
		t.Fatalf("buildAdjustLockItems() = %+v, want d1 with 8 cores", lockedItems[0])
	}
}

func TestValidateAdjustDemandStatuses(t *testing.T) {
	demandIDs := []string{"d1", "d2"}
	statusMap := map[string]enumor.DemandStatus{
		"d1": enumor.DemandStatusCanApply,
		"d2": enumor.DemandStatusSpentAll,
	}

	if err := validateAdjustDemandStatuses(demandIDs, statusMap); err == nil {
		t.Fatal("validateAdjustDemandStatuses() expected error for spent all demand")
	}

	statusMap["d2"] = enumor.DemandStatusLocked
	if err := validateAdjustDemandStatuses(demandIDs, statusMap); err == nil {
		t.Fatal("validateAdjustDemandStatuses() expected error for locked demand")
	}

	statusMap["d2"] = enumor.DemandStatusCanApply
	if err := validateAdjustDemandStatuses(demandIDs, statusMap); err != nil {
		t.Fatalf("validateAdjustDemandStatuses() error = %v", err)
	}

	if err := validateAdjustDemandStatuses([]string{"d3"}, statusMap); err == nil {
		t.Fatal("validateAdjustDemandStatuses() expected error for missing demand")
	}
}

func TestResolveAdjustDemandClassAllAdd(t *testing.T) {
	kt := &kit.Kit{Ctx: context.Background(), Rid: "test-rid"}
	c := &Controller{}

	req := &ptypes.AdjustRPDemandReq{
		DemandClass: enumor.DemandClassCVM,
		Adjusts: []ptypes.AdjustRPDemandReqElem{
			{AdjustType: enumor.RPDemandAdjustTypeAdd},
		},
	}

	demandClass, err := c.resolveAdjustDemandClass(kt, req)
	if err != nil {
		t.Fatalf("resolveAdjustDemandClass() error = %v", err)
	}
	if demandClass != enumor.DemandClassCVM {
		t.Fatalf("resolveAdjustDemandClass() = %s, want %s", demandClass, enumor.DemandClassCVM)
	}

	req.DemandClass = ""
	if _, err = c.resolveAdjustDemandClass(kt, req); err == nil {
		t.Fatal("resolveAdjustDemandClass() expected error when demand_class is empty for all add")
	}
}
