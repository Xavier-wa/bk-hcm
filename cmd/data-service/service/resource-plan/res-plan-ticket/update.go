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

package resplanticket

import (
	rpproto "hcm/pkg/api/data-service/resource-plan"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	rpst "hcm/pkg/dal/table/resource-plan/res-plan-sub-ticket"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	rpts "hcm/pkg/dal/table/resource-plan/res-plan-ticket-status"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"

	"github.com/jmoiron/sqlx"
)

// OverwriteResPlanTicket overwrites resource plan ticket and resets related sub tickets and status.
func (svc *service) OverwriteResPlanTicket(cts *rest.Contexts) (interface{}, error) {
	req := new(rpproto.OverwriteResPlanTicketReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		if err := svc.overwriteResPlanTicketWithTx(cts.Kit, txn, req); err != nil {
			logs.Errorf("overwrite res plan ticket with tx failed, err: %v, ticket id: %s, rid: %s",
				err, req.TicketID, cts.Kit.Rid)
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("overwrite res plan ticket failed, err: %v, ticket id: %s, rid: %s",
			err, req.TicketID, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}

func (svc *service) overwriteResPlanTicketWithTx(kt *kit.Kit, txn *sqlx.Tx,
	req *rpproto.OverwriteResPlanTicketReq) error {

	ticketUpdate := buildResPlanTicketTableFromUpdateReq(req.Ticket, kt.User)
	if err := svc.dao.ResPlanTicket().UpdateWithTx(kt, txn, tools.EqualExpression("id", req.TicketID),
		ticketUpdate); err != nil {
		logs.Errorf("update res plan ticket failed, err: %v, ticket id: %s, rid: %s", err, req.TicketID, kt.Rid)
		return err
	}

	subUpdate := &rpst.ResPlanSubTicketTable{
		TicketID: constant.ResPlanDropTicketID(req.TicketID),
		Status:   enumor.RPSubTicketStatusFailed,
		Reviser:  kt.User,
	}
	if _, err := svc.dao.ResPlanSubTicket().UpdateWithTx(kt, txn,
		tools.EqualExpression("ticket_id", req.TicketID), subUpdate); err != nil {
		logs.Errorf("unbind res plan sub tickets failed, err: %v, ticket id: %s, rid: %s", err, req.TicketID, kt.Rid)
		return err
	}

	statusUpdate := &rpts.ResPlanTicketStatusTable{
		TicketID: req.TicketID,
		Status:   enumor.RPTicketStatusAuditing,
	}
	if err := svc.dao.ResPlanTicketStatus().UpdateWithTx(kt, txn,
		tools.EqualExpression("ticket_id", req.TicketID), statusUpdate); err != nil {
		logs.Errorf("update res plan ticket status failed, err: %v, ticket id: %s, rid: %s", err, req.TicketID, kt.Rid)
		return err
	}

	return nil
}

func buildResPlanTicketTableFromUpdateReq(req rpproto.ResPlanTicketUpdateReq, reviser string) *rpt.ResPlanTicketTable {
	record := &rpt.ResPlanTicketTable{
		Remark:           req.Remark,
		DemandClass:      req.DemandClass,
		SubmittedAt:      req.SubmittedAt,
		OriginalOS:       req.OriginalOS,
		OriginalCpuCore:  req.OriginalCPUCore,
		OriginalMemory:   req.OriginalMemory,
		OriginalDiskSize: req.OriginalDiskSize,
		UpdatedOS:        req.UpdatedOS,
		UpdatedCpuCore:   req.UpdatedCPUCore,
		UpdatedMemory:    req.UpdatedMemory,
		UpdatedDiskSize:  req.UpdatedDiskSize,
		Reviser:          reviser,
	}
	if req.Demands != nil {
		record.Demands = cvt.PtrToVal(req.Demands)
	}

	return record
}
