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

	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"

	"github.com/jmoiron/sqlx"
)

// BatchUpdateZiyanCvmGenerateRecord batch update ziyan cvm generate record
func (svc *service) BatchUpdateZiyanCvmGenerateRecord(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchUpdateZiyanCvmGenerateRecordReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		for _, updateReq := range req.GenerateRecords {
			recordReq := buildUpdateGenerateRecord(updateReq, cts.Kit.User)
			whereReq := tools.ExpressionAnd(tools.RuleEqual("generate_id", updateReq.GenerateID),
				tools.RuleEqual("suborder_id", updateReq.SuborderID))
			if err := svc.dao.ZiyanCvmGenerateRecord().Update(cts.Kit, txn, whereReq, recordReq); err != nil {
				return nil, fmt.Errorf("update ziyan cvm generate record failed, err: %v, generateID: %s, "+
					"suborderID: %s", err, updateReq.GenerateID, updateReq.SuborderID)
			}
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// buildUpdateGenerateRecord 构建更新请求对象，只设置非零值字段
func buildUpdateGenerateRecord(updateReq cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq,
	reviser string) *cvmapplytable.ZiyanCvmGenerateRecord {
	recordReq := &cvmapplytable.ZiyanCvmGenerateRecord{
		GenerateType:    cvt.PtrToVal(updateReq.GenerateType),
		TaskID:          cvt.PtrToVal(updateReq.TaskID),
		TaskLink:        cvt.PtrToVal(updateReq.TaskLink),
		RequestInfo:     updateReq.RequestInfo,
		Status:          updateReq.Status,
		IsMatched:       updateReq.IsMatched,
		Message:         cvt.PtrToVal(updateReq.Message),
		TotalNum:        updateReq.TotalNum,
		SuccessNum:      updateReq.SuccessNum,
		SuccessList:     cvt.PtrToVal(updateReq.SuccessList),
		StartAt:         cvt.PtrToVal(updateReq.StartAt),
		EndAt:           cvt.PtrToVal(updateReq.EndAt),
		IsManualMatched: cvt.PtrToVal(updateReq.IsManualMatched),
		Reviser:         reviser,
	}

	return recordReq
}
