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
)

// mappedMainAccount 云账号映射结果，承载写入域后续所需的账号维度信息。
type mappedMainAccount struct {
	// ID 二级账号 ID
	ID string
	// RootAccountID 一级账号 ID
	RootAccountID string
	// RootAccountCloudID 一级账号云上 ID
	RootAccountCloudID string
	// CloudID 二级账号云上 ID
	CloudID string
	// OpProductID 运营产品 ID
	OpProductID int64
}

// mapMainAccount 由 (cloud_id, vendor) 在调用方所属租户内唯一定位二级账号。
// 映射不到唯一账号时返回 RecordNotFound，且必须在鉴权之前返回。
func (b *billPrepaidSvc) mapMainAccount(kt *kit.Kit, item *asbill.PrepaidItemSyncItem) (*mappedMainAccount, error) {
	listReq := &core.ListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("cloud_id", item.MainAccountCloudID),
			tools.RuleEqual("vendor", item.Vendor),
		),
		Page: core.NewDefaultBasePage(),
	}
	resp, err := b.client.DataService().Global.MainAccount.List(kt, listReq)
	if err != nil {
		logs.Errorf("list main account for prepaid sync failed, err: %v, cloud_id: %s, vendor: %s, rid: %s",
			err, item.MainAccountCloudID, item.Vendor, kt.Rid)
		return nil, err
	}
	if len(resp.Details) != 1 {
		return nil, errf.Newf(errf.RecordNotFound,
			"main account not uniquely matched, cloud_id: %s, vendor: %s, tenant_id: %s, matched: %d",
			item.MainAccountCloudID, item.Vendor, kt.TenantID, len(resp.Details))
	}

	account := resp.Details[0]
	rootAccount, err := b.client.DataService().Global.RootAccount.GetBasicInfo(kt, account.ParentAccountID)
	if err != nil {
		logs.Errorf("get root account for prepaid sync failed, err: %v, root_account_id: %s, rid: %s",
			err, account.ParentAccountID, kt.Rid)
		return nil, err
	}
	if rootAccount == nil {
		logs.Errorf("root account not found, root_account_id: %s, rid: %s", account.ParentAccountID, kt.Rid)
		return nil, errf.Newf(errf.RecordNotFound, "root account not found, root_account_id: %s",
			account.ParentAccountID)
	}

	return &mappedMainAccount{
		ID:                 account.ID,
		RootAccountID:      account.ParentAccountID,
		RootAccountCloudID: rootAccount.CloudID,
		CloudID:            account.CloudID,
		OpProductID:        account.OpProductID,
	}, nil
}

// getSummaryRoot 取一级账号的账单汇总，用于币种一致性比对。
func (b *billPrepaidSvc) getSummaryRoot(kt *kit.Kit, rootAccountID string) (*billcore.SummaryRoot, error) {
	listReq := &dsbill.BillSummaryRootListReq{
		Filter: tools.ExpressionAnd(tools.RuleEqual("root_account_id", rootAccountID)),
		Page:   core.NewDefaultBasePage(),
	}
	resp, err := b.client.DataService().Global.Bill.ListBillSummaryRoot(kt, listReq)
	if err != nil {
		logs.Errorf("list summary root for prepaid sync failed, err: %v, root_account_id: %s, rid: %s",
			err, rootAccountID, kt.Rid)
		return nil, err
	}
	if len(resp.Details) == 0 {
		return nil, errf.Newf(errf.RecordNotFound, "summary root not found, root_account_id: %s", rootAccountID)
	}

	return resp.Details[0], nil
}

// getPrepaidItemByUniqueKey 按唯一键 uuid + 订单账期查询已存在的预付费主单。
func (b *billPrepaidSvc) getPrepaidItemByUniqueKey(kt *kit.Kit, item *asbill.PrepaidItemSyncItem) (
	*billcore.PrepaidItem, error) {

	listReq := &dsbill.PrepaidItemListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("uuid", item.UUID),
			tools.RuleEqual("order_year", item.OrderYear),
			tools.RuleEqual("order_month", item.OrderMonth),
		),
		Page: core.NewDefaultBasePage(),
	}
	resp, err := b.client.DataService().Global.Bill.ListBillPrepaidItem(kt, listReq)
	if err != nil {
		logs.Errorf("list prepaid item by unique key failed, err: %v, uuid: %s, rid: %s", err, item.UUID, kt.Rid)
		return nil, err
	}
	if len(resp.Details) == 0 {
		return nil, nil
	}

	return resp.Details[0], nil
}

// checkSyncGate 覆盖重推双闸门：已定账拒绝、存在推送中调账拒绝，两者均零变更返回。
func (b *billPrepaidSvc) checkSyncGate(kt *kit.Kit, exist *billcore.PrepaidItem) error {
	if exist == nil {
		return nil
	}

	if exist.SettleState == enumor.BillSettleStateSettled {
		return errf.Newf(errf.Aborted,
			"prepaid item has been settled, please use a new uuid or order month, id: %s, uuid: %s",
			exist.ID, exist.UUID)
	}

	listReq := &dsbill.BillAdjustmentItemListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("source", enumor.BillAdjustmentSourcePrepaid),
			tools.RuleEqual("source_id", exist.ID),
			tools.RuleEqual("push_status", enumor.BillAdjustmentPushStatusPushing),
		),
		Page: core.NewCountPage(),
	}
	resp, err := b.client.DataService().Global.Bill.ListBillAdjustmentItem(kt, listReq)
	if err != nil {
		logs.Errorf("count pushing adjustment failed, err: %v, prepaid_id: %s, rid: %s", err, exist.ID, kt.Rid)
		return err
	}
	if resp.Count > 0 {
		return errf.Newf(errf.Aborted,
			"prepaid item is pushing to obs, please retry later, id: %s, pushing_num: %d", exist.ID, resp.Count)
	}

	return nil
}
