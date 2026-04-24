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

	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	"hcm/pkg/dal/dao/tools"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"

	"github.com/jmoiron/sqlx"
)

// BatchUpdateZiyanCvmModifyRecord batch update ziyan cvm modify record
func (svc *service) BatchUpdateZiyanCvmModifyRecord(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchUpdateZiyanCvmModifyRecordReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		for _, updateReq := range req.Records {
			recordReq := buildUpdateRecord(updateReq)
			if err := svc.dao.ZiyanCvmModifyRecord().Update(
				cts.Kit, txn, tools.EqualExpression("id", updateReq.ID), recordReq); err != nil {
				return nil, fmt.Errorf("update ziyan cvm modify record failed, err: %v, id: %s",
					err, updateReq.ID)
			}
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// buildUpdateRecord 构建更新请求对象，只设置非零值字段
func buildUpdateRecord(updateReq cvmapplyproto.ZiyanCvmModifyRecordUpdateReq) *cvmapplytable.ZiyanCvmModifyRecord {
	recordReq := &cvmapplytable.ZiyanCvmModifyRecord{
		ID:                   updateReq.ID,
		SuborderID:           updateReq.SuborderID,
		BkUsername:           updateReq.BkUsername,
		PreRegion:            updateReq.PreRegion,
		PreZone:              updateReq.PreZone,
		PreDeviceType:        updateReq.PreDeviceType,
		PreImageID:           updateReq.PreImageID,
		PreDiskType:          updateReq.PreDiskType,
		PreNetworkType:       updateReq.PreNetworkType,
		PreVpc:               updateReq.PreVpc,
		PreSubnet:            updateReq.PreSubnet,
		PreSystemDiskType:    updateReq.PreSystemDiskType,
		PreDataDisk:          updateReq.PreDataDisk,
		PreZones:             updateReq.PreZones,
		PreTotalNum:          updateReq.PreTotalNum,
		PreReplicas:          updateReq.PreReplicas,
		PreDiskSize:          updateReq.PreDiskSize,
		PreSystemDiskSize:    updateReq.PreSystemDiskSize,
		PreSystemDiskNum:     updateReq.PreSystemDiskNum,
		PreResAssign:         cvt.PtrToVal(updateReq.PreResAssign),
		PreBkAssetID:         updateReq.PreBkAssetID,
		PreInheritInstanceID: updateReq.PreInheritInstanceID,
		CurRegion:            updateReq.CurRegion,
		CurZone:              updateReq.CurZone,
		CurDeviceType:        updateReq.CurDeviceType,
		CurImageID:           updateReq.CurImageID,
		CurDiskType:          updateReq.CurDiskType,
		CurNetworkType:       updateReq.CurNetworkType,
		CurVpc:               updateReq.CurVpc,
		CurSubnet:            updateReq.CurSubnet,
		CurSystemDiskType:    updateReq.CurSystemDiskType,
		CurDataDisk:          updateReq.CurDataDisk,
		CurZones:             updateReq.CurZones,
		CurTotalNum:          updateReq.CurTotalNum,
		CurReplicas:          updateReq.CurReplicas,
		CurDiskSize:          updateReq.CurDiskSize,
		CurSystemDiskSize:    updateReq.CurSystemDiskSize,
		CurSystemDiskNum:     updateReq.CurSystemDiskNum,
		CurResAssign:         cvt.PtrToVal(updateReq.CurResAssign),
		CurBkAssetID:         updateReq.CurBkAssetID,
		CurInheritInstanceID: updateReq.CurInheritInstanceID,
		Status:               cvt.PtrToVal(updateReq.Status),
		Approver:             updateReq.Approver,
	}

	return recordReq
}
