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

package returnplansubticket

import (
	"fmt"

	rpproto "hcm/pkg/api/data-service/return-plan"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	tablerst "hcm/pkg/dal/table/return-plan/return-plan-sub-ticket"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"

	"github.com/jmoiron/sqlx"
)

// BatchUpdateReturnPlanSubTicket batch update return plan sub ticket.
func (svc *service) BatchUpdateReturnPlanSubTicket(cts *rest.Contexts) (interface{}, error) {
	req := new(rpproto.ReturnPlanSubTicketBatchUpdateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		if err := svc.batchUpdateReturnPlanSubTicketWithTx(cts.Kit, txn, req.SubTickets); err != nil {
			logs.Errorf("batch update return plan sub ticket with tx failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("batch update return plan sub ticket failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

func (svc *service) batchUpdateReturnPlanSubTicketWithTx(kt *kit.Kit, txn *sqlx.Tx,
	updateReqs []rpproto.ReturnPlanSubTicketUpdateReq) error {

	for _, updateReq := range updateReqs {
		record := &tablerst.ReturnPlanSubTicketTable{
			SubType:     updateReq.SubType,
			ObsProject:  updateReq.ObsProject,
			ResPoolName: updateReq.ResPoolName,
			Status:      updateReq.Status,
			CrpSN:       updateReq.CrpSN,
			CrpURL:      updateReq.CrpURL,
			SubmittedAt: updateReq.SubmittedAt,
			Reviser:     kt.User,
		}
		if updateReq.SubDetails != nil {
			record.SubDetails = cvt.PtrToVal(updateReq.SubDetails)
		}
		if updateReq.Message != nil {
			record.Message = cvt.PtrToVal(updateReq.Message)
		}

		if _, err := svc.dao.ReturnPlanSubTicket().UpdateWithTx(kt, txn,
			tools.EqualExpression("id", updateReq.ID), record); err != nil {
			logs.Errorf("update return plan sub ticket failed, err: %v, id: %s, rid: %s", err, updateReq.ID, kt.Rid)
			return err
		}
	}

	return nil
}

// UpdateReturnPlanSubTicketStatusCAS update return plan sub ticket status with cas.
func (svc *service) UpdateReturnPlanSubTicketStatusCAS(cts *rest.Contexts) (interface{}, error) {
	req := new(rpproto.ReturnPlanSubTicketStatusUpdateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	effected, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		rules := []*filter.AtomRule{
			tools.RuleEqual("ticket_id", req.TicketID),
			tools.RuleEqual("status", req.Source),
		}
		if len(req.IDs) > 0 {
			rules = append(rules, tools.RuleIn("id", req.IDs))
		}
		updateFilter := tools.ExpressionAnd(rules...)

		record := &tablerst.ReturnPlanSubTicketTable{
			Status:  req.Target,
			Reviser: cts.Kit.User,
		}
		if req.CrpSN != nil {
			record.CrpSN = cvt.PtrToVal(req.CrpSN)
		}
		if req.CrpURL != nil {
			record.CrpURL = cvt.PtrToVal(req.CrpURL)
		}
		if req.Message != nil {
			record.Message = cvt.PtrToVal(req.Message)
		}

		effected, err := svc.dao.ReturnPlanSubTicket().UpdateWithTx(cts.Kit, txn, updateFilter, record)
		if err != nil {
			logs.Errorf("update return plan sub ticket status failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		return effected, nil
	})
	if err != nil {
		logs.Errorf("update return plan sub ticket status failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	// 入参指定 ID 时，需确保更新行数与提供的 ID 一致
	if len(req.IDs) > 0 && effected != int64(len(req.IDs)) {
		logs.Errorf("update return plan sub ticket status failed, expected row: %d, actual row: %d, rid: %s",
			len(req.IDs), effected, cts.Kit.Rid)
		return nil, fmt.Errorf("update return plan sub ticket status failed, effected rows: %d", effected)
	}

	return nil, nil
}
