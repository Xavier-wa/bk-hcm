/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package scheduler

import (
	"fmt"
	"strings"

	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/cvmapi"
)

// VerifyApplyCapacity checks real-time capacity for every CVM suborder in the apply request.
// Suborder types other than CVM are skipped. Require types exempt from capacity verification
// (e.g. small-amount green channel) are skipped entirely.
// Returns (pass=false, human-readable reason, nil) for business failures;
// returns (false, "", error) for system errors (downstream service unavailable, etc.).
func (s *scheduler) VerifyApplyCapacity(kt *kit.Kit, input *types.ApplyReq) (bool, string, error) {
	if input.RequireType.NotNeedVerifyCapacity() {
		logs.Infof("skip capacity verify for require type: %d, bizID: %d, rid: %s", input.RequireType,
			input.BkBizId, kt.Rid)
		return true, "", nil
	}

	for idx, sub := range input.Suborders {
		if sub == nil || sub.ResourceType != types.ResourceTypeCvm || sub.Spec == nil {
			continue
		}
		pass, reason, err := s.verifySuborderCapacity(kt, input, idx, sub)
		if err != nil {
			return false, "", err
		}
		if !pass {
			return false, reason, nil
		}
	}

	return true, "", nil
}

// verifySuborderCapacity checks real-time available capacity for a single CVM suborder.
// Zone resolution and capacity query logic are delegated to generator methods, ensuring
// the pre-check is always consistent with the production allocation path.
//
// Two query strategies (mirroring the production generator):
//   - IsCVMSeparateCampus: one region-level query with CvmSeparateCampus, aggregate all zones.
//   - Concentrate: one query per candidate zone, sum across zones.
func (s *scheduler) verifySuborderCapacity(kt *kit.Kit, input *types.ApplyReq, idx int,
	sub *types.Suborder) (bool, string, error) {

	spec := sub.Spec
	replicas := int64(sub.Replicas)

	// Construct a minimal ApplyOrder so we can reuse generator methods directly.
	order := &types.ApplyOrder{
		RequireType: input.RequireType,
		BkBizId:     input.BkBizId,
		Spec:        spec,
	}

	// Zone resolution: delegates to GetApplyOrderMultiZones — the exact same function
	// used by the production GenerateCVM path, ensuring consistent zone semantics.
	candidateZones, err := s.generator.GetApplyOrderMultiZones(kt, order)
	if err != nil {
		logs.Errorf("failed to resolve candidate zones for capacity verify, err: %v, suborder: %d, bizID: %d, "+
			"rid: %s", err, idx+1, input.BkBizId, kt.Rid)
		return false, "", err
	}

	var available int64

	if spec.IsCVMSeparateCampus() {
		// Separate campus: one region-level query with CvmSeparateCampus aggregates all zones.
		// Mirrors generateCVMSeparate → getCapacity(CvmSeparateCampus, "", "").
		zoneCapacity, err := s.generator.QueryCapacity(kt, order, cvmapi.CvmSeparateCampus, "", "",
			candidateZones, true)
		if err != nil {
			logs.Errorf("failed to get capacity for separate campus verify, err: %v, suborder: %d, "+
				"bizID: %d, rid: %s", err, idx+1, input.BkBizId, kt.Rid)
			return false, "", err
		}
		for _, zone := range candidateZones {
			available += zoneCapacity[zone]
		}
	} else {
		// Concentrate: query each candidate zone individually and accumulate.
		// Production queries per-subnet via buildSubnetFuzzyZone; zone-level is an intentional
		// approximation for the read-only pre-check (no subnet write, no resource lock).
		for _, zone := range candidateZones {
			zoneCapacity, err := s.generator.QueryCapacity(kt, order, zone, spec.Vpc, spec.Subnet,
				candidateZones, true)
			if err != nil {
				logs.Errorf("failed to get capacity for concentrate verify, err: %v, zone: %s, "+
					"suborder: %d, bizID: %d, rid: %s", err, zone, idx+1, input.BkBizId, kt.Rid)
				return false, "", err
			}
			available += zoneCapacity[zone]
		}
	}

	if available < replicas {
		reason := buildCapacityReason(idx, spec, candidateZones, replicas, available)
		logs.Infof("suborder capacity not enough, suborder: %d, need: %d, available: %d, zones: %v, bizID: %d, "+
			"rid: %s", idx+1, replicas, available, candidateZones, input.BkBizId, kt.Rid)
		return false, reason, nil
	}

	return true, "", nil
}

// buildCapacityReason builds a human-readable capacity shortage message for a suborder,
// suitable for display to end users and AI agents.
func buildCapacityReason(idx int, spec *types.ResourceSpec, zones []string, need, available int64) string {
	return fmt.Sprintf("子单%d（机型 %s、地域 %s、可用区 %s）可申领容量不足，需要 %d 台，当前实时可用 %d 台，"+
		"请减少申请数量、更换机型或调整可用区后重试。", idx+1, spec.DeviceType, spec.Region,
		strings.Join(zones, "/"), need, available)
}
