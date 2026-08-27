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

package billprepaiditem

import (
	"hcm/pkg/api/core"
	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	tableaudit "hcm/pkg/dal/table/audit"
	tablebill "hcm/pkg/dal/table/bill"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"

	"github.com/jmoiron/sqlx"
)

// SyncBillPrepaidItem 在单事务内完成预付费主单与 N+1 调账的落库或覆盖重建。
func (svc *service) SyncBillPrepaidItem(cts *rest.Contexts) (interface{}, error) {
	req := new(dsbill.PrepaidItemSyncReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	exist, err := svc.getPrepaidItemByUniqueKey(cts.Kit, req.Item)
	if err != nil {
		return nil, err
	}

	result, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		return svc.syncWithTx(cts.Kit, txn, req, exist)
	})
	if err != nil {
		logs.Errorf("sync account bill prepaid item failed, err: %v, uuid: %s, order: %d-%02d, rid: %s",
			err, req.Item.UUID, req.Item.OrderYear, req.Item.OrderMonth, cts.Kit.Rid)
		return nil, err
	}

	return result, nil
}

// syncWithTx 事务内主流程：主单 upsert → 整组调账删除重建 → 审计留痕。
func (svc *service) syncWithTx(kt *kit.Kit, txn *sqlx.Tx, req *dsbill.PrepaidItemSyncReq,
	exist *tablebill.AccountBillPrepaidItem) (*dsbill.PrepaidItemSyncResult, error) {

	result := new(dsbill.PrepaidItemSyncResult)

	model := buildPrepaidItemModel(kt, req.Item)
	switch {
	case exist == nil:
		id, err := svc.dao.AccountBillPrepaidItem().CreateWithTx(kt, txn, model)
		if err != nil {
			return nil, err
		}
		result.ID, result.Created = id, true
	default:
		result.ID = exist.ID
		// 覆盖重推保持首次入库的定账态与创建人不变，只刷新业务字段。
		model.SettleState = exist.SettleState
		model.Creator = exist.Creator
		flt := tools.EqualExpression("id", exist.ID)
		if err := svc.dao.AccountBillPrepaidItem().UpdateWithTx(kt, txn, flt, model); err != nil {
			return nil, err
		}

		delIDs, err := svc.deleteAdjustmentsBySourceWithTx(kt, txn, exist.ID)
		if err != nil {
			return nil, err
		}
		result.DeletedAdjustmentIDs = delIDs
	}

	adjIDs, err := svc.createAdjustmentsWithTx(kt, txn, result.ID, req.AdjustmentItems)
	if err != nil {
		return nil, err
	}
	result.AdjustmentIDs = adjIDs

	if err = svc.createSyncAuditWithTx(kt, txn, req, result); err != nil {
		return nil, err
	}

	return result, nil
}

// getPrepaidItemByUniqueKey 按唯一键 uuid + 订单账期查询已存在的主单。
func (svc *service) getPrepaidItemByUniqueKey(kt *kit.Kit, item *dsbill.PrepaidItemCreateReq) (
	*tablebill.AccountBillPrepaidItem, error) {

	flt := tools.ExpressionAnd(
		tools.RuleEqual("uuid", item.UUID),
		tools.RuleEqual("order_year", item.OrderYear),
		tools.RuleEqual("order_month", item.OrderMonth),
	)
	opt := &types.ListOption{Filter: flt, Page: core.NewDefaultBasePage()}
	resp, err := svc.dao.AccountBillPrepaidItem().List(kt, opt)
	if err != nil {
		logs.Errorf("list account bill prepaid item by unique key failed, err: %v, uuid: %s, rid: %s",
			err, item.UUID, kt.Rid)
		return nil, err
	}
	if len(resp.Details) == 0 {
		return nil, nil
	}

	return &resp.Details[0], nil
}

// deleteAdjustmentsBySourceWithTx 物理删除主单名下由预付费派生的整组调账。
func (svc *service) deleteAdjustmentsBySourceWithTx(kt *kit.Kit, txn *sqlx.Tx, prepaidID string) ([]string, error) {
	delIDs, err := svc.listAdjustmentIDsBySource(kt, prepaidID)
	if err != nil {
		return nil, err
	}
	if len(delIDs) == 0 {
		return nil, nil
	}

	for _, batch := range slice.Split(delIDs, int(filter.DefaultMaxInLimit)) {
		if err = svc.dao.AccountBillAdjustmentItem().DeleteWithTx(kt, txn,
			tools.ContainersExpression("id", batch)); err != nil {
			logs.Errorf("delete prepaid adjustment item failed, err: %v, prepaid_id: %s, rid: %s",
				err, prepaidID, kt.Rid)
			return nil, err
		}
	}

	return delIDs, nil
}

// listAdjustmentIDsBySource 分页取回主单名下由预付费派生的全部调账 ID。
func (svc *service) listAdjustmentIDsBySource(kt *kit.Kit, prepaidID string) ([]string, error) {
	flt := tools.ExpressionAnd(
		tools.RuleEqual("source", enumor.BillAdjustmentSourcePrepaid),
		tools.RuleEqual("source_id", prepaidID),
	)

	delIDs := make([]string, 0)
	for start := uint32(0); ; start += uint32(core.DefaultMaxPageLimit) {
		opt := &types.ListOption{
			Filter: flt,
			Page:   &core.BasePage{Start: start, Limit: core.DefaultMaxPageLimit},
			Fields: []string{"id"},
		}
		resp, err := svc.dao.AccountBillAdjustmentItem().List(kt, opt)
		if err != nil {
			logs.Errorf("list prepaid adjustment item to delete failed, err: %v, prepaid_id: %s, rid: %s",
				err, prepaidID, kt.Rid)
			return nil, err
		}

		for _, one := range resp.Details {
			delIDs = append(delIDs, one.ID)
		}

		if uint(len(resp.Details)) < core.DefaultMaxPageLimit {
			return delIDs, nil
		}
	}
}

// createAdjustmentsWithTx 按主单 ID 回填 source_id 后批量创建调账条目。
func (svc *service) createAdjustmentsWithTx(kt *kit.Kit, txn *sqlx.Tx, prepaidID string,
	items []dsbill.BillAdjustmentItemCreateReq) ([]string, error) {

	models := make([]tablebill.AccountBillAdjustmentItem, 0, len(items))
	for idx := range items {
		models = append(models, buildAdjustmentModel(kt, prepaidID, &items[idx]))
	}

	ids := make([]string, 0, len(models))
	for _, batch := range slice.Split(models, constant.BatchOperationMaxLimit) {
		batchIDs, err := svc.dao.AccountBillAdjustmentItem().CreateWithTx(kt, txn, batch)
		if err != nil {
			logs.Errorf("create prepaid adjustment item failed, err: %v, prepaid_id: %s, rid: %s",
				err, prepaidID, kt.Rid)
			return nil, err
		}
		ids = append(ids, batchIDs...)
	}

	return ids, nil
}

// buildAdjustmentModel 把调账创建请求转换为调账表模型。
func buildAdjustmentModel(kt *kit.Kit, prepaidID string,
	req *dsbill.BillAdjustmentItemCreateReq) tablebill.AccountBillAdjustmentItem {

	return tablebill.AccountBillAdjustmentItem{
		RootAccountID:  req.RootAccountID,
		MainAccountID:  req.MainAccountID,
		Vendor:         req.Vendor,
		ProductID:      req.ProductID,
		BkBizID:        req.BkBizID,
		BillYear:       req.BillYear,
		BillMonth:      req.BillMonth,
		BillDay:        req.BillDay,
		Type:           string(req.Type),
		ResClass:       req.ResClass,
		ResSubClass:    cvt.ValToPtr(req.ResSubClass),
		Memo:           req.Memo,
		Operator:       req.Operator,
		Currency:       req.Currency,
		Cost:           &tabletypes.Decimal{Decimal: req.Cost},
		RMBCost:        &tabletypes.Decimal{Decimal: req.RMBCost},
		State:          enumor.BillAdjustmentStateConfirmed,
		Source:         enumor.BillAdjustmentSourcePrepaid,
		SourceID:       prepaidID,
		PushStatus:     enumor.BillAdjustmentPushStatusUnpushed,
		PushFailReason: cvt.ValToPtr(""),
		SettleState:    enumor.BillSettleStateUnsettled,
		Creator:        kt.User,
	}
}

// createSyncAuditWithTx 为本次同步写入一条审计记录，首次写入记 create、覆盖重推记 update。
func (svc *service) createSyncAuditWithTx(kt *kit.Kit, txn *sqlx.Tx, req *dsbill.PrepaidItemSyncReq,
	result *dsbill.PrepaidItemSyncResult) error {

	action := enumor.Update
	if result.Created {
		action = enumor.Create
	}

	audit := &tableaudit.AuditTable{
		ResID:     result.ID,
		ResType:   enumor.AccountBillPrepaidItemAuditResType,
		Action:    action,
		BkBizID:   constant.UnassignedBiz,
		Vendor:    req.Item.Vendor,
		AccountID: req.Item.MainAccountID,
		Operator:  kt.User,
		Source:    kt.GetRequestSource(),
		Rid:       kt.Rid,
		AppCode:   kt.AppCode,
		Detail: &tableaudit.BasicDetail{
			Data: map[string]interface{}{
				"uuid":             req.Item.UUID,
				"item":             req.Item,
				"adjustment_ids":   result.AdjustmentIDs,
				"adjustment_items": req.AdjustmentItems,
			},
			Changed: buildAuditChanged(result.DeletedAdjustmentIDs),
		},
	}

	if err := svc.dao.Audit().BatchCreateWithTx(kt, txn, []*tableaudit.AuditTable{audit}); err != nil {
		logs.Errorf("create prepaid sync audit failed, err: %v, prepaid_id: %s, rid: %s", err, result.ID, kt.Rid)
		return err
	}

	return nil
}

// buildAuditChanged 覆盖重推时把被物理删除的旧调账组写入审计 changed，首次写入返回 nil。
func buildAuditChanged(deletedIDs []string) interface{} {
	if len(deletedIDs) == 0 {
		return nil
	}

	return map[string]interface{}{"deleted_adjustment_ids": deletedIDs}
}
