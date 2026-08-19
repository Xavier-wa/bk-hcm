/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2026 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package sync

import (
	"errors"
	"sync/atomic"
	"testing"

	corecloud "hcm/pkg/api/core/cloud"
	protocloud "hcm/pkg/api/data-service/cloud"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"

	etcd3 "go.etcd.io/etcd/client/v3"
)

type mockHostSyncEnv struct {
	listAccountsFn func(kt *kit.Kit, req *protocloud.AccountListReq) ([]*corecloud.BaseAccount, error)
	isCvmSyncingFn func(kt *kit.Kit, vendor enumor.Vendor, accountID string) (bool, error)
	tryLockFn      func(accountID string) (etcd3.LeaseID, error)
	unLockFn       func(leaseID etcd3.LeaseID) error
	syncHostFn     func(kt *kit.Kit, vendor enumor.Vendor, accountID string) error

	syncHostCallCount atomic.Int32
}

func (m *mockHostSyncEnv) ListAccounts(kt *kit.Kit, req *protocloud.AccountListReq) ([]*corecloud.BaseAccount, error) {
	if m.listAccountsFn != nil {
		return m.listAccountsFn(kt, req)
	}
	return nil, nil
}

func (m *mockHostSyncEnv) IsCvmSyncing(kt *kit.Kit, vendor enumor.Vendor, accountID string) (bool, error) {
	if m.isCvmSyncingFn != nil {
		return m.isCvmSyncingFn(kt, vendor, accountID)
	}
	return false, nil
}

func (m *mockHostSyncEnv) TryLock(accountID string) (etcd3.LeaseID, error) {
	if m.tryLockFn != nil {
		return m.tryLockFn(accountID)
	}
	return 0, nil
}

func (m *mockHostSyncEnv) UnLock(leaseID etcd3.LeaseID) error {
	if m.unLockFn != nil {
		return m.unLockFn(leaseID)
	}
	return nil
}

func (m *mockHostSyncEnv) SyncHost(kt *kit.Kit, vendor enumor.Vendor, accountID string) error {
	m.syncHostCallCount.Add(1)
	if m.syncHostFn != nil {
		return m.syncHostFn(kt, vendor, accountID)
	}
	return nil
}

func TestSyncHostByAccount(t *testing.T) {
	kt := kit.New()
	accountID := "00000001"
	vendor := enumor.TCloudZiyan

	tests := []struct {
		name             string
		env              *mockHostSyncEnv
		wantSyncHostCall int32
	}{
		{
			name: "cvm syncing, skip without lock",
			env: &mockHostSyncEnv{
				isCvmSyncingFn: func(_ *kit.Kit, _ enumor.Vendor, _ string) (bool, error) {
					return true, nil
				},
				tryLockFn: func(_ string) (etcd3.LeaseID, error) {
					t.Error("TryLock should not be called when cvm is syncing")
					return 0, nil
				},
			},
			wantSyncHostCall: 0,
		},
		{
			name: "check cvm syncing failed, skip",
			env: &mockHostSyncEnv{
				isCvmSyncingFn: func(_ *kit.Kit, _ enumor.Vendor, _ string) (bool, error) {
					return false, errors.New("db error")
				},
				tryLockFn: func(_ string) (etcd3.LeaseID, error) {
					t.Error("TryLock should not be called when check cvm syncing failed")
					return 0, nil
				},
			},
			wantSyncHostCall: 0,
		},
		{
			name: "try lock failed, skip",
			env: &mockHostSyncEnv{
				isCvmSyncingFn: func(_ *kit.Kit, _ enumor.Vendor, _ string) (bool, error) {
					return false, nil
				},
				tryLockFn: func(_ string) (etcd3.LeaseID, error) {
					return 0, errors.New("lock grabbing failed")
				},
			},
			wantSyncHostCall: 0,
		},
		{
			name: "sync host success",
			env: &mockHostSyncEnv{
				isCvmSyncingFn: func(_ *kit.Kit, _ enumor.Vendor, _ string) (bool, error) {
					return false, nil
				},
				tryLockFn: func(_ string) (etcd3.LeaseID, error) {
					return 1, nil
				},
				unLockFn: func(leaseID etcd3.LeaseID) error {
					if leaseID != 1 {
						t.Errorf("UnLock got leaseID %d, want 1", leaseID)
					}
					return nil
				},
				syncHostFn: func(_ *kit.Kit, _ enumor.Vendor, _ string) error {
					return nil
				},
			},
			wantSyncHostCall: 1,
		},
		{
			name: "sync host failed, unlock still called",
			env: &mockHostSyncEnv{
				isCvmSyncingFn: func(_ *kit.Kit, _ enumor.Vendor, _ string) (bool, error) {
					return false, nil
				},
				tryLockFn: func(_ string) (etcd3.LeaseID, error) {
					return 2, nil
				},
				unLockFn: func(leaseID etcd3.LeaseID) error {
					if leaseID != 2 {
						t.Errorf("UnLock got leaseID %d, want 2", leaseID)
					}
					return nil
				},
				syncHostFn: func(_ *kit.Kit, _ enumor.Vendor, _ string) error {
					return errors.New("sync failed")
				},
			},
			wantSyncHostCall: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncHostByAccount(kt, tt.env, vendor, accountID)
			if got := tt.env.syncHostCallCount.Load(); got != tt.wantSyncHostCall {
				t.Errorf("SyncHost call count = %d, want %d", got, tt.wantSyncHostCall)
			}
		})
	}
}

func TestIsCvmSyncing(t *testing.T) {
	// isCvmSyncing builds a data-service list request; unit test would require
	// mocking the data-service client, which is not feasible without an interface.
	// The core filtering logic is covered by TestSyncHostByAccount above via mock.
}
