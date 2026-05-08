/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2022 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package dao supplies all the apply order modify record related operations.
package dao

import (
	"encoding/json"
	"fmt"
	"time"

	"hcm/cmd/woa-server/dal/task/table"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/times"
)

// ModifyRecord supplies all the apply order modify record related operations.
type ModifyRecord interface {
	// CreateModifyRecord creates apply order modify record in db
	CreateModifyRecord(kt *kit.Kit, apiClientSet *client.ClientSet, inst *table.ModifyRecord) (string, error)
	// GetModifyRecord gets apply order modify record by filter from db
	GetModifyRecord(kt *kit.Kit, apiClientSet *client.ClientSet, filterExpr *filter.Expression) (
		*table.ModifyRecord, error)
	// CountModifyRecord gets apply order modify record count by filter from db
	CountModifyRecord(kt *kit.Kit, apiClientSet *client.ClientSet, filterExpr *filter.Expression) (uint64, error)
	// FindManyModifyRecord gets modify record list by filter from db
	FindManyModifyRecord(kt *kit.Kit, apiClientSet *client.ClientSet, filterExpr *filter.Expression,
		page *core.BasePage) ([]*table.ModifyRecord, error)
	// UpdateModifyRecord updates apply order modify record by filter and doc in db
	UpdateModifyRecord(kt *kit.Kit, apiClientSet *client.ClientSet, filterExpr *filter.Expression,
		updateData *cvmapplyproto.ZiyanCvmModifyRecordUpdateReq) error
}

var _ ModifyRecord = new(modifyRecordDao)

type modifyRecordDao struct {
}

// CreateModifyRecord creates apply order modify record in db
func (m *modifyRecordDao) CreateModifyRecord(kt *kit.Kit, apiClientSet *client.ClientSet,
	inst *table.ModifyRecord) (string, error) {

	if apiClientSet == nil {
		return "", fmt.Errorf("data service client not initialized")
	}

	createReq, err := convertToCreateReq(inst)
	if err != nil {
		logs.Errorf("convert modify record to create req failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmModifyRecordReq{
		Records: []cvmapplyproto.ZiyanCvmModifyRecordCreateReq{*createReq},
	}

	resp, err := apiClientSet.DataService().TCloudZiyan.ZiyanCvmModifyRecord.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("create modify record failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	if len(resp.IDs) == 0 {
		return "", errf.Newf(errf.InvalidParameter, "create generate record failed, ids is empty, resp: %+v", resp)
	}

	return resp.IDs[0], nil
}

// GetModifyRecord gets apply order modify record by filter from db
func (m *modifyRecordDao) GetModifyRecord(kt *kit.Kit, apiClientSet *client.ClientSet,
	filterExpr *filter.Expression) (*table.ModifyRecord, error) {

	if apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmModifyRecordListReq{
		Filter: filterExpr,
		Page:   &core.BasePage{Start: 0, Limit: 1},
	}

	resp, err := apiClientSet.DataService().TCloudZiyan.ZiyanCvmModifyRecord.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return nil, err
	}

	if len(resp.Details) == 0 {
		return nil, fmt.Errorf("modify record not found")
	}

	return convertMySQLToModifyRecord(resp.Details[0])
}

// CountModifyRecord gets apply order modify record count by filter from db
func (m *modifyRecordDao) CountModifyRecord(kt *kit.Kit, apiClientSet *client.ClientSet,
	filterExpr *filter.Expression) (uint64, error) {

	if apiClientSet == nil {
		return 0, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmModifyRecordListReq{
		Filter: filterExpr,
		Page:   &core.BasePage{Count: true, Start: 0, Limit: 1},
	}

	resp, err := apiClientSet.DataService().TCloudZiyan.ZiyanCvmModifyRecord.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return 0, err
	}

	return resp.Count, nil
}

// FindManyModifyRecord gets modify record list by filter from db
func (m *modifyRecordDao) FindManyModifyRecord(kt *kit.Kit, apiClientSet *client.ClientSet,
	filterExpr *filter.Expression, page *core.BasePage) ([]*table.ModifyRecord, error) {

	if apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmModifyRecordListReq{
		Filter: filterExpr,
		Page:   page,
	}
	resp, err := apiClientSet.DataService().TCloudZiyan.ZiyanCvmModifyRecord.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return nil, err
	}

	insts := make([]*table.ModifyRecord, 0, len(resp.Details))
	for _, mysqlRecord := range resp.Details {
		record, err := convertMySQLToModifyRecord(mysqlRecord)
		if err != nil {
			return nil, err
		}
		insts = append(insts, record)
	}

	return insts, nil
}

// UpdateModifyRecord updates apply order modify record by filter and doc in db
func (m *modifyRecordDao) UpdateModifyRecord(kt *kit.Kit, apiClientSet *client.ClientSet,
	filterExpr *filter.Expression, updateData *cvmapplyproto.ZiyanCvmModifyRecordUpdateReq) error {

	if apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	if updateData == nil {
		return fmt.Errorf("update data is nil")
	}

	listReq := &cvmapplyproto.ZiyanCvmModifyRecordListReq{
		Filter: filterExpr,
		Page:   &core.BasePage{Start: 0, Limit: uint(constant.BatchOperationMaxLimit)},
	}
	for {
		listResp, err := apiClientSet.DataService().TCloudZiyan.ZiyanCvmModifyRecord.List(kt.Ctx, kt.Header(), listReq)
		if err != nil {
			return err
		}

		if len(listResp.Details) == 0 {
			break
		}

		updateReqs := make([]cvmapplyproto.ZiyanCvmModifyRecordUpdateReq, 0, len(listResp.Details))
		for _, record := range listResp.Details {
			updateReq := *updateData
			updateReq.ID = record.ID
			updateReqs = append(updateReqs, updateReq)
		}

		for start := 0; start < len(updateReqs); start += constant.BatchOperationMaxLimit {
			end := start + constant.BatchOperationMaxLimit
			if end > len(updateReqs) {
				end = len(updateReqs)
			}

			batchReq := &cvmapplyproto.BatchUpdateZiyanCvmModifyRecordReq{
				Records: updateReqs[start:end],
			}
			if err = apiClientSet.DataService().TCloudZiyan.ZiyanCvmModifyRecord.BatchUpdate(
				kt.Ctx, kt.Header(), batchReq); err != nil {
				return err
			}
		}

		if len(listResp.Details) < int(listReq.Page.Limit) {
			break
		}
		listReq.Page.Start += uint32(listReq.Page.Limit)
	}

	return nil
}

// convertToCreateReq converts table.ModifyRecord to ZiyanCvmModifyRecordCreateReq
func convertToCreateReq(inst *table.ModifyRecord) (*cvmapplyproto.ZiyanCvmModifyRecordCreateReq, error) {
	if inst == nil || inst.Details == nil || inst.Details.PreData == nil || inst.Details.CurData == nil {
		return nil, fmt.Errorf("modify record or details is nil")
	}

	preDataDiskJSON, err := convertToJsonField(inst.Details.PreData.DataDisk)
	if err != nil {
		return nil, fmt.Errorf("serialize pre_data_disk failed: %w", err)
	}
	preZonesJSON, err := convertToJsonField(inst.Details.PreData.Zones)
	if err != nil {
		return nil, fmt.Errorf("serialize pre_zones failed: %w", err)
	}
	curDataDiskJSON, err := convertToJsonField(inst.Details.CurData.DataDisk)
	if err != nil {
		return nil, fmt.Errorf("serialize cur_data_disk failed: %w", err)
	}
	curZonesJSON, err := convertToJsonField(inst.Details.CurData.Zones)
	if err != nil {
		return nil, fmt.Errorf("serialize cur_zones failed: %w", err)
	}

	createReq := &cvmapplyproto.ZiyanCvmModifyRecordCreateReq{
		SuborderID:           inst.SuborderID,
		BkUsername:           inst.User,
		PreTotalNum:          cvt.ValToPtr(inst.Details.PreData.TotalNum),
		PreReplicas:          cvt.ValToPtr(inst.Details.PreData.Replicas),
		PreRegion:            inst.Details.PreData.Region,
		PreZone:              inst.Details.PreData.Zone,
		PreDeviceType:        inst.Details.PreData.DeviceType,
		PreImageID:           inst.Details.PreData.ImageId,
		PreDiskSize:          cvt.ValToPtr(int(inst.Details.PreData.DiskSize)),
		PreDiskType:          inst.Details.PreData.DiskType,
		PreNetworkType:       inst.Details.PreData.NetworkType,
		PreVpc:               inst.Details.PreData.Vpc,
		PreSubnet:            inst.Details.PreData.Subnet,
		PreSystemDiskType:    inst.Details.PreData.SystemDisk.DiskType,
		PreSystemDiskSize:    cvt.ValToPtr(int(inst.Details.PreData.SystemDisk.DiskSize)),
		PreSystemDiskNum:     cvt.ValToPtr(int(inst.Details.PreData.SystemDisk.DiskNum)),
		PreDataDisk:          preDataDiskJSON,
		PreZones:             preZonesJSON,
		PreResAssign:         inst.Details.PreData.ResAssign,
		PreBkAssetID:         inst.Details.PreData.BkAssetID,
		PreInheritInstanceID: inst.Details.PreData.InheritInstanceID,
		CurTotalNum:          cvt.ValToPtr(inst.Details.CurData.TotalNum),
		CurReplicas:          cvt.ValToPtr(inst.Details.CurData.Replicas),
		CurRegion:            inst.Details.CurData.Region,
		CurZone:              inst.Details.CurData.Zone,
		CurDeviceType:        inst.Details.CurData.DeviceType,
		CurImageID:           inst.Details.CurData.ImageId,
		CurDiskSize:          cvt.ValToPtr(int(inst.Details.CurData.DiskSize)),
		CurDiskType:          inst.Details.CurData.DiskType,
		CurNetworkType:       inst.Details.CurData.NetworkType,
		CurVpc:               inst.Details.CurData.Vpc,
		CurSubnet:            inst.Details.CurData.Subnet,
		CurSystemDiskType:    inst.Details.CurData.SystemDisk.DiskType,
		CurSystemDiskSize:    cvt.ValToPtr(int(inst.Details.CurData.SystemDisk.DiskSize)),
		CurSystemDiskNum:     cvt.ValToPtr(int(inst.Details.CurData.SystemDisk.DiskNum)),
		CurDataDisk:          curDataDiskJSON,
		CurZones:             curZonesJSON,
		CurResAssign:         inst.Details.CurData.ResAssign,
		CurBkAssetID:         inst.Details.CurData.BkAssetID,
		CurInheritInstanceID: inst.Details.CurData.InheritInstanceID,
		Status:               inst.Status,
		Approver:             inst.Approver,
		CreatedAt:            tabletypes.Time(inst.CreatedAt.In(time.Local).Format(constant.TimeStdFormat)),
		UpdatedAt:            tabletypes.Time(inst.UpdatedAt.In(time.Local).Format(constant.TimeStdFormat)),
	}

	return createReq, nil
}

func convertToJsonField(data interface{}) (tabletypes.JsonField, error) {
	if data == nil {
		return tabletypes.JsonField("[]"), nil
	}
	if jf, ok := data.(tabletypes.JsonField); ok {
		return jf, nil
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return tabletypes.JsonField("[]"), err
	}
	return tabletypes.JsonField(jsonData), nil
}

// unmarshalDataDisk unmarshals JSON data to DiskSpec slice
func unmarshalDataDisk(jsonData tabletypes.JsonField) ([]enumor.DiskSpec, error) {
	var dataDisk []enumor.DiskSpec
	if len(jsonData) == 0 {
		return dataDisk, nil
	}

	if err := json.Unmarshal([]byte(jsonData), &dataDisk); err == nil {
		filteredDataDisk := make([]enumor.DiskSpec, 0, len(dataDisk))
		for _, disk := range dataDisk {
			if disk != (enumor.DiskSpec{}) {
				filteredDataDisk = append(filteredDataDisk, disk)
			}
		}
		return filteredDataDisk, nil
	}

	var singleDisk enumor.DiskSpec
	if err := json.Unmarshal([]byte(jsonData), &singleDisk); err == nil {
		if singleDisk == (enumor.DiskSpec{}) {
			return dataDisk, nil
		}
		return []enumor.DiskSpec{singleDisk}, nil
	}

	return nil, fmt.Errorf("unsupported data_disk json: %s", jsonData)
}

// unmarshalZones unmarshals JSON data to string slice
func unmarshalZones(jsonData tabletypes.JsonField) ([]string, error) {
	var zones []string
	if len(jsonData) == 0 {
		return zones, nil
	}

	if err := json.Unmarshal([]byte(jsonData), &zones); err == nil {
		filteredZones := make([]string, 0, len(zones))
		for _, zone := range zones {
			if zone != "" {
				filteredZones = append(filteredZones, zone)
			}
		}
		return filteredZones, nil
	}

	var singleZone string
	if err := json.Unmarshal([]byte(jsonData), &singleZone); err == nil {
		if singleZone == "" {
			return zones, nil
		}
		return []string{singleZone}, nil
	}

	return nil, fmt.Errorf("unsupported zones json: %s", jsonData)
}

// buildSystemDisk builds DiskSpec from MySQL fields
func buildSystemDisk(diskType enumor.DiskType, diskSize, diskNum *int) enumor.DiskSpec {
	if diskType == "" {
		return enumor.DiskSpec{}
	}
	return enumor.DiskSpec{
		DiskType: diskType,
		DiskSize: uint(cvt.PtrToVal(diskSize)),
		DiskNum:  uint(cvt.PtrToVal(diskNum)),
	}
}

// buildPreModifyData builds PreData from MySQL record
func buildPreModifyData(mysqlRecord *cvmapplytable.ZiyanCvmModifyRecord,
	preSystemDisk enumor.DiskSpec, preDataDisk []enumor.DiskSpec, preZones []string) *table.ModifyData {

	return &table.ModifyData{
		TotalNum:          cvt.PtrToVal(mysqlRecord.PreTotalNum),
		Replicas:          cvt.PtrToVal(mysqlRecord.PreReplicas),
		Region:            mysqlRecord.PreRegion,
		Zone:              mysqlRecord.PreZone,
		DeviceType:        mysqlRecord.PreDeviceType,
		ImageId:           mysqlRecord.PreImageID,
		DiskSize:          int64(cvt.PtrToVal(mysqlRecord.PreDiskSize)),
		DiskType:          mysqlRecord.PreDiskType,
		NetworkType:       mysqlRecord.PreNetworkType,
		Vpc:               mysqlRecord.PreVpc,
		Subnet:            mysqlRecord.PreSubnet,
		SystemDisk:        preSystemDisk,
		DataDisk:          preDataDisk,
		Zones:             preZones,
		ResAssign:         mysqlRecord.PreResAssign,
		BkAssetID:         mysqlRecord.PreBkAssetID,
		InheritInstanceID: mysqlRecord.PreInheritInstanceID,
	}
}

// buildCurModifyData builds CurData from MySQL record
func buildCurModifyData(mysqlRecord *cvmapplytable.ZiyanCvmModifyRecord,
	curSystemDisk enumor.DiskSpec, curDataDisk []enumor.DiskSpec, curZones []string) *table.ModifyData {
	return &table.ModifyData{
		TotalNum:          cvt.PtrToVal(mysqlRecord.CurTotalNum),
		Replicas:          cvt.PtrToVal(mysqlRecord.CurReplicas),
		Region:            mysqlRecord.CurRegion,
		Zone:              mysqlRecord.CurZone,
		DeviceType:        mysqlRecord.CurDeviceType,
		ImageId:           mysqlRecord.CurImageID,
		DiskSize:          int64(cvt.PtrToVal(mysqlRecord.CurDiskSize)),
		DiskType:          mysqlRecord.CurDiskType,
		NetworkType:       mysqlRecord.CurNetworkType,
		Vpc:               mysqlRecord.CurVpc,
		Subnet:            mysqlRecord.CurSubnet,
		SystemDisk:        curSystemDisk,
		DataDisk:          curDataDisk,
		Zones:             curZones,
		ResAssign:         mysqlRecord.CurResAssign,
		BkAssetID:         mysqlRecord.CurBkAssetID,
		InheritInstanceID: mysqlRecord.CurInheritInstanceID,
	}
}

// convertMySQLToModifyRecord converts MySQL record to table.ModifyRecord
func convertMySQLToModifyRecord(mysqlRecord *cvmapplytable.ZiyanCvmModifyRecord) (*table.ModifyRecord, error) {
	// Unmarshal JSON fields
	preDataDisk, err := unmarshalDataDisk(mysqlRecord.PreDataDisk)
	if err != nil {
		return nil, fmt.Errorf("unmarshal pre_data_disk failed: %w", err)
	}
	preZones, err := unmarshalZones(mysqlRecord.PreZones)
	if err != nil {
		return nil, fmt.Errorf("unmarshal pre_zones failed: %w", err)
	}
	curDataDisk, err := unmarshalDataDisk(mysqlRecord.CurDataDisk)
	if err != nil {
		return nil, fmt.Errorf("unmarshal cur_data_disk failed: %w", err)
	}
	curZones, err := unmarshalZones(mysqlRecord.CurZones)
	if err != nil {
		return nil, fmt.Errorf("unmarshal cur_zones failed: %w", err)
	}

	// Build system disks
	preSystemDisk := buildSystemDisk(
		mysqlRecord.PreSystemDiskType, mysqlRecord.PreSystemDiskSize, mysqlRecord.PreSystemDiskNum)
	curSystemDisk := buildSystemDisk(
		mysqlRecord.CurSystemDiskType, mysqlRecord.CurSystemDiskSize, mysqlRecord.CurSystemDiskNum)

	// Build modify data
	preData := buildPreModifyData(mysqlRecord, preSystemDisk, preDataDisk, preZones)
	curData := buildCurModifyData(mysqlRecord, curSystemDisk, curDataDisk, curZones)

	// Parse timestamps
	createAt, err := times.ParseTypesTime(mysqlRecord.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at failed: %w", err)
	}
	updateAt, err := times.ParseTypesTime(mysqlRecord.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at failed: %w", err)
	}

	return &table.ModifyRecord{
		ID:         mysqlRecord.ID,
		SuborderID: mysqlRecord.SuborderID,
		User:       mysqlRecord.BkUsername,
		Details: &table.ModifyDetail{
			PreData: preData,
			CurData: curData,
		},
		CreatedAt: createAt,
		UpdatedAt: updateAt,
		Status:    mysqlRecord.Status,
		Approver:  mysqlRecord.Approver,
	}, nil
}
