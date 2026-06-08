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

// Package returner implements device returner
// which deals with resource return tasks.
package returner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"hcm/cmd/woa-server/dal/task/dao"
	"hcm/cmd/woa-server/dal/task/table"
	"hcm/cmd/woa-server/logics/plan"
	"hcm/cmd/woa-server/logics/task/recycler/event"
	daltypes "hcm/cmd/woa-server/storage/dal/types"
	recovertask "hcm/cmd/woa-server/types/task"
	"hcm/pkg"
	"hcm/pkg/api/core"
	cond "hcm/pkg/condition"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/mapstr"
	"hcm/pkg/dal"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/thirdparty/cvmapi"
	"hcm/pkg/thirdparty/erpapi"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/metadata"
	"hcm/pkg/tools/querybuilder"

	"go.mongodb.org/mongo-driver/mongo"
)

// Returner deal with device return tasks
type Returner struct {
	cmdbCli   cmdb.Client
	cvm       cvmapi.CVMClientInterface
	erp       erpapi.ErpClientInterface
	ctx       context.Context
	planLogic plan.Logics
}

// New creates a returner
func New(ctx context.Context, thirdCli *thirdparty.Client, cmdbCli cmdb.Client, planLogic plan.Logics) (*Returner,
	error) {

	returner := &Returner{
		cmdbCli:   cmdbCli,
		cvm:       thirdCli.CVM,
		erp:       thirdCli.Erp,
		ctx:       ctx,
		planLogic: planLogic,
	}

	return returner, nil
}

// DealRecycleOrder deals with recycle order by running returning tasks
func (r *Returner) DealRecycleOrder(kt *kit.Kit, order *table.RecycleOrder) *event.Event {
	task, err := r.initReturnTask(order)
	if err != nil {
		logs.Errorf("failed to init return task for order %s, err: %v, rid: %s", order.SuborderID, err, kt.Rid)
		return &event.Event{Type: event.ReturnFailed, Error: err}
	}

	return r.dealReturnTask(kt, task)
}

func (r *Returner) getRecycleHosts(orderId string) ([]*table.RecycleHost, error) {
	filter := map[string]interface{}{
		"suborder_id": orderId,
	}

	page := metadata.BasePage{
		Start: 0,
		Limit: pkg.BKMaxInstanceLimit,
	}

	insts, err := dao.Set().RecycleHost().FindManyRecycleHost(context.Background(), page, filter)
	if err != nil {
		logs.Errorf("failed to get recycle hosts, err: %v", err)
		return nil, err
	}

	return insts, nil
}

func (r *Returner) initReturnTask(order *table.RecycleOrder) (*table.ReturnTask, error) {
	filter := &mapstr.MapStr{
		"suborder_id": order.SuborderID,
	}

	task, err := dao.Set().ReturnTask().GetReturnTask(context.Background(), filter)
	if errors.Is(err, daltypes.ErrDocumentNotFound) {
		now := time.Now()
		newTask := &table.ReturnTask{
			OrderID:            order.OrderID,
			SuborderID:         order.SuborderID,
			ResourceType:       order.ResourceType,
			RecycleType:        order.RecycleType,
			ReturnPlan:         order.ReturnPlan,
			SkipConfirm:        order.SkipConfirm,
			Status:             table.ReturnStatusInit,
			TaskID:             "",
			TaskLink:           "",
			CreateAt:           now,
			UpdateAt:           now,
			ReturnForecast:     order.ReturnForecast,
			ReturnForecastTime: order.ReturnForecastTime,
			User:               order.User,
			BkBizID:            order.BizID,
		}

		if err = dao.Set().ReturnTask().CreateReturnTask(context.Background(), newTask); err != nil {
			logs.Errorf("failed to create return task for order %s, err: %v", order.SuborderID, err)
			return nil, err
		}

		return newTask, nil
	}

	return task, err
}

func (r *Returner) dealReturnTask(kt *kit.Kit, task *table.ReturnTask) *event.Event {
	// get hosts by order id
	hosts, err := r.getRecycleHosts(task.SuborderID)
	if err != nil {
		logs.Errorf("failed to get recycle hosts by order id: %d, err: %v, rid: %s", task.SuborderID, err, kt.Rid)
		return &event.Event{Type: event.ReturnFailed, Error: err}
	}

	filterHosts, err := r.filterUpdateRecycleHosts(kt, task.SuborderID, hosts)
	if err != nil {
		logs.Errorf("failed to get recycle hosts by order id: %d, err: %v, rid: %s", task.SuborderID, err, kt.Rid)
		return &event.Event{Type: event.ReturnFailed, Error: err}
	}

	// 记录日志
	logs.Infof("recycler:logics:cvm:dealReturnTask:start, suborderID: %s, task: %+v, hostNum: %d, filterHostNum: %d, "+
		"rid: %s", task.SuborderID, cvt.PtrToVal(task), len(hosts), len(filterHosts), kt.Rid)

	switch task.Status {
	case table.ReturnStatusInit:
		return r.returnHosts(kt, task, filterHosts)
	case table.ReturnStatusRunning:
		return r.QueryReturnStatus(kt, task, filterHosts)
	case table.ReturnStatusSuccess:
		ev := &event.Event{Type: event.ReturnSuccess, Error: nil}
		return ev
	case table.ReturnStatusFailed:
		ev := &event.Event{
			Type:  event.ReturnFailed,
			Error: fmt.Errorf("return task is already failed, need not deal again"),
		}
		return ev
	default:
		logs.Warnf("failed to deal return task for order %s, for unknown status %s, rid: %s",
			task.SuborderID, task.Status, kt.Rid)
		ev := &event.Event{
			Type: event.ReturnFailed,
			Error: fmt.Errorf("failed to deal return task for order %s, for unknown status %s", task.SuborderID,
				task.Status),
		}
		return ev
	}
}

func (r *Returner) returnHosts(kt *kit.Kit, task *table.ReturnTask, hosts []*table.RecycleHost) *event.Event {
	taskId := ""
	var err error
	switch task.ResourceType {
	case table.ResourceTypeCvm:
		// gap is the gap time between transit and return.
		// sleep at lease 5 minutes before create cvm return order,
		// to wait for YUNTI plan product information syncing.
		// otherwise, return cost may belong to the wrong plan product.
		gap := task.CreateAt.Add(time.Minute * 5).Sub(time.Now())
		if gap > 0 {
			time.Sleep(gap)
		}

		taskId, err = r.returnCvm(kt, task, hosts)
	case table.ResourceTypePm:
		taskId, err = r.returnPm(kt, task, hosts)
	default:
		err = fmt.Errorf("failed to return hosts, for unsupported resource type %s", task.ResourceType)
	}
	return r.updateReturnState(kt, err, taskId, task, hosts)
}

func (r *Returner) updateReturnState(kt *kit.Kit, err error, taskId string, task *table.ReturnTask,
	hosts []*table.RecycleHost) *event.Event {

	if err == nil && taskId == "" {
		err = fmt.Errorf("failed to return hosts, for return order id is empty, taskID: %s, subOrderID: %s",
			taskId, task.SuborderID)
	}

	// update order info
	if err != nil {
		if errUpdate := r.UpdateOrderInfo(context.Background(), task.SuborderID, recovertask.Handler, 0,
			uint(len(hosts)), 0, err.Error()); errUpdate != nil {
			logs.Errorf("recycler:logics:cvm:returnHosts:failed, failed to update recycle order %s info, err: %v, "+
				"rid: %s", task.SuborderID, errUpdate, kt.Rid)
			return &event.Event{Type: event.ReturnFailed, Error: errUpdate}
		}

		return &event.Event{Type: event.ReturnFailed, Error: err}
	}

	// 更新回收任务、回收主机的数据
	eventInfo, err := r.updateRecycleHostTaskInfo(kt, task, taskId, len(hosts))
	if err != nil {
		return &event.Event{Type: event.ReturnFailed, Error: err}
	}

	return eventInfo
}

func (r *Returner) updateRecycleHostTaskInfo(kt *kit.Kit, task *table.ReturnTask, taskId string, hostsNum int) (
	*event.Event, error) {

	// 回收任务
	task.TaskID = taskId
	task.Status = table.ReturnStatusRunning
	// 回收订单
	recycleOrder := &table.RecycleOrder{
		SuccessNum: 0,
		PendingNum: uint(hostsNum),
	}
	// 回收主机
	recycleHost := &table.RecycleHost{
		Stage:  table.RecycleStageReturn,
		Status: table.RecycleStatusReturning,
	}

	eventInfo := &event.Event{Type: event.ReturnHandling}
	// 这批主机需要回收到[资源池]，需要更新回收任务、回收主机的状态为SUCCESS
	if taskId == enumor.RollingServerResourcePoolTask {
		task.Status = table.ReturnStatusSuccess
		task.Message = "success"
		recycleOrder.SuccessNum = uint(hostsNum)
		recycleOrder.PendingNum = 0
		recycleOrder.Message = "success"
		recycleHost.Stage = table.RecycleStageDone
		recycleHost.Status = table.RecycleStatusDone
		eventInfo = &event.Event{Type: event.ReturnSuccess}
	}

	return eventInfo, dal.RunTransaction(kit.New(), func(sc mongo.SessionContext) error {
		if errUpdate := r.UpdateOrderInfo(sc, task.SuborderID, "AUTO", recycleOrder.SuccessNum, recycleOrder.FailedNum,
			recycleOrder.PendingNum, recycleOrder.Message); errUpdate != nil {
			logs.Errorf("recycler:logics:cvm:returnHosts:failed, failed to update recycle info, suborderID: %s, "+
				"err: %v, taskId: %s, rid: %s", task.SuborderID, errUpdate, task.TaskID, kt.Rid)
			return errUpdate
		}

		// update return task info
		if err := r.UpdateReturnTaskInfo(sc, task, task.Status, task.Message); err != nil {
			logs.Errorf("recycler:logics:cvm:returnHosts:failed, failed to update return task info, suborderID: %s, "+
				"err: %v, taskId: %s, rid: %s", task.SuborderID, err, task.TaskID, kt.Rid)
			return err
		}

		// update recycle host info
		if err := r.updateHostInfo(sc, task, task.TaskID, recycleHost.Stage, recycleHost.Status); err != nil {
			logs.Errorf("recycler:logics:cvm:returnHosts:failed, failed to update recycle host info, suborderID: %s, "+
				"err: %v, taskId: %s, rid: %s", task.SuborderID, err, task.TaskID, kt.Rid)
			return err
		}
		return nil
	})
}

// QueryReturnStatus 查询return任务状态
func (r *Returner) QueryReturnStatus(kt *kit.Kit, task *table.ReturnTask, hosts []*table.RecycleHost) *event.Event {
	if task.TaskID == "" {
		ev := &event.Event{
			Type:  event.ReturnFailed,
			Error: fmt.Errorf("failed to query return order, for order id is empty"),
		}
		return ev
	}

	// query timeout 2 weeks 这里查询回收单据状态的时间不能限制的太短（尤其是现在CRP侧要求隔离15天）
	// 否则重试单据时也会被卡住，所以改成按更新时间计算查询超时
	timeout := task.UpdateAt.AddDate(0, 0, 14)
	if time.Now().After(timeout) {
		ev := &event.Event{
			Type:  event.ReturnFailed,
			Error: fmt.Errorf("query return order %s timeout, exceeds 2 weeks", task.SuborderID),
		}
		return ev
	}
	ev := &event.Event{
		Type:  event.ReturnFailed,
		Error: fmt.Errorf("failed to query return order, for unsupported resource type %s", task.ResourceType),
	}
	switch task.ResourceType {
	case table.ResourceTypeCvm:
		ev = r.queryCvmOrder(kt, task, hosts)
	case table.ResourceTypePm:
		ev = r.queryPmOrder(kt, task, hosts)
	default:
		// do nothing
	}

	err := r.UpdateReturnTaskStatus(kt, task.SuborderID, "", "")
	if err != nil {
		// ignore update status error
		logs.Infof("failed to update return task status after query return status, "+
			"err: %v, suborderID: %s, taskId: %s, rid: %s", err, task.SuborderID, task.TaskID, kt.Rid)
	}
	return ev
}

// UpdateOrderInfo 更新回收订单信息
func (r *Returner) UpdateOrderInfo(ctx context.Context, orderId, handler string, success, failed, pending uint,
	msg string) error {
	filter := mapstr.MapStr{
		"suborder_id": orderId,
	}

	now := time.Now()
	update := mapstr.MapStr{
		"success_num": success,
		"failed_num":  failed,
		"pending_num": pending,
		"message":     msg,
		"update_at":   now,
	}

	if len(handler) > 0 {
		update["handler"] = handler
	}

	if err := dao.Set().RecycleOrder().UpdateRecycleOrder(ctx, &filter, &update); err != nil {
		logs.Errorf("failed to update return task, order id: %s, err: %v", orderId, err)
		return err
	}

	return nil
}

// UpdateReturnTaskInfo 更新回收任务信息
func (r *Returner) UpdateReturnTaskInfo(ctx context.Context, task *table.ReturnTask, status table.ReturnStatus,
	msg string) error {

	filter := mapstr.MapStr{
		"suborder_id": task.SuborderID,
	}

	now := time.Now()
	update := mapstr.MapStr{
		"status":    status,
		"update_at": now,
	}
	taskId := task.TaskID
	if len(taskId) > 0 && taskId != enumor.RollingServerResourcePoolTask {
		link := ""
		switch task.ResourceType {
		case table.ResourceTypeCvm:
			link = cvmapi.CvmReturnLinkPrefix + taskId
		case table.ResourceTypePm:
			link = erpapi.ReturnOrderLinkPrefix + taskId
		}
		update["task_id"] = taskId
		update["task_link"] = link
	}

	if len(msg) > 0 {
		update["message"] = msg
	}

	if err := dao.Set().ReturnTask().UpdateReturnTask(ctx, &filter, &update); err != nil {
		logs.Errorf("failed to update return task, order id: %s, err: %v", task.SuborderID, err)
		return err
	}

	return nil
}

// UpdateReturnTaskStatus 更新回收任务的状态，如果状态为空字符串，只更新update_at字段
func (r *Returner) UpdateReturnTaskStatus(kt *kit.Kit, suborderID string, status table.ReturnStatus,
	msg string) error {

	filter := mapstr.MapStr{
		"suborder_id": suborderID,
	}

	now := time.Now()
	update := mapstr.MapStr{
		"update_at": now,
	}
	if len(status) > 0 {
		update["status"] = status
	}

	if len(msg) > 0 {
		update["message"] = msg
	}

	if err := dao.Set().ReturnTask().UpdateReturnTask(kt.Ctx, &filter, &update); err != nil {
		logs.Errorf("failed to update return task status, order id: %s, err: %v, rid: %s", suborderID, err, kt.Rid)
		return err
	}

	return nil
}

func (r *Returner) updateHostInfo(ctx context.Context, task *table.ReturnTask, taskId string,
	hostStage table.RecycleStage, hostStatus table.RecycleStatus) error {

	now := time.Now()
	link := ""
	if len(taskId) != 0 && taskId != enumor.RollingServerResourcePoolTask {
		switch task.ResourceType {
		case table.ResourceTypeCvm:
			link = cvmapi.CvmReturnLinkPrefix + taskId
		case table.ResourceTypePm:
			link = erpapi.ReturnOrderLinkPrefix + taskId
		}
	}

	filter := mapstr.MapStr{
		"suborder_id": task.SuborderID,
	}

	update := mapstr.MapStr{
		"stage":       hostStage,
		"status":      hostStatus,
		"return_id":   taskId,
		"return_link": link,
		"update_at":   now,
	}

	if err := dao.Set().RecycleHost().UpdateRecycleHost(ctx, &filter, &update); err != nil {
		logs.Errorf("failed to update recycle host, order id: %s, err: %v", task.SuborderID, err)
		return err
	}

	return nil
}

func (r *Returner) rollbackTransit(kt *kit.Kit, hosts []*table.RecycleHost) {
	if len(hosts) == 0 {
		return
	}

	// 1. transit hosts from cr module back to idle module
	assetIDs := make([]string, 0)
	bizID := hosts[0].BizID
	operator := hosts[0].Operator
	for _, host := range hosts {
		assetIDs = append(assetIDs, host.AssetID)
	}
	if err := r.transferHost2BizIdle(kt, assetIDs, bizID); err != nil {
		logs.Warnf("failed to transfer host from CR transit module back to idle module")
		return
	}

	// 2. get new host IDs after transit hosts back to idle module
	// note: host ID change after transit back to business idle module
	hostIDs, err := r.getHostIDByAsset(kt, assetIDs, bizID)
	if err != nil {
		logs.Warnf("failed to get host ID by asset, err: %v", err)
		return
	}

	// 3. transit hosts from idle module to recycle module
	if err := r.transferHost2BizRecycle(kt, hostIDs, bizID); err != nil {
		logs.Warnf("failed to transfer host to recycle module")
		return
	}

	// 4. set hosts operator
	if err := r.setHostOperator(hostIDs, operator); err != nil {
		logs.Warnf("failed to set host operator, err: %v", err)
		return
	}
}

// transferHost2BizIdle transfer hosts from CR transit module back to idle module in CMDB
func (r *Returner) transferHost2BizIdle(kt *kit.Kit, assetIds []string, destBizId int64) error {
	// once 10 hosts at most
	maxNum := 10
	begin := 0
	end := begin
	length := len(assetIds)

	// transfer hosts from destBiz-CR_IEG_资源服务系统专用退回中转勿改勿删 back to destBiz-空闲机
	for begin < length {
		end += maxNum
		if end > length {
			end = length
		}

		req := &cmdb.CrTransitIdleReq{
			BkBizId:  destBizId,
			AssetIDs: assetIds[begin:end],
		}

		err := r.cmdbCli.HostsCrTransit2Idle(kt, req)
		begin = end
		if err != nil {
			logs.Errorf("failed to transfer host back to idle module, err: %v", err)
			return err
		}
		logs.Infof("transfer host back to idle module success, hosts: %v", req.AssetIDs)
	}

	return nil
}

// getHostIDByAsset get host ID by asset ID
func (r *Returner) getHostIDByAsset(kt *kit.Kit, assetIDs []string, bizID int64) ([]int64, error) {
	hostIDs := make([]int64, 0)

	req := &cmdb.ListBizHostParams{
		BizID: bizID,
		HostPropertyFilter: &cmdb.QueryFilter{
			Rule: querybuilder.CombinedRule{
				Condition: querybuilder.ConditionAnd,
				Rules: []querybuilder.Rule{
					querybuilder.AtomRule{
						Field:    "bk_asset_id",
						Operator: querybuilder.OperatorIn,
						Value:    assetIDs,
					},
					// support bk_cloud_id 0 only
					querybuilder.AtomRule{
						Field:    "bk_cloud_id",
						Operator: querybuilder.OperatorEqual,
						Value:    0,
					},
				},
			},
		},
		Fields: []string{
			"bk_host_id",
			"bk_asset_id",
			"bk_host_innerip",
		},
		Page: &cmdb.BasePage{
			Start: 0,
			Limit: pkg.BKMaxInstanceLimit,
		},
	}

	resp, err := r.cmdbCli.ListBizHost(kt, req)
	if err != nil {
		logs.Errorf("failed to get cc host info, err: %v", err)
		return nil, err
	}

	for _, host := range resp.Info {
		hostIDs = append(hostIDs, host.BkHostID)
	}

	return hostIDs, nil
}

// transferHost2BizRecycle transfer hosts from business idle module to recycle module in CMDB
func (r *Returner) transferHost2BizRecycle(kt *kit.Kit, hostIDs []int64, bizID int64) error {
	// get business recycle module id
	moduleID, err := r.getBizRecycleModuleID(kt, bizID)
	if err != nil {
		logs.Errorf("failed to get biz %d recycle module ID, err: %v", bizID, err)
		return err
	}

	// once 10 hosts at most
	req := &cmdb.TransferHostReq{
		From: cmdb.TransferHostSrcInfo{
			FromBizID: bizID,
			HostIDs:   hostIDs,
		},
		To: cmdb.TransferHostDstInfo{
			ToBizID:    bizID,
			ToModuleID: moduleID,
		},
	}

	err = r.cmdbCli.TransferHost(kt, req)
	if err != nil {
		logs.Errorf("failed to transfer host to recycle module %d, err: %v", moduleID, err)
		return err
	}

	return nil
}

// getBizRecycleModuleID get business recycle module ID
func (r *Returner) getBizRecycleModuleID(kt *kit.Kit, bizID int64) (int64, error) {
	moduleID, err := r.cmdbCli.GetBizRecycleModuleID(kt, bizID)
	if err != nil {
		logs.Errorf("failed to get biz internal module, err: %v", err)
		return 0, fmt.Errorf("failed to get biz internal module, err: %v", err)
	}
	return moduleID, nil
}

// setHostOperator set host operator in cc 3.0
func (r *Returner) setHostOperator(hostIDs []int64, operator string) error {
	req := &cmdb.UpdateHostsReq{
		Update: make([]*cmdb.UpdateHostProperty, 0),
	}

	for _, hostID := range hostIDs {
		update := &cmdb.UpdateHostProperty{
			HostID: hostID,
			Properties: map[string]interface{}{
				"operator":        operator,
				"bk_bak_operator": operator,
			},
		}
		req.Update = append(req.Update, update)
	}

	kt := core.NewBackendKit()
	_, err := r.cmdbCli.UpdateHosts(kt, req)
	if err != nil {
		return err
	}
	return nil
}

func (r *Returner) filterUpdateRecycleHosts(kt *kit.Kit, subOrderID string, hosts []*table.RecycleHost) (
	[]*table.RecycleHost, error) {

	assetIDs := make([]string, 0)
	for _, host := range hosts {
		assetIDs = append(assetIDs, host.AssetID)
	}

	// 查询已回收的主机
	hostExists, err := r.getRecycleHostsByAssetIDsStatus(kt, subOrderID, assetIDs, table.RecycleStatusDone)
	if err != nil {
		logs.Errorf("failed to get recycle hosts by assetids and status, subOrderID: %s, err: %v, assetIDs: %v, "+
			"rid: %s", subOrderID, err, assetIDs, kt.Rid)
		return nil, err
	}
	hostExistsMap := cvt.SliceToMap(hostExists, func(rh *table.RecycleHost) (string, *table.RecycleHost) {
		return rh.AssetID, rh
	})

	filterHosts := make([]*table.RecycleHost, 0)
	for _, host := range hosts {
		hostDone, ok := hostExistsMap[host.AssetID]
		if !ok {
			filterHosts = append(filterHosts, host)
			continue
		}

		// 更新该子订单中已回收的主机状态
		err = r.updateHostRecycleInfoByOrderAssetID(kt, subOrderID, host.AssetID,
			table.RecycleStatusDone, table.RecycleStageDone, hostDone.ReturnID, hostDone.ReturnLink)
		if err != nil {
			return nil, err
		}
	}
	return filterHosts, nil
}

func (r *Returner) getRecycleHostsByAssetIDsStatus(kt *kit.Kit, excludeSuborderID string, assetIDs []string,
	status table.RecycleStatus) ([]*table.RecycleHost, error) {

	filter := map[string]interface{}{
		"asset_id": map[string]interface{}{"$in": assetIDs},
		"status":   status,
		// 排除当前子单数据
		"suborder_id": map[string]interface{}{cond.BKDBNE: excludeSuborderID},
	}

	page := metadata.BasePage{
		Start: 0,
		Limit: pkg.BKMaxInstanceLimit,
	}

	insts, err := dao.Set().RecycleHost().FindManyRecycleHost(kt.Ctx, page, filter)
	if err != nil {
		logs.Errorf("failed to get recycle hosts by assetids and status, err: %v, assetIDs: %v, status: %s, rid: %s",
			err, assetIDs, status, kt.Rid)
		return nil, err
	}

	return insts, nil
}

func (r *Returner) updateHostRecycleInfoByOrderAssetID(kt *kit.Kit, subOrderID, assetID string,
	status table.RecycleStatus, stage table.RecycleStage, taskId, taskURL string) error {

	filter := mapstr.MapStr{
		"suborder_id": subOrderID,
		"assetID":     assetID,
	}

	update := mapstr.MapStr{
		"stage":       stage,
		"status":      status,
		"return_id":   taskId,
		"return_link": taskURL,
		"update_at":   time.Now(),
	}

	if err := dao.Set().RecycleHost().UpdateRecycleHost(kt.Ctx, &filter, &update); err != nil {
		logs.Errorf("failed to update recycle host by order asset id, subOrderID: %s, assetID: %s, err: %v, rid: %s",
			subOrderID, assetID, err, kt.Rid)
		return err
	}

	return nil
}

// HandleReturnPlan 回收主机后, 处理返还预测
func (r *Returner) HandleReturnPlan(kt *kit.Kit, order *table.RecycleOrder) *event.Event {
	// 回收时设置了返还预测 才需要同步
	filter := &mapstr.MapStr{
		"suborder_id": order.SuborderID,
	}

	task, err := dao.Set().ReturnTask().GetReturnTask(context.Background(), filter)
	if err != nil {
		logs.Errorf("failed to get return task, err: %v, subOrderID: %s, rid: %s", err, order.SuborderID, kt.Rid)
		return &event.Event{
			Type:  event.ReturnPlanFailed,
			Error: fmt.Errorf("failed to get return task, err: %v", err),
		}
	}
	if task == nil {
		logs.Errorf("return task not found, subOrderID: %s, rid: %s", order.SuborderID, kt.Rid)
		return &event.Event{
			Type:  event.ReturnPlanFailed,
			Error: fmt.Errorf("return task not found"),
		}
	}
	if err := r.planLogic.ApplyDestroyOrderToResPlanDemand(kt, task.TaskID); err != nil {
		logs.Errorf("failed to apply destroy order to res plan demand, err: %v, taskID: %s, rid: %s",
			err, task.TaskID, kt.Rid)
		return &event.Event{
			Type:  event.ReturnPlanFailed,
			Error: fmt.Errorf("failed to apply destroy order to res plan demand, err: %v", err),
		}
	}
	return &event.Event{
		Type: event.ReturnPlanSuccess,
	}
}
