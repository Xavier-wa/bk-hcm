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
	"strings"

	"hcm/cmd/hc-service/logics/res-sync/common"
	"hcm/cmd/hc-service/logics/res-sync/tcloud"
	adcore "hcm/pkg/adaptor/types/core"
	typescvm "hcm/pkg/adaptor/types/cvm"
	"hcm/pkg/api/core"
	"hcm/pkg/api/core/cloud/cvm"
	"hcm/pkg/api/data-service/cloud"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/tools/assert"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"
)

// Host 对比从cc获取的主机，与本地的主机的差异，进行本地主机的新增、更新、删除操作
func (cli *client) Host(kt *kit.Kit, params *SyncHostParams) (*SyncResult, error) {
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

	cloudHosts, err := cli.getCloudHost(kt, params.AccountID, ccHosts)
	if err != nil {
		logs.Errorf("get cloud host failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	// 同时按 bk_host_id 和 cloud_id 两个维度预查 db 存量主机：当前物理机的cloud_id用的是机器的固资号，
	// cc 主机重建会导致 bk_host_id 变化，但 cloud_id 不变，只按 bk_host_id 查会漏掉旧记录，使其在 diff 时被误判为新增，
	// 从而触发 cloud_id + vendor 唯一键冲突
	dbHosts, err := cli.listHostFromDBForDiff(kt, params.HostIDs, cloudHosts)
	if err != nil {
		logs.Errorf("list host from db failed, err: %v, hostIDs: %v, rid: %s", err, params.HostIDs, kt.Rid)
		return nil, err
	}

	if len(cloudHosts) == 0 && len(dbHosts) == 0 {
		return new(SyncResult), nil
	}

	addSlice, updateMap, delCloudIDs := common.Diff[cvm.Cvm[cvm.TCloudZiyanHostExtension],
		cvm.Cvm[cvm.TCloudZiyanHostExtension]](cloudHosts, dbHosts, isHostChange)

	if len(delCloudIDs) > 0 {
		if err = cli.deleteHost(kt, delCloudIDs); err != nil {
			return nil, err
		}
	}

	if len(updateMap) > 0 {
		if err = cli.updateHost(kt, convToUpdate(updateMap)); err != nil {
			return nil, err
		}
	}

	if len(addSlice) > 0 {
		if err = cli.createHost(kt, convToCreate(addSlice)); err != nil {
			return nil, err
		}
	}

	return new(SyncResult), nil

}

func (cli *client) getCloudHost(kt *kit.Kit, accountID string, ccHosts []cmdb.Host) (
	[]cvm.Cvm[cvm.TCloudZiyanHostExtension], error) {

	if len(ccHosts) == 0 {
		return make([]cvm.Cvm[cvm.TCloudZiyanHostExtension], 0), nil
	}

	hostIDs := make([]int64, 0, len(ccHosts))
	for _, host := range ccHosts {
		hostIDs = append(hostIDs, host.BkHostID)
	}
	hostBizIDMap, err := cli.getHostBizID(kt, hostIDs)
	if err != nil {
		logs.Errorf("get host biz id failed, err: %v, host ids: %v, rid: %s", err, hostIDs, kt.Rid)
		return nil, err
	}

	hostMap := make(map[string]cvm.Cvm[cvm.TCloudZiyanHostExtension])
	regionCloudIDMap := make(map[string][]string)
	for i := range ccHosts {
		ccHost := ccHosts[i]
		// cc中可能会存在连固资号都没有的脏数据，这里需要把他们剔除掉
		if ccHost.BkCloudInstID == "" && ccHost.BkAssetID == "" {
			logs.Errorf("host(%d) asset id is invalid, rid: %s", ccHost.BkHostID, kt.Rid)
			continue
		}

		bizID, ok := hostBizIDMap[ccHost.BkHostID]
		if !ok {
			logs.Errorf("can not find host(%d) biz id, rid: %s", ccHost.BkHostID, kt.Rid)
			continue
		}

		host := convertToHost(&ccHost, accountID, bizID)
		hostMap[host.CloudID] = host

		if ccHost.SvrSourceTypeID != cmdb.SvrSourceTypeIDCVM {
			continue
		}

		if ccHost.BkCloudRegion == "" {
			logs.Warnf("host id(%d) region data is nil, rid: %s", ccHost.BkHostID, kt.Rid)
			continue
		}

		if _, ok := regionCloudIDMap[host.Region]; !ok {
			regionCloudIDMap[host.Region] = make([]string, 0)
		}

		regionCloudIDMap[host.Region] = append(regionCloudIDMap[host.Region], host.CloudID)
	}

	return cli.fillCloudFields(kt, accountID, regionCloudIDMap, hostMap)

}

func (cli *client) getHostBizID(kt *kit.Kit, hostIDs []int64) (map[int64]int64, error) {
	if len(hostIDs) == 0 {
		return make(map[int64]int64), nil
	}

	hostBizIDMap := make(map[int64]int64)
	for _, batch := range slice.Split(hostIDs, int(core.DefaultMaxPageLimit)) {
		req := &cmdb.HostModuleRelationParams{HostID: batch}
		relationRes, err := cmdb.CmdbClient().FindHostBizRelations(kt, req)
		if err != nil {
			logs.Errorf("fail to find cmdb topo relation, err: %v, req: %+v, rid: %s", err, req, kt.Rid)
			return nil, err
		}

		for _, relation := range *relationRes {
			hostBizIDMap[relation.HostID] = relation.BizID
		}
	}

	return hostBizIDMap, nil
}

func (cli *client) fillCloudFields(kt *kit.Kit, accountID string, regionCloudIDMap map[string][]string,
	hostMap map[string]cvm.Cvm[cvm.TCloudZiyanHostExtension]) ([]cvm.Cvm[cvm.TCloudZiyanHostExtension], error) {

	for region, cloudIDs := range regionCloudIDMap {
		cloudVpcIDs := make([]string, 0)
		cloudSubnetIDs := make([]string, 0)
		cvms := make([]typescvm.TCloudCvm, 0)

		for _, batch := range slice.Split(cloudIDs, adcore.TCloudQueryLimit) {
			opt := &typescvm.TCloudListOption{
				Region:   region,
				CloudIDs: batch,
				Page:     &adcore.TCloudPage{Offset: 0, Limit: adcore.TCloudQueryLimit},
			}
			res, err := cli.cloudCli.ListCvm(kt, opt)
			if err != nil {
				logs.Errorf("[%s] list cvm from cloud failed, err: %v, account: %s, opt: %v, rid: %s",
					enumor.TCloudZiyan, err, accountID, opt, kt.Rid)
				return nil, err
			}
			cvms = append(cvms, res...)

			for _, one := range res {
				cloudVpcIDs = append(cloudVpcIDs, converter.PtrToVal(one.VirtualPrivateCloud.VpcId))
				cloudSubnetIDs = append(cloudSubnetIDs, converter.PtrToVal(one.VirtualPrivateCloud.SubnetId))
			}
		}

		vpcMap, err := cli.getVpcMap(kt, accountID, region, cloudVpcIDs)
		if err != nil {
			return nil, err
		}

		subnetMap, err := cli.getSubnetMap(kt, accountID, region, cloudSubnetIDs)
		if err != nil {
			return nil, err
		}

		for _, one := range cvms {
			if _, exsit := vpcMap[converter.PtrToVal(one.VirtualPrivateCloud.VpcId)]; !exsit {
				return nil, fmt.Errorf("cvm %s can not find vpc", converter.PtrToVal(one.InstanceId))
			}

			if _, exsit := subnetMap[converter.PtrToVal(one.VirtualPrivateCloud.SubnetId)]; !exsit {
				return nil, fmt.Errorf("cvm %s can not find subnet", converter.PtrToVal(one.InstanceId))
			}

			cloudID := converter.PtrToVal(one.InstanceId)
			host, ok := hostMap[cloudID]
			if !ok {
				logs.Errorf("host is not exist, cloud id: %s, rid: %s", cloudID, kt.Rid)
				continue
			}

			// 补充云上字段
			host.Name = converter.PtrToVal(one.InstanceName)
			host.Zone = converter.PtrToVal(one.Placement.Zone)
			host.ImageID = converter.PtrToVal(one.ImageId)
			host.Status = converter.PtrToVal(one.InstanceState)
			host.CloudExpiredTime = converter.PtrToVal(one.ExpiredTime)
			host.CloudVpcIDs = []string{converter.PtrToVal(one.VirtualPrivateCloud.VpcId)}
			host.VpcIDs = []string{vpcMap[converter.PtrToVal(one.VirtualPrivateCloud.VpcId)].VpcID}
			host.CloudSubnetIDs = []string{converter.PtrToVal(one.VirtualPrivateCloud.SubnetId)}
			host.SubnetIDs = []string{subnetMap[converter.PtrToVal(one.VirtualPrivateCloud.SubnetId)]}
			host.CloudCreatedTime = converter.PtrToVal(one.CreatedTime)
			if host.Extension == nil {
				host.Extension = &cvm.TCloudZiyanHostExtension{}
			}
			host.Extension.TCloudCvmExtension = tcloud.BuildCVMExtension(one)

			hostMap[cloudID] = host
		}
	}

	res := make([]cvm.Cvm[cvm.TCloudZiyanHostExtension], 0)
	for _, host := range hostMap {
		res = append(res, host)
	}

	return res, nil
}

func convertToHost(ccHost *cmdb.Host, accountID string, bizID int64) cvm.Cvm[cvm.TCloudZiyanHostExtension] {
	cloudID := ccHost.BkCloudInstID
	// 当主机不存在bk_cloud_inst_id时，需要用固资号进行填充，保证cloud id唯一
	if cloudID == "" {
		cloudID = ccHost.BkAssetID
	}

	innerIpv4 := make([]string, 0)
	if len(ccHost.BkHostInnerIP) != 0 {
		innerIpv4 = splitIP(ccHost.BkHostInnerIP)
	}
	innerIpv6 := make([]string, 0)
	if len(ccHost.BkHostInnerIPv6) != 0 {
		innerIpv6 = splitIP(ccHost.BkHostInnerIPv6)
	}
	outerIpv4 := make([]string, 0)
	if len(ccHost.BkHostOuterIP) != 0 {
		outerIpv4 = splitIP(ccHost.BkHostOuterIP)
	}
	outerIpv6 := make([]string, 0)
	if len(ccHost.BkHostOuterIPv6) != 0 {
		outerIpv6 = splitIP(ccHost.BkHostOuterIPv6)
	}

	host := cvm.Cvm[cvm.TCloudZiyanHostExtension]{
		BaseCvm: cvm.BaseCvm{
			CloudID:              cloudID,
			Name:                 ccHost.BkHostName,
			BkBizID:              bizID,
			BkHostID:             ccHost.BkHostID,
			BkCloudID:            ccHost.BkCloudID,
			BkAssetID:            ccHost.BkAssetID,
			AccountID:            accountID,
			Region:               ccHost.BkCloudRegion,
			Zone:                 ccHost.BkCloudZone,
			CloudVpcIDs:          []string{ccHost.BkCloudVpcID},
			CloudSubnetIDs:       []string{ccHost.BkCloudSubnetID},
			OsName:               ccHost.BkOSName,
			PrivateIPv4Addresses: innerIpv4,
			PrivateIPv6Addresses: innerIpv6,
			PublicIPv4Addresses:  outerIpv4,
			PublicIPv6Addresses:  outerIpv6,
			MachineType:          ccHost.SvrDeviceClassName,
		},
		Extension: &cvm.TCloudZiyanHostExtension{
			HostName:        ccHost.BkHostName,
			SvrSourceTypeID: ccHost.SvrSourceTypeID,
			SrvStatus:       ccHost.SrvStatus,
			SvrDeviceClass:  ccHost.SvrDeviceClass,
			BkDisk:          ccHost.BkDisk,
			BkCpu:           ccHost.BkCpu,
			BkOSName:        ccHost.BkOSName,
			Operator:        ccHost.Operator,
			BkBakOperator:   ccHost.BkBakOperator,
		},
	}

	return host
}

func splitIP(ip string) []string {
	return strings.Split(ip, ",")
}

func isHostChange(cloud cvm.Cvm[cvm.TCloudZiyanHostExtension], db cvm.Cvm[cvm.TCloudZiyanHostExtension]) bool {
	if db.BkBizID != cloud.BkBizID {
		return true
	}

	if db.BkHostID != cloud.BkHostID {
		return true
	}

	if db.Region != cloud.Region {
		return true
	}

	if db.Zone != cloud.Zone {
		return true
	}

	if db.AccountID != cloud.AccountID {
		return true
	}

	if db.CloudID != cloud.CloudID {
		return true
	}

	if db.Name != cloud.Name {
		return true
	}

	if db.BkAssetID != cloud.BkAssetID {
		return true
	}

	if !assert.IsStringSliceEqual(db.CloudVpcIDs, cloud.CloudVpcIDs) {
		return true
	}
	if !assert.IsStringSliceEqual(db.VpcIDs, cloud.VpcIDs) {
		return true
	}

	if !assert.IsStringSliceEqual(db.CloudSubnetIDs, cloud.CloudSubnetIDs) {
		return true
	}
	if !assert.IsStringSliceEqual(db.SubnetIDs, cloud.SubnetIDs) {
		return true
	}

	if db.CloudImageID != cloud.CloudImageID {
		return true
	}

	if db.OsName != cloud.OsName {
		return true
	}

	if db.Status != cloud.Status {
		return true
	}

	if db.MachineType != cloud.MachineType {
		return true
	}

	if db.CloudCreatedTime != cloud.CloudCreatedTime {
		return true
	}

	if db.CloudExpiredTime != cloud.CloudExpiredTime {
		return true
	}

	if isHostIPChange(db, cloud) {
		return true
	}

	if isHostExtensionChange(db, cloud) {
		return true
	}

	return false
}

func isHostIPChange(cloud cvm.Cvm[cvm.TCloudZiyanHostExtension], db cvm.Cvm[cvm.TCloudZiyanHostExtension]) bool {
	if !assert.IsStringSliceEqual(db.PrivateIPv4Addresses, cloud.PrivateIPv4Addresses) {
		return true
	}

	if !assert.IsStringSliceEqual(db.PublicIPv4Addresses, cloud.PublicIPv4Addresses) {
		return true
	}

	if !assert.IsStringSliceEqual(db.PrivateIPv6Addresses, cloud.PrivateIPv6Addresses) {
		return true
	}

	if !assert.IsStringSliceEqual(db.PublicIPv6Addresses, cloud.PublicIPv6Addresses) {
		return true
	}

	return false
}

func isHostExtensionChange(cloud cvm.Cvm[cvm.TCloudZiyanHostExtension], db cvm.Cvm[cvm.TCloudZiyanHostExtension]) bool {
	if db.Extension == nil || cloud.Extension == nil {
		return true
	}

	if db.Extension.HostName != cloud.Extension.HostName {
		return true
	}

	if db.Extension.SvrSourceTypeID != cloud.Extension.SvrSourceTypeID {
		return true
	}

	if db.Extension.SrvStatus != cloud.Extension.SrvStatus {
		return true
	}

	if db.BkAssetID != cloud.BkAssetID {
		return true
	}

	if db.Extension.SvrDeviceClass != cloud.Extension.SvrDeviceClass {
		return true
	}

	if db.Extension.BkCpu != cloud.Extension.BkCpu {
		return true
	}

	if db.Extension.BkDisk != cloud.Extension.BkDisk {
		return true
	}

	if db.Extension.BkOSName != cloud.Extension.BkOSName {
		return true
	}

	if db.Extension.Operator != cloud.Extension.Operator {
		return true
	}

	if db.Extension.BkBakOperator != cloud.Extension.BkBakOperator {
		return true
	}

	if tcloud.IsCvmExtensionChange(cloud.Extension.TCloudCvmExtension, db.Extension.TCloudCvmExtension) {
		return true
	}

	return false
}

// RemoveHostFromCC 对比根据的主机，删除本地多余的主机
func (cli *client) RemoveHostFromCC(kt *kit.Kit, params *DelHostParams) error {
	if params == nil {
		logs.Errorf("params is nil, rid: %s", kt.Rid)
		return fmt.Errorf("params is nil")
	}

	if err := params.Validate(); err != nil {
		logs.Errorf("param is invalid, err: %v, rid: %s", err, kt.Rid)
		return errf.NewFromErr(errf.InvalidParameter, err)
	}

	if len(params.DelHostIDs) != 0 {
		return cli.deleteHostByHostID(kt, params.DelHostIDs)
	}

	return cli.removeBizHost(kt, params.BizID, params.CCBizExistHostIDs)
}

func (cli *client) deleteHostByHostID(kt *kit.Kit, hostIDs []int64) error {
	if len(hostIDs) == 0 {
		return nil
	}

	for _, batch := range slice.Split(hostIDs, constant.BatchOperationMaxLimit) {
		deleteReq := &cloud.CvmBatchDeleteReq{Filter: tools.ExpressionAnd(tools.RuleEqual("vendor", enumor.TCloudZiyan),
			tools.RuleIn("bk_host_id", batch))}

		if err := cli.dbCli.Global.Cvm.BatchDeleteCvm(kt.Ctx, kt.Header(), deleteReq); err != nil {
			logs.Errorf("[%s] request dataservice to batch delete host failed, err: %v, req: %+v, rid: %s",
				enumor.TCloudZiyan, err, deleteReq, kt.Rid)
			return err
		}
	}

	return nil
}

func (cli *client) removeBizHost(kt *kit.Kit, bizID int64, ccBizExistHostIDs map[int64]struct{}) error {
	if len(ccBizExistHostIDs) == 0 {
		ccHosts, err := cli.getHostFromCCByBizID(kt, bizID, []string{"bk_host_id"})
		if err != nil {
			logs.Errorf("get host from cc failed, err: %v, bizID: %d, rid: %s", err, bizID, kt.Rid)
			return err
		}

		for _, host := range ccHosts {
			ccBizExistHostIDs[host.BkHostID] = struct{}{}
		}
	}

	dbHosts, err := cli.listHostFromDBByBizID(kt, bizID, []string{"id", "bk_host_id"})
	if err != nil {
		logs.Errorf("list host from db failed, err: %v, bizID: %d, rid: %s", err, bizID, kt.Rid)
		return err
	}

	delHostIDs := make([]int64, 0)
	for _, host := range dbHosts {
		if _, ok := ccBizExistHostIDs[host.BkHostID]; !ok {
			delHostIDs = append(delHostIDs, host.BkHostID)
		}
	}

	if err = cli.deleteHostByHostID(kt, delHostIDs); err != nil {
		logs.Errorf("delete host by host id failed, err: %v, ids: %v, rid: %s", err, delHostIDs, kt.Rid)
		return err
	}

	return nil
}

func (cli *client) getHostFromCCByBizID(kt *kit.Kit, bizID int64, fields []string) ([]cmdb.Host, error) {
	params := &cmdb.ListBizHostParams{
		BizID:  bizID,
		Fields: fields,
		Page:   &cmdb.BasePage{Start: 0, Limit: int64(core.DefaultMaxPageLimit), Sort: "bk_host_id"},
		HostPropertyFilter: &cmdb.QueryFilter{
			Rule: &cmdb.CombinedRule{
				Condition: "AND",
				Rules:     []cmdb.Rule{&cmdb.AtomRule{Field: "bk_cloud_id", Operator: "equal", Value: 0}},
			},
		},
	}

	return cli.getBizHostFromCC(kt, params)
}

func (cli *client) getHostFromCCByHostIDs(kt *kit.Kit, hostIDs []int64, fields []string) ([]cmdb.Host, error) {
	res := make([]cmdb.Host, 0)
	for _, batch := range slice.Split(hostIDs, int(core.DefaultMaxPageLimit)) {
		params := &cmdb.ListHostReq{
			Fields: fields,
			Page:   cmdb.BasePage{Start: 0, Limit: int64(core.DefaultMaxPageLimit)},
			HostPropertyFilter: &cmdb.QueryFilter{
				Rule: &cmdb.CombinedRule{
					Condition: "AND",
					Rules: []cmdb.Rule{
						&cmdb.AtomRule{Field: "bk_cloud_id", Operator: "equal", Value: 0},
						&cmdb.AtomRule{Field: "bk_host_id", Operator: "in", Value: batch},
					},
				},
			},
		}

		resp, err := cmdb.CmdbClient().ListHost(kt, params)
		if err != nil {
			logs.Errorf("get host from cc failed, err: %v, params: %+v, rid: %s", err, params, kt.Rid)
			return nil, err
		}

		for _, host := range resp.Info {
			res = append(res, host)
		}
	}

	return res, nil
}

func (cli *client) getBizHostFromCCByHostIDs(kt *kit.Kit, bizID int64, hostIDs []int64, fields []string) ([]cmdb.Host,
	error) {

	res := make([]cmdb.Host, 0)
	for _, batch := range slice.Split(hostIDs, int(core.DefaultMaxPageLimit)) {
		params := &cmdb.ListBizHostParams{
			BizID:  bizID,
			Fields: fields,
			Page:   &cmdb.BasePage{Start: 0, Limit: int64(core.DefaultMaxPageLimit), Sort: "bk_host_id"},
			HostPropertyFilter: &cmdb.QueryFilter{
				Rule: &cmdb.CombinedRule{
					Condition: "AND",
					Rules: []cmdb.Rule{
						&cmdb.AtomRule{Field: "bk_cloud_id", Operator: "equal", Value: 0},
						&cmdb.AtomRule{Field: "bk_host_id", Operator: "in", Value: batch},
					},
				},
			},
		}

		hosts, err := cli.getBizHostFromCC(kt, params)
		if err != nil {
			logs.Errorf("get host from cc failed, err: %v, params: %+v, rid: %s", err, params, kt.Rid)
			return nil, err
		}
		res = append(res, hosts...)
	}

	return res, nil
}

func (cli *client) getBizHostFromCC(kt *kit.Kit, params *cmdb.ListBizHostParams) ([]cmdb.Host, error) {
	hosts := make([]cmdb.Host, 0)
	for {
		result, err := cmdb.CmdbClient().ListBizHost(kt, params)
		if err != nil {
			logs.Errorf("call cmdb to list biz host failed, err: %v, req: %+v, rid: %s", err, params, kt.Rid)
			return nil, err
		}

		hosts = append(hosts, result.Info...)

		if len(result.Info) < int(core.DefaultMaxPageLimit) {
			break
		}

		params.Page.Start += int64(core.DefaultMaxPageLimit)
	}

	return hosts, nil
}

func (cli *client) listHostFromDBByBizID(kt *kit.Kit, bizID int64,
	fields []string) ([]cvm.Cvm[cvm.TCloudZiyanHostExtension], error) {

	req := &cloud.CvmListReq{
		Field:  fields,
		Filter: tools.ExpressionAnd(tools.RuleEqual("vendor", enumor.TCloudZiyan), tools.RuleEqual("bk_biz_id", bizID)),
		Page: &core.BasePage{
			Start: 0,
			Limit: core.DefaultMaxPageLimit,
			Sort:  "id",
		},
	}

	return cli.listHostFromDB(kt, req)
}

func (cli *client) listHostFromDBByHostIDs(kt *kit.Kit, hostIDs []int64) ([]cvm.Cvm[cvm.TCloudZiyanHostExtension],
	error) {

	res := make([]cvm.Cvm[cvm.TCloudZiyanHostExtension], 0)
	for _, batch := range slice.Split(hostIDs, constant.BatchOperationMaxLimit) {
		req := &cloud.CvmListReq{
			Filter: tools.ExpressionAnd(tools.RuleEqual("vendor", enumor.TCloudZiyan),
				tools.RuleIn("bk_host_id", batch)),
			Page: &core.BasePage{
				Start: 0,
				Limit: constant.BatchOperationMaxLimit,
				Sort:  "id",
			},
		}
		hosts, err := cli.listHostFromDB(kt, req)
		if err != nil {
			logs.Errorf("list host from db failed ,err: %v, req: %+v, rid: %s", err, req, kt.Rid)
			return nil, err
		}

		res = append(res, hosts...)
	}

	return res, nil
}

func (cli *client) listHostFromDBByCloudIDs(kt *kit.Kit, cloudIDs []string) (
	[]cvm.Cvm[cvm.TCloudZiyanHostExtension], error) {

	res := make([]cvm.Cvm[cvm.TCloudZiyanHostExtension], 0)
	for _, batch := range slice.Split(cloudIDs, constant.BatchOperationMaxLimit) {
		req := &cloud.CvmListReq{
			Filter: tools.ExpressionAnd(tools.RuleEqual("vendor", enumor.TCloudZiyan),
				tools.RuleIn("cloud_id", batch)),
			Page: &core.BasePage{
				Start: 0,
				Limit: constant.BatchOperationMaxLimit,
				Sort:  "id",
			},
		}
		hosts, err := cli.listHostFromDB(kt, req)
		if err != nil {
			logs.Errorf("list host from db by cloud ids failed, err: %v, req: %+v, rid: %s", err, req, kt.Rid)
			return nil, err
		}

		res = append(res, hosts...)
	}

	return res, nil
}

func (cli *client) listHostFromDBForDiff(kt *kit.Kit, hostIDs []int64,
	cloudHosts []cvm.Cvm[cvm.TCloudZiyanHostExtension]) ([]cvm.Cvm[cvm.TCloudZiyanHostExtension], error) {

	dbHosts, err := cli.listHostFromDBByHostIDs(kt, hostIDs)
	if err != nil {
		logs.Errorf("list host from db by host ids failed, err: %v, hostIDs: %v, rid: %s", err, hostIDs, kt.Rid)
		return nil, err
	}

	cloudIDs := make([]string, 0, len(cloudHosts))
	for i := range cloudHosts {
		cloudIDs = append(cloudIDs, cloudHosts[i].CloudID)
	}
	if len(cloudIDs) == 0 {
		return dbHosts, nil
	}

	dbHostsByCloudID, err := cli.listHostFromDBByCloudIDs(kt, cloudIDs)
	if err != nil {
		logs.Errorf("list host from db by cloud ids failed, err: %v, cloudIDs: %v, rid: %s", err, cloudIDs, kt.Rid)
		return nil, err
	}

	existIDs := make(map[string]struct{}, len(dbHosts))
	for i := range dbHosts {
		existIDs[dbHosts[i].GetID()] = struct{}{}
	}
	for i := range dbHostsByCloudID {
		if _, ok := existIDs[dbHostsByCloudID[i].GetID()]; ok {
			continue
		}
		existIDs[dbHostsByCloudID[i].GetID()] = struct{}{}
		dbHosts = append(dbHosts, dbHostsByCloudID[i])
	}

	return dbHosts, nil
}

// listHostFromDB 从db中查询主机
func (cli *client) listHostFromDB(kt *kit.Kit, req *cloud.CvmListReq) ([]cvm.Cvm[cvm.TCloudZiyanHostExtension], error) {
	hosts := make([]cvm.Cvm[cvm.TCloudZiyanHostExtension], 0)
	for {
		result, err := cli.dbCli.TCloudZiyan.Cvm.ListCvmExt(kt.Ctx, kt.Header(), req)
		if err != nil {
			logs.ErrorJson("[%s] request dataservice to list cvm failed, err: %v, req: %v, rid: %s", enumor.TCloudZiyan,
				err, req, kt.Rid)
			return nil, err
		}

		hosts = append(hosts, result.Details...)

		if len(result.Details) < int(core.DefaultMaxPageLimit) {
			break
		}

		req.Page.Start += uint32(core.DefaultMaxPageLimit)
	}

	return hosts, nil
}

func (cli *client) deleteHost(kt *kit.Kit, cloudIDs []string) error {
	if len(cloudIDs) <= 0 {
		return nil
	}

	for _, batch := range slice.Split(cloudIDs, constant.BatchOperationMaxLimit) {
		deleteReq := &cloud.CvmBatchDeleteReq{
			Filter: tools.ExpressionAnd(tools.RuleIn("cloud_id", batch), tools.RuleEqual("vendor", enumor.TCloudZiyan)),
		}
		if err := cli.dbCli.Global.Cvm.BatchDeleteCvm(kt.Ctx, kt.Header(), deleteReq); err != nil {
			logs.Errorf("[%s] request dataservice to batch delete host failed, err: %v, req: %+v, rid: %s",
				enumor.TCloudZiyan, err, deleteReq, kt.Rid)
			return err
		}

		logs.Infof("[%s] sync host to delete host success, count: %d, cloudIDs: %+v, rid: %s", enumor.TCloudZiyan,
			len(batch), batch, kt.Rid)
	}

	return nil
}

func convToCreate(hosts []cvm.Cvm[cvm.TCloudZiyanHostExtension]) []cloud.CvmBatchCreate[cvm.TCloudZiyanHostExtension] {
	res := make([]cloud.CvmBatchCreate[cvm.TCloudZiyanHostExtension], 0)
	for _, host := range hosts {
		res = append(res, cloud.CvmBatchCreate[cvm.TCloudZiyanHostExtension]{
			CloudID:              host.CloudID,
			Name:                 host.Name,
			BkBizID:              host.BkBizID,
			BkHostID:             host.BkHostID,
			BkAssetID:            host.BkAssetID,
			BkCloudID:            host.BkCloudID,
			AccountID:            host.AccountID,
			Region:               host.Region,
			Zone:                 host.Zone,
			CloudVpcIDs:          host.CloudVpcIDs,
			VpcIDs:               host.VpcIDs,
			CloudSubnetIDs:       host.CloudSubnetIDs,
			SubnetIDs:            host.SubnetIDs,
			CloudImageID:         host.CloudImageID,
			ImageID:              host.ImageID,
			OsName:               host.OsName,
			Memo:                 host.Memo,
			Status:               host.Status,
			PrivateIPv4Addresses: host.PrivateIPv4Addresses,
			PrivateIPv6Addresses: host.PrivateIPv6Addresses,
			PublicIPv4Addresses:  host.PublicIPv4Addresses,
			PublicIPv6Addresses:  host.PublicIPv6Addresses,
			MachineType:          host.MachineType,
			CloudCreatedTime:     host.CloudCreatedTime,
			CloudLaunchedTime:    host.CloudLaunchedTime,
			CloudExpiredTime:     host.CloudExpiredTime,
			Extension:            host.Extension,
		})
	}

	return res
}

func (cli *client) createHost(kt *kit.Kit, hosts []cloud.CvmBatchCreate[cvm.TCloudZiyanHostExtension]) error {
	if len(hosts) == 0 {
		return nil
	}

	for _, batch := range slice.Split(hosts, constant.BatchOperationMaxLimit) {
		createReq := &cloud.CvmBatchCreateReq[cvm.TCloudZiyanHostExtension]{Cvms: batch}
		_, err := cli.dbCli.TCloudZiyan.Cvm.BatchCreateCvm(kt.Ctx, kt.Header(), createReq)
		if err != nil {
			logs.Errorf("create host failed, err: %v, req: %+v, rid: %s", err, createReq, kt.Rid)
			return err
		}
	}

	return nil
}

func convToUpdate(
	hosts map[string]cvm.Cvm[cvm.TCloudZiyanHostExtension],
) []cloud.CvmBatchUpdateWithExtension[cvm.TCloudZiyanHostExtension] {

	res := make([]cloud.CvmBatchUpdateWithExtension[cvm.TCloudZiyanHostExtension], 0)
	for id, host := range hosts {
		res = append(res, cloud.CvmBatchUpdateWithExtension[cvm.TCloudZiyanHostExtension]{
			CvmBatchUpdate: cloud.CvmBatchUpdate{
				ID:                   id,
				Name:                 host.Name,
				BkBizID:              host.BkBizID,
				BkHostID:             host.BkHostID,
				BkAssetID:            host.BkAssetID,
				BkCloudID:            &host.BkCloudID,
				Region:               host.Region,
				Zone:                 host.Zone,
				CloudVpcIDs:          host.CloudVpcIDs,
				VpcIDs:               host.VpcIDs,
				CloudSubnetIDs:       host.CloudSubnetIDs,
				SubnetIDs:            host.SubnetIDs,
				CloudImageID:         host.CloudImageID,
				ImageID:              host.ImageID,
				OsName:               host.OsName,
				Memo:                 host.Memo,
				Status:               host.Status,
				PrivateIPv4Addresses: host.PrivateIPv4Addresses,
				PrivateIPv6Addresses: host.PrivateIPv6Addresses,
				PublicIPv4Addresses:  host.PublicIPv4Addresses,
				PublicIPv6Addresses:  host.PublicIPv6Addresses,
				MachineType:          host.MachineType,
				CloudCreatedTime:     host.CloudCreatedTime,
				CloudLaunchedTime:    host.CloudLaunchedTime,
				CloudExpiredTime:     host.CloudExpiredTime,
			},
			Extension: host.Extension,
		})
	}

	return res
}

func (cli *client) updateHost(kt *kit.Kit,
	hosts []cloud.CvmBatchUpdateWithExtension[cvm.TCloudZiyanHostExtension]) error {
	if len(hosts) == 0 {
		return nil
	}

	for _, batch := range slice.Split(hosts, constant.BatchOperationMaxLimit) {
		updateReq := &cloud.CvmBatchUpdateReq[cvm.TCloudZiyanHostExtension]{Cvms: batch}
		if err := cli.dbCli.TCloudZiyan.Cvm.BatchUpdateCvm(kt.Ctx, kt.Header(), updateReq); err != nil {
			logs.Errorf("update host failed, err: %v, req: %+v, rid: %s", err, updateReq, kt.Rid)
			return err
		}
	}

	return nil
}
