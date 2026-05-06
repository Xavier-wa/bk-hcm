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

// Package model implements the object model for the service.
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

type applyStep struct {
	apiClientSet *client.ClientSet
}

// CreateApplyStep creates apply order step info in db
func (a *applyStep) CreateApplyStep(kt *kit.Kit, inst *types.ApplyStep) error {
	if a.apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmApplyStepReq{
		ApplySteps: []cvmapplyproto.ZiyanCvmApplyStepCreateReq{
			{
				StepID:     inst.StepId,
				SuborderID: inst.SubOrderId,
				StepName:   inst.StepName,
				Status:     inst.Status,
				Message:    inst.Message,
				TotalNum:   inst.TotalNum,
				SuccessNum: inst.SuccessNum,
				FailedNum:  inst.FailedNum,
				RunningNum: inst.RunningNum,
				StartAt:    inst.StartAt.Format(constant.DateTimeLayout),
				EndAt:      inst.EndAt.Format(constant.DateTimeLayout),
			},
		},
	}

	_, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyStep.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("create apply step failed, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// GetApplyStep gets apply order step info by filter from db
func (a *applyStep) GetApplyStep(kt *kit.Kit, filterExpr *filter.Expression) (
	*types.ApplyStep, error) {

	if a.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmApplyStepListReq{
		Filter: filterExpr,
		Page:   &core.BasePage{Limit: 1},
	}

	resp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyStep.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return nil, err
	}

	if len(resp.Details) == 0 {
		return nil, fmt.Errorf("apply step not found")
	}

	return convertMySQLToApplyStep(resp.Details[0])
}

// CountApplyStep gets apply step count by filter from db
func (a *applyStep) CountApplyStep(kt *kit.Kit, filterExpr *filter.Expression) (
	uint64, error) {

	if a.apiClientSet == nil {
		return 0, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmApplyStepListReq{
		Filter: filterExpr,
		Page:   core.NewCountPage(),
	}

	resp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyStep.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return 0, err
	}

	return resp.Count, nil
}

// FindManyApplyStep gets apply order step info list by filter from db
func (a *applyStep) FindManyApplyStep(kt *kit.Kit, filterExpr *filter.Expression) (
	[]*types.ApplyStep, error) {

	if a.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmApplyStepListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}

	insts := make([]*types.ApplyStep, 0)
	for {
		resp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyStep.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, mysqlRecord := range resp.Details {
			step, err := convertMySQLToApplyStep(mysqlRecord)
			if err != nil {
				return nil, err
			}
			insts = append(insts, step)
		}

		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	return insts, nil
}

// UpdateApplyStep updates apply order step info by filter and doc in db
func (a *applyStep) UpdateApplyStep(kt *kit.Kit, filterExpr *filter.Expression,
	updateData *cvmapplyproto.ZiyanCvmApplyStepUpdateReq) error {

	if a.apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	// First, get the records to update
	listReq := &cvmapplyproto.ZiyanCvmApplyStepListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}

	listResp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyStep.List(kt.Ctx, kt.Header(), listReq)
	if err != nil {
		return err
	}

	if len(listResp.Details) == 0 {
		return nil
	}

	// Batch update
	updateReqs := make([]cvmapplyproto.ZiyanCvmApplyStepUpdateReq, 0, len(listResp.Details))
	for _, record := range listResp.Details {
		updateReq := *updateData
		updateReq.ID = record.ID
		updateReqs = append(updateReqs, updateReq)
	}

	batchReq := &cvmapplyproto.BatchUpdateZiyanCvmApplyStepReq{
		ApplySteps: updateReqs,
	}

	return a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyStep.BatchUpdate(kt.Ctx, kt.Header(), batchReq)
}

// convertMySQLToApplyStep converts MySQL record to types.ApplyStep
func convertMySQLToApplyStep(mysqlRecord *cvmapplytable.ZiyanCvmApplyStep) (*types.ApplyStep, error) {
	startAt, err := times.ParseDateTime(constant.TimeStdFormat, mysqlRecord.StartAt)
	if err != nil {
		logs.Errorf("parse start at failed, err: %v", err)
		return nil, err
	}

	endAt, err := times.ParseDateTime(constant.TimeStdFormat, mysqlRecord.EndAt)
	if err != nil {
		logs.Errorf("parse end at failed, err: %v", err)
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

	return &types.ApplyStep{
		StepId:     mysqlRecord.StepID,
		SubOrderId: mysqlRecord.SuborderID,
		StepName:   mysqlRecord.StepName,
		Status:     cvt.PtrToVal(mysqlRecord.Status),
		Message:    mysqlRecord.Message,
		TotalNum:   cvt.PtrToVal(mysqlRecord.TotalNum),
		SuccessNum: cvt.PtrToVal(mysqlRecord.SuccessNum),
		FailedNum:  cvt.PtrToVal(mysqlRecord.FailedNum),
		RunningNum: cvt.PtrToVal(mysqlRecord.RunningNum),
		CreateAt:   createdAt,
		UpdateAt:   updatedAt,
		StartAt:    startAt,
		EndAt:      endAt,
	}, nil
}
