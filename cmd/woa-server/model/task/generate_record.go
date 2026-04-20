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

// Package model implements all db operations of apply order generate record
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
	"hcm/pkg/criteria/errf"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/json"
	"hcm/pkg/tools/times"
)

type generateRecord struct {
	apiClientSet *client.ClientSet
}

// CreateGenerateRecord creates apply order generate record in db
func (g *generateRecord) CreateGenerateRecord(kt *kit.Kit, inst *types.GenerateRecord) (string, error) {
	if g.apiClientSet == nil {
		return "", fmt.Errorf("data service client not initialized")
	}

	// Convert SuccessList from []string to JsonField
	// 注意：即使是空切片也需要 marshal，否则 JsonField 零值会被序列化为 {} 而非 []
	successList := inst.SuccessList
	if successList == nil {
		successList = make([]string, 0)
	}
	jsonBytes, err := json.Marshal(successList)
	if err != nil {
		logs.Errorf("marshal success list failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}
	successListJSON := tabletypes.JsonField(jsonBytes)

	createReq := cvmapplyproto.ZiyanCvmGenerateRecordCreateReq{
		GenerateID:   inst.GenerateId,
		SuborderID:   inst.SubOrderId,
		GenerateType: inst.GenerateType,
		TaskID:       inst.TaskId,
		TaskLink:     inst.TaskLink,
		RequestInfo:  inst.RequestInfo,
		Status:       inst.Status,
		IsMatched:    inst.IsMatched,
		Message:      inst.Message,
		TotalNum:     inst.TotalNum,
		SuccessNum:   inst.SuccessNum,
		SuccessList:  successListJSON,
		StartAt:      times.ConvStdTimeFormat(inst.StartAt),
		EndAt:        times.ConvStdTimeFormat(inst.EndAt),
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmGenerateRecordReq{
		GenerateRecords: []cvmapplyproto.ZiyanCvmGenerateRecordCreateReq{createReq},
	}

	resp, err := g.apiClientSet.DataService().TCloudZiyan.ZiyanCvmGenerateRecord.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("create generate record failed, err: %v, rid: %s", err, kt.Rid)
		return "", err
	}

	if len(resp.IDs) == 0 {
		return "", errf.Newf(errf.InvalidParameter, "create generate record failed, ids is empty, resp: %+v", resp)
	}

	return resp.IDs[0], nil
}

// GetGenerateRecord gets apply order generate record by filter from db
func (g *generateRecord) GetGenerateRecord(kt *kit.Kit, filterExpr *filter.Expression) (*types.GenerateRecord, error) {

	if g.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmGenerateRecordListReq{
		Filter: filterExpr,
		Page:   &core.BasePage{Start: 0, Limit: 1},
	}

	resp, err := g.apiClientSet.DataService().TCloudZiyan.ZiyanCvmGenerateRecord.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return nil, err
	}

	if len(resp.Details) == 0 {
		return nil, fmt.Errorf("generate record not found")
	}

	return convertMySQLToGenerateRecord(resp.Details[0])
}

// CountGenerateRecord gets apply order generate record count by filter from db
func (g *generateRecord) CountGenerateRecord(kt *kit.Kit, filterExpr *filter.Expression) (uint64, error) {

	if g.apiClientSet == nil {
		return 0, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmGenerateRecordListReq{
		Filter: filterExpr,
		Page:   core.NewCountPage(),
	}

	resp, err := g.apiClientSet.DataService().TCloudZiyan.ZiyanCvmGenerateRecord.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return 0, err
	}

	return resp.Count, nil
}

// FindManyGenerateRecord gets generate record list by filter from db
func (g *generateRecord) FindManyGenerateRecord(kt *kit.Kit, filterExpr *filter.Expression, page *core.BasePage) (
	[]*types.GenerateRecord, error) {

	if g.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	isAll := false
	if page == nil || page.Limit == 0 {
		page = core.NewDefaultBasePage()
		isAll = true
	}

	insts := make([]*types.GenerateRecord, 0)
	req := &cvmapplyproto.ZiyanCvmGenerateRecordListReq{
		Filter: filterExpr,
		Page:   page,
	}
	for {
		resp, err := g.apiClientSet.DataService().TCloudZiyan.ZiyanCvmGenerateRecord.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, mysqlRecord := range resp.Details {
			record, err := convertMySQLToGenerateRecord(mysqlRecord)
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

// UpdateGenerateRecord updates apply order generate record by filter and doc in db
func (g *generateRecord) UpdateGenerateRecord(kt *kit.Kit, filterExpr *filter.Expression,
	updateData *cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq) error {

	if g.apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	if updateData == nil {
		return fmt.Errorf("update generate record data is nil")
	}

	listReq := &cvmapplyproto.ZiyanCvmGenerateRecordListReq{
		Filter: filterExpr,
		Page:   &core.BasePage{Start: 0, Limit: uint(constant.BatchOperationMaxLimit)},
	}
	for {
		listResp, err := g.apiClientSet.DataService().TCloudZiyan.ZiyanCvmGenerateRecord.List(
			kt.Ctx, kt.Header(), listReq)
		if err != nil {
			return err
		}

		if len(listResp.Details) == 0 {
			break
		}

		updateReqs := make([]cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq, 0, len(listResp.Details))
		for _, record := range listResp.Details {
			updateReq := *updateData
			updateReq.GenerateID = record.GenerateID
			updateReq.SuborderID = record.SuborderID
			updateReqs = append(updateReqs, updateReq)
		}

		for start := 0; start < len(updateReqs); start += constant.BatchOperationMaxLimit {
			end := start + constant.BatchOperationMaxLimit
			if end > len(updateReqs) {
				end = len(updateReqs)
			}

			batchReq := &cvmapplyproto.BatchUpdateZiyanCvmGenerateRecordReq{
				GenerateRecords: updateReqs[start:end],
			}
			if err = g.apiClientSet.DataService().TCloudZiyan.ZiyanCvmGenerateRecord.BatchUpdate(
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

// AggregateAll generate record aggregate all operation
func (g *generateRecord) AggregateAll(ctx context.Context, pipeline interface{}, result interface{},
	opts ...*daltypes.AggregateOpts) error {

	if err := mongodb.Client().Table(pkg.BKTableNameGenerateRecord).AggregateAll(ctx, pipeline, result,
		opts...); err != nil {
		return err
	}

	return nil
}

// convertMySQLToGenerateRecord converts MySQL record to types.GenerateRecord
func convertMySQLToGenerateRecord(mysqlRecord *cvmapplytable.ZiyanCvmGenerateRecord) (*types.GenerateRecord, error) {
	var successList []string
	rawSuccessList := string(mysqlRecord.SuccessList)
	if len(mysqlRecord.SuccessList) > 0 && rawSuccessList != "{}" {
		if err := json.Unmarshal([]byte(mysqlRecord.SuccessList), &successList); err != nil {
			return nil, err
		}
	}

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

	return &types.GenerateRecord{
		SubOrderId:      mysqlRecord.SuborderID,
		GenerateId:      mysqlRecord.GenerateID,
		GenerateType:    mysqlRecord.GenerateType,
		TaskId:          mysqlRecord.TaskID,
		TaskLink:        mysqlRecord.TaskLink,
		RequestInfo:     mysqlRecord.RequestInfo,
		Status:          cvt.PtrToVal(mysqlRecord.Status),
		IsMatched:       cvt.PtrToVal(mysqlRecord.IsMatched),
		Message:         mysqlRecord.Message,
		TotalNum:        cvt.PtrToVal(mysqlRecord.TotalNum),
		SuccessNum:      cvt.PtrToVal(mysqlRecord.SuccessNum),
		SuccessList:     successList,
		CreateAt:        createdAt,
		UpdateAt:        updatedAt,
		StartAt:         startAt,
		EndAt:           endAt,
		IsManualMatched: mysqlRecord.IsManualMatched,
	}, nil
}
