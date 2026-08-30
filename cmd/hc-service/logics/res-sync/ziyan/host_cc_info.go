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

	"hcm/pkg/api/core/cloud/cvm"
	"hcm/pkg/api/data-service/cloud"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/tools/converter"
)

// HostCCInfo 增量同步入口，按 cc 与 db 的存在性把主机路由到创建、更新、删除三条处理。
// 与 HostWithRelRes 的差别在于：更新与删除不访问云，只刷新 cc 来源字段，
// 仅创建（db 无记录，需要从无到有补全云上信息）转由 HostWithRelRes 完整链路处理。
func (cli *client) HostCCInfo(kt *kit.Kit, params *SyncHostParams) (*SyncResult, error) {
	if params == nil {
		logs.Errorf("params is nil, rid: %s", kt.Rid)
		return nil, fmt.Errorf("params is nil")
	}

	if err := params.Validate(); err != nil {
		logs.Errorf("param is invalid, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	// 需要不带业务id去查询主机，防止在前面耗时过程中，主机已被转移到其他业务，这里查不到主机导致把db里的数据误删的问题
	ccHosts, err := cli.getHostFromCCByHostIDs(kt, params.HostIDs, cmdb.HostFields)
	if err != nil {
		logs.Errorf("get host from cc by host id failed, err: %v, ids: %v, rid: %s", err, params.HostIDs, kt.Rid)
		return nil, err
	}

	ccHostMap := buildCCHostMapByCloudID(kt, ccHosts)

	// 同时按 bk_host_id 和 cloud_id 两个维度预查 db 存量主机，理由同 Host()：
	// cc 主机重建会导致 bk_host_id 变化但 cloud_id 不变，只按 bk_host_id 查会漏掉旧记录被误判为新增
	dbHosts, err := cli.listHostFromDBForDiffByCloudIDs(kt, params.HostIDs, converter.MapKeyToSlice(ccHostMap))
	if err != nil {
		logs.Errorf("list host from db failed, err: %v, hostIDs: %v, rid: %s", err, params.HostIDs, kt.Rid)
		return nil, err
	}

	// 业务归属查询前置
	hostIDs := make([]int64, 0, len(ccHostMap))
	for _, ccHost := range ccHostMap {
		hostIDs = append(hostIDs, ccHost.BkHostID)
	}
	hostBizIDMap, err := cli.getHostBizID(kt, hostIDs)
	if err != nil {
		logs.Errorf("get host biz id failed, err: %v, host ids: %v, rid: %s", err, hostIDs, kt.Rid)
		return nil, err
	}

	creates, updates, deletes := classifyHost(kt, params.AccountID, ccHostMap, dbHosts,
		hostBizIDMap)

	// 处理顺序与 Host() 保持一致：删除 -> 更新 -> 新增
	if len(deletes) > 0 {
		if err = cli.deleteHost(kt, deletes); err != nil {
			return nil, err
		}
	}

	if len(updates) > 0 {
		if err = cli.updateHost(kt, updates); err != nil {
			return nil, err
		}
	}

	if len(creates) > 0 {
		createParams := &SyncHostParams{AccountID: params.AccountID, BizID: params.BizID, HostIDs: creates}
		if _, err = cli.HostWithRelRes(kt, createParams); err != nil {
			logs.Errorf("sync host with rel res for created host failed, err: %v, ids: %v, rid: %s", err,
				creates, kt.Rid)
			return nil, err
		}
	}

	logs.Infof("[%s] sync host cc info success, create: %d, update: %d, delete: %d, rid: %s", enumor.TCloudZiyan,
		len(creates), len(updates), len(deletes), kt.Rid)

	return new(SyncResult), nil
}

// buildCCHostMapByCloudID 以 cloud id 为键索引 cc 主机，仅做字段读取，不构建 cc 字段期望态。
func buildCCHostMapByCloudID(kt *kit.Kit, ccHosts []cmdb.Host) map[string]cmdb.Host {
	ccHostMap := make(map[string]cmdb.Host, len(ccHosts))
	for i := range ccHosts {
		// 当主机不存在bk_cloud_inst_id时，用固资号兜底作为cloud id，保证唯一
		cloudID := ccHosts[i].BkCloudInstID
		if cloudID == "" {
			cloudID = ccHosts[i].BkAssetID
		}
		// cc中可能会存在连固资号都没有的脏数据，这里需要把他们剔除掉
		if cloudID == "" {
			logs.Errorf("host(%d) asset id is invalid, rid: %s", ccHosts[i].BkHostID, kt.Rid)
			continue
		}

		ccHostMap[cloudID] = ccHosts[i]
	}

	return ccHostMap
}

// classifyHost 按 cloud id 配对 cc 与 db 的主机，产出三类处理结果：
// 待创建 host id、cc-only 更新、待删除 cloud id。cc 与 db 都不存在的主机不在两个集合中，天然被跳过。
// 查不到业务归属或 cc 字段无变化的主机不产生更新；纯函数，依赖的数据全部由调用方前置查询。
func classifyHost(kt *kit.Kit, accountID string, ccHostMap map[string]cmdb.Host,
	dbHosts []cvm.Cvm[cvm.TCloudZiyanHostExtension], hostBizIDMap map[int64]int64) (
	createHostIDs []int64, updates []cloud.CvmBatchUpdateWithExtension[cvm.TCloudZiyanHostExtension],
	delCloudIDs []string) {

	dbHostMap := make(map[string]cvm.Cvm[cvm.TCloudZiyanHostExtension], len(dbHosts))
	for i := range dbHosts {
		dbHostMap[dbHosts[i].CloudID] = dbHosts[i]
	}

	createHostIDs = make([]int64, 0)
	updates = make([]cloud.CvmBatchUpdateWithExtension[cvm.TCloudZiyanHostExtension], 0)
	delCloudIDs = make([]string, 0)

	for cloudID, ccHost := range ccHostMap {
		dbHost, exist := dbHostMap[cloudID]
		if !exist {
			createHostIDs = append(createHostIDs, ccHost.BkHostID)
			continue
		}

		bizID, ok := hostBizIDMap[ccHost.BkHostID]
		if !ok {
			logs.Errorf("can not find host(%d) biz id, rid: %s", ccHost.BkHostID, kt.Rid)
			continue
		}

		ccExpect := convertToHost(&ccHost, accountID, bizID)
		if !isHostCCChange(ccExpect, dbHost) {
			continue
		}

		updates = append(updates, convToCCOnlyUpdate(ccExpect, dbHost))
	}

	for cloudID := range dbHostMap {
		if _, exist := ccHostMap[cloudID]; !exist {
			delCloudIDs = append(delCloudIDs, cloudID)
		}
	}

	return createHostIDs, updates, delCloudIDs
}

// convToCCOnlyUpdate 构造只含 cc 来源字段的更新：云来源字段留空，data-service 只更新非零字段，故不覆盖；
// Status 为必填校验字段，回填 db 现值；extension 在 data-service 侧整体 json merge，以 db 为基底仅覆盖 cc 字段。
func convToCCOnlyUpdate(ccExpect, dbHost cvm.Cvm[cvm.TCloudZiyanHostExtension]) (
	cloud.CvmBatchUpdateWithExtension[cvm.TCloudZiyanHostExtension]) {

	extension := converter.ValToPtr(converter.PtrToVal(dbHost.Extension))
	if ccExpect.Extension != nil {
		extension.HostName = ccExpect.Extension.HostName
		extension.SvrSourceTypeID = ccExpect.Extension.SvrSourceTypeID
		extension.SrvStatus = ccExpect.Extension.SrvStatus
		extension.SvrDeviceClass = ccExpect.Extension.SvrDeviceClass
		extension.BkDisk = ccExpect.Extension.BkDisk
		extension.BkCpu = ccExpect.Extension.BkCpu
		extension.BkOSName = ccExpect.Extension.BkOSName
		extension.Operator = ccExpect.Extension.Operator
		extension.BkBakOperator = ccExpect.Extension.BkBakOperator
	}

	return cloud.CvmBatchUpdateWithExtension[cvm.TCloudZiyanHostExtension]{
		CvmBatchUpdate: cloud.CvmBatchUpdate{
			ID:                   dbHost.ID,
			BkBizID:              ccExpect.BkBizID,
			BkHostID:             ccExpect.BkHostID,
			BkAssetID:            ccExpect.BkAssetID,
			BkCloudID:            &ccExpect.BkCloudID,
			Region:               ccExpect.Region,
			OsName:               ccExpect.OsName,
			MachineType:          ccExpect.MachineType,
			PrivateIPv4Addresses: ccExpect.PrivateIPv4Addresses,
			PrivateIPv6Addresses: ccExpect.PrivateIPv6Addresses,
			PublicIPv4Addresses:  ccExpect.PublicIPv4Addresses,
			PublicIPv6Addresses:  ccExpect.PublicIPv6Addresses,
			Status:               dbHost.Status,
		},
		Extension: extension,
	}
}
