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

package billprepaid

import (
	"strings"

	asbill "hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	billcore "hcm/pkg/api/core/bill"
	dataservice "hcm/pkg/api/data-service"
	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/slice"
)

// DeleteBillPrepaidItem 按订单年月批量删除预付费账单及其派生调账。
func (b *billPrepaidSvc) DeleteBillPrepaidItem(cts *rest.Contexts) (interface{}, error) {
	req := new(asbill.PrepaidItemDeleteReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	resp, err := b.doDeletePrepaidItem(cts.Kit, req)
	if err != nil {
		logs.Errorf("delete prepaid item failed, err: %v, order: %d-%02d, rid: %s",
			err, req.OrderYear, req.OrderMonth, cts.Kit.Rid)
		return nil, err
	}

	logs.Infof("delete prepaid item success, order: %d-%02d, deleted_count: %d, rid: %s",
		req.OrderYear, req.OrderMonth, resp.DeletedCount, cts.Kit.Rid)
	return resp, nil
}

// doDeletePrepaidItem 鉴权过滤 → 闸门判定 → 调用 data-service 级联删除。
func (b *billPrepaidSvc) doDeletePrepaidItem(kt *kit.Kit, req *asbill.PrepaidItemDeleteReq) (
	*asbill.PrepaidItemDeleteResp, error) {

	authFlt, authorized, err := b.buildPrepaidDeleteAuthFilter(kt)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return &asbill.PrepaidItemDeleteResp{}, nil
	}

	items, err := b.listPrepaidItemByOrderMonth(kt, req, authFlt)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return &asbill.PrepaidItemDeleteResp{}, nil
	}

	if err = b.checkDeleteGate(kt, items); err != nil {
		return nil, err
	}
	if err = b.deletePrepaidItems(kt, collectPrepaidIDs(items)); err != nil {
		return nil, err
	}

	return &asbill.PrepaidItemDeleteResp{DeletedCount: len(items)}, nil
}

// buildPrepaidDeleteAuthFilter 按预付费删除权限生成二级账号行过滤。
// authorized 为 false 时调用方应直接返回空结果，既不删全量也不报错。
func (b *billPrepaidSvc) buildPrepaidDeleteAuthFilter(kt *kit.Kit) (filter.RuleFactory, bool, error) {
	authRes := &meta.ListAuthResInput{Type: meta.AccountBillPrepaid, Action: meta.Delete}
	authInst, err := b.authorizer.ListAuthorizedInstances(kt, authRes)
	if err != nil {
		logs.Errorf("list authorized prepaid delete instances failed, err: %v, rid: %s", err, kt.Rid)
		return nil, false, err
	}

	authFlt, authorized := resolvePrepaidDeleteAuth(authInst.IsAny, authInst.IDs)
	return authFlt, authorized, nil
}

// resolvePrepaidDeleteAuth 按已授权实例三分支生成行过滤：全量 / 指定二级账号 / 无权限。
func resolvePrepaidDeleteAuth(isAny bool, ids []string) (filter.RuleFactory, bool) {
	if isAny {
		return nil, true
	}
	if len(ids) == 0 {
		return nil, false
	}

	return buildMainAccountInRule(ids), true
}

// buildDeleteReqFilter 按订单年月与可选的云二级账号 ID 列表生成删除筛选条件。
func buildDeleteReqFilter(req *asbill.PrepaidItemDeleteReq) *filter.Expression {
	expr := &filter.Expression{
		Op: filter.And,
		Rules: []filter.RuleFactory{
			tools.RuleEqual("order_year", req.OrderYear),
			tools.RuleEqual("order_month", req.OrderMonth),
		},
	}
	if len(req.MainAccountCloudIDs) != 0 {
		expr.Rules = append(expr.Rules, buildInRule("main_account_cloud_id", req.MainAccountCloudIDs))
	}

	return expr
}

// listPrepaidItemByOrderMonth 按订单年月分页列出已授权的预付费主单。
func (b *billPrepaidSvc) listPrepaidItemByOrderMonth(kt *kit.Kit, req *asbill.PrepaidItemDeleteReq,
	authFlt filter.RuleFactory) ([]*billcore.PrepaidItem, error) {

	flt := mergeAuthFilter(buildDeleteReqFilter(req), authFlt)

	items := make([]*billcore.PrepaidItem, 0)
	for start := uint32(0); ; start += uint32(core.DefaultMaxPageLimit) {
		listReq := &dsbill.PrepaidItemListReq{
			Filter: flt,
			Page:   &core.BasePage{Start: start, Limit: core.DefaultMaxPageLimit},
			Fields: []string{"id", "uuid", "settle_state"},
		}
		resp, err := b.client.DataService().Global.Bill.ListBillPrepaidItem(kt, listReq)
		if err != nil {
			logs.Errorf("list prepaid item to delete failed, err: %v, order: %d-%02d, rid: %s",
				err, req.OrderYear, req.OrderMonth, kt.Rid)
			return nil, err
		}
		items = append(items, resp.Details...)
		if len(resp.Details) < int(core.DefaultMaxPageLimit) {
			return items, nil
		}
	}
}

// checkDeleteGate 删除双闸门：已定账拒绝、存在推送中调账拒绝，两者均整批零变更。
func (b *billPrepaidSvc) checkDeleteGate(kt *kit.Kit, items []*billcore.PrepaidItem) error {
	if err := checkSettledPrepaidItem(items); err != nil {
		return err
	}

	adjs, err := b.listPrepaidAdjustment(kt, collectPrepaidIDs(items), "id", "source_id", "push_status")
	if err != nil {
		return err
	}

	return checkPushingPrepaidAdjustment(adjs)
}

// deletePrepaidItems 按已鉴权 ID 分片调用 data-service 级联删除。
func (b *billPrepaidSvc) deletePrepaidItems(kt *kit.Kit, ids []string) error {
	for _, batch := range slice.Split(ids, int(filter.DefaultMaxInLimit)) {
		delReq := &dataservice.BatchDeleteReq{Filter: tools.ContainersExpression("id", batch)}
		if err := b.client.DataService().Global.Bill.BatchDeleteBillPrepaidItem(kt, delReq); err != nil {
			logs.Errorf("batch delete prepaid item failed, err: %v, ids: %v, rid: %s", err, batch, kt.Rid)
			return err
		}
	}

	return nil
}

// checkSettledPrepaidItem 任一主单已定账则整批拒绝。
func checkSettledPrepaidItem(items []*billcore.PrepaidItem) error {
	rejected := collectRejectedPrepaidIDs(items, func(item *billcore.PrepaidItem) bool {
		return item.SettleState == enumor.BillSettleStateSettled
	})
	if len(rejected) == 0 {
		return nil
	}

	return errf.Newf(errf.Aborted,
		"prepaid item has been settled, please use a new uuid or order month, ids: %s",
		strings.Join(rejected, ","))
}

// checkPushingPrepaidAdjustment 任一关联调账处于推送中则整批拒绝。
func checkPushingPrepaidAdjustment(items []*billcore.AdjustmentItem) error {
	rejected := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range items {
		if item.PushStatus != enumor.BillAdjustmentPushStatusPushing {
			continue
		}
		if _, ok := seen[item.SourceID]; ok {
			continue
		}
		seen[item.SourceID] = struct{}{}
		rejected = append(rejected, item.SourceID)
	}
	if len(rejected) == 0 {
		return nil
	}

	return errf.Newf(errf.Aborted,
		"prepaid item is pushing to obs, please retry later, ids: %s", strings.Join(rejected, ","))
}

// collectPrepaidIDs 收集预付费主单 ID。
func collectPrepaidIDs(items []*billcore.PrepaidItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

// collectRejectedPrepaidIDs 收集命中拒绝条件的主单 ID。
func collectRejectedPrepaidIDs(items []*billcore.PrepaidItem, reject func(*billcore.PrepaidItem) bool) []string {
	rejected := make([]string, 0)
	for _, item := range items {
		if reject(item) {
			rejected = append(rejected, item.ID)
		}
	}
	return rejected
}
