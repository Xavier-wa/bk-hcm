/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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
	"sync"
	"sync/atomic"
	"time"

	"hcm/cmd/woa-server/logics/task/scheduler/record"
	model "hcm/cmd/woa-server/model/task"
	types "hcm/cmd/woa-server/types/task"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"
)

// recoverGenerateStep 恢复generateStep为StepStatusInit｜StepStatusHandling的订单
func (r *applyRecoverer) recoverGenerateStep(kt *kit.Kit, order *types.ApplyOrder,
	generateStep *types.ApplyStep) error {

	// get generate records
	generateRecords, err := r.schedulerIf.GetGenerateRecords(kt, order.SubOrderId)
	if err != nil {
		logs.Errorf("failed to get generate records, err: %v, suborderId: %s, rid: %s", err, order.SubOrderId, kt.Rid)
		return err
	}
	// 没有generate记录或者还未进入生产，直接触发生产监听事件
	if generateStep.Status == types.StepStatusInit || len(generateRecords) == 0 {
		return r.recoverMatchingNoGenerate(kt, order.SubOrderId)
	}
	// 对于PM类型机器，不区分区生产和集中生产
	if order.ResourceType == types.ResourceTypePm {
		for _, generateRecord := range generateRecords {
			if msg, err := r.recoverPmResource(kt, generateRecord, order); err != nil {
				if err := r.dealGenerateFailed(kt, order, generateRecord.GenerateId, msg); err != nil {
					logs.Errorf("failed to update generate status to failed, err: %v, subOrderId: %s, rid: %s", err,
						order.SubOrderId, kt.Rid)
					return err
				}
				return err
			}
		}
		return nil
	}

	return r.recoverGenerate(kt, generateRecords, order)

}

func (r *applyRecoverer) recoverPmHandling(kt *kit.Kit, generateRecord *types.GenerateRecord,
	order *types.ApplyOrder) (string, error) {

	devices, err := r.getDeviceByOrder(kt, order.SubOrderId)
	if err != nil {
		logs.Errorf("failed to get device by order, err: %v, subOrderId: %s, rid: %s", err, order.SubOrderId,
			kt.Rid)
		return "", err
	}
	if len(devices) == 0 {
		logs.Infof("unkown generate status, check cmdb to find if matched pm, subOrderId: %s, generateId: %s, "+
			"status: %d, rid: %s", order.SubOrderId, generateRecord.GenerateId, generateRecord.Status, kt.Rid)
		msg := "unkown generate status, check cmdb to find if matched pm"
		return msg, fmt.Errorf("unkown generate status, check cmdb to find if matched pm, subOrderId: %s, "+
			"generateId: %s, status: %d", order.SubOrderId, generateRecord.GenerateId, generateRecord.Status)
	}

	if len(devices) >= int(order.TotalNum) {
		successIps := make([]string, 0)
		for _, host := range devices {
			successIps = append(successIps, host.Ip)
		}
		err = r.schedulerIf.GetGenerator().UpdateGenerateRecord(kt, order,
			generateRecord.GenerateId, types.GenerateStatusSuccess, "success", "", successIps)
		if err != nil {
			logs.Errorf("failed to match pm when update generate record, err: %v, subOrderId: %s, rid: %s", err,
				order.SubOrderId, kt.Rid)
			return "", err
		}
		// update generate step record
		if err = record.UpdateGenerateStep(kt, order.SubOrderId, order.TotalNum, nil); err != nil {
			logs.Errorf("failed to update generate step, subOrderId: %s, err: %v, rid: %s", order.SubOrderId, err,
				kt.Rid)
			return "", err
		}
		return "", nil
	}
	// 机器数量小于生产数量，重新触发匹配
	return "", r.schedulerIf.GetGenerator().MatchPM(kt, order)
}

func (r *applyRecoverer) recoverPmResource(kt *kit.Kit, generateRecord *types.GenerateRecord,
	order *types.ApplyOrder) (string, error) {

	switch generateRecord.Status {
	case types.GenerateStatusInit:
		// 上一个record单设置失败，重新生成record单据
		if err := r.updateGenerateRecord(kt, generateRecord.GenerateId, types.GenerateStatusFailed); err != nil {
			logs.Errorf("failed to recover generate record to failed status, err: %v, subOrderId: %s, rid: %s",
				err, order.SubOrderId, kt.Rid)
			return "", err
		}
		return "", r.schedulerIf.GetGenerator().MatchPM(kt, order)
	case types.GenerateStatusHandling:
		return r.recoverPmHandling(kt, generateRecord, order)
	case types.GenerateStatusSuccess:
		return "", r.recoverGenerateSuccess(kt, order, generateRecord)
	case types.GenerateStatusFailed:
		logs.Infof("ignore generate failed order, subOrderId: %s, rid: %s", order.SubOrderId, kt.Rid)
	case types.GenerateStatusSuspend:
		logs.Infof("ignore generate suspend order, subOrderId: %s, rid: %s", order.SubOrderId, kt.Rid)
	default:
		logs.Errorf("recover concentrated generate: unknown generate status, subOrderId: %s, generateId: %s, "+
			"status: %d, rid: %s", order.SubOrderId, generateRecord.GenerateId, generateRecord.Status, kt.Rid)
		return "", fmt.Errorf("recover concentrated generate: unknown generate status, subOrderId: %s, "+
			"generateId: %s, status: %d", order.SubOrderId, generateRecord.GenerateId, generateRecord.Status)
	}
	return "", nil
}

// recoverGenerate 恢复生产未完成的订单
func (r *applyRecoverer) recoverGenerate(kt *kit.Kit, generateRecords []*types.GenerateRecord,
	order *types.ApplyOrder) error {

	// 生产订单中，若有未完成订单generateRecord为init时，设置状态为GenerateStatusSuspend，不再触发后续生产
	isSuspend := false
	dealRecords := make([]*types.GenerateRecord, 0)
	for _, record := range generateRecords {
		if record.Status == types.GenerateStatusInit {
			// 未获得云梯生产id,未知是否调用生产，此记录设置状态为GenerateStatusSuspend，不再触发后续生产
			if err := r.updateGenerateSuspend(kt, record.GenerateId, types.GenerateStatusSuspend); err != nil {
				logs.Errorf("failed to update generate suspend, err: %v, generateId: %s, rid: %s", err,
					record.GenerateId, kt.Rid)
			}
			isSuspend = true
			continue
		}
		dealRecords = append(dealRecords, record)
	}

	if err := r.recoverGenerateHandling(kt, order, dealRecords, isSuspend); err != nil {
		for _, record := range dealRecords {
			if err = r.updateGenerateRecord(kt, record.GenerateId, types.GenerateStatusFailed); err != nil {
				logs.Errorf("failed to recover generate record to failed status, err: %v, subOrderId: %s, rid: %s",
					err, order.SubOrderId, kt.Rid)
				return err
			}
		}

		if err = r.updateGenerateFailedStep(kt, order, ""); err != nil {
			logs.Errorf("failed to update generate failed step, err: %v, subOrderId: %s, rid: %s", err,
				order.SubOrderId, kt.Rid)
			return err
		}

		if err = r.terminateApplyOrder(kt, order.SubOrderId); err != nil {
			logs.Errorf("failed to recover generate init concentrate generate orders, err: %v, subOrderId: %s, rid: %s",
				err, order.SubOrderId, kt.Rid)
			return err
		}

		logs.Errorf("failed to recover generate orders, err: %v, subOrderId: %s, rid: %s", err, order.SubOrderId,
			kt.Rid)
		return err
	}
	return nil
}

// recoverMatchingNoGenerate 恢复状态为ApplyStatusMatching但尚未生成记录的订单
func (r *applyRecoverer) recoverMatchingNoGenerate(kt *kit.Kit, subOrderId string) error {
	// 处理Matching状态但是没有generateRecord的情况，恢复订单状态为ApplyStatusWaitForMatch，触发监听器
	if err := r.recoverStartStep(kt, subOrderId, types.StepNameGenerate); err != nil {
		logs.Errorf("failed to recover generate start step, err: %v, subOrderId: %s, rid: %s", err, subOrderId, kt.Rid)
		return err
	}

	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", subOrderId),
		tools.RuleEqual("status", types.ApplyStatusMatching),
	)

	update := &cvmapplyproto.ZiyanCvmApplySuborderUpdateReq{
		Status: types.ApplyStatusWaitForMatch,
	}

	if err := model.Operation().ApplyOrder().UpdateApplyOrder(kt, filter, update); err != nil {
		logs.Errorf("failed to recover and update apply order update status, err: %v, suborderId: %s, rid: %s",
			err, subOrderId, kt.Rid)
		return err
	}
	return nil
}

// terminateApplyOrder 恢复申请状态为ApplyStatusMatching且generateRecord状态为init的订单
func (r *applyRecoverer) terminateApplyOrder(kt *kit.Kit, subOrderId string) error {
	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", subOrderId),
		tools.RuleEqual("status", types.ApplyStatusMatching),
	)

	update := &cvmapplyproto.ZiyanCvmApplySuborderUpdateReq{
		Stage:  types.TicketStageSuspend,
		Status: types.ApplyStatusTerminate,
	}

	if err := model.Operation().ApplyOrder().UpdateApplyOrder(kt, filter, update); err != nil {
		logs.Errorf("failed to update apply order status to apply status terminate, err: %v, suborderId: %s, rid: %s",
			err, subOrderId, kt.Rid)
		return err
	}
	return nil
}

// recoverGenerateSuccess 恢复集中机器订单，申请状态为ApplyStatusMatching且generateRecord状态为success订单
func (r *applyRecoverer) recoverGenerateSuccess(kt *kit.Kit, order *types.ApplyOrder,
	generateRecord *types.GenerateRecord) error {

	// update generate step record
	if err := record.UpdateGenerateStep(kt, order.SubOrderId, order.TotalNum, nil); err != nil {
		logs.Errorf("failed to update generate step, err: %v, subOrderId: %s, rid: %s", err, order.SubOrderId, kt.Rid)
		return err
	}
	logs.Infof("finished dispatch order, subOrderId: %s, rid: %s", order.SubOrderId, kt.Rid)

	if err := r.updateGenerateRecord(kt, generateRecord.GenerateId, types.GenerateStatusSuccess); err != nil {
		logs.Errorf("failed to update generate record, err: %v, suborderId: %s, rid: %s", err, order.SubOrderId,
			kt.Rid)
		return err
	}

	return nil
}

// recoverGenerateHandling 恢复订单状态为ApplyStatusMatching且generateRecord的状态为handling
func (r *applyRecoverer) recoverGenerateHandling(kt *kit.Kit, order *types.ApplyOrder,
	recordInfos []*types.GenerateRecord, isSuspend bool) error {

	errorNum, generateNum := int64(0), int64(0)
	wg := sync.WaitGroup{}
	for _, recordInfo := range recordInfos {
		generateId := recordInfo.GenerateId
		taskId := recordInfo.TaskId

		wg.Add(1)
		go func(taskId string, generateId string, order *types.ApplyOrder, recordInfo *types.GenerateRecord) {
			defer wg.Done()
			switch recordInfo.Status {
			case types.GenerateStatusHandling:
				err := r.schedulerIf.AddCvmDevices(kt, taskId, generateId, order)
				if err != nil {
					logs.Errorf("failed to check and update cvm device, err: %v, suborderId: %s, rid: %s", err,
						order.SubOrderId, kt.Rid)
					atomic.AddInt64(&errorNum, 1)
					return
				}
				logs.Infof("success to launch cvm, suborderId: %s, generateId: %s, rid: %s", order.SubOrderId,
					generateId, kt.Rid)
				atomic.AddInt64(&generateNum, 1)

			case types.GenerateStatusSuccess:
				if err := r.updateGenerateRecord(kt, generateId, types.GenerateStatusSuccess); err != nil {
					logs.Errorf("failed to update generate record, err: %v, suborderId: %s, rid: %s ", err,
						order.SubOrderId, kt.Rid)
					atomic.AddInt64(&errorNum, 1)
				}
				atomic.AddInt64(&generateNum, 1)

			default:
				logs.Errorf("recover generate: unKown generate status, suborderId: %s, generateId: %s, status: %d, "+
					"rid: %s", order.SubOrderId, generateId, recordInfo.Status, kt.Rid)
			}
		}(taskId, generateId, order, recordInfo)
	}
	wg.Wait()

	if generateNum == 0 {
		logs.Errorf("recover failed to generate cvm separate, for no zone has generate record, subOrderId: %s, rid: %s",
			order.SubOrderId, kt.Rid)
		return fmt.Errorf("recover failed to generate cvm separate, for no zone has generate record, subOrderId: %s",
			order.SubOrderId)
	}
	// 有生产错误记录，且不是suspend状态，更新apply order状态
	if errorNum > 0 && !isSuspend {
		// check all generate records and update apply order status
		if err := r.schedulerIf.UpdateOrderStatus(kt, order.ResourceType, order.SubOrderId); err != nil {
			logs.Errorf("failed to update order status, err: %v, subOrderId: %s, rid: %s", err, order.SubOrderId,
				kt.Rid)
		}
	}

	// update generate step record, skip err, continue generate devices
	if err := record.UpdateGenerateStep(kt, order.SubOrderId, order.TotalNum, nil); err != nil {
		logs.Errorf("failed to update generate step, err: %v, subOrderId: %s, rid: %s", err, order.SubOrderId, kt.Rid)
	}

	return nil
}

// updateGenerateRecord 更新生成记录以触发generate监听器
func (r *applyRecoverer) updateGenerateRecord(kt *kit.Kit, generateID string, status types.GenerateStepStatus) error {
	filter := tools.ExpressionAnd(tools.RuleEqual("generate_id", generateID))
	update := &cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq{
		Status: cvt.ValToPtr(status),
	}
	if err := model.Operation().GenerateRecord().UpdateGenerateRecord(kt, filter, update); err != nil {
		logs.Errorf("failed to update generate record, err: %v, generateID: %s, rid: %s", err, generateID, kt.Rid)
		return err
	}

	return nil
}

// updateGenerateRecord updates generate record to trigger generater listener
func (r *applyRecoverer) updateGenerateSuspend(kt *kit.Kit, generateID string, status types.GenerateStepStatus) error {
	filter := tools.ExpressionAnd(tools.RuleEqual("generate_id", generateID))
	update := &cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq{
		Status: cvt.ValToPtr(status),
		Message: cvt.ValToPtr("generate failed, unknown if generate interface was called, task_id not obtained, " +
			"check machines"),
	}
	if err := model.Operation().GenerateRecord().UpdateGenerateRecord(kt, filter, update); err != nil {
		logs.Errorf("failed to update generateRecord, unknown if generate interface was called, task_id not obtained,"+
			"err: %v, generateID: %s, rid: %s", err, generateID, kt.Rid)
		return err
	}

	return nil
}

// dealGenerateFailed 生产失败时处理生产步骤、生产记录及订单状态
func (r *applyRecoverer) dealGenerateFailed(kt *kit.Kit, order *types.ApplyOrder, generateId string, msg string) error {
	if err := r.updateGenerateFailedStep(kt, order, msg); err != nil {
		logs.Errorf("failed to update generate failed step, err: %v, subOrderId: %s, rid: %s", err, order.SubOrderId,
			kt.Rid)
		return err
	}

	if err := r.updateGenerateRecord(kt, generateId, types.GenerateStatusFailed); err != nil {
		logs.Errorf("failed to recover generate record to failed status, err: %v, subOrderId: %s, rid: %s",
			err, order.SubOrderId, kt.Rid)
		return err
	}

	if err := r.terminateApplyOrder(kt, order.SubOrderId); err != nil {
		logs.Errorf("failed to recover generate init concentrate generate orders, err: %v, subOrderId: %s, rid: %s",
			err, order.SubOrderId, kt.Rid)
		return err
	}
	return nil
}

func (r *applyRecoverer) updateGenerateFailedStep(kt *kit.Kit, order *types.ApplyOrder, msg string) error {
	now := time.Now()
	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", order.SubOrderId),
		tools.RuleEqual("step_name", types.StepNameGenerate),
	)
	update := &cvmapplyproto.ZiyanCvmApplyStepUpdateReq{
		Status:  cvt.ValToPtr(types.StepStatusFailed),
		Message: msg,
		EndAt:   now.Format(constant.DateTimeLayout),
	}

	if err := model.Operation().ApplyStep().UpdateApplyStep(kt, filter, update); err != nil {
		logs.Errorf("failed to update apply generate step status to apply status failed, err: %v, suborderId: %s, "+
			"rid: %s", err, order.SubOrderId, kt.Rid)
		return err
	}

	return nil
}

func (r *applyRecoverer) getDeviceByOrder(kt *kit.Kit, subOrderId string) ([]*types.DeviceInfo, error) {
	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", subOrderId))
	devices, err := model.Operation().DeviceInfo().GetDeviceInfo(kt, filter)
	if err != nil {
		logs.Errorf("failed to get device by subOrderId, subOrderId: %s, err: %v, rid: %s", subOrderId, err, kt.Rid)
		return nil, err
	}

	return devices, nil
}
