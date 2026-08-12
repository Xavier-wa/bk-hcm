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
	rpproto "hcm/pkg/api/data-service/return-plan"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	tablert "hcm/pkg/dal/table/return-plan/return-plan-ticket"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"

	"github.com/jmoiron/sqlx"
)

// UpdateReturnPlanTicket update return plan ticket by id.
func (svc *service) UpdateReturnPlanTicket(cts *rest.Contexts) (interface{}, error) {
	req := new(rpproto.ReturnPlanTicketUpdateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	record := &tablert.ReturnPlanTicketTable{
		Type:        req.Type,
		Status:      req.Status,
		Remark:      req.Remark,
		SubmittedAt: req.SubmittedAt,
		Reviser:     cts.Kit.User,
	}
	if req.Details != nil {
		record.Details = cvt.PtrToVal(req.Details)
	}
	if req.Message != nil {
		record.Message = cvt.PtrToVal(req.Message)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		if err := svc.dao.ReturnPlanTicket().UpdateWithTx(cts.Kit, txn,
			tools.EqualExpression("id", req.ID), record); err != nil {
			logs.Errorf("update return plan ticket with tx failed, err: %v, id: %s, rid: %s",
				err, req.ID, cts.Kit.Rid)
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("update return plan ticket failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
