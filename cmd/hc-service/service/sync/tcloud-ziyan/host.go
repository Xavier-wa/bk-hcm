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

package ziyan

import (
	"fmt"
	"strconv"

	ressync "hcm/cmd/hc-service/logics/res-sync"
	"hcm/cmd/hc-service/logics/res-sync/ziyan"
	"hcm/cmd/hc-service/service/sync/handler"
	"hcm/pkg/api/core"
	corecvm "hcm/pkg/api/core/cloud/cvm"
	"hcm/pkg/api/hc-service/sync"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/tools/converter"
)

// SyncHostWithRelRes ....
func (svc *service) SyncHostWithRelRes(cts *rest.Contexts) (interface{}, error) {
	return nil, handler.ResourceSyncV2[corecvm.Cvm[corecvm.TCloudZiyanHostExtension]](cts,
		&hostHandler{cli: svc.syncCli})
}

// SyncHostWithRelResByCond ....
func (svc *service) SyncHostWithRelResByCond(cts *rest.Contexts) (interface{}, error) {
	req := new(sync.TCloudZiyanSyncHostByCondReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	syncCli, err := svc.syncCli.TCloudZiyan(cts.Kit, req.AccountID)
	if err != nil {
		return nil, err
	}
	op := &hostHandler{syncCli: syncCli}
	params := &ziyan.SyncHostParams{AccountID: req.AccountID, BizID: req.BizID, HostIDs: req.HostIDs}
	if err := op.SyncByCond(cts.Kit, params); err != nil {
		logs.Errorf("sync host by condition failed, err: %v, req: %+v, rid: %s", err, req, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

// SyncHostCCInfoByCond 增量同步入口，只刷新 cc 来源字段，创建的主机转完整链路处理
func (svc *service) SyncHostCCInfoByCond(cts *rest.Contexts) (interface{}, error) {
	req := new(sync.TCloudZiyanSyncHostByCondReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	syncCli, err := svc.syncCli.TCloudZiyan(cts.Kit, req.AccountID)
	if err != nil {
		return nil, err
	}

	params := &ziyan.SyncHostParams{AccountID: req.AccountID, BizID: req.BizID, HostIDs: req.HostIDs}
	if _, err := syncCli.HostCCInfo(cts.Kit, params); err != nil {
		logs.Errorf("sync host cc info by condition failed, err: %v, req: %+v, rid: %s", err, req, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

// DeleteHostByCond ....
func (svc *service) DeleteHostByCond(cts *rest.Contexts) (interface{}, error) {
	req := new(sync.TCloudZiyanDelHostByCondReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	syncCli, err := svc.syncCli.TCloudZiyan(cts.Kit, req.AccountID)
	if err != nil {
		return nil, err
	}
	op := &hostHandler{syncCli: syncCli}
	if err := op.DeleteHost(cts.Kit, &ziyan.DelHostParams{DelHostIDs: req.HostIDs}); err != nil {
		logs.Errorf("sync host by condition failed, err: %v, req: %+v, rid: %s", err, req, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

// hostHandler cvm sync handler.
type hostHandler struct {
	cli ressync.Interface

	// Prepare 构建参数
	request *sync.TCloudZiyanSyncHostReq
	syncCli ziyan.Interface
	offset  uint64
}

var _ handler.HandlerV2[corecvm.Cvm[corecvm.TCloudZiyanHostExtension]] = new(hostHandler)

// Prepare ...
func (hd *hostHandler) Prepare(cts *rest.Contexts) error {
	req := new(sync.TCloudZiyanSyncHostReq)
	if err := cts.DecodeInto(req); err != nil {
		return errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return errf.NewFromErr(errf.InvalidParameter, err)
	}

	syncCli, err := hd.cli.TCloudZiyan(cts.Kit, req.AccountID)
	if err != nil {
		return err
	}

	hd.request = req
	hd.syncCli = syncCli

	return nil
}

// Next ...
func (hd *hostHandler) Next(kt *kit.Kit) ([]corecvm.Cvm[corecvm.TCloudZiyanHostExtension], error) {
	params := &cmdb.ListBizHostParams{
		BizID:  hd.request.BizID,
		Fields: []string{"bk_host_id"},
		Page: &cmdb.BasePage{
			Start: int64(hd.offset),
			Limit: int64(core.DefaultMaxPageLimit),
			Sort:  "bk_host_id",
		},
		HostPropertyFilter: &cmdb.QueryFilter{
			Rule: &cmdb.CombinedRule{
				Condition: "AND",
				Rules:     []cmdb.Rule{&cmdb.AtomRule{Field: "bk_cloud_id", Operator: "equal", Value: 0}},
			},
		},
	}

	result, err := cmdb.CmdbClient().ListBizHost(kt, params)
	if err != nil {
		logs.Errorf("call cmdb to list biz host failed, err: %v, req: %+v, rid: %s", err, params, kt.Rid)
		return nil, err
	}

	if len(result.Info) == 0 {
		return nil, nil
	}

	hosts := make([]corecvm.Cvm[corecvm.TCloudZiyanHostExtension], 0)
	for _, host := range result.Info {
		hosts = append(hosts, corecvm.Cvm[corecvm.TCloudZiyanHostExtension]{
			BaseCvm: corecvm.BaseCvm{CloudID: strconv.FormatInt(host.BkHostID, 10)},
		})
	}

	hd.offset += uint64(core.DefaultMaxPageLimit)

	return hosts, nil
}

// Sync ...
func (hd *hostHandler) Sync(kt *kit.Kit, hosts []corecvm.Cvm[corecvm.TCloudZiyanHostExtension]) error {
	ids := make([]int64, len(hosts))
	for i, host := range hosts {
		id, err := strconv.ParseInt(host.GetCloudID(), 10, 64)
		if err != nil {
			logs.Errorf("failed to convert value to type int64, err: %v, val: %v, rid: %s", err, host.GetCloudID(),
				kt.Rid)
			return err
		}

		ids[i] = id
	}

	params := &ziyan.SyncHostParams{
		AccountID: hd.request.AccountID,
		BizID:     hd.request.BizID,
		HostIDs:   ids,
	}

	return hd.SyncByCond(kt, params)
}

// SyncByCond ...
func (hd *hostHandler) SyncByCond(kt *kit.Kit, params *ziyan.SyncHostParams) error {
	if _, err := hd.syncCli.HostWithRelRes(kt, params); err != nil {
		logs.Errorf("sync tcloud ziyan host with rel res failed, err: %v, opt: %v, rid: %s", err, params, kt.Rid)
		return err
	}

	return nil
}

// RemoveDeletedFromCloud ...
func (hd *hostHandler) RemoveDeletedFromCloud(kt *kit.Kit, allCloudIDMap map[string]struct{}) error {
	ccBizExistHostIDs := make(map[int64]struct{}, len(allCloudIDMap))
	for hostIDStr := range allCloudIDMap {
		hostID, err := strconv.ParseInt(hostIDStr, 10, 64)
		if err != nil {
			logs.Errorf("failed to convert value to type int64, err: %v, val: %v, rid: %s", err, hostIDStr, kt.Rid)
			return err
		}
		ccBizExistHostIDs[hostID] = struct{}{}
	}

	params := &ziyan.DelHostParams{BizID: hd.request.BizID, CCBizExistHostIDs: ccBizExistHostIDs}
	return hd.DeleteHost(kt, params)
}

// DeleteHost ...
func (hd *hostHandler) DeleteHost(kt *kit.Kit, params *ziyan.DelHostParams) error {
	err := hd.syncCli.RemoveHostFromCC(kt, params)
	if err != nil {
		logs.Errorf("remove host by cc host ids failed, err: %v, params: %+v, rid: %s", err, converter.PtrToVal(params),
			kt.Rid)
		return err
	}

	return nil
}

// SyncConcurrent ...
func (hd *hostHandler) SyncConcurrent() uint {
	if hd.request != nil && hd.request.Concurrent != 0 {
		return hd.request.Concurrent
	}
	// read from config file
	_, syncing := cc.HCService().SyncConfig.GetSyncConcurrent(enumor.Ziyan, enumor.CvmCloudResType,
		cc.ConcurrentWildcard)
	return max(syncing, 1)
}

// Describe ...
func (hd *hostHandler) Describe() string {
	if hd.request == nil {
		return fmt.Sprintf("ziyan %s(-)", hd.Resource())
	}
	return fmt.Sprintf("ziyan %s(bizID=%d,account=%s)", hd.Resource(), hd.request.BizID, hd.request.AccountID)
}

// Resource ...
func (hd *hostHandler) Resource() enumor.CloudResourceType {
	return enumor.CvmCloudResType
}
