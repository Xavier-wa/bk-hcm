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

// Package model implements all db related operations.
package model

import (
	"fmt"

	types "hcm/cmd/woa-server/types/task"
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

type deliverRecord struct {
	apiClientSet *client.ClientSet
}

// CreateDeliverRecord creates apply order deliver record in db
func (d *deliverRecord) CreateDeliverRecord(kt *kit.Kit, inst *types.DeliverRecord) error {

	if d.apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmDeliverRecordReq{
		Records: []cvmapplyproto.ZiyanCvmDeliverRecordCreateReq{
			{
				SuborderID:       inst.SubOrderId,
				IP:               inst.Ip,
				AssetID:          inst.AssetId,
				Status:           inst.Status,
				Message:          inst.Message,
				Deliverer:        inst.Deliverer,
				GenerateTaskID:   inst.GenerateTaskId,
				GenerateTaskLink: inst.GenerateTaskLink,
				InitTaskID:       inst.InitTaskId,
				InitTaskLink:     inst.InitTaskLink,
				IsManualMatched:  inst.IsManualMatched,
				StartAt:          inst.StartAt.String(),
				EndAt:            inst.EndAt.String(),
			},
		},
	}

	_, err := d.apiClientSet.DataService().TCloudZiyan.ZiyanCvmDeliverRecord.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("create deliver record failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// GetDeliverRecord gets apply order deliver record by filter from db
func (d *deliverRecord) GetDeliverRecord(kt *kit.Kit, filterExpr *filter.Expression) (
	*types.DeliverRecord, error) {

	if d.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmDeliverRecordListReq{
		Filter: filterExpr,
		Page:   &core.BasePage{Limit: 1},
	}

	resp, err := d.apiClientSet.DataService().TCloudZiyan.ZiyanCvmDeliverRecord.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return nil, err
	}

	if len(resp.Details) == 0 {
		return nil, fmt.Errorf("deliver record not found")
	}

	return convertMySQLToDeliverRecord(kt, resp.Details[0])
}

// CountDeliverRecord gets apply order deliver record count by filter from db
func (d *deliverRecord) CountDeliverRecord(kt *kit.Kit, filterExpr *filter.Expression) (uint64, error) {

	if d.apiClientSet == nil {
		return 0, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmDeliverRecordListReq{
		Filter: filterExpr,
		Page:   core.NewCountPage(),
	}

	resp, err := d.apiClientSet.DataService().TCloudZiyan.ZiyanCvmDeliverRecord.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return 0, err
	}

	return resp.Count, nil
}

// FindManyDeliverRecord gets deliver record list by filter from db
func (d *deliverRecord) FindManyDeliverRecord(kt *kit.Kit, filterExpr *filter.Expression, page *core.BasePage) ([]*types.DeliverRecord, error) {

	if d.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmDeliverRecordListReq{
		Filter: filterExpr,
		Page:   page,
	}

	resp, err := d.apiClientSet.DataService().TCloudZiyan.ZiyanCvmDeliverRecord.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return nil, err
	}

	insts := make([]*types.DeliverRecord, 0, len(resp.Details))
	for _, mysqlRecord := range resp.Details {
		inst, err := convertMySQLToDeliverRecord(kt, mysqlRecord)
		if err != nil {
			return nil, err
		}
		insts = append(insts, inst)
	}

	return insts, nil
}

// UpdateDeliverRecord updates apply order deliver record by filter and doc in db
func (d *deliverRecord) UpdateDeliverRecord(kt *kit.Kit, filterExpr *filter.Expression,
	updateData *cvmapplyproto.ZiyanCvmDeliverRecordUpdateReq) error {

	if d.apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	// First, get the records to update
	listReq := &cvmapplyproto.ZiyanCvmDeliverRecordListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}

	listResp, err := d.apiClientSet.DataService().TCloudZiyan.ZiyanCvmDeliverRecord.List(kt.Ctx, kt.Header(), listReq)
	if err != nil {
		return err
	}

	if len(listResp.Details) == 0 {
		return nil
	}

	// Batch update
	updateReqs := make([]cvmapplyproto.ZiyanCvmDeliverRecordUpdateReq, 0, len(listResp.Details))
	for _, record := range listResp.Details {
		updateReq := *updateData
		updateReq.ID = record.ID
		updateReqs = append(updateReqs, updateReq)
	}

	batchReq := &cvmapplyproto.BatchUpdateZiyanCvmDeliverRecordReq{
		Records: updateReqs,
	}

	return d.apiClientSet.DataService().TCloudZiyan.ZiyanCvmDeliverRecord.BatchUpdate(kt.Ctx, kt.Header(), batchReq)
}

// convertMySQLToDeliverRecord converts MySQL record to types.DeliverRecord
func convertMySQLToDeliverRecord(kt *kit.Kit, mysqlRecord *cvmapplytable.ZiyanCvmDeliverRecord) (
	*types.DeliverRecord, error) {

	startAt, err := times.ParseDateTime(constant.TimeStdFormat, mysqlRecord.StartAt)
	if err != nil {
		logs.Errorf("parse start at failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	endAt, err := times.ParseDateTime(constant.TimeStdFormat, mysqlRecord.EndAt)
	if err != nil {
		logs.Errorf("parse end at failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	createdAt, err := times.ParseTypesTime(mysqlRecord.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at failed: %w", err)
	}
	updatedAt, err := times.ParseTypesTime(mysqlRecord.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at failed: %w", err)
	}

	return &types.DeliverRecord{
		SubOrderId:       mysqlRecord.SuborderID,
		Ip:               mysqlRecord.IP,
		AssetId:          mysqlRecord.AssetID,
		Status:           cvt.PtrToVal(mysqlRecord.Status),
		Message:          mysqlRecord.Message,
		Deliverer:        mysqlRecord.Deliverer,
		GenerateTaskId:   mysqlRecord.GenerateTaskID,
		GenerateTaskLink: mysqlRecord.GenerateTaskLink,
		InitTaskId:       mysqlRecord.InitTaskID,
		InitTaskLink:     mysqlRecord.InitTaskLink,
		IsManualMatched:  cvt.PtrToVal(mysqlRecord.IsManualMatched),
		StartAt:          startAt,
		EndAt:            endAt,
		CreateAt:         createdAt,
		UpdateAt:         updatedAt,
	}, nil
}
