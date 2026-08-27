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
	asbill "hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	billcore "hcm/pkg/api/core/bill"
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

// ListBillPrepaidItem 查询预付费账单列表。
func (b *billPrepaidSvc) ListBillPrepaidItem(cts *rest.Contexts) (interface{}, error) {
	req := new(asbill.PrepaidItemListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	authFlt, authorized, err := b.buildMainAccountAuthFilter(cts.Kit)
	if err != nil {
		return nil, err
	}
	// 无任何被授权实例时直接返回空列表，既不返回全量也不报错。
	if !authorized {
		return &asbill.PrepaidItemListResult{Details: make([]*asbill.PrepaidItemResult, 0)}, nil
	}

	listReq := &dsbill.PrepaidItemListReq{Filter: mergeAuthFilter(req.Filter, authFlt), Page: req.Page}
	resp, err := b.client.DataService().Global.Bill.ListBillPrepaidItem(cts.Kit, listReq)
	if err != nil {
		logs.Errorf("list prepaid item failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	if req.Page.Count {
		return &asbill.PrepaidItemListResult{Count: resp.Count}, nil
	}

	details, err := b.fillAccountingInfo(cts.Kit, resp.Details)
	if err != nil {
		return nil, err
	}

	return &asbill.PrepaidItemListResult{Details: details}, nil
}

// buildMainAccountAuthFilter 生成实例级行过滤规则，返回值 authorized 为 false 时调用方应直接返回空列表。
func (b *billPrepaidSvc) buildMainAccountAuthFilter(kt *kit.Kit) (filter.RuleFactory, bool, error) {
	// 查询侧不新增 action，统一复用二级账号查看权限的已授权实例。
	authRes := &meta.ListAuthResInput{Type: meta.MainAccount, Action: meta.Find}
	authInst, err := b.authorizer.ListAuthorizedInstances(kt, authRes)
	if err != nil {
		logs.Errorf("list authorized main account failed, err: %v, rid: %s", err, kt.Rid)
		return nil, false, err
	}

	if authInst.IsAny {
		return nil, true, nil
	}
	if len(authInst.IDs) == 0 {
		return nil, false, nil
	}

	return buildMainAccountInRule(authInst.IDs), true, nil
}

// buildMainAccountInRule 生成 main_account_id IN ids 的行过滤规则。
func buildMainAccountInRule(ids []string) filter.RuleFactory {
	return buildInRule("main_account_id", ids)
}

// buildInRule 生成 field IN ids 的行过滤规则。
// ids 可能超过单条 IN 上限，按 DefaultMaxInLimit 切分后再用 OR 组合。
func buildInRule(field string, ids []string) filter.RuleFactory {
	rules := make([]filter.RuleFactory, 0)
	for _, batch := range slice.Split(ids, int(filter.DefaultMaxInLimit)) {
		rules = append(rules, tools.RuleIn(field, batch))
	}

	return tools.CombineOrRules(rules)
}

// mergeAuthFilter 把调用方的过滤条件与实例级行过滤按 AND 组合。
// authRule 为 nil 表示调用方持有全量权限，此时不叠加任何账号过滤。
// 调用方表达式整体作为一条子规则挂入，避免其顶层 op 为 or 时把鉴权条件也纳入 or 而绕过行级过滤。
func mergeAuthFilter(reqFilter *filter.Expression, authRule filter.RuleFactory) *filter.Expression {
	if authRule == nil {
		return reqFilter
	}

	merged := &filter.Expression{Op: filter.And, Rules: []filter.RuleFactory{authRule}}
	if reqFilter != nil && !reqFilter.IsEmpty() {
		merged.Rules = append(merged.Rules, reqFilter)
	}

	return merged
}

// fillAccountingInfo 为每条主单派生核算状态与累计核算金额。
func (b *billPrepaidSvc) fillAccountingInfo(kt *kit.Kit, items []*billcore.PrepaidItem) (
	[]*asbill.PrepaidItemResult, error) {

	results := make([]*asbill.PrepaidItemResult, 0, len(items))
	if len(items) == 0 {
		return results, nil
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	statMap, err := b.statPrepaidAdjustment(kt, ids)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		stat := statMap[item.ID]
		if stat == nil {
			stat = new(prepaidAdjustmentStat)
		}
		results = append(results, &asbill.PrepaidItemResult{
			PrepaidItem:      item,
			AccountingState:  billcore.DerivePrepaidAccountingState(stat.PushedIncreaseNum, stat.IncreaseNum),
			AccountedCost:    stat.AccountedCost,
			AccountedRMBCost: stat.AccountedRMBCost,
		})
	}

	return results, nil
}

// listPrepaidAdjustment 拉取给定主单下的全部预付费派生调账。
func (b *billPrepaidSvc) listPrepaidAdjustment(kt *kit.Kit, sourceIDs []string, fields ...string) (
	[]*billcore.AdjustmentItem, error) {

	items := make([]*billcore.AdjustmentItem, 0)
	for _, batch := range slice.Split(sourceIDs, int(filter.DefaultMaxInLimit)) {
		part, err := b.listPrepaidAdjustmentBatch(kt, batch, fields...)
		if err != nil {
			return nil, err
		}
		items = append(items, part...)
	}

	return items, nil
}

// listPrepaidAdjustmentBatch 按一批不超过 IN 上限的主单 ID 分页拉取调账。
func (b *billPrepaidSvc) listPrepaidAdjustmentBatch(kt *kit.Kit, sourceIDs []string, fields ...string) (
	[]*billcore.AdjustmentItem, error) {

	items := make([]*billcore.AdjustmentItem, 0)
	for start := uint32(0); ; start += uint32(core.DefaultMaxPageLimit) {
		listReq := &dsbill.BillAdjustmentItemListReq{
			Filter: tools.ExpressionAnd(
				tools.RuleEqual("source", enumor.BillAdjustmentSourcePrepaid),
				tools.RuleIn("source_id", sourceIDs),
			),
			Page:   &core.BasePage{Start: start, Limit: core.DefaultMaxPageLimit},
			Fields: fields,
		}
		resp, err := b.client.DataService().Global.Bill.ListBillAdjustmentItem(kt, listReq)
		if err != nil {
			logs.Errorf("list prepaid adjustment failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		items = append(items, resp.Details...)
		if len(resp.Details) < int(core.DefaultMaxPageLimit) {
			break
		}
	}

	return items, nil
}
