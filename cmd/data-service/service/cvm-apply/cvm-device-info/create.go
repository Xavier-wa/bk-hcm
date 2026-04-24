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

// BatchCreateZiyanCvmDeviceInfo batch create ziyan cvm device info
func (svc *service) BatchCreateZiyanCvmDeviceInfo(cts *rest.Contexts) (interface{}, error) {
	req := new(cvmapplyproto.BatchCreateZiyanCvmDeviceInfoReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}
	deviceIDs, err := svc.dao.Txn().AutoTxn(cts.Kit, func(txn *sqlx.Tx, opt *orm.TxnOption) (interface{}, error) {
		devices := make([]cvmapplytable.ZiyanCvmDeviceInfo, 0, len(req.Devices))
		for _, createReq := range req.Devices {
			device := cvmapplytable.ZiyanCvmDeviceInfo{
				OrderID:          createReq.OrderID,
				SuborderID:       createReq.SuborderID,
				GenerateID:       createReq.GenerateID,
				BkBizID:          createReq.BkBizID,
				BkUsername:       createReq.BkUsername,
				BkHostID:         createReq.BkHostID,
				IP:               createReq.IP,
				AssetID:          createReq.AssetID,
				InstanceID:       createReq.InstanceID,
				RequireType:      createReq.RequireType,
				ResourceType:     createReq.ResourceType,
				DeviceType:       createReq.DeviceType,
				Description:      createReq.Description,
				Remark:           createReq.Remark,
				ZoneName:         createReq.ZoneName,
				ZoneID:           createReq.ZoneID,
				CloudZone:        createReq.CloudZone,
				CloudRegion:      createReq.CloudRegion,
				ModuleName:       createReq.ModuleName,
				RackID:           createReq.RackID,
				IsMatched:        cvt.ValToPtr(createReq.IsMatched),
				IsChecked:        cvt.ValToPtr(createReq.IsChecked),
				IsInited:         cvt.ValToPtr(createReq.IsInited),
				IsDelivered:      cvt.ValToPtr(createReq.IsDelivered),
				Deliverer:        createReq.Deliverer,
				GenerateTaskID:   createReq.GenerateTaskID,
				GenerateTaskLink: createReq.GenerateTaskLink,
				InitTaskID:       createReq.InitTaskID,
				InitTaskLink:     createReq.InitTaskLink,
				IsManualMatched:  createReq.IsManualMatched,
				OwnerIP:          createReq.OwnerIP,
				Creator:          cts.Kit.User,
				Reviser:          cts.Kit.User,
			}
			devices = append(devices, device)
		}
		ids, err := svc.dao.ZiyanCvmDeviceInfo().CreateWithTx(cts.Kit, txn, devices)
		if err != nil {
			return nil, fmt.Errorf("create ziyan cvm device info failed, err: %v, devices: %+v", err, devices)
		}
		return ids, nil
	})
	if err != nil {
		return nil, err
	}

	ids, ok := deviceIDs.([]string)
	if !ok {
		return nil, fmt.Errorf("batch create ziyan cvm device info but return id type is not []string, "+
			"id type: %v", reflect.TypeOf(deviceIDs).String())
	}

	return &core.BatchCreateResult{IDs: ids}, nil
}
