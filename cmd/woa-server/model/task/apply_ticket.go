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
	"hcm/pkg/criteria/errf"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	tabletype "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/times"
)

type applyTicket struct {
	apiClientSet *client.ClientSet
}

// CreateApplyTicket creates apply ticket in db
func (a *applyTicket) CreateApplyTicket(kt *kit.Kit, inst *types.ApplyTicket) (
	uint64, error) {

	if a.apiClientSet == nil {
		return 0, fmt.Errorf("data service client not initialized")
	}

	var followerJSON tabletype.JsonField
	var err error
	if len(inst.Follower) > 0 {
		followerJSON, err = MarshalToJsonField(inst.Follower)
		if err != nil {
			return 0, fmt.Errorf("marshal follower failed: %v", err)
		}
	}

	req := &cvmapplyproto.BatchCreateZiyanCvmApplyOrderReq{
		ApplyOrders: []cvmapplyproto.ZiyanCvmApplyOrderCreateReq{
			{
				ProductType:  inst.ProductType,
				ItsmTicketID: inst.ItsmTicketId,
				Stage:        inst.Stage,
				BkBizID:      inst.BkBizId,
				BkUsername:   inst.User,
				Follower:     followerJSON,
				EnableNotice: inst.EnableNotice,
				RequireType:  inst.RequireType,
				ExpectTime:   inst.ExpectTime,
				Remark:       inst.Remark,
				Suborders:    inst.Suborders,
			},
		},
	}

	resp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyOrder.BatchCreate(kt.Ctx, kt.Header(), req)
	if err != nil {
		logs.Errorf("create apply ticket failed, err: %v, inst: %+v, rid: %s", err, cvt.PtrToVal(inst), kt.Rid)
		return 0, err
	}
	if len(resp.IDs) == 0 {
		return 0, errf.Newf(errf.InvalidParameter, "create apply ticket failed, ids is empty, resp: %+v", resp)
	}

	return resp.IDs[0], nil
}

// GetApplyTicket gets apply ticket by filter from db
func (a *applyTicket) GetApplyTicket(kt *kit.Kit, filterExpr *filter.Expression) (
	*types.ApplyTicket, error) {

	if a.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmApplyOrderListReq{
		Filter: filterExpr,
		Page:   &core.BasePage{Limit: 1},
	}

	resp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyOrder.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return nil, err
	}

	if len(resp.Details) == 0 {
		return nil, fmt.Errorf("apply ticket not found")
	}

	return convertMySQLToApplyTicket(resp.Details[0])
}

// CountApplyTicket gets apply ticket count by filter from db
func (a *applyTicket) CountApplyTicket(kt *kit.Kit, filterExpr *filter.Expression) (
	uint64, error) {

	if a.apiClientSet == nil {
		return 0, fmt.Errorf("data service client not initialized")
	}

	req := &cvmapplyproto.ZiyanCvmApplyOrderListReq{
		Filter: filterExpr,
		Page:   core.NewCountPage(),
	}

	resp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyOrder.List(kt.Ctx, kt.Header(), req)
	if err != nil {
		return 0, err
	}

	return resp.Count, nil
}

// FindManyApplyTicket gets apply ticket list by filter from db
func (a *applyTicket) FindManyApplyTicket(kt *kit.Kit, filterExpr *filter.Expression,
	page *core.BasePage) ([]*types.ApplyTicket, error) {

	if a.apiClientSet == nil {
		return nil, fmt.Errorf("data service client not initialized")
	}

	isAll := false
	if page == nil || page.Limit == 0 {
		page = core.NewDefaultBasePage()
		isAll = true
	}

	insts := make([]*types.ApplyTicket, 0)
	req := &cvmapplyproto.ZiyanCvmApplyOrderListReq{
		Filter: filterExpr,
		Page:   page,
	}

	for {
		resp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyOrder.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			return nil, err
		}

		for _, mysqlRecord := range resp.Details {
			inst, err := convertMySQLToApplyTicket(mysqlRecord)
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

// UpdateApplyTicket updates apply ticket by filter and doc in db
func (a *applyTicket) UpdateApplyTicket(kt *kit.Kit, filterExpr *filter.Expression,
	updateData *cvmapplyproto.ZiyanCvmApplyOrderUpdateReq) error {

	if a.apiClientSet == nil {
		return fmt.Errorf("data service client not initialized")
	}

	// First, get the records to update
	listReq := &cvmapplyproto.ZiyanCvmApplyOrderListReq{
		Filter: filterExpr,
		Page:   core.NewDefaultBasePage(),
	}

	listResp, err := a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyOrder.List(kt.Ctx, kt.Header(), listReq)
	if err != nil {
		return err
	}

	if len(listResp.Details) == 0 {
		return nil
	}

	// Batch update
	updateReqs := make([]cvmapplyproto.ZiyanCvmApplyOrderUpdateReq, 0, len(listResp.Details))
	for _, record := range listResp.Details {
		updateReq := *updateData
		updateReq.OrderID = record.OrderID
		updateReqs = append(updateReqs, updateReq)
	}

	batchReq := &cvmapplyproto.BatchUpdateZiyanCvmApplyOrderReq{
		ApplyOrders: updateReqs,
	}

	return a.apiClientSet.DataService().TCloudZiyan.ZiyanCvmApplyOrder.BatchUpdate(kt.Ctx, kt.Header(), batchReq)
}

// AggregateAll apply ticket aggregate all operation
func (a *applyTicket) AggregateAll(ctx context.Context, pipeline interface{}, result interface{},
	opts ...*daltypes.AggregateOpts) error {

	if err := mongodb.Client().Table(pkg.BKTableNameApplyTicket).AggregateAll(ctx, pipeline, result, opts...); err != nil {
		return err
	}

	return nil
}

// convertMySQLToApplyTicket converts MySQL record to types.ApplyTicket
func convertMySQLToApplyTicket(mysqlRecord *cvmapplytable.ZiyanCvmApplyOrder) (*types.ApplyTicket, error) {
	// Parse JSON fields
	var follower []string
	var err error
	if len(mysqlRecord.Follower) > 0 && mysqlRecord.Follower != "{}" {
		err = json.Unmarshal([]byte(mysqlRecord.Follower), &follower)
		if err != nil {
			logs.Errorf("unmarshal follower failed, err: %v", err)
			return nil, err
		}
	}

	var suborders []*types.Suborder
	if len(mysqlRecord.Suborders) > 0 && mysqlRecord.Suborders != "{}" {
		err = json.Unmarshal([]byte(mysqlRecord.Suborders), &suborders)
		if err != nil {
			logs.Errorf("unmarshal suborders failed, err: %v", err)
			return nil, err
		}
	}

	var oldSuborders []*types.Suborder
	if len(mysqlRecord.OldSuborders) > 0 && mysqlRecord.OldSuborders != "{}" {
		err = json.Unmarshal([]byte(mysqlRecord.OldSuborders), &oldSuborders)
		if err != nil {
			logs.Errorf("unmarshal oldSuborders failed, err: %v", err)
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

	return &types.ApplyTicket{
		OrderId:      mysqlRecord.OrderID,
		ItsmTicketId: mysqlRecord.ItsmTicketID,
		Stage:        mysqlRecord.Stage,
		BkBizId:      mysqlRecord.BkBizID,
		User:         mysqlRecord.BkUsername,
		Follower:     follower,
		EnableNotice: mysqlRecord.EnableNotice,
		RequireType:  mysqlRecord.RequireType,
		ExpectTime:   mysqlRecord.ExpectTime,
		Remark:       mysqlRecord.Remark,
		Suborders:    suborders,
		OldSuborders: oldSuborders,
		ProductType:  mysqlRecord.ProductType,
		CreateAt:     createdAt,
		UpdateAt:     updatedAt,
	}, nil
}
