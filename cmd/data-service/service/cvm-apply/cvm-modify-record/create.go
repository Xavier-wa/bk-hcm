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

// Package cvmmodifyrecord ...
package cvmmodifyrecord

import (
	"fmt"
	"reflect"

	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/rest"

	"github.com/jmoiron/sqlx"
)

// BatchCreateZiyanCvmModifyRecord batch create ziyan cvm modify record
func (svc *service) BatchCreateZiyanCvmModifyRecord(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchCreateZiyanCvmModifyRecordReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	recordIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		records := make([]cvmapplytable.ZiyanCvmModifyRecord, 0, len(req.Records))
		for _, createReq := range req.Records {
			record := cvmapplytable.ZiyanCvmModifyRecord{
				ID:                   createReq.ID,
				SuborderID:           createReq.SuborderID,
				BkUsername:           createReq.BkUsername,
				PreTotalNum:          createReq.PreTotalNum,
				PreReplicas:          createReq.PreReplicas,
				PreRegion:            createReq.PreRegion,
				PreZone:              createReq.PreZone,
				PreDeviceType:        createReq.PreDeviceType,
				PreImageID:           createReq.PreImageID,
				PreDiskSize:          createReq.PreDiskSize,
				PreDiskType:          createReq.PreDiskType,
				PreNetworkType:       createReq.PreNetworkType,
				PreVpc:               createReq.PreVpc,
				PreSubnet:            createReq.PreSubnet,
				PreSystemDiskType:    createReq.PreSystemDiskType,
				PreSystemDiskSize:    createReq.PreSystemDiskSize,
				PreSystemDiskNum:     createReq.PreSystemDiskNum,
				PreDataDisk:          createReq.PreDataDisk,
				PreZones:             createReq.PreZones,
				PreResAssign:         createReq.PreResAssign,
				PreBkAssetID:         createReq.PreBkAssetID,
				PreInheritInstanceID: createReq.PreInheritInstanceID,
				CurTotalNum:          createReq.CurTotalNum,
				CurReplicas:          createReq.CurReplicas,
				CurRegion:            createReq.CurRegion,
				CurZone:              createReq.CurZone,
				CurDeviceType:        createReq.CurDeviceType,
				CurImageID:           createReq.CurImageID,
				CurDiskSize:          createReq.CurDiskSize,
				CurDiskType:          createReq.CurDiskType,
				CurNetworkType:       createReq.CurNetworkType,
				CurVpc:               createReq.CurVpc,
				CurSubnet:            createReq.CurSubnet,
				CurSystemDiskType:    createReq.CurSystemDiskType,
				CurSystemDiskSize:    createReq.CurSystemDiskSize,
				CurSystemDiskNum:     createReq.CurSystemDiskNum,
				CurDataDisk:          createReq.CurDataDisk,
				CurZones:             createReq.CurZones,
				CurResAssign:         createReq.CurResAssign,
				CurBkAssetID:         createReq.CurBkAssetID,
				CurInheritInstanceID: createReq.CurInheritInstanceID,
				Status:               createReq.Status,
				Approver:             createReq.Approver,
			}
			records = append(records, record)
		}
		ids, err := svc.dao.ZiyanCvmModifyRecord().CreateWithTx(cts.Kit, txn, records)
		if err != nil {
			return nil, fmt.Errorf("create ziyan cvm modify record failed, err: %v, records: %+v", err, records)
		}
		return ids, nil
	})
	if err != nil {
		return nil, err
	}

	ids, ok := recordIDs.([]string)
	if !ok {
		return nil, fmt.Errorf("batch create ziyan cvm modify record but return id type is not []string, "+
			"id type: %v", reflect.TypeOf(recordIDs).String())
	}

	return &core.BatchCreateResult{IDs: ids}, nil
}
