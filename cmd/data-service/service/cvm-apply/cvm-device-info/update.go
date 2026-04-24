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

// Package cvmdeviceinfo ...
package cvmdeviceinfo

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

// BatchUpdateZiyanCvmDeviceInfo batch update ziyan cvm device info
func (svc *service) BatchUpdateZiyanCvmDeviceInfo(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchUpdateZiyanCvmDeviceInfoReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	_, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		for _, updateReq := range req.Devices {
			deviceReq := buildUpdateDevice(updateReq, cts.Kit.User)
			if err := svc.dao.ZiyanCvmDeviceInfo().Update(
				cts.Kit, txn, tools.EqualExpression("id", updateReq.ID), deviceReq); err != nil {
				return nil, fmt.Errorf("update ziyan cvm device info failed, err: %v, id: %s",
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

// buildUpdateDevice 构建更新请求对象，只设置非零值字段
func buildUpdateDevice(updateReq cvmapplyproto.ZiyanCvmDeviceInfoUpdateReq,
	reviser string) *cvmapplytable.ZiyanCvmDeviceInfo {
	deviceReq := &cvmapplytable.ZiyanCvmDeviceInfo{
		ID:               updateReq.ID,
		OrderID:          updateReq.OrderID,
		SuborderID:       updateReq.SuborderID,
		GenerateID:       cvt.PtrToVal(updateReq.GenerateID),
		BkBizID:          updateReq.BkBizID,
		BkUsername:       updateReq.BkUsername,
		BkHostID:         cvt.PtrToVal(updateReq.BkHostID),
		IP:               updateReq.IP,
		AssetID:          updateReq.AssetID,
		InstanceID:       updateReq.InstanceID,
		RequireType:      cvt.PtrToVal(updateReq.RequireType),
		ResourceType:     cvt.PtrToVal(updateReq.ResourceType),
		DeviceType:       updateReq.DeviceType,
		Description:      updateReq.Description,
		Remark:           updateReq.Remark,
		ZoneName:         updateReq.ZoneName,
		ZoneID:           cvt.PtrToVal(updateReq.ZoneID),
		CloudZone:        updateReq.CloudZone,
		CloudRegion:      updateReq.CloudRegion,
		ModuleName:       updateReq.ModuleName,
		RackID:           updateReq.RackID,
		IsMatched:        updateReq.IsMatched,
		IsChecked:        updateReq.IsChecked,
		IsInited:         updateReq.IsInited,
		IsDelivered:      updateReq.IsDelivered,
		Deliverer:        updateReq.Deliverer,
		GenerateTaskID:   updateReq.GenerateTaskID,
		GenerateTaskLink: updateReq.GenerateTaskLink,
		InitTaskID:       updateReq.InitTaskID,
		InitTaskLink:     updateReq.InitTaskLink,
		IsManualMatched:  cvt.PtrToVal(updateReq.IsManualMatched),
		OwnerIP:          cvt.PtrToVal(updateReq.OwnerIP),
		Reviser:          reviser,
	}

	return deviceReq
}
