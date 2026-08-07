/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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

package returner

import (
	"hcm/cmd/woa-server/dal/task/dao"
	"hcm/cmd/woa-server/dal/task/table"
	"hcm/pkg"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/tools/metadata"
	"hcm/pkg/tools/querybuilder"
	"hcm/pkg/tools/slice"
)

// RollbackReturnFailedHosts 将子单下退回失败的主机从回收中转池退回业务空闲机模块。
//
// 该方法幂等：已位于业务内置模块（空闲机/故障机/待回收）的主机视为已离开回收中转池，跳过且不计为失败。
// 回滚失败时返回错误，调用方据此阻断单据进入终止态。
func (r *Returner) RollbackReturnFailedHosts(kt *kit.Kit, order *table.RecycleOrder) error {
	hosts, err := r.getReturnFailedHosts(kt, order.SuborderID)
	if err != nil {
		return err
	}
	if len(hosts) == 0 {
		logs.Infof("no return failed host to rollback, orderID: %d, suborderID: %s, rid: %s",
			order.OrderID, order.SuborderID, kt.Rid)
		return nil
	}

	return r.rollbackHostsToBizIdle(kt, order, hosts)
}

// rollbackHostsToBizIdle 把给定主机退回业务空闲机模块
func (r *Returner) rollbackHostsToBizIdle(kt *kit.Kit, order *table.RecycleOrder,
	hosts []*table.RecycleHost) error {

	assetIDs := make([]string, 0, len(hosts))
	for _, host := range hosts {
		assetIDs = append(assetIDs, host.AssetID)
	}

	// 已在业务内置模块的主机说明已离开回收中转池，无需再次流转
	settled, err := r.listHostsInBizInternalModule(kt, order.BizID, assetIDs)
	if err != nil {
		logs.Errorf("failed to check host location before rollback, err: %v, orderID: %d, suborderID: %s, rid: %s",
			err, order.OrderID, order.SuborderID, kt.Rid)
		return err
	}

	pending := filterUnsettledAssets(assetIDs, settled)
	if len(pending) == 0 {
		logs.Infof("all return failed hosts already rolled back, orderID: %d, suborderID: %s, total: %d, rid: %s",
			order.OrderID, order.SuborderID, len(assetIDs), kt.Rid)
		return nil
	}

	if err = r.transferHost2BizIdle(kt, pending, order.BizID); err != nil {
		logs.Errorf("failed to rollback return failed hosts to idle module, err: %v, orderID: %d, "+
			"suborderID: %s, assetIDs: %v, rid: %s", err, order.OrderID, order.SuborderID, pending, kt.Rid)
		return err
	}

	logs.Infof("rollback return failed hosts success, orderID: %d, suborderID: %s, total: %d, skipped: %d, rid: %s",
		order.OrderID, order.SuborderID, len(assetIDs), len(assetIDs)-len(pending), kt.Rid)

	return nil
}

// filterUnsettledAssets 返回尚未落在业务内置模块的固资编号
func filterUnsettledAssets(assetIDs []string, settled map[string]struct{}) []string {
	unsettled := make([]string, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		if _, ok := settled[assetID]; !ok {
			unsettled = append(unsettled, assetID)
		}
	}

	return unsettled
}

// returnFailedHostFilter 构造子单下「退回失败」主机的查询条件。
// status 条件是按主机粒度回滚的关键，缺失会把已回收成功的主机一并纳入回滚范围。
func returnFailedHostFilter(suborderID string) map[string]interface{} {
	return map[string]interface{}{
		"suborder_id": suborderID,
		"status":      table.RecycleStatusReturnFailed,
	}
}

// getReturnFailedHosts 获取子单下状态为「退回失败」的回收主机。
// 同一子单内主机状态可能不一致（部分回收成功、部分失败），因此必须按主机粒度筛选。
func (r *Returner) getReturnFailedHosts(kt *kit.Kit, suborderID string) ([]*table.RecycleHost, error) {
	filter := returnFailedHostFilter(suborderID)

	recycleHosts := make([]*table.RecycleHost, 0)
	startIndex := 0
	for {
		page := metadata.BasePage{
			Start: startIndex,
			Limit: pkg.BKMaxInstanceLimit,
		}
		hosts, err := dao.Set().RecycleHost().FindManyRecycleHost(kt.Ctx, page, filter)
		if err != nil {
			logs.Errorf("failed to get return failed hosts, err: %v, suborderID: %s, rid: %s",
				err, suborderID, kt.Rid)
			return nil, err
		}
		recycleHosts = append(recycleHosts, hosts...)
		if len(hosts) < pkg.BKMaxInstanceLimit {
			break
		}
		startIndex += pkg.BKMaxInstanceLimit
	}

	return recycleHosts, nil
}

// listHostsInBizInternalModule 从给定固资编号中筛出已位于业务内置模块的主机
func (r *Returner) listHostsInBizInternalModule(kt *kit.Kit, bizID int64, assetIDs []string) (
	map[string]struct{}, error) {

	moduleIDs, err := r.cmdbCli.GetBizInternalModuleIDs(kt, bizID)
	if err != nil {
		logs.Errorf("failed to get biz internal module ids, err: %v, bizID: %d, rid: %s", err, bizID, kt.Rid)
		return nil, err
	}

	settled := make(map[string]struct{}, len(assetIDs))
	for _, batch := range slice.Split(assetIDs, pkg.BKMaxInstanceLimit) {
		req := &cmdb.ListBizHostParams{
			BizID:       bizID,
			BkModuleIDs: moduleIDs,
			HostPropertyFilter: &cmdb.QueryFilter{
				Rule: querybuilder.CombinedRule{
					Condition: querybuilder.ConditionAnd,
					Rules: []querybuilder.Rule{
						querybuilder.AtomRule{
							Field:    pkg.BKAssetIDField,
							Operator: querybuilder.OperatorIn,
							Value:    batch,
						},
					},
				},
			},
			Fields: []string{"bk_asset_id"},
			Page: &cmdb.BasePage{
				Start: 0,
				Limit: pkg.BKMaxInstanceLimit,
			},
		}

		resp, err := r.cmdbCli.ListBizHost(kt, req)
		if err != nil {
			logs.Errorf("failed to list biz host in internal module, err: %v, bizID: %d, rid: %s", err, bizID, kt.Rid)
			return nil, err
		}

		for _, host := range resp.Info {
			settled[host.BkAssetID] = struct{}{}
		}
	}

	return settled, nil
}
