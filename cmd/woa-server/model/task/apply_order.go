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

// Package model implements all db operations of apply order
package model

import (
	"context"
	"encoding/json"
	"fmt"

	daltypes "hcm/cmd/woa-server/storage/dal/types"
	"hcm/cmd/woa-server/storage/driver/mongodb"
	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	tabletype "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/times"
)

type applyOrder struct {
	apiClientSet *client.ClientSet
}

// CreateApplyOrder creates apply order in db
func (a *applyOrder) CreateApplyOrder(kt *kit.Kit, inst *types.ApplyOrder) error {
	if a.apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	createReq, err := buildApplySuborderCreateReq(inst)
	if err != nil {
		return err
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmApplySuborderReq{
		ApplySuborders: []cvmapplyproto.ZiyanCvmApplySuborderCreateReq{createReq},
	}

	_, err = a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplySuborder.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("create suborder failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// buildApplySuborderCreateReq builds create request from ApplyOrder
func buildApplySuborderCreateReq(inst *types.ApplyOrder) (cvmapplyproto.ZiyanCvmApplySuborderCreateReq, error) {
	var followerJSON tabletype.JsonField
	var err error
	if len(inst.Follower) > 0 {
		followerJSON, err = MarshalToJsonField(inst.Follower)
		if err != nil {
			return cvmapplyproto.ZiyanCvmApplySuborderCreateReq{}, fmt.Errorf("marshal follower failed: %v", err)
		}
	}

	createReq := cvmapplyproto.ZiyanCvmApplySuborderCreateReq{
		SuborderID:        inst.SubOrderId,
		OrderID:           inst.OrderId,
		BkBizID:           inst.BkBizId,
		BkUsername:        inst.User,
		Follower:          followerJSON,
		Auditor:           inst.Auditor,
		Source:            inst.Source,
		ProductType:       inst.ProductType,
		RequireType:       inst.RequireType,
		ExpectTime:        inst.ExpectTime,
		ResourceType:      inst.ResourceType,
		AntiAffinityLevel: inst.AntiAffinityLevel,
		EnableDiskCheck:   cvt.ValToPtr(inst.EnableDiskCheck),
		ObsProject:        inst.ObsProject,
		Description:       inst.Description,
		Remark:            inst.Remark,
		Stage:             inst.Stage,
		Status:            inst.Status,
		RetryTime:         inst.RetryTime,
		ModifyTime:        inst.ModifyTime,
		OriginNum:         inst.OriginNum,
		TotalNum:          inst.TotalNum,
		SuccessNum:        inst.SuccessNum,
		PendingNum:        inst.PendingNum,
		AppliedCore:       inst.AppliedCore,
	}

	if err = fillSpecFields(&createReq, inst.Spec); err != nil {
		return cvmapplyproto.ZiyanCvmApplySuborderCreateReq{}, err
	}

	if err = fillUpgradeCVMList(&createReq, inst.UpgradeCVMList); err != nil {
		return cvmapplyproto.ZiyanCvmApplySuborderCreateReq{}, err
	}

	return createReq, nil
}

// fillSpecFields fills spec fields into create request
func fillSpecFields(createReq *cvmapplyproto.ZiyanCvmApplySuborderCreateReq, spec *types.ResourceSpec) error {
	if spec == nil {
		return nil
	}

	createReq.Region = spec.Region
	createReq.Zone = spec.Zone
	createReq.DeviceGroup = spec.DeviceGroup
	createReq.DeviceSize = spec.DeviceSize
	createReq.DeviceType = spec.DeviceType
	createReq.ImageID = spec.ImageId
	createReq.Image = spec.Image
	createReq.DiskSize = spec.DiskSize
	createReq.DiskType = spec.DiskType
	createReq.NetworkType = spec.NetworkType
	createReq.Vpc = spec.Vpc
	createReq.Subnet = spec.Subnet
	createReq.OsType = spec.OsType
	createReq.RaidType = spec.RaidType
	createReq.Isp = spec.Isp
	createReq.ChargeType = spec.ChargeType
	createReq.ChargeMonths = spec.ChargeMonths
	createReq.InheritInstanceID = spec.InheritInstanceId
	createReq.BkAssetID = spec.BkAssetID
	createReq.ResAssign = spec.ResAssign
	createReq.CPUThreadSwitch = spec.CPUThreadSwitch

	return fillSpecJSONFields(createReq, spec)
}

// fillSpecJSONFields fills JSON fields in spec
func fillSpecJSONFields(createReq *cvmapplyproto.ZiyanCvmApplySuborderCreateReq, spec *types.ResourceSpec) error {
	var err error

	if len(spec.FailedZoneIDs) > 0 {
		createReq.FailedZoneIds, err = MarshalToJsonField(spec.FailedZoneIDs)
		if err != nil {
			return fmt.Errorf("marshal failed zone ids failed: %v", err)
		}
	}

	createReq.SystemDisk, err = MarshalToJsonField(spec.SystemDisk)
	if err != nil {
		return fmt.Errorf("marshal system disk failed: %v", err)
	}

	if spec.DataDisk != nil {
		createReq.DataDisk, err = MarshalToJsonField(spec.DataDisk)
		if err != nil {
			return fmt.Errorf("marshal data disk failed: %v", err)
		}
	}

	if len(spec.Zones) > 0 {
		createReq.Zones, err = MarshalToJsonField(spec.Zones)
		if err != nil {
			return fmt.Errorf("marshal zones failed: %v", err)
		}
	}

	return nil
}

// fillUpgradeCVMList fills upgrade CVM list into create request
func fillUpgradeCVMList(createReq *cvmapplyproto.ZiyanCvmApplySuborderCreateReq, upgradeCVMList interface{}) error {
	if upgradeCVMList == nil {
		return nil
	}

	upgradeCVMListJSON, err := MarshalToJsonField(upgradeCVMList)
	if err != nil {
		return fmt.Errorf("marshal upgrade cvm list failed: %v", err)
	}

	createReq.UpgradeCvmList = upgradeCVMListJSON
	return nil
}

// MarshalToJsonField marshals data to JsonField
func MarshalToJsonField(data interface{}) (tabletype.JsonField, error) {
	if data == nil {
		return "", nil
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return tabletype.JsonField(jsonBytes), nil
}

// GetApplyOrder gets apply order by filter from db
func (a *applyOrder) GetApplyOrder(kt *kit.Kit, filterExpr *filter.Expression) (
	*types.ApplyOrder, error) {

	if a.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmApplySuborderListReq{
		Filter: filterExpr,
		Page:   &core.BasePage{Limit: 1},
	}
	resp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplySuborder.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return nil, err
	}

	if len(resp.Details) == 0 {
		return nil, fmt.Errorf("apply order not found")
	}

	// Convert MySQL record to types.ApplyOrder
	return ConvertMySQLToApplyOrder(resp.Details[0])
}

// CountApplyOrder gets apply order count by filter from db
func (a *applyOrder) CountApplyOrder(kt *kit.Kit, filterExpr *filter.Expression) (
	uint64, error) {

	if a.apiClientSet == nil {
		return 0, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmApplySuborderListReq{
		Filter: filterExpr,
		Page:   core.NewCountPage(),
	}
	resp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplySuborder.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return 0, err
	}

	return resp.Count, nil
}

// FindManyApplyOrder gets apply order list by filter from db
func (a *applyOrder) FindManyApplyOrder(kt *kit.Kit, filterExpr *filter.Expression,
	page *core.BasePage) ([]*types.ApplyOrder, error) {

	if a.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	isAll := false
	if page == nil || page.Limit == 0 {
		page = core.NewDefaultBasePage()
		isAll = true
	}

	insts := make([]*types.ApplyOrder, 0)
	req := &cvmapplyproto.ZiyanCvmApplySuborderListReq{
		Filter: filterExpr,
		Page:   page,
	}
	for {
		resp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplySuborder.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, mysqlRecord := range resp.Details {
			inst, err := ConvertMySQLToApplyOrder(mysqlRecord)
			if err != nil {
				return nil, err
			}
			insts = append(insts, inst)
		}
		if !isAll || len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	return insts, nil
}

// UpdateApplyOrder updates apply order by filter and update data in db
func (a *applyOrder) UpdateApplyOrder(kt *kit.Kit, filterExpr *filter.Expression,
	updateData *cvmapplyproto.ZiyanCvmApplySuborderUpdateReq) error {

	if a.apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	// 循环分页拉取全量匹配记录，避免记录数超过单页上限导致漏更新
	listReq := &cvmapplyproto.ZiyanCvmApplySuborderListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}
	updateReqs := make([]cvmapplyproto.ZiyanCvmApplySuborderUpdateReq, 0)
	for {
		listResp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplySuborder.List(
			kt.Ctx, kt.Header(), listReq)
		if err != nil {
			return err
		}

		for _, record := range listResp.Details {
			updateReq := *updateData
			updateReq.SuborderID = record.SuborderID
			updateReqs = append(updateReqs, updateReq)
		}

		if len(listResp.Details) < int(listReq.Page.Limit) {
			break
		}
		listReq.Page.Start += uint32(listReq.Page.Limit)
	}

	if len(updateReqs) == 0 {
		filterExprJSON, err := json.Marshal(filterExpr)
		if err != nil {
			logs.Errorf("update apply order no records to update, marshal filterExpr failed, err: %+v, rid: %s",
				err, kt.Rid)
			return err
		}
		logs.Warnf("update apply order no records to update, filterExprJSON: %s, updateData: %v, rid: %s",
			filterExprJSON, cvt.PtrToVal(updateData), kt.Rid)
		return nil
	}

	// 分批更新，每批不超过 constant.BatchOperationMaxLimit(100) 条
	for start := 0; start < len(updateReqs); start += constant.BatchOperationMaxLimit {
		end := start + constant.BatchOperationMaxLimit
		if end > len(updateReqs) {
			end = len(updateReqs)
		}

		batchReq := &cvmapplyproto.BatchUpdateZiyanCvmApplySuborderReq{
			ApplySuborders: updateReqs[start:end],
		}
		if err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplySuborder.BatchUpdate(
			kt.Ctx, kt.Header(), batchReq); err != nil {
			return err
		}
	}
	return nil
}

// AggregateAll apply order aggregate all operation
func (a *applyOrder) AggregateAll(ctx context.Context, pipeline interface{}, result interface{},
	opts ...*daltypes.AggregateOpts) error {

	if err := mongodb.Client().Table(pkg.BKTableNameApplyOrder).AggregateAll(ctx, pipeline, result,
		opts...); err != nil {
		return err
	}

	return nil
}

// unmarshalJsonField 解析 JSON 字段，空值或 "{}" 时跳过。
func unmarshalJsonField(raw tabletype.JsonField, target interface{}, fieldName string) error {
	if len(raw) == 0 || raw == "{}" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		logs.Errorf("unmarshal %s failed, err: %v", fieldName, err)
		return err
	}
	return nil
}

// ConvertMySQLToApplyOrder converts MySQL record to types.ApplyOrder
func ConvertMySQLToApplyOrder(mysqlRecord *cvmapplytable.ZiyanCvmApplySuborder) (*types.ApplyOrder, error) {
	var failedZoneIDs, zones []string
	var systemDisk enumor.DiskSpec
	var dataDisk []enumor.DiskSpec
	for _, item := range []struct {
		raw   tabletype.JsonField
		dst   interface{}
		field string
	}{
		{mysqlRecord.FailedZoneIds, &failedZoneIDs, "failed_zone_ids"},
		{mysqlRecord.SystemDisk, &systemDisk, "system_disk"},
		{mysqlRecord.DataDisk, &dataDisk, "data_disk"},
		{mysqlRecord.Zones, &zones, "zones"},
	} {
		if err := unmarshalJsonField(item.raw, item.dst, item.field); err != nil {
			return nil, err
		}
	}
	createdAt, err := times.ParseTypesTime(mysqlRecord.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at failed: %w", err)
	}
	updatedAt, err := times.ParseTypesTime(mysqlRecord.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at failed: %w", err)
	}
	return &types.ApplyOrder{
		SubOrderId:    mysqlRecord.SuborderID,
		OrderId:       mysqlRecord.OrderID,
		BkBizId:       mysqlRecord.BkBizID,
		User:          mysqlRecord.BkUsername,
		Auditor:       mysqlRecord.Auditor,
		RequireType:   mysqlRecord.RequireType,
		ExpectTime:    mysqlRecord.ExpectTime,
		ResourceType:  mysqlRecord.ResourceType,
		Source:        mysqlRecord.Source,
		ProductType:   mysqlRecord.ProductType,
		Stage:         mysqlRecord.Stage,
		Status:        mysqlRecord.Status,
		OriginNum:     cvt.PtrToVal(mysqlRecord.OriginNum),
		TotalNum:      cvt.PtrToVal(mysqlRecord.TotalNum),
		SuccessNum:    cvt.PtrToVal(mysqlRecord.SuccessNum),
		PendingNum:    cvt.PtrToVal(mysqlRecord.PendingNum),
		AppliedCore:   cvt.PtrToVal(mysqlRecord.AppliedCore),
		DeliveredCore: cvt.PtrToVal(mysqlRecord.DeliveredCore),
		ObsProject:    mysqlRecord.ObsProject,
		RetryTime:     cvt.PtrToVal(mysqlRecord.RetryTime),
		ModifyTime:    cvt.PtrToVal(mysqlRecord.ModifyTime),
		CreateAt:      createdAt,
		UpdateAt:      updatedAt,
		Spec: &types.ResourceSpec{
			Region:            mysqlRecord.Region,
			Zone:              mysqlRecord.Zone,
			DeviceType:        mysqlRecord.DeviceType,
			ImageId:           mysqlRecord.ImageID,
			Image:             mysqlRecord.Image,
			DiskSize:          mysqlRecord.DiskSize,
			DiskType:          mysqlRecord.DiskType,
			NetworkType:       mysqlRecord.NetworkType,
			Vpc:               mysqlRecord.Vpc,
			Subnet:            mysqlRecord.Subnet,
			OsType:            mysqlRecord.OsType,
			RaidType:          mysqlRecord.RaidType,
			Isp:               mysqlRecord.Isp,
			ChargeType:        mysqlRecord.ChargeType,
			ChargeMonths:      mysqlRecord.ChargeMonths,
			InheritInstanceId: mysqlRecord.InheritInstanceID,
			BkAssetID:         mysqlRecord.BkAssetID,
			FailedZoneIDs:     failedZoneIDs,
			SystemDisk:        systemDisk,
			DataDisk:          dataDisk,
			Zones:             zones,
			ResAssign:         mysqlRecord.ResAssign,
			CPUThreadSwitch:   mysqlRecord.CPUThreadSwitch,
		},
	}, nil
}
