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

// Package cvmapplyorder ...
package cvmapplyorder

import (
	"fmt"

	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/rest"

	"github.com/jmoiron/sqlx"
)

// BatchUpdateZiyanCvmApplyOrder batch update ziyan cvm apply order
func (svc *service) BatchUpdateZiyanCvmApplyOrder(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchUpdateZiyanCvmApplyOrderReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		for _, updateReq := range req.ApplyOrders {
			orderReq := &cvmapplytable.ZiyanCvmApplyOrder{
				OrderID: updateReq.OrderID,
				Reviser: cts.Kit.User,
			}
			if len(updateReq.ProductType) != 0 {
				orderReq.ProductType = updateReq.ProductType
			}
			if len(updateReq.ItsmTicketID) != 0 {
				orderReq.ItsmTicketID = updateReq.ItsmTicketID
			}
			if len(updateReq.Stage) != 0 {
				orderReq.Stage = updateReq.Stage
			}
			if updateReq.BkBizID != 0 {
				orderReq.BkBizID = updateReq.BkBizID
			}
			if len(updateReq.BkUsername) != 0 {
				orderReq.BkUsername = updateReq.BkUsername
			}
			if len(updateReq.Follower) != 0 {
				orderReq.Follower = updateReq.Follower
			}
			if updateReq.EnableNotice != nil {
				orderReq.EnableNotice = *updateReq.EnableNotice
			}
			if updateReq.RequireType > 0 {
				orderReq.RequireType = updateReq.RequireType
			}
			if updateReq.ExpectTime != nil {
				orderReq.ExpectTime = *updateReq.ExpectTime
			}
			if len(updateReq.Remark) != 0 {
				orderReq.Remark = updateReq.Remark
			}
			if len(updateReq.Suborders) > 0 {
				subordersJSON, err := tabletypes.NewJsonField(updateReq.Suborders)
				if err != nil {
					return nil, err
				}
				orderReq.Suborders = subordersJSON
			}
			if len(updateReq.OldSuborders) > 0 {
				oldSubordersJSON, err := tabletypes.NewJsonField(updateReq.OldSuborders)
				if err != nil {
					return nil, err
				}
				orderReq.OldSuborders = oldSubordersJSON
			}
			if err := svc.dao.ZiyanCvmApplyOrder().Update(
				cts.Kit, txn, tools.EqualExpression("order_id", updateReq.OrderID), orderReq); err != nil {
				return nil, fmt.Errorf("update ziyan cvm apply order failed, err: %v, id: %d", err, updateReq.OrderID)
			}
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}
