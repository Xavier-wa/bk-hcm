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

package global

import (
	"errors"

	rpproto "hcm/pkg/api/data-service/resource-plan"
	"hcm/pkg/client/common"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/slice"
)

// resPlanDemandLockOpFunc is a single lock or unlock HTTP call.
type resPlanDemandLockOpFunc func(kt *kit.Kit, req *rpproto.ResPlanDemandLockOpReq) error

// LockResPlanDemand lock resource plan demand. Items over BatchOperationMaxLimit are split into
// multiple requests; a later-batch failure unlocks already locked batches.
func (b *ResourcePlanClient) LockResPlanDemand(kt *kit.Kit, req *rpproto.ResPlanDemandLockOpReq) error {
	return lockResPlanDemandInBatches(kt, req, b.lockResPlanDemandOnce, b.unlockResPlanDemandOnce)
}

// UnlockResPlanDemand unlock resource plan demand. Items over BatchOperationMaxLimit are split
// into multiple requests. A later-batch failure does not re-lock earlier batches.
func (b *ResourcePlanClient) UnlockResPlanDemand(kt *kit.Kit, req *rpproto.ResPlanDemandLockOpReq) error {
	return unlockResPlanDemandInBatches(kt, req, b.unlockResPlanDemandOnce)
}

func (b *ResourcePlanClient) lockResPlanDemandOnce(kt *kit.Kit, req *rpproto.ResPlanDemandLockOpReq) error {
	return common.RequestNoResp[rpproto.ResPlanDemandLockOpReq](
		b.client, rest.PATCH, kt, req, "/res_plans/res_plan_demands/lock")
}

func (b *ResourcePlanClient) unlockResPlanDemandOnce(kt *kit.Kit, req *rpproto.ResPlanDemandLockOpReq) error {
	return common.RequestNoResp[rpproto.ResPlanDemandLockOpReq](
		b.client, rest.PATCH, kt, req, "/res_plans/res_plan_demands/unlock")
}

// lockResPlanDemandInBatches splits lock items by BatchOperationMaxLimit.
// If a later batch fails, already locked batches are unlocked before returning the lock error.
func lockResPlanDemandInBatches(kt *kit.Kit, req *rpproto.ResPlanDemandLockOpReq,
	lockOnce, unlockOnce resPlanDemandLockOpFunc) error {

	if req == nil {
		return errors.New("req is nil")
	}

	if len(req.LockedItems) <= constant.BatchOperationMaxLimit {
		return lockOnce(kt, req)
	}

	lockedBatches := make([]*rpproto.ResPlanDemandLockOpReq, 0)
	for _, batch := range slice.Split(req.LockedItems, constant.BatchOperationMaxLimit) {
		batchReq := &rpproto.ResPlanDemandLockOpReq{LockedItems: batch}
		if err := lockOnce(kt, batchReq); err != nil {
			for _, done := range lockedBatches {
				if unlockErr := unlockOnce(kt, done); unlockErr != nil {
					logs.Errorf("unlock locked res plan demand batch failed, err: %v, count: %d, rid: %s",
						unlockErr, len(done.LockedItems), kt.Rid)
				}
			}
			return err
		}
		lockedBatches = append(lockedBatches, batchReq)
	}

	return nil
}

// unlockResPlanDemandInBatches splits unlock items by BatchOperationMaxLimit.
func unlockResPlanDemandInBatches(kt *kit.Kit, req *rpproto.ResPlanDemandLockOpReq,
	unlockOnce resPlanDemandLockOpFunc) error {

	if req == nil {
		return errors.New("req is nil")
	}

	if len(req.LockedItems) <= constant.BatchOperationMaxLimit {
		return unlockOnce(kt, req)
	}

	for _, batch := range slice.Split(req.LockedItems, constant.BatchOperationMaxLimit) {
		batchReq := &rpproto.ResPlanDemandLockOpReq{LockedItems: batch}
		if err := unlockOnce(kt, batchReq); err != nil {
			return err
		}
	}

	return nil
}
