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

// Package cvmapplysuborder ...
package cvmapplysuborder

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

// BatchUpdateZiyanCvmApplySuborder batch update ziyan cvm apply suborder
func (svc *service) BatchUpdateZiyanCvmApplySuborder(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchUpdateZiyanCvmApplySuborderReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		for _, updateReq := range req.ApplySuborders {
			suborderReq := buildUpdateSuborder(updateReq, cts.Kit.User)
			if err := svc.dao.ZiyanCvmApplySuborder().Update(
				cts.Kit, txn, tools.EqualExpression("suborder_id", updateReq.SuborderID), suborderReq); err != nil {
				return nil, fmt.Errorf("update ziyan cvm apply suborder failed, err: %v, id: %s",
					err, updateReq.SuborderID)
			}
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// buildUpdateSuborder 构建更新请求对象，只设置非零值字段
func buildUpdateSuborder(updateReq cvmapplyproto.ZiyanCvmApplySuborderUpdateReq,
	reviser string) *cvmapplytable.ZiyanCvmApplySuborder {
	suborderReq := &cvmapplytable.ZiyanCvmApplySuborder{
		SuborderID:        updateReq.SuborderID,
		OrderID:           updateReq.OrderID,
		BkUsername:        updateReq.BkUsername,
		Follower:          cvt.PtrToVal(updateReq.Follower),
		Auditor:           updateReq.Auditor,
		Source:            updateReq.Source,
		ProductType:       updateReq.ProductType,
		RequireType:       updateReq.RequireType,
		ExpectTime:        cvt.PtrToVal(updateReq.ExpectTime),
		ResourceType:      updateReq.ResourceType,
		AntiAffinityLevel: updateReq.AntiAffinityLevel,
		EnableDiskCheck:   updateReq.EnableDiskCheck,
		ObsProject:        updateReq.ObsProject,
		Description:       updateReq.Description,
		Remark:            updateReq.Remark,
		Region:            updateReq.Region,
		Zone:              updateReq.Zone,
		DeviceGroup:       updateReq.DeviceGroup,
		DeviceSize:        updateReq.DeviceSize,
		DeviceType:        updateReq.DeviceType,
		ImageID:           updateReq.ImageID,
		Image:             updateReq.Image,
		DiskSize:          cvt.PtrToVal(updateReq.DiskSize),
		DiskType:          updateReq.DiskType,
		NetworkType:       updateReq.NetworkType,
		Vpc:               updateReq.Vpc,
		Subnet:            updateReq.Subnet,
		OsType:            updateReq.OsType,
		RaidType:          updateReq.RaidType,
		Isp:               updateReq.Isp,
		FailedZoneIds:     cvt.PtrToVal(updateReq.FailedZoneIds),
		ChargeType:        updateReq.ChargeType,
		ChargeMonths:      cvt.PtrToVal(updateReq.ChargeMonths),
		InheritInstanceID: updateReq.InheritInstanceID,
		BkAssetID:         updateReq.BkAssetID,
		ResAssign:         updateReq.ResAssign,
		CPUThreadSwitch:   updateReq.CPUThreadSwitch,
		SystemDisk:        cvt.PtrToVal(updateReq.SystemDisk),
		DataDisk:          cvt.PtrToVal(updateReq.DataDisk),
		Zones:             cvt.PtrToVal(updateReq.Zones),
		UpgradeCvmList:    cvt.PtrToVal(updateReq.UpgradeCvmList),
		Stage:             updateReq.Stage,
		Status:            updateReq.Status,
		RetryTime:         updateReq.RetryTime,
		ModifyTime:        updateReq.ModifyTime,
		AppliedCore:       updateReq.AppliedCore,
		DeliveredCore:     updateReq.DeliveredCore,
		PlanExpendGroup:   cvt.PtrToVal(updateReq.PlanExpendGroup),
		OriginNum:         updateReq.OriginNum,
		TotalNum:          updateReq.TotalNum,
		SuccessNum:        updateReq.SuccessNum,
		PendingNum:        updateReq.PendingNum,
		FailedNum:         updateReq.FailedNum,
		Reviser:           reviser,
	}

	return suborderReq
}
