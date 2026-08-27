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
	dataservice "hcm/pkg/api/data-service"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	tableaudit "hcm/pkg/dal/table/audit"
	tablebill "hcm/pkg/dal/table/bill"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/slice"

	"github.com/jmoiron/sqlx"
)

// BatchDeleteBillPrepaidItem batch delete account bill prepaid item by filter.
func (svc *service) BatchDeleteBillPrepaidItem(cts *rest.Contexts) (interface{}, error) {
	req := new(dataservice.BatchDeleteReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	items, err := svc.listDeletePrepaidItems(cts.Kit, req.Filter)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}

	_, err = svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		return nil, svc.deletePrepaidWithTx(cts.Kit, txn, items)
	})
	if err != nil {
		logs.Errorf("delete account bill prepaid item failed, err: %v, count: %d, rid: %s",
			err, len(items), cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

// deletePrepaidWithTx 同事务内级联删除派生调账、主单并写入删除审计。
func (svc *service) deletePrepaidWithTx(kt *kit.Kit, txn *sqlx.Tx, items []tablebill.AccountBillPrepaidItem) error {
	audits := make([]*tableaudit.AuditTable, 0, len(items))
	ids := make([]string, 0, len(items))
	for idx := range items {
		delAdjIDs, err := svc.deleteAdjustmentsBySourceWithTx(kt, txn, items[idx].ID)
		if err != nil {
			return err
		}
		ids = append(ids, items[idx].ID)
		audits = append(audits, buildDeleteAudit(kt, &items[idx], delAdjIDs))
	}

	if err := svc.deletePrepaidIDsWithTx(kt, txn, ids); err != nil {
		return err
	}

	return svc.createDeleteAuditWithTx(kt, txn, audits)
}

// deletePrepaidIDsWithTx 按 ID 分片物理删除预付费主单。
func (svc *service) deletePrepaidIDsWithTx(kt *kit.Kit, txn *sqlx.Tx, ids []string) error {
	for _, batch := range slice.Split(ids, int(filter.DefaultMaxInLimit)) {
		if err := svc.dao.AccountBillPrepaidItem().DeleteWithTx(kt, txn,
			tools.ContainersExpression("id", batch)); err != nil {
			logs.Errorf("delete account bill prepaid item failed, err: %v, ids: %v, rid: %s", err, batch, kt.Rid)
			return err
		}
	}

	return nil
}

// createDeleteAuditWithTx 按批量上限写入删除审计。
func (svc *service) createDeleteAuditWithTx(kt *kit.Kit, txn *sqlx.Tx, audits []*tableaudit.AuditTable) error {
	for _, batch := range slice.Split(audits, constant.BatchOperationMaxLimit) {
		if err := svc.dao.Audit().BatchCreateWithTx(kt, txn, batch); err != nil {
			logs.Errorf("create prepaid delete audit failed, err: %v, count: %d, rid: %s", err, len(batch), kt.Rid)
			return err
		}
	}

	return nil
}

// listDeletePrepaidItems 按过滤条件分页列出待删除主单。
func (svc *service) listDeletePrepaidItems(kt *kit.Kit, expr *filter.Expression) (
	[]tablebill.AccountBillPrepaidItem, error) {

	items := make([]tablebill.AccountBillPrepaidItem, 0)
	for start := uint32(0); ; start += uint32(core.DefaultMaxPageLimit) {
		opt := &types.ListOption{
			Filter: expr,
			Page:   &core.BasePage{Start: start, Limit: core.DefaultMaxPageLimit},
			Fields: []string{"id", "uuid", "vendor", "main_account_id"},
		}
		listResp, err := svc.dao.AccountBillPrepaidItem().List(kt, opt)
		if err != nil {
			logs.Errorf("list account bill prepaid item to delete failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		items = append(items, listResp.Details...)
		if uint(len(listResp.Details)) < core.DefaultMaxPageLimit {
			return items, nil
		}
	}
}

// buildDeleteAudit 构造单条预付费主单的删除审计。
func buildDeleteAudit(kt *kit.Kit, item *tablebill.AccountBillPrepaidItem,
	deletedAdjIDs []string) *tableaudit.AuditTable {
	return &tableaudit.AuditTable{
		ResID:     item.ID,
		ResType:   enumor.AccountBillPrepaidItemAuditResType,
		Action:    enumor.Delete,
		BkBizID:   constant.UnassignedBiz,
		Vendor:    item.Vendor,
		AccountID: item.MainAccountID,
		Operator:  kt.User,
		Source:    kt.GetRequestSource(),
		Rid:       kt.Rid,
		AppCode:   kt.AppCode,
		Detail: &tableaudit.BasicDetail{
			Data: map[string]interface{}{
				"uuid": item.UUID,
			},
			Changed: buildAuditChanged(deletedAdjIDs),
		},
	}
}
