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
	"reflect"

	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/rest"

	"github.com/jmoiron/sqlx"
)

// BatchCreateZiyanCvmApplyOrder batch create ziyan cvm apply order
func (svc *service) BatchCreateZiyanCvmApplyOrder(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchCreateZiyanCvmApplyOrderReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	orderIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		orders := make([]cvmapplytable.ZiyanCvmApplyOrder, 0, len(req.ApplyOrders))
		for _, createReq := range req.ApplyOrders {
			subordersJSON, err := tabletypes.NewJsonField(createReq.Suborders)
			if err != nil {
				return nil, err
			}

			var oldSubordersJSON tabletypes.JsonField
			if len(createReq.OldSuborders) > 0 {
				oldSubordersJSON, err = tabletypes.NewJsonField(createReq.OldSuborders)
				if err != nil {
					return nil, err
				}
			}

			orders = append(orders, cvmapplytable.ZiyanCvmApplyOrder{
				OrderID:      createReq.OrderID,
				ProductType:  createReq.ProductType,
				ItsmTicketID: createReq.ItsmTicketID,
				Stage:        createReq.Stage,
				BkBizID:      createReq.BkBizID,
				BkUsername:   createReq.BkUsername,
				Follower:     createReq.Follower,
				EnableNotice: createReq.EnableNotice,
				RequireType:  createReq.RequireType,
				ExpectTime:   createReq.ExpectTime,
				Remark:       createReq.Remark,
				Suborders:    subordersJSON,
				OldSuborders: oldSubordersJSON,
				Creator:      cts.Kit.User,
				Reviser:      cts.Kit.User,
			})
		}
		ids, err := svc.dao.ZiyanCvmApplyOrder().CreateWithTx(cts.Kit, txn, orders)
		if err != nil {
			return nil, fmt.Errorf("create ziyan cvm apply order failed, err: %v, orders: %+v", err, orders)
		}
		return ids, nil
	})
	if err != nil {
		return nil, err
	}

	ids, ok := orderIDs.([]uint64)
	if !ok {
		return nil, fmt.Errorf("batch create ziyan cvm apply order but return id type is not uint64, "+
			"id type: %v", reflect.TypeOf(orderIDs).String())
	}

	return &cvmapplyproto.BatchCreateCvmApplyOrderResult{IDs: ids}, nil
}
