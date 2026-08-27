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
	billcore "hcm/pkg/api/core/bill"
	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/types"
	tablebill "hcm/pkg/dal/table/bill"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"
)

// ListBillPrepaidItem list account bill prepaid item with options.
func (svc *service) ListBillPrepaidItem(cts *rest.Contexts) (interface{}, error) {
	req := new(dsbill.PrepaidItemListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	opt := &types.ListOption{
		Filter: req.Filter,
		Page:   req.Page,
		Fields: req.Fields,
	}
	data, err := svc.dao.AccountBillPrepaidItem().List(cts.Kit, opt)
	if err != nil {
		logs.Errorf("list account bill prepaid item failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	details := make([]*billcore.PrepaidItem, len(data.Details))
	for idx := range data.Details {
		details[idx] = ConvPrepaidItem(&data.Details[idx])
	}

	return &dsbill.PrepaidItemListResult{Details: details, Count: data.Count}, nil
}

// ConvPrepaidItem 把预付费主表记录转换为对外的核心结构。
func ConvPrepaidItem(m *tablebill.AccountBillPrepaidItem) *billcore.PrepaidItem {
	return &billcore.PrepaidItem{
		ID:                 m.ID,
		UUID:               m.UUID,
		OrderYear:          m.OrderYear,
		OrderMonth:         m.OrderMonth,
		Vendor:             m.Vendor,
		RootAccountID:      m.RootAccountID,
		MainAccountID:      m.MainAccountID,
		RootAccountCloudID: m.RootAccountCloudID,
		MainAccountCloudID: m.MainAccountCloudID,
		ProductID:          m.ProductID,
		ResourceID:         m.ResourceID,
		InvoiceID:          m.InvoiceID,
		GPUType:            m.GPUType,
		DeviceNum:          m.DeviceNum,
		CardNum:            m.CardNum,
		ProductName:        m.ProductName,
		ProductSpec:        m.ProductSpec,
		Region:             m.Region,
		UsageStartAt:       cvt.PtrToVal(m.UsageStartAt),
		UsageEndAt:         cvt.PtrToVal(m.UsageEndAt),
		OrderAt:            m.OrderAt,
		Currency:           m.Currency,
		Cost:               cvt.PtrToVal(m.Cost).Decimal,
		RMBCost:            cvt.PtrToVal(m.RMBCost).Decimal,
		SettleState:        m.SettleState,
		Creator:            m.Creator,
		Reviser:            m.Reviser,
		CreatedAt:          m.CreatedAt.String(),
		UpdatedAt:          m.UpdatedAt.String(),
	}
}
