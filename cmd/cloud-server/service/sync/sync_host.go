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

package sync

import (
	"fmt"
	"time"

	"hcm/cmd/cloud-server/service/sync/detail"
	"hcm/cmd/cloud-server/service/sync/lock"
	other "hcm/cmd/cloud-server/service/sync/other"
	tziyan "hcm/cmd/cloud-server/service/sync/tcloud-ziyan"
	"hcm/pkg/api/core"
	corecloud "hcm/pkg/api/core/cloud"
	protocloud "hcm/pkg/api/data-service/cloud"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"

	etcd3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/sync/errgroup"
)

// hostSyncEnv abstracts external dependencies of host sync.
type hostSyncEnv interface {
	ListAccounts(kt *kit.Kit, req *protocloud.AccountListReq) ([]*corecloud.BaseAccount, error)
	IsCvmSyncing(kt *kit.Kit, vendor enumor.Vendor, accountID string) (bool, error)
	TryLock(accountID string) (etcd3.LeaseID, error)
	UnLock(leaseID etcd3.LeaseID) error
	SyncHost(kt *kit.Kit, vendor enumor.Vendor, accountID string) error
}

type env struct {
	cliSet *client.ClientSet
}

// ListAccounts lists resource accounts of the given vendor with retry.
func (e *env) ListAccounts(kt *kit.Kit, req *protocloud.AccountListReq) ([]*corecloud.BaseAccount, error) {
	return listAccountWithRetry(kt, e.cliSet.DataService(), req)
}

// IsCvmSyncing checks whether a cvm sync is in progress for the account.
func (e *env) IsCvmSyncing(kt *kit.Kit, vendor enumor.Vendor, accountID string) (bool, error) {
	sd := &detail.SyncDetail{
		Kt:        kt,
		DataCli:   e.cliSet.DataService(),
		AccountID: accountID,
		Vendor:    string(vendor),
	}
	return isCvmSyncing(sd)
}

func (e *env) TryLock(accountID string) (etcd3.LeaseID, error) {
	return lock.Manager.TryLock(lock.ResKey(accountID, string(enumor.CvmCloudResType)))
}

func (e *env) UnLock(leaseID etcd3.LeaseID) error {
	return lock.Manager.UnLock(leaseID)
}

// SyncHost triggers host sync for the account, dispatches by vendor.
func (e *env) SyncHost(kt *kit.Kit, vendor enumor.Vendor, accountID string) error {
	switch vendor {
	case enumor.TCloudZiyan:
		return tziyan.SyncHostResource(kt, e.cliSet, &tziyan.SyncAllResourceOption{AccountID: accountID})
	case enumor.Other:
		return other.SyncHostResource(kt, e.cliSet, &other.SyncAllResourceOption{AccountID: accountID})
	default:
		return fmt.Errorf("sync host does not support vendor: %s", vendor)
	}
}

// SyncHostsByTenant 按租户同步指定厂商下全部账号的主机，目标租户取 kt.TenantID。
func SyncHostsByTenant(kt *kit.Kit, vendors []enumor.Vendor, cliSet *client.ClientSet) {
	tenantID := kt.TenantID

	defer func() {
		if r := recover(); r != nil {
			logs.Errorf("sync hosts by tenant panic, err: %v, tenant: %s, rid: %s", r, tenantID, kt.Rid)
		}
	}()

	start := time.Now()
	logs.Infof("sync hosts by tenant start, tenant: %s, vendors: %v, time: %v, rid: %s",
		tenantID, vendors, start, kt.Rid)
	defer func() {
		logs.Infof("sync hosts by tenant end, tenant: %s, cost: %v, rid: %s",
			tenantID, time.Since(start), kt.Rid)
	}()

	e := &env{cliSet: cliSet}
	for _, vendor := range vendors {
		syncHostByVendor(kt, e, vendor)
	}
}

// syncHostByVendor 同步单个厂商下全部账号的主机，账号之间并发，单账号跳过逻辑见 syncHostByAccount。
func syncHostByVendor(kt *kit.Kit, e hostSyncEnv, vendor enumor.Vendor) {
	listReq := &protocloud.AccountListReq{
		Filter: &filter.Expression{Op: filter.And, Rules: []filter.RuleFactory{
			&filter.AtomRule{Field: "vendor", Op: filter.Equal.Factory(), Value: vendor},
			&filter.AtomRule{Field: "type", Op: filter.Equal.Factory(), Value: enumor.ResourceAccount},
		}},
		Page: &core.BasePage{Start: 0, Limit: core.DefaultMaxPageLimit},
	}
	start := uint32(0)
	for {
		listReq.Page.Start = start
		accounts, err := e.ListAccounts(kt, listReq)
		if err != nil {
			logs.Errorf("list account failed, err: %v, rid: %s", err, kt.Rid)
			break
		}

		eg := new(errgroup.Group)
		eg.SetLimit(constant.SyncConcurrencyDefaultMaxLimit)
		for _, acc := range accounts {
			accountID := acc.ID
			eg.Go(func() error {
				// 单账号跳过/失败不中断其他账号，错误已在 syncHostByAccount 内记录
				syncHostByAccount(kt, e, vendor, accountID)
				return nil
			})
		}
		_ = eg.Wait()

		if len(accounts) < int(core.DefaultMaxPageLimit) {
			break
		}
		start += uint32(core.DefaultMaxPageLimit)
	}
}

// syncHostByAccount 同步单个账号的主机。
func syncHostByAccount(kt *kit.Kit, e hostSyncEnv, vendor enumor.Vendor, accountID string) {
	syncing, err := e.IsCvmSyncing(kt, vendor, accountID)
	if err != nil {
		logs.Errorf("check cvm sync status failed, err: %v, accountID: %s, rid: %s", err, accountID, kt.Rid)
		return
	}
	if syncing {
		logs.Infof("cvm sync is in progress, skip sync host, accountID: %s, rid: %s", accountID, kt.Rid)
		return
	}

	leaseID, err := e.TryLock(accountID)
	if err != nil {
		logs.Warnf("host sync is in progress, skip sync host, accountID: %s, rid: %s", accountID, kt.Rid)
		return
	}
	defer func() {
		if err := e.UnLock(leaseID); err != nil {
			logs.Errorf("unlock sync host failed, err: %v, accountID: %s, leaseID: %d, rid: %s",
				err, accountID, leaseID, kt.Rid)
		}
	}()

	if err := e.SyncHost(kt, vendor, accountID); err != nil {
		logs.Errorf("sync host failed, err: %v, vendor: %s, accountID: %s, rid: %s",
			err, vendor, accountID, kt.Rid)
	}
}

// isCvmSyncing 判断该账号的主机是否已有同步在进行（res_status 为 syncing）。
func isCvmSyncing(sd *detail.SyncDetail) (bool, error) {
	listReq := &core.ListReq{
		Filter: &filter.Expression{Op: filter.And, Rules: []filter.RuleFactory{
			&filter.AtomRule{Field: "account_id", Op: filter.Equal.Factory(), Value: sd.AccountID},
			&filter.AtomRule{Field: "vendor", Op: filter.Equal.Factory(), Value: sd.Vendor},
			&filter.AtomRule{Field: "res_name", Op: filter.Equal.Factory(), Value: enumor.CvmCloudResType},
			&filter.AtomRule{Field: "res_status", Op: filter.Equal.Factory(), Value: string(enumor.Syncing)},
		}},
		Page: core.NewCountPage(),
	}
	result, err := sd.DataCli.Global.AccountSyncDetail.List(sd.Kt, listReq)
	if err != nil {
		return false, err
	}

	return result.Count > 0, nil
}
