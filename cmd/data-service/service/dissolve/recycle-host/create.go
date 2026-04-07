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

package recyclehost

import (
	"fmt"
	"reflect"

	"hcm/pkg/api/core"
	dsproto "hcm/pkg/api/data-service/dissolve"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	hostdefine "hcm/pkg/dal/table/dissolve/host"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"

	"github.com/jmoiron/sqlx"
)

// BatchCreateRecycleHost batch create dissolve recycle host records.
func (svc *service) BatchCreateRecycleHost(cts *rest.Contexts) (interface{}, error) {
	req := new(dsproto.BatchCreateRecycleHostReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	hostIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		hosts := make([]hostdefine.RecycleHostTable, 0, len(req.Hosts))
		for _, createReq := range req.Hosts {
			hosts = append(hosts, hostdefine.RecycleHostTable{
				AssetID:      cvt.ValToPtr(createReq.AssetID),
				InnerIP:      cvt.ValToPtr(createReq.InnerIP),
				Module:       cvt.ValToPtr(createReq.Module),
				AbolishPhase: cvt.ValToPtr(createReq.AbolishPhase),
				ProjectName:  cvt.ValToPtr(createReq.ProjectName),
			})
		}
		ids, err := svc.dao.RecycleHost().CreateWithTx(cts.Kit, txn, hosts)
		if err != nil {
			return nil, fmt.Errorf("create dissolve recycle host failed, err: %v", err)
		}
		return ids, nil
	})
	if err != nil {
		return nil, err
	}

	ids, ok := hostIDs.([]string)
	if !ok {
		return nil, fmt.Errorf("batch create dissolve recycle host but return id type is not string, "+
			"id type: %v", reflect.TypeOf(hostIDs).String())
	}

	return &core.BatchCreateResult{IDs: ids}, nil
}
