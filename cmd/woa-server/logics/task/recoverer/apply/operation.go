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

package apply

import (
	"fmt"
	"time"

	model "hcm/cmd/woa-server/model/task"
	types "hcm/cmd/woa-server/types/task"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/sopsapi"
	cvt "hcm/pkg/tools/converter"
)

// getTickets get apply ticket with stage and create_at between recoverTime and expireTime
func (r *applyRecoverer) getRunningTickets(kt *kit.Kit, recoverTime time.Time,
	expireTime time.Time, stage types.TicketStage) (order []*types.ApplyTicket, err error) {

	filter := tools.ExpressionAnd(
		tools.RuleEqual("stage", stage),
		tools.RuleGreaterThanEqual("created_at", expireTime.Format(constant.TimeStdFormat)),
		tools.RuleLessThan("created_at", recoverTime.Format(constant.TimeStdFormat)),
	)

	tickets, err := model.Operation().ApplyTicket().FindManyApplyTicket(kt, filter, nil)
	if err != nil {
		logs.Errorf("failed to get apply ticket with RUNNING stage, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	return tickets, nil
}

// getAuditTickets 获取状态为TicketStageAudit订单
func (r *applyRecoverer) getAuditTickets(kt *kit.Kit, recoverTime time.Time,
	expireTime time.Time, stage types.TicketStage) ([]*types.ApplyTicket, error) {
	filter := tools.ExpressionAnd(
		tools.RuleEqual("stage", stage),
		tools.RuleGreaterThanEqual("created_at", expireTime.Format(constant.TimeStdFormat)),
		tools.RuleLessThan("created_at", recoverTime.Format(constant.TimeStdFormat)),
	)

	auditTickets, err := model.Operation().ApplyTicket().FindManyApplyTicket(kt, filter, nil)
	if err != nil {
		logs.Errorf("failed to get apply ticket with AUDIT stage, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	return auditTickets, nil
}

// getOrderStep get step by suborderId and stepName
func (r *applyRecoverer) getOrderStep(kt *kit.Kit, stepName string, suborderId string) (*types.ApplyStep, error) {
	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("step_name", stepName),
	)
	step, err := model.Operation().ApplyStep().GetApplyStep(kt, filter)
	if err != nil {
		logs.Errorf("failed to get step: %s, err: %v, subOrderId: %s, rid: %s", stepName, err, suborderId, kt.Rid)
		return nil, err
	}

	return step, nil
}

// getSuborders 根据order获得子单
func (r *applyRecoverer) getSuborders(kt *kit.Kit, orderId uint64) ([]*types.ApplyOrder, error) {
	filter := tools.ExpressionAnd(tools.RuleEqual("order_id", orderId))
	orders, err := model.Operation().ApplyOrder().FindManyApplyOrder(kt, filter, nil)
	if err != nil {
		logs.Errorf("failed to list apply order by orderId, err: %v, orderId: %d, rid: %s", err, orderId, kt.Rid)
		return nil, err
	}
	return orders, nil
}

// getHostBizID 利用ip获得业务ID
func (r *applyRecoverer) getHostBizID(kt *kit.Kit, ip string) (int64, error) {
	// 根据IP获取主机信息
	hostInfo, err := r.cmdbCli.GetHostInfoByIP(kt, ip, 0)
	if err != nil {
		logs.Errorf("recover: deliver status handling, get host info by host ip failed, err: %v, ip: %s, rid: %s", err,
			ip, kt.Rid)
		return 0, err
	}
	// 根据BkHostID去cmdb获取bkBizID
	hostIds := []int64{hostInfo.BkHostID}
	bkBizIDs, err := r.cmdbCli.GetHostBizIds(kt, hostIds)
	if err != nil {
		logs.Errorf("recover: get host info by host id failed, err: %v, ip: %s, BkHostID: %d, rid: %s", err, ip,
			hostInfo.BkHostID, kt.Rid)
		return 0, err
	}
	bkBizID, ok := bkBizIDs[hostInfo.BkHostID]
	if !ok {
		logs.Errorf("can not find bizId by hostId: %d, ip: %s, rid: %s", hostInfo.BkHostID, ip, kt.Rid)
		return 0, fmt.Errorf("can not find bizId by hostId: %d, ip: %s", hostInfo.BkHostID, ip)
	}

	return bkBizID, nil
}

// getDeviceByIp 利用ip获得设备信息
func (r *applyRecoverer) getDeviceByIp(kt *kit.Kit, subOrderID string, ip string) (*types.DeviceInfo, error) {
	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", subOrderID), tools.RuleEqual("ip", ip))

	devices, err := model.Operation().DeviceInfo().GetDeviceInfo(kt, filter)
	if err != nil {
		logs.Errorf("failed to get device by ip, err: %v, ip: %s, subOrderId: %s, rid: %s",
			err, ip, subOrderID, kt.Rid)
		return nil, err
	}
	if len(devices) != 1 {
		logs.Errorf("get too many or few devices by ip, ip: %s, subOrderId: %s, rid: %s", ip, subOrderID, kt.Rid)
		return nil, fmt.Errorf("get too many or few devices by ip, ip: %s, subOrderId: %s", ip, subOrderID)
	}

	return devices[0], nil
}

// getInitTask 通过bkBizId和orderId获取sops任务列表
func (r *applyRecoverer) getInitTask(kt *kit.Kit, bkBizId int64, orderId string, ip string) ([]*sopsapi.GetTaskListRst,
	error) {
	orderSopsName := fmt.Sprintf("%s-%s", ip, orderId)
	sopsTasks, err := r.sopsCli.GetTaskList(kt.Ctx, kt.Header(), bkBizId, orderSopsName)
	if err != nil {
		logs.Errorf("failed to get sops task list, err: %v, orderId: %s, rid: %s ", err, orderId, kt.Rid)
		return nil, err
	}

	return sopsTasks, nil
}

// RecoverStartStep recover apply order step with start info
func (r *applyRecoverer) recoverStartStep(kt *kit.Kit, suborderId string, stepName string) error {
	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("step_name", stepName),
		tools.RuleEqual("status", types.StepStatusHandling),
	)
	update := &cvmapplyproto.ZiyanCvmApplyStepUpdateReq{
		Status:  cvt.ValToPtr(types.StepStatusInit),
		Message: "initing",
	}

	if err := model.Operation().ApplyStep().UpdateApplyStep(kt, filter, update); err != nil {
		logs.Errorf("failed to recover start order, err: %v, subOrderId: %s, stepName: %s, rid: %s", err, suborderId,
			stepName, kt.Rid)
		return err
	}
	return nil
}

// getBizID 根据IP获取所属业务ID
func (r *applyRecoverer) getBizID(kt *kit.Kit, ip string) (int64, error) {
	// 根据IP获取主机信息
	hostInfo, err := r.cmdbCli.GetHostInfoByIP(kt, ip, 0)
	if err != nil {
		logs.Errorf("recover: deliver status handling, get host info by host ip failed, ip: %s, err: %v, rid: %s", ip,
			err, kt.Rid)
		return 0, err
	}
	// 根据BkHostID去cmdb获取bkBizID
	bkBizIDs, err := r.cmdbCli.GetHostBizIds(kt, []int64{hostInfo.BkHostID})
	if err != nil {
		logs.Errorf("recover: deliver status handling, get host info by host id failed, ip: %s, BkHostID: %d, "+
			"err: %v, rid: %s", ip, hostInfo.BkHostID, err, kt.Rid)
		return 0, err
	}
	bkBizID, ok := bkBizIDs[hostInfo.BkHostID]
	if !ok {
		logs.Errorf("can not find biz id by hostId: %d, rid: %s", hostInfo.BkHostID, kt.Rid)
		return 0, fmt.Errorf("can not find biz id by hostId: %d", hostInfo.BkHostID)
	}

	return bkBizID, nil
}
