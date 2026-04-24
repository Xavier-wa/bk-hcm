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

// Package record provides record functions
package record

import (
	"time"

	"hcm/cmd/woa-server/model/task"
	types "hcm/cmd/woa-server/types/task"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"
)

// CreateCommitStep init apply order commit step info
func CreateCommitStep(kt *kit.Kit, suborderId string, replicas uint, stepID int) error {
	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("step_name", types.StepNameCommit),
	)
	stepCnt, err := model.Operation().ApplyStep().CountApplyStep(kt, filter)
	if err != nil {
		logs.Errorf("failed to create commit step, err: %v", err)
		return err
	}
	if stepCnt > 0 {
		return nil
	}

	// create step if no record in db
	now := time.Now()
	step := &types.ApplyStep{
		SubOrderId: suborderId,
		StepId:     stepID,
		StepName:   types.StepNameCommit,
		Status:     types.StepStatusSuccess,
		Message:    types.StepMsgSuccess,
		TotalNum:   replicas,
		SuccessNum: replicas,
		FailedNum:  0,
		RunningNum: 0,
		CreateAt:   now,
		UpdateAt:   now,
		StartAt:    now,
		EndAt:      now,
	}
	if err = model.Operation().ApplyStep().CreateApplyStep(kt, step); err != nil {
		logs.Errorf("failed to create commit step, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// CreateGenerateStep init apply order generate step info
func CreateGenerateStep(kt *kit.Kit, suborderId string, replicas uint,
	stepID int) error {

	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("step_name", types.StepNameGenerate),
	)
	stepCnt, err := model.Operation().ApplyStep().CountApplyStep(kt, filter)
	if err != nil {
		logs.Errorf("failed to create generate step, err: %v, rid: %s", err, kt.Rid)
		return err
	}
	if stepCnt > 0 {
		return nil
	}

	// create step if no record in db
	now := time.Now()
	step := &types.ApplyStep{
		SubOrderId: suborderId,
		StepId:     stepID,
		StepName:   types.StepNameGenerate,
		Status:     types.StepStatusInit,
		Message:    types.StepMsgInit,
		TotalNum:   replicas,
		SuccessNum: 0,
		FailedNum:  0,
		RunningNum: 0,
		CreateAt:   now,
		UpdateAt:   now,
	}
	if err = model.Operation().ApplyStep().CreateApplyStep(kt, step); err != nil {
		logs.Errorf("failed to create generate step, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// UpdateGenerateStep update apply order generate step info
func UpdateGenerateStep(kt *kit.Kit, suborderId string,
	total uint, errStep error) error {

	now := time.Now()
	if errStep != nil {
		filter := tools.ExpressionAnd(
			tools.RuleEqual("suborder_id", suborderId),
			tools.RuleEqual("step_name", types.StepNameGenerate),
		)
		update := &cvmapplyproto.ZiyanCvmApplyStepUpdateReq{
			Status:  cvt.ValToPtr(types.StepStatusFailed),
			Message: errStep.Error(),
			EndAt:   now.Format(constant.DateTimeLayout),
		}

		if err := model.Operation().ApplyStep().UpdateApplyStep(kt, filter, update); err != nil {
			logs.Errorf("failed to update generate step, err: %v, rid: %s", err, kt.Rid)
			return err
		}
		return nil
	}

	devices, err := getUnreleasedDevice(kt, suborderId)
	if err != nil {
		logs.Errorf("failed to update generate step, err: %v", err)
		return err
	}

	status := types.StepStatusHandling
	message := types.StepMsgHandling
	count := uint(len(devices))
	if count >= total {
		status = types.StepStatusSuccess
		message = types.StepMsgSuccess
	}

	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("step_name", types.StepNameGenerate),
	)

	update := &cvmapplyproto.ZiyanCvmApplyStepUpdateReq{
		Status:     cvt.ValToPtr(status),
		Message:    message,
		SuccessNum: cvt.ValToPtr(count),
	}

	if status == types.StepStatusSuccess {
		update.EndAt = now.Format(constant.DateTimeLayout)
	}

	if err = model.Operation().ApplyStep().UpdateApplyStep(kt, filter, update); err != nil {
		logs.Errorf("failed to update generate step, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// CreateInitStep init apply order init step info
func CreateInitStep(kt *kit.Kit, suborderId string, replicas uint, stepID int) error {
	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("step_name", types.StepNameInit),
	)
	stepCnt, err := model.Operation().ApplyStep().CountApplyStep(kt, filter)
	if err != nil {
		logs.Errorf("failed to create init step, err: %v, rid: %s", err, kt.Rid)
		return err
	}
	if stepCnt > 0 {
		return nil
	}

	// create step if no record in db
	now := time.Now()
	step := &types.ApplyStep{
		SubOrderId: suborderId,
		StepId:     stepID,
		StepName:   types.StepNameInit,
		Status:     types.StepStatusInit,
		Message:    types.StepMsgInit,
		TotalNum:   replicas,
		SuccessNum: 0,
		FailedNum:  0,
		RunningNum: 0,
		CreateAt:   now,
		UpdateAt:   now,
	}
	if err = model.Operation().ApplyStep().CreateApplyStep(kt, step); err != nil {
		logs.Errorf("failed to create init step, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// UpdateInitStep update apply order init step info
func UpdateInitStep(kt *kit.Kit, suborderId string, total uint) error {
	devices, err := getUnreleasedDevice(kt, suborderId)
	if err != nil {
		logs.Errorf("failed to update init step, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	status := types.StepStatusHandling
	message := types.StepMsgHandling
	count := uint(0)
	for _, device := range devices {
		if device.IsInited {
			count++
		}
	}
	if count >= total {
		status = types.StepStatusSuccess
		message = types.StepMsgSuccess
	}

	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("step_name", types.StepNameInit),
	)

	now := time.Now()
	update := &cvmapplyproto.ZiyanCvmApplyStepUpdateReq{
		Status:     cvt.ValToPtr(status),
		Message:    message,
		SuccessNum: cvt.ValToPtr(count),
	}

	if status == types.StepStatusSuccess {
		update.EndAt = now.Format(constant.DateTimeLayout)
	}

	if err = model.Operation().ApplyStep().UpdateApplyStep(kt, filter, update); err != nil {
		logs.Errorf("failed to update init step, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// CreateDiskCheckStep init apply order disk check step info
func CreateDiskCheckStep(kt *kit.Kit, suborderId string, replicas uint, stepID int) error {
	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("step_name", types.StepNameDiskCheck),
	)
	stepCnt, err := model.Operation().ApplyStep().CountApplyStep(kt, filter)
	if err != nil {
		logs.Errorf("failed to create disk check step, err: %v, rid: %s", err, kt.Rid)
		return err
	}
	if stepCnt > 0 {
		return nil
	}

	// create step if no record in db
	now := time.Now()
	step := &types.ApplyStep{
		SubOrderId: suborderId,
		StepId:     stepID,
		StepName:   types.StepNameDiskCheck,
		Status:     types.StepStatusInit,
		Message:    types.StepMsgInit,
		TotalNum:   replicas,
		SuccessNum: 0,
		FailedNum:  0,
		RunningNum: 0,
		CreateAt:   now,
		UpdateAt:   now,
	}
	if err = model.Operation().ApplyStep().CreateApplyStep(kt, step); err != nil {
		logs.Errorf("failed to create disk check step, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// UpdateDiskCheckStep update apply order disk check step info
func UpdateDiskCheckStep(kt *kit.Kit, suborderId string, total uint) error {
	devices, err := getUnreleasedDevice(kt, suborderId)
	if err != nil {
		logs.Errorf("failed to update disk check step, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	status := types.StepStatusHandling
	message := types.StepMsgHandling
	count := uint(0)
	for _, device := range devices {
		if device.IsDiskChecked {
			count++
		}
	}
	if count >= total {
		status = types.StepStatusSuccess
		message = types.StepMsgSuccess
	}

	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("step_name", types.StepNameDiskCheck),
	)

	now := time.Now()
	update := &cvmapplyproto.ZiyanCvmApplyStepUpdateReq{
		Status:     cvt.ValToPtr(status),
		Message:    message,
		SuccessNum: cvt.ValToPtr(count),
	}

	if status == types.StepStatusSuccess {
		update.EndAt = now.Format(constant.DateTimeLayout)
	}

	if err = model.Operation().ApplyStep().UpdateApplyStep(kt, filter, update); err != nil {
		logs.Errorf("failed to update disk check step, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// CreateDeliverStep init apply order deliver step info
func CreateDeliverStep(kt *kit.Kit, suborderId string, replicas uint, stepID int) error {
	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("step_name", types.StepNameDeliver),
	)
	stepCnt, err := model.Operation().ApplyStep().CountApplyStep(kt, filter)
	if err != nil {
		logs.Errorf("failed to create deliver step, err: %v, rid: %s", err, kt.Rid)
		return err
	}
	if stepCnt > 0 {
		return nil
	}

	// create step if no record in db
	now := time.Now()
	step := &types.ApplyStep{
		SubOrderId: suborderId,
		StepId:     stepID,
		StepName:   types.StepNameDeliver,
		Status:     types.StepStatusInit,
		Message:    types.StepMsgInit,
		TotalNum:   replicas,
		SuccessNum: 0,
		FailedNum:  0,
		RunningNum: 0,
		CreateAt:   now,
		UpdateAt:   now,
	}
	if err = model.Operation().ApplyStep().CreateApplyStep(kt, step); err != nil {
		logs.Errorf("failed to create deliver step, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// UpdateDeliverStep update apply order deliver step info
func UpdateDeliverStep(kt *kit.Kit, suborderId string, total uint) error {
	devices, err := getUnreleasedDevice(kt, suborderId)
	if err != nil {
		logs.Errorf("failed to update deliver step, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	status := types.StepStatusHandling
	message := types.StepMsgHandling
	count := uint(0)
	for _, device := range devices {
		if device.IsDelivered {
			count++
		}
	}
	if count >= total {
		status = types.StepStatusSuccess
		message = types.StepMsgSuccess
	}

	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("step_name", types.StepNameDeliver),
	)

	now := time.Now()
	update := &cvmapplyproto.ZiyanCvmApplyStepUpdateReq{
		Status:     cvt.ValToPtr(status),
		Message:    message,
		SuccessNum: cvt.ValToPtr(count),
	}

	if status == types.StepStatusSuccess {
		update.EndAt = now.Format(constant.DateTimeLayout)
	}

	if err = model.Operation().ApplyStep().UpdateApplyStep(kt, filter, update); err != nil {
		logs.Errorf("failed to update deliver step, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// StartStep update apply order step with start info
func StartStep(kt *kit.Kit, suborderId string, stepName string) error {
	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("step_name", stepName),
		tools.RuleEqual("status", types.StepStatusInit),
	)

	now := time.Now()
	update := &cvmapplyproto.ZiyanCvmApplyStepUpdateReq{
		Status:  cvt.ValToPtr(types.StepStatusHandling),
		Message: types.StepMsgHandling,
		StartAt: now.Format(constant.DateTimeLayout),
	}

	if err := model.Operation().ApplyStep().UpdateApplyStep(kt, filter, update); err != nil {
		logs.Errorf("failed to start order %s step name %s, err: %v, rid: %s", suborderId, stepName, err, kt.Rid)
		return err
	}

	return nil
}

// getUnreleasedDevice gets unreleased devices binding to given apply order
func getUnreleasedDevice(kt *kit.Kit, subOrderID string) ([]*types.DeviceInfo, error) {
	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", subOrderID))
	devices, err := model.Operation().DeviceInfo().GetDeviceInfo(kt, filter)
	if err != nil {
		logs.Errorf("failed to get binding devices to subOrderID: %s, err: %v, rid: %s", subOrderID, err, kt.Rid)
		return nil, err
	}

	return devices, nil
}
