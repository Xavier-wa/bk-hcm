/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package model implements model layer
package model

import (
	"context"
	"fmt"

	daltypes "hcm/cmd/woa-server/storage/dal/types"
	"hcm/cmd/woa-server/storage/driver/mongodb"
	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/times"
)

type deviceInfo struct {
	apiClientSet *client.ClientSet
}

// CreateDeviceInfos create device infos in db
func (d *deviceInfo) CreateDeviceInfos(kt *kit.Kit, insts []*types.DeviceInfo) error {
	if d.apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	createReqs := make([]cvmapplyproto.ZiyanCvmDeviceInfoCreateReq, 0, len(insts))
	for _, inst := range insts {
		createReqs = append(createReqs, cvmapplyproto.ZiyanCvmDeviceInfoCreateReq{
			OrderID:          int64(inst.OrderId),
			SuborderID:       inst.SubOrderId,
			GenerateID:       inst.GenerateId,
			BkBizID:          int64(inst.BkBizId),
			BkUsername:       inst.User,
			BkHostID:         inst.BkHostId,
			IP:               inst.Ip,
			AssetID:          inst.AssetId,
			InstanceID:       inst.InstanceID,
			RequireType:      inst.RequireType,
			ResourceType:     inst.ResourceType,
			DeviceType:       inst.DeviceType,
			Description:      inst.Description,
			Remark:           inst.Remark,
			ZoneName:         inst.ZoneName,
			ZoneID:           int64(inst.ZoneID),
			CloudZone:        inst.CloudZone,
			CloudRegion:      inst.CloudRegion,
			ModuleName:       inst.ModuleName,
			RackID:           inst.Equipment,
			IsMatched:        inst.IsMatched,
			IsChecked:        inst.IsChecked,
			IsInited:         inst.IsInited,
			IsDelivered:      inst.IsDelivered,
			Deliverer:        inst.Deliverer,
			GenerateTaskID:   inst.GenerateTaskId,
			GenerateTaskLink: inst.GenerateTaskLink,
			InitTaskID:       inst.InitTaskId,
			InitTaskLink:     inst.InitTaskLink,
			IsManualMatched:  inst.IsManualMatched,
			OwnerIP:          inst.OwnerIP,
		})
	}

	// 分批创建，每批最多 BatchOperationMaxLimit(100) 个
	for start := 0; start < len(createReqs); start += constant.BatchOperationMaxLimit {
		end := start + constant.BatchOperationMaxLimit
		if end > len(createReqs) {
			end = len(createReqs)
		}

		req := &cvmapplyproto.BatchCreateZiyanCvmDeviceInfoReq{
			Devices: createReqs[start:end],
		}

		_, err := d.apiClientSet.DataService().TCloudZiyan.ZiyanCvmDeviceInfo.BatchCreate(kt.Ctx, kt.Header(), req)
		if err != nil {
			logs.Errorf("batch create device infos failed, batch [%d:%d], total: %d, err: %v, rid: %s",
				start, end, len(createReqs), err, kt.Rid)
			return err
		}
	}

	return nil
}

// GetDeviceInfo gets device info by filter from db
func (d *deviceInfo) GetDeviceInfo(kt *kit.Kit, filterExpr *filter.Expression) (
	[]*types.DeviceInfo, error) {

	if d.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmDeviceInfoListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}

	insts := make([]*types.DeviceInfo, 0)
	for {
		resp, err := d.apiClientSet.DataService().TCloudZiyan.ZiyanCvmDeviceInfo.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, mysqlRecord := range resp.Details {
			deviceItem, err := convertMySQLToDeviceInfo(mysqlRecord)
			if err != nil {
				return nil, err
			}
			insts = append(insts, deviceItem)
		}

		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	return insts, nil
}

// CountDeviceInfo gets apply order device info count by filter from db
func (d *deviceInfo) CountDeviceInfo(kt *kit.Kit, filterExpr *filter.Expression) (
	uint64, error) {

	if d.apiClientSet == nil {
		return 0, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmDeviceInfoListReq{
		Filter: filterExpr,
		Page:   core.NewCountPage(),
	}

	resp, err := d.apiClientSet.DataService().TCloudZiyan.ZiyanCvmDeviceInfo.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return 0, err
	}

	return resp.Count, nil
}

// FindManyDeviceInfo gets device info list by filter from db
func (d *deviceInfo) FindManyDeviceInfo(kt *kit.Kit, filterExpr *filter.Expression,
	page *core.BasePage) ([]*types.DeviceInfo, error) {

	if d.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	isAll := false
	if page == nil || page.Limit == 0 {
		page = core.NewDefaultBasePage()
		isAll = true
	}

	insts := make([]*types.DeviceInfo, 0)
	req := &cvmapplyproto.ZiyanCvmDeviceInfoListReq{
		Filter: filterExpr,
		Page:   page,
	}
	for {
		resp, err := d.apiClientSet.DataService().TCloudZiyan.ZiyanCvmDeviceInfo.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, mysqlRecord := range resp.Details {
			deviceItem, err := convertMySQLToDeviceInfo(mysqlRecord)
			if err != nil {
				return nil, err
			}
			insts = append(insts, deviceItem)
		}

		if !isAll || len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	return insts, nil
}

// UpdateDeviceInfo updates device info by filter and doc in db
func (d *deviceInfo) UpdateDeviceInfo(kt *kit.Kit, filterExpr *filter.Expression,
	updateData *cvmapplyproto.ZiyanCvmDeviceInfoUpdateReq) error {

	if d.apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	// 分页查询所有匹配的记录
	listReq := &cvmapplyproto.ZiyanCvmDeviceInfoListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}

	updateReqs := make([]cvmapplyproto.ZiyanCvmDeviceInfoUpdateReq, 0)
	for {
		listResp, err := d.apiClientSet.DataService().TCloudZiyan.ZiyanCvmDeviceInfo.List(kt.Ctx, kt.Header(), listReq)
		if err != nil {
			return err
		}

		for _, record := range listResp.Details {
			updateReq := *updateData
			updateReq.ID = record.ID
			updateReqs = append(updateReqs, updateReq)
		}

		if len(listResp.Details) < int(listReq.Page.Limit) {
			break
		}
		listReq.Page.Start += uint32(listReq.Page.Limit)
	}

	if len(updateReqs) == 0 {
		return nil
	}

	// 分批更新，每批最多 BatchOperationMaxLimit(100) 个
	for start := 0; start < len(updateReqs); start += constant.BatchOperationMaxLimit {
		end := start + constant.BatchOperationMaxLimit
		if end > len(updateReqs) {
			end = len(updateReqs)
		}

		batchReq := &cvmapplyproto.BatchUpdateZiyanCvmDeviceInfoReq{
			Devices: updateReqs[start:end],
		}

		if err := d.apiClientSet.DataService().TCloudZiyan.ZiyanCvmDeviceInfo.BatchUpdate(
			kt.Ctx, kt.Header(), batchReq); err != nil {
			logs.Errorf("batch update device infos failed, batch [%d:%d], total: %d, err: %v, rid: %s",
				start, end, len(updateReqs), err, kt.Rid)
			return err
		}
	}

	return nil
}

// AggregateAll device info aggregate all operation
func (d *deviceInfo) AggregateAll(ctx context.Context, pipeline interface{}, result interface{},
	opts ...*daltypes.AggregateOpts) error {

	if err := mongodb.Client().Table(pkg.BKTableNameDeviceInfo).AggregateAll(ctx, pipeline, result,
		opts...); err != nil {
		return err
	}

	return nil
}

// Distinct gets device info distinct result from db
func (d *deviceInfo) Distinct(ctx context.Context, field string, filter map[string]interface{}) (
	[]interface{}, error) {
	insts, err := mongodb.Client().Table(pkg.BKTableNameDeviceInfo).Distinct(ctx, field, filter)
	if err != nil {
		return nil, err
	}

	return insts, nil
}

// convertMySQLToDeviceInfo converts MySQL record to types.DeviceInfo
func convertMySQLToDeviceInfo(mysqlRecord *cvmapplytable.ZiyanCvmDeviceInfo) (*types.DeviceInfo, error) {
	createdAt, err := times.ParseTypesTime(mysqlRecord.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at failed: %w", err)
	}
	updatedAt, err := times.ParseTypesTime(mysqlRecord.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at failed: %w", err)
	}

	return &types.DeviceInfo{
		OrderId:          uint64(mysqlRecord.OrderID),
		SubOrderId:       mysqlRecord.SuborderID,
		GenerateId:       mysqlRecord.GenerateID,
		BkBizId:          int(mysqlRecord.BkBizID),
		User:             mysqlRecord.BkUsername,
		BkHostId:         mysqlRecord.BkHostID,
		Ip:               mysqlRecord.IP,
		AssetId:          mysqlRecord.AssetID,
		InstanceID:       mysqlRecord.InstanceID,
		RequireType:      mysqlRecord.RequireType,
		ResourceType:     mysqlRecord.ResourceType,
		DeviceType:       mysqlRecord.DeviceType,
		Description:      mysqlRecord.Description,
		Remark:           mysqlRecord.Remark,
		ZoneName:         mysqlRecord.ZoneName,
		ZoneID:           int(mysqlRecord.ZoneID),
		CloudZone:        mysqlRecord.CloudZone,
		CloudRegion:      mysqlRecord.CloudRegion,
		ModuleName:       mysqlRecord.ModuleName,
		Equipment:        mysqlRecord.RackID,
		IsMatched:        cvt.PtrToVal(mysqlRecord.IsMatched),
		IsChecked:        cvt.PtrToVal(mysqlRecord.IsChecked),
		IsInited:         cvt.PtrToVal(mysqlRecord.IsInited),
		IsDelivered:      cvt.PtrToVal(mysqlRecord.IsDelivered),
		Deliverer:        mysqlRecord.Deliverer,
		GenerateTaskId:   mysqlRecord.GenerateTaskID,
		GenerateTaskLink: mysqlRecord.GenerateTaskLink,
		InitTaskId:       mysqlRecord.InitTaskID,
		InitTaskLink:     mysqlRecord.InitTaskLink,
		IsManualMatched:  mysqlRecord.IsManualMatched,
		OwnerIP:          mysqlRecord.OwnerIP,
		CreateAt:         createdAt,
		UpdateAt:         updatedAt,
	}, nil
}
