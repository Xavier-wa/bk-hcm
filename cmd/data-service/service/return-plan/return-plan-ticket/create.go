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

package returnplanticket

import (
	"fmt"

	"hcm/pkg/api/core"
	rpproto "hcm/pkg/api/data-service/return-plan"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	"hcm/pkg/logs"
	"hcm/pkg/rest"

	"github.com/jmoiron/sqlx"
)

// CreateReturnPlanTicket create return plan ticket.
func (svc *service) CreateReturnPlanTicket(cts *rest.Contexts) (interface{}, error) {
	req := new(rpproto.ReturnPlanTicketCreateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	createID, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		model := tablert.ReturnPlanTicketTable{
			Type:            req.Type,
			Details:         req.Details,
			Applicant:       req.Applicant,
			BkBizID:         req.BkBizID,
			BkBizName:       req.BkBizName,
			OpProductID:     req.OpProductID,
			OpProductName:   req.OpProductName,
			PlanProductID:   req.PlanProductID,
			PlanProductName: req.PlanProductName,
			VirtualDeptID:   req.VirtualDeptID,
			VirtualDeptName: req.VirtualDeptName,
			Status:          req.Status,
			Message:         req.Message,
			Remark:          req.Remark,
			SubmittedAt:     req.SubmittedAt,
			Creator:         cts.Kit.User,
			Reviser:         cts.Kit.User,
		}

		ids, err := svc.dao.ReturnPlanTicket().CreateWithTx(cts.Kit, txn, []tablert.ReturnPlanTicketTable{model})
		if err != nil {
			logs.Errorf("create return plan ticket with tx failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, err
		}
		return ids[0], nil
	})
	if err != nil {
		logs.Errorf("create return plan ticket failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	id, ok := createID.(string)
	if !ok {
		logs.Errorf("create return plan ticket but return id type not string, rid: %s", cts.Kit.Rid)
		return nil, fmt.Errorf("create return plan ticket but return id type not string")
	}

	return &core.CreateResult{ID: id}, nil
}
