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
	"fmt"

	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/times"
)

type initRecord struct {
	apiClientSet *client.ClientSet
}

// CreateInitRecord creates apply order init record in db
func (i *initRecord) CreateInitRecord(kt *kit.Kit, inst *types.InitRecord) error {
	if i.apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	createReq := cvmapplyproto.ZiyanCvmApplyInitTaskCreateReq{
		SuborderID: inst.SubOrderId,
		IP:         inst.Ip,
		TaskID:     inst.TaskId,
		TaskLink:   inst.TaskLink,
		Status:     inst.Status,
		Message:    inst.Message,
		StartAt:    times.ConvStdTimeFormat(inst.StartAt),
		EndAt:      times.ConvStdTimeFormat(inst.EndAt),
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmApplyInitTaskReq{
		InitTasks: []cvmapplyproto.ZiyanCvmApplyInitTaskCreateReq{createReq},
	}

	_, err := i.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyInitTask.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("create init record failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// GetInitRecord gets apply order init record by filter from db
func (i *initRecord) GetInitRecord(kt *kit.Kit, filterExpr *filter.Expression) (
	*types.InitRecord, error) {

	if i.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmApplyInitTaskListReq{
		Filter: filterExpr,
		Page:   &core.BasePage{Start: 0, Limit: 1},
	}

	resp, err := i.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyInitTask.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return nil, err
	}

	if len(resp.Details) == 0 {
		return nil, errf.Newf(errf.RecordNotFound, "init record not found")
	}

	return convertMySQLToInitRecord(resp.Details[0])
}

// CountInitRecord gets apply order init record count by filter from db
func (i *initRecord) CountInitRecord(kt *kit.Kit, filterExpr *filter.Expression) (
	uint64, error) {

	if i.apiClientSet == nil {
		return 0, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmApplyInitTaskListReq{
		Filter: filterExpr,
		Page:   core.NewCountPage(),
	}

	resp, err := i.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyInitTask.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return 0, err
	}

	return resp.Count, nil
}

// FindManyInitRecord gets init record list by filter from db
func (i *initRecord) FindManyInitRecord(kt *kit.Kit, filterExpr *filter.Expression,
	page *core.BasePage) ([]*types.InitRecord, error) {

	if i.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	isAll := false
	if page == nil || page.Limit == 0 {
		page = core.NewDefaultBasePage()
		isAll = true
	}

	insts := make([]*types.InitRecord, 0)
	req := &cvmapplyproto.ZiyanCvmApplyInitTaskListReq{
		Filter: filterExpr,
		Page:   page,
	}
	for {
		resp, err := i.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyInitTask.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, mysqlRecord := range resp.Details {
			record, err := convertMySQLToInitRecord(mysqlRecord)
			if err != nil {
				return nil, err
			}
			insts = append(insts, record)
		}

		if !isAll || len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	return insts, nil
}

// UpdateInitRecord updates apply order init record by filter and doc in db
func (i *initRecord) UpdateInitRecord(kt *kit.Kit, filterExpr *filter.Expression,
	updateData *cvmapplyproto.ZiyanCvmApplyInitTaskUpdateReq) error {

	if i.apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	// First, get the records to update
	listReq := &cvmapplyproto.ZiyanCvmApplyInitTaskListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}

	listResp, err := i.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyInitTask.List(kt.Ctx, kt.Header(), listReq)
	if err != nil {
		return err
	}

	if len(listResp.Details) == 0 {
		return nil
	}

	// Batch update
	updateReqs := make([]cvmapplyproto.ZiyanCvmApplyInitTaskUpdateReq, 0, len(listResp.Details))
	for _, record := range listResp.Details {
		updateReq := *updateData
		updateReq.ID = record.ID
		updateReqs = append(updateReqs, updateReq)
	}

	batchReq := &cvmapplyproto.BatchUpdateZiyanCvmApplyInitTaskReq{
		InitTasks: updateReqs,
	}

	return i.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyInitTask.BatchUpdate(kt.Ctx, kt.Header(), batchReq)
}

// convertMySQLToInitRecord converts MySQL record to types.InitRecord
func convertMySQLToInitRecord(mysqlRecord *cvmapplytable.ZiyanCvmApplyInitTask) (*types.InitRecord, error) {
	startAt, err := times.ParseDateTime(constant.TimeStdFormat, mysqlRecord.StartAt)
	if err != nil {
		return nil, err
	}

	endAt, err := times.ParseDateTime(constant.TimeStdFormat, mysqlRecord.EndAt)
	if err != nil {
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

	return &types.InitRecord{
		SubOrderId: mysqlRecord.SuborderID,
		Ip:         mysqlRecord.IP,
		TaskId:     mysqlRecord.TaskID,
		TaskLink:   mysqlRecord.TaskLink,
		Status:     cvt.PtrToVal(mysqlRecord.Status),
		Message:    mysqlRecord.Message,
		CreateAt:   createdAt,
		UpdateAt:   updatedAt,
		StartAt:    startAt,
		EndAt:      endAt,
	}, nil
}
