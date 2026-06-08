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
	"reflect"

	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/orm"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/rest"
	cvt "hcm/pkg/tools/converter"

	"github.com/jmoiron/sqlx"
)

// BatchCreateZiyanCvmApplySuborder batch create ziyan cvm apply suborder
func (svc *service) BatchCreateZiyanCvmApplySuborder(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchCreateZiyanCvmApplySuborderReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	suborderIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		suborders := make([]cvmapplytable.ZiyanCvmApplySuborder, 0, len(req.ApplySuborders))
		for _, createReq := range req.ApplySuborders {
			suborders = append(suborders, convertToSuborder(&createReq, cts.Kit.User))
		}
		ids, err := svc.dao.ZiyanCvmApplySuborder().CreateWithTx(cts.Kit, txn, suborders)
		if err != nil {
			return nil, fmt.Errorf("create ziyan cvm apply suborder failed, err: %v, suborders: %+v", err, suborders)
		}
		return ids, nil
	})
	if err != nil {
		return nil, err
	}

	ids, ok := suborderIDs.([]string)
	if !ok {
		return nil, fmt.Errorf("batch create ziyan cvm apply suborder but return id type is not []string, "+
			"id type: %v", reflect.TypeOf(suborderIDs).String())
	}

	return &core.BatchCreateResult{IDs: ids}, nil
}

// convertToSuborder 将创建请求转换为子订单表对象
func convertToSuborder(createReq *cvmapplyproto.ZiyanCvmApplySuborderCreateReq, user string) cvmapplytable.ZiyanCvmApplySuborder {
	// 如果为空，默认写入 []
	if createReq.Follower.IsEmpty() {
		createReq.Follower = types.JsonField("[]")
	}
	if createReq.DataDisk.IsEmpty() {
		createReq.DataDisk = types.JsonField("[]")
	}
	if createReq.Zones.IsEmpty() {
		createReq.Zones = types.JsonField("[]")
	}
	if createReq.FailedZoneIds.IsEmpty() {
		createReq.FailedZoneIds = types.JsonField("[]")
	}

	return cvmapplytable.ZiyanCvmApplySuborder{
		SuborderID:        createReq.SuborderID,
		OrderID:           createReq.OrderID,
		BkBizID:           createReq.BkBizID,
		BkUsername:        createReq.BkUsername,
		Follower:          createReq.Follower,
		Auditor:           createReq.Auditor,
		Source:            createReq.Source,
		ProductType:       createReq.ProductType,
		RequireType:       createReq.RequireType,
		ExpectTime:        createReq.ExpectTime,
		ResourceType:      createReq.ResourceType,
		AntiAffinityLevel: createReq.AntiAffinityLevel,
		EnableDiskCheck:   createReq.EnableDiskCheck,
		ObsProject:        createReq.ObsProject,
		Description:       createReq.Description,
		Remark:            createReq.Remark,
		Region:            createReq.Region,
		Zone:              createReq.Zone,
		DeviceGroup:       createReq.DeviceGroup,
		DeviceSize:        createReq.DeviceSize,
		DeviceType:        createReq.DeviceType,
		ImageID:           createReq.ImageID,
		Image:             createReq.Image,
		DiskSize:          createReq.DiskSize,
		DiskType:          createReq.DiskType,
		NetworkType:       createReq.NetworkType,
		Vpc:               cvt.ValToPtr(createReq.Vpc),
		Subnet:            cvt.ValToPtr(createReq.Subnet),
		OsType:            createReq.OsType,
		RaidType:          createReq.RaidType,
		Isp:               createReq.Isp,
		FailedZoneIds:     createReq.FailedZoneIds,
		ChargeType:        createReq.ChargeType,
		ChargeMonths:      createReq.ChargeMonths,
		InheritInstanceID: createReq.InheritInstanceID,
		BkAssetID:         createReq.BkAssetID,
		ResAssign:         cvt.ValToPtr(createReq.ResAssign),
		CPUThreadSwitch:   createReq.CPUThreadSwitch,
		SystemDisk:        createReq.SystemDisk,
		DataDisk:          createReq.DataDisk,
		Zones:             createReq.Zones,
		UpgradeCvmList:    createReq.UpgradeCvmList,
		Stage:             createReq.Stage,
		Status:            createReq.Status,
		RetryTime:         cvt.ValToPtr(createReq.RetryTime),
		ModifyTime:        cvt.ValToPtr(createReq.ModifyTime),
		AppliedCore:       cvt.ValToPtr(createReq.AppliedCore),
		DeliveredCore:     cvt.ValToPtr(createReq.DeliveredCore),
		PlanExpendGroup:   createReq.PlanExpendGroup,
		OriginNum:         cvt.ValToPtr(createReq.OriginNum),
		TotalNum:          cvt.ValToPtr(createReq.TotalNum),
		SuccessNum:        cvt.ValToPtr(createReq.SuccessNum),
		PendingNum:        cvt.ValToPtr(createReq.PendingNum),
		FailedNum:         cvt.ValToPtr(createReq.FailedNum),
		Creator:           user,
		Reviser:           user,
	}
}
