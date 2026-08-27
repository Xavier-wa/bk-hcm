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
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"
)

// ListBillPrepaidSplitItem 查询预付费账单的整合明细。
func (b *billPrepaidSvc) ListBillPrepaidSplitItem(cts *rest.Contexts) (interface{}, error) {
	id := cts.PathParameter("id").String()
	if len(id) == 0 {
		return nil, errf.New(errf.InvalidParameter, "prepaid item id is required")
	}

	item, err := b.getAuthorizedPrepaidItem(cts.Kit, id)
	if err != nil {
		return nil, err
	}

	adjustments, err := b.listPrepaidAdjustment(cts.Kit, []string{item.ID})
	if err != nil {
		return nil, err
	}

	details := convSplitItems(adjustments)
	return &asbill.PrepaidSplitItemListResult{Count: uint64(len(details)), Details: details}, nil
}

// getAuthorizedPrepaidItem 查询主单并做实例级鉴权，越权或不存在统一返回 RecordNotFound 不泄露数据。
func (b *billPrepaidSvc) getAuthorizedPrepaidItem(kt *kit.Kit, id string) (*billcore.PrepaidItem, error) {
	authRule, authorized, err := b.buildMainAccountAuthFilter(kt)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, errf.Newf(errf.RecordNotFound, "prepaid item not found, id: %s", id)
	}

	flt := mergeAuthFilter(tools.ExpressionAnd(tools.RuleEqual("id", id)), authRule)

	listReq := &dsbill.PrepaidItemListReq{Filter: flt, Page: core.NewDefaultBasePage()}
	resp, err := b.client.DataService().Global.Bill.ListBillPrepaidItem(kt, listReq)
	if err != nil {
		logs.Errorf("get prepaid item failed, err: %v, id: %s, rid: %s", err, id, kt.Rid)
		return nil, err
	}
	if len(resp.Details) == 0 {
		return nil, errf.Newf(errf.RecordNotFound, "prepaid item not found, id: %s", id)
	}

	return resp.Details[0], nil
}

// convSplitItems 把调账条目转换为月度分摊行。
func convSplitItems(items []*billcore.AdjustmentItem) []*asbill.PrepaidSplitItemResult {
	details := make([]*asbill.PrepaidSplitItemResult, 0, len(items))
	for _, item := range items {
		details = append(details, &asbill.PrepaidSplitItemResult{
			AdjustmentID: item.ID,
			BillYear:     item.BillYear,
			BillMonth:    item.BillMonth,
			Accounted:    item.PushStatus == enumor.BillAdjustmentPushStatusPushed,
			Type:         item.Type,
			Cost:         item.Cost,
			RMBCost:      item.RMBCost,
			Currency:     item.Currency,
			ResClass:     item.ResClass,
			ResSubClass:  item.ResSubClass,
			PushStatus:   item.PushStatus,
			SettleState:  item.SettleState,
			Memo:         cvt.ValToPtr(item.Memo),
		})
	}

	return details
}
