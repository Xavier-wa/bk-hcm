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

package billregioncityrel

import (
	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	tablebill "hcm/pkg/dal/table/bill"
	"hcm/pkg/rest"

	"github.com/jmoiron/sqlx"
)

// UpdateBillRegionCityRel updates an account bill region city rel record.
func (svc *service) UpdateBillRegionCityRel(cts *rest.Contexts) (interface{}, error) {
	req := new(dsbill.BillRegionCityRelUpdateReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	updateData := &tablebill.AccountBillRegionCityRel{
		ID:      req.ID,
		Reviser: cts.Kit.User,
	}
	if len(req.Region) > 0 {
		updateData.Region = req.Region
	}
	if len(req.Vendor) > 0 {
		updateData.Vendor = req.Vendor
	}
	if req.CityID != nil {
		updateData.CityID = *req.CityID
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		return nil, svc.dao.AccountBillRegionCityRel().UpdateByIDWithTx(cts.Kit, txn, req.ID, updateData)
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}
