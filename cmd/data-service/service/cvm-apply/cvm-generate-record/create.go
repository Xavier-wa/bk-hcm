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

// Package cvmgeneraterecord ...
package cvmgeneraterecord

import (
	"fmt"
	"reflect"

	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"

	"github.com/jmoiron/sqlx"
)

// BatchCreateZiyanCvmGenerateRecord batch create ziyan cvm generate record
func (svc *service) BatchCreateZiyanCvmGenerateRecord(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchCreateZiyanCvmGenerateRecordReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	generateIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		records := make([]cvmapplytable.ZiyanCvmGenerateRecord, 0, len(req.GenerateRecords))
		for _, createReq := range req.GenerateRecords {
			record := cvmapplytable.ZiyanCvmGenerateRecord{
				GenerateID:      createReq.GenerateID,
				SuborderID:      createReq.SuborderID,
				GenerateType:    createReq.GenerateType,
				TaskID:          createReq.TaskID,
				TaskLink:        createReq.TaskLink,
				RequestInfo:     createReq.RequestInfo,
				Status:          cvt.ValToPtr(createReq.Status),
				IsMatched:       cvt.ValToPtr(createReq.IsMatched),
				Message:         createReq.Message,
				TotalNum:        cvt.ValToPtr(createReq.TotalNum),
				SuccessNum:      cvt.ValToPtr(createReq.SuccessNum),
				SuccessList:     createReq.SuccessList,
				StartAt:         createReq.StartAt,
				EndAt:           createReq.EndAt,
				IsManualMatched: createReq.IsManualMatched,
				Creator:         cts.Kit.User,
				Reviser:         cts.Kit.User,
			}
			records = append(records, record)
		}
		ids, err := svc.dao.ZiyanCvmGenerateRecord().CreateWithTx(cts.Kit, txn, records)
		if err != nil {
			return nil, fmt.Errorf("create ziyan cvm generate record failed, err: %v, records: %+v", err, records)
		}
		return ids, nil
	})
	if err != nil {
		return nil, err
	}

	ids, ok := generateIDs.([]string)
	if !ok {
		return nil, fmt.Errorf("batch create ziyan cvm generate record but return id type is not []string, "+
			"id type: %v", reflect.TypeOf(generateIDs).String())
	}

	return &core.BatchCreateResult{IDs: ids}, nil
}
