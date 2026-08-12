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

	"hcm/pkg/api/core"
	rpproto "hcm/pkg/api/data-service/return-plan"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	tablerst "hcm/pkg/dal/table/return-plan/return-plan-sub-ticket"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/tools/util"

	"github.com/jmoiron/sqlx"
)

// BatchCreateReturnPlanSubTicket batch create return plan sub ticket.
func (svc *service) BatchCreateReturnPlanSubTicket(cts *rest.Contexts) (interface{}, error) {
	req := new(rpproto.ReturnPlanSubTicketBatchCreateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	createIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		recordIDs, err := svc.batchCreateReturnPlanSubTicketWithTx(cts.Kit, txn, req.SubTickets)
		if err != nil {
			logs.Errorf("batch create return plan sub ticket with tx failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		return recordIDs, nil
	})
	if err != nil {
		logs.Errorf("batch create return plan sub ticket failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	ids, err := util.GetStrSliceByInterface(createIDs)
	if err != nil {
		logs.Errorf("batch create return plan sub ticket but return ids type not []string, err: %v, rid: %s",
			err, cts.Kit.Rid)
		return nil, fmt.Errorf("batch create return plan sub ticket but return ids type not []string, err: %v", err)
	}

	return &core.BatchCreateResult{IDs: ids}, nil
}

func (svc *service) batchCreateReturnPlanSubTicketWithTx(kt *kit.Kit, txn *sqlx.Tx,
	createReqs []rpproto.ReturnPlanSubTicketCreateReq) ([]string, error) {

	models := make([]tablerst.ReturnPlanSubTicketTable, len(createReqs))
	for idx, item := range createReqs {
		models[idx] = tablerst.ReturnPlanSubTicketTable{
			TicketID:        item.TicketID,
			SubType:         item.SubType,
			SubDetails:      item.SubDetails,
			BkBizID:         item.BkBizID,
			BkBizName:       item.BkBizName,
			OpProductID:     item.OpProductID,
			OpProductName:   item.OpProductName,
			PlanProductID:   item.PlanProductID,
			PlanProductName: item.PlanProductName,
			VirtualDeptID:   item.VirtualDeptID,
			VirtualDeptName: item.VirtualDeptName,
			ObsProject:      item.ObsProject,
			ResPoolName:     item.ResPoolName,
			Status:          item.Status,
			CrpSN:           item.CrpSN,
			CrpURL:          item.CrpURL,
			Message:         item.Message,
			SubmittedAt:     item.SubmittedAt,
			Creator:         kt.User,
			Reviser:         kt.User,
		}
	}

	recordIDs, err := svc.dao.ReturnPlanSubTicket().CreateWithTx(kt, txn, models)
	if err != nil {
		logs.Errorf("create return plan sub ticket failed, err: %v, rid: %s", err, kt.Rid)
		return nil, fmt.Errorf("create return plan sub ticket failed, err: %v", err)
	}

	return recordIDs, nil
}
