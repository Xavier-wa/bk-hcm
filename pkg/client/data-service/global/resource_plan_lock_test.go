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
	"fmt"
	"testing"

	rpproto "hcm/pkg/api/data-service/resource-plan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/kit"

	"github.com/stretchr/testify/assert"
)

func buildResPlanDemandLockOpReq(count int) *rpproto.ResPlanDemandLockOpReq {
	items := make([]rpproto.ResPlanDemandLockOpItem, count)
	for i := 0; i < count; i++ {
		items[i] = rpproto.ResPlanDemandLockOpItem{
			ID:       fmt.Sprintf("demand-%d", i),
			TicketID: "ticket-1",
		}
	}
	return &rpproto.ResPlanDemandLockOpReq{LockedItems: items}
}

func TestLockResPlanDemandInBatches_NilReq(t *testing.T) {
	kt := kit.New()
	err := lockResPlanDemandInBatches(kt, nil,
		func(_ *kit.Kit, _ *rpproto.ResPlanDemandLockOpReq) error {
			t.Fatal("lock should not be called when req is nil")
			return nil
		},
		func(_ *kit.Kit, _ *rpproto.ResPlanDemandLockOpReq) error {
			t.Fatal("unlock should not be called when req is nil")
			return nil
		})
	assert.EqualError(t, err, "req is nil")
}

func TestLockResPlanDemandInBatches_RequestCount(t *testing.T) {
	kt := kit.New()
	tests := []struct {
		name      string
		itemCount int
		wantCalls int
	}{
		{name: "empty items still one request", itemCount: 0, wantCalls: 1},
		{name: "one item one request", itemCount: 1, wantCalls: 1},
		{name: "max limit one request", itemCount: constant.BatchOperationMaxLimit, wantCalls: 1},
		{name: "over limit two requests", itemCount: constant.BatchOperationMaxLimit + 1, wantCalls: 2},
		{name: "two full batches", itemCount: constant.BatchOperationMaxLimit * 2, wantCalls: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lockCalls := 0
			err := lockResPlanDemandInBatches(kt, buildResPlanDemandLockOpReq(tc.itemCount),
				func(_ *kit.Kit, req *rpproto.ResPlanDemandLockOpReq) error {
					lockCalls++
					assert.LessOrEqual(t, len(req.LockedItems), constant.BatchOperationMaxLimit)
					return nil
				},
				func(_ *kit.Kit, _ *rpproto.ResPlanDemandLockOpReq) error {
					t.Fatal("unlock should not be called on success")
					return nil
				})
			assert.NoError(t, err)
			assert.Equal(t, tc.wantCalls, lockCalls)
		})
	}
}

func TestLockResPlanDemandInBatches_RollbackOnSecondBatchFailed(t *testing.T) {
	kt := kit.New()
	req := buildResPlanDemandLockOpReq(constant.BatchOperationMaxLimit + 1)

	lockCalls := 0
	var unlockedIDs []string
	lockErr := errors.New("lock batch 2 failed")
	err := lockResPlanDemandInBatches(kt, req,
		func(_ *kit.Kit, batchReq *rpproto.ResPlanDemandLockOpReq) error {
			lockCalls++
			if lockCalls == 2 {
				return lockErr
			}
			return nil
		},
		func(_ *kit.Kit, batchReq *rpproto.ResPlanDemandLockOpReq) error {
			for _, item := range batchReq.LockedItems {
				unlockedIDs = append(unlockedIDs, item.ID)
			}
			return nil
		})

	assert.Equal(t, lockErr, err)
	assert.Equal(t, 2, lockCalls)
	assert.Equal(t, constant.BatchOperationMaxLimit, len(unlockedIDs))
	for i := 0; i < constant.BatchOperationMaxLimit; i++ {
		assert.Equal(t, fmt.Sprintf("demand-%d", i), unlockedIDs[i])
	}
}

func TestLockResPlanDemandInBatches_RollbackUnlockFailedStillReturnLockErr(t *testing.T) {
	kt := kit.New()
	req := buildResPlanDemandLockOpReq(constant.BatchOperationMaxLimit + 1)

	lockCalls := 0
	lockErr := errors.New("lock batch 2 failed")
	err := lockResPlanDemandInBatches(kt, req,
		func(_ *kit.Kit, _ *rpproto.ResPlanDemandLockOpReq) error {
			lockCalls++
			if lockCalls == 2 {
				return lockErr
			}
			return nil
		},
		func(_ *kit.Kit, _ *rpproto.ResPlanDemandLockOpReq) error {
			return errors.New("unlock failed")
		})

	assert.Equal(t, lockErr, err)
	assert.Equal(t, 2, lockCalls)
}

func TestUnlockResPlanDemandInBatches_NilReq(t *testing.T) {
	kt := kit.New()
	err := unlockResPlanDemandInBatches(kt, nil, func(_ *kit.Kit, _ *rpproto.ResPlanDemandLockOpReq) error {
		t.Fatal("unlock should not be called when req is nil")
		return nil
	})
	assert.EqualError(t, err, "req is nil")
}

func TestUnlockResPlanDemandInBatches_RequestCount(t *testing.T) {
	kt := kit.New()
	tests := []struct {
		name      string
		itemCount int
		wantCalls int
	}{
		{name: "empty items still one request", itemCount: 0, wantCalls: 1},
		{name: "one item one request", itemCount: 1, wantCalls: 1},
		{name: "max limit one request", itemCount: constant.BatchOperationMaxLimit, wantCalls: 1},
		{name: "over limit two requests", itemCount: constant.BatchOperationMaxLimit + 1, wantCalls: 2},
		{name: "two full batches", itemCount: constant.BatchOperationMaxLimit * 2, wantCalls: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unlockCalls := 0
			err := unlockResPlanDemandInBatches(kt, buildResPlanDemandLockOpReq(tc.itemCount),
				func(_ *kit.Kit, req *rpproto.ResPlanDemandLockOpReq) error {
					unlockCalls++
					assert.LessOrEqual(t, len(req.LockedItems), constant.BatchOperationMaxLimit)
					return nil
				})
			assert.NoError(t, err)
			assert.Equal(t, tc.wantCalls, unlockCalls)
		})
	}
}

func TestUnlockResPlanDemandInBatches_SecondBatchFailedNotRelock(t *testing.T) {
	kt := kit.New()
	req := buildResPlanDemandLockOpReq(constant.BatchOperationMaxLimit + 1)

	unlockCalls := 0
	unlockErr := errors.New("unlock batch 2 failed")
	var firstBatchCount int
	err := unlockResPlanDemandInBatches(kt, req, func(_ *kit.Kit, batchReq *rpproto.ResPlanDemandLockOpReq) error {
		unlockCalls++
		if unlockCalls == 1 {
			firstBatchCount = len(batchReq.LockedItems)
		}
		if unlockCalls == 2 {
			return unlockErr
		}
		return nil
	})

	assert.Equal(t, unlockErr, err)
	assert.Equal(t, 2, unlockCalls)
	assert.Equal(t, constant.BatchOperationMaxLimit, firstBatchCount)
}
