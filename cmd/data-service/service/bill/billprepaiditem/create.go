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
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	tablebill "hcm/pkg/dal/table/bill"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"

	"github.com/jmoiron/sqlx"
)

// CreateBillPrepaidItem create account bill prepaid item.
func (svc *service) CreateBillPrepaidItem(cts *rest.Contexts) (interface{}, error) {
	req := new(dsbill.PrepaidItemCreateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	id, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		return svc.dao.AccountBillPrepaidItem().CreateWithTx(cts.Kit, txn, buildPrepaidItemModel(cts.Kit, req))
	})
	if err != nil {
		logs.Errorf("create account bill prepaid item failed, err: %v, uuid: %s, rid: %s", err, req.UUID, cts.Kit.Rid)
		return nil, err
	}

	return &core.CreateResult{ID: id.(string)}, nil
}

// buildPrepaidItemModel 把创建请求转换为预付费主表模型。
func buildPrepaidItemModel(kt *kit.Kit, req *dsbill.PrepaidItemCreateReq) *tablebill.AccountBillPrepaidItem {
	return &tablebill.AccountBillPrepaidItem{
		UUID:               req.UUID,
		OrderYear:          req.OrderYear,
		OrderMonth:         req.OrderMonth,
		Vendor:             req.Vendor,
		RootAccountID:      req.RootAccountID,
		MainAccountID:      req.MainAccountID,
		RootAccountCloudID: req.RootAccountCloudID,
		MainAccountCloudID: req.MainAccountCloudID,
		ProductID:          req.ProductID,
		ResourceID:         req.ResourceID,
		InvoiceID:          req.InvoiceID,
		GPUType:            req.GPUType,
		DeviceNum:          req.DeviceNum,
		CardNum:            req.CardNum,
		ProductName:        req.ProductName,
		ProductSpec:        req.ProductSpec,
		Region:             req.Region,
		UsageStartAt:       cvt.ValToPtr(req.UsageStartAt),
		UsageEndAt:         cvt.ValToPtr(req.UsageEndAt),
		OrderAt:            req.OrderAt,
		Currency:           req.Currency,
		Cost:               &tabletypes.Decimal{Decimal: req.Cost},
		RMBCost:            &tabletypes.Decimal{Decimal: req.RMBCost},
		SettleState:        req.SettleState,
		Creator:            kt.User,
		Reviser:            kt.User,
	}
}
