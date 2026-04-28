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
	"fmt"

	"hcm/pkg/api/core"
	dataservice "hcm/pkg/api/data-service"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/dal/dao/types"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/slice"

	"github.com/jmoiron/sqlx"
)

// BatchDeleteBillRegionCityRel deletes account bill region city rel records.
func (svc *service) BatchDeleteBillRegionCityRel(cts *rest.Contexts) (interface{}, error) {
	req := new(dataservice.BatchDeleteReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	delIDs := make([]string, 0)
	opt := &types.ListOption{
		Filter: req.Filter,
		Page:   core.NewDefaultBasePage(),
	}
	for {
		listResp, err := svc.dao.AccountBillRegionCityRel().List(cts.Kit, opt)
		if err != nil {
			logs.Errorf("list account bill region city rel for delete failed, err: %v, rid: %s", err, cts.Kit.Rid)
			return nil, fmt.Errorf("list account bill region city rel for delete failed, err: %v", err)
		}
		for _, one := range listResp.Details {
			delIDs = append(delIDs, one.ID)
		}
		if len(listResp.Details) < int(opt.Page.Limit) {
			break
		}
		opt.Page.Start += uint32(opt.Page.Limit)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		for _, batch := range slice.Split(delIDs, int(filter.DefaultMaxInLimit)) {
			delFilter := tools.ContainersExpression("id", batch)
			if err := svc.dao.AccountBillRegionCityRel().DeleteWithTx(cts.Kit, txn, delFilter); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	if err != nil {
		logs.Errorf("delete account bill region city rel failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return nil, nil
}
