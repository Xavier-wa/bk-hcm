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

package record

import (
	"time"

	"hcm/cmd/woa-server/model/task"
	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"
)

// CreateInitRecord create resource apply init record.
//
// 返回值 created:
//   - true:  当次调用实际向 DB 写入了一条新记录；
//   - false: (suborder_id, ip) 已存在记录（包括 count>0 提前返回，以及count=0 后 INSERT 命中唯一索引冲突两种并发场景）
//
// 上层调用方必须根据 created 判断后续是否要发起新的标准运维(sops)初始化任务，
// 避免并发场景下重复发起；created=false 时通常应复用 DB 中已有记录的任务信息。
func CreateInitRecord(kt *kit.Kit, suborderId, ip string) (bool, error) {
	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", suborderId), tools.RuleEqual("ip", ip))
	cnt, err := model.Operation().InitRecord().CountInitRecord(kt, filter)
	if err != nil {
		logs.Errorf("failed to count init record, err: %v, rid: %s", err, kt.Rid)
		return false, err
	}
	if cnt > 0 {
		return false, nil
	}

	now := time.Now()
	record := &types.InitRecord{
		SubOrderId: suborderId,
		Ip:         ip,
		TaskId:     "",
		TaskLink:   "",
		Status:     types.InitStatusInit,
		Message:    "initing",
		CreateAt:   now,
		UpdateAt:   now,
		StartAt:    now,
		EndAt:      now,
	}
	if err = model.Operation().InitRecord().CreateInitRecord(kt, record); err != nil {
		// 并发场景：在 count 之后、insert 之前被其他协程抢先创建，由表唯一索引兜底
		// 返回 RecordDuplicated。视为已存在，由上层决定是否复用现有记录。
		if errf.IsDuplicated(err) {
			logs.Warnf("init record already exists due to concurrent create, suborderID: %s, ip: %s, err: %v, rid: %s",
				suborderId, ip, err, kt.Rid)
			return false, nil
		}
		logs.Errorf("failed to create init record, suborderID: %s, ip: %s, err: %v, rid: %s",
			suborderId, ip, err, kt.Rid)
		return false, err
	}

	return true, nil
}

// UpdateInitRecord update resource apply init record
//
// 状态机说明：
//   - InitStatusInit / InitStatusHandling 是中间态，允许更新到任意状态
//   - InitStatusFailed 在 matcher.initDevice 中是可重试的中间态，允许被覆盖（failed → handling/success/failed）
//   - InitStatusSuccess 是不可逆的终态，已经成功的 IP 初始化记录不应被任何并发任务再覆盖
//
// 因此 filter 仅排除当前已是 InitStatusSuccess 的记录；目标 status 不做限制。
func UpdateInitRecord(kt *kit.Kit, suborderId, ip, taskId, taskUrl, message string,
	status types.InitStepStatus) error {

	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", suborderId),
		tools.RuleEqual("ip", ip),
		tools.RuleNotEqual("status", types.InitStatusSuccess),
	)

	now := time.Now()
	update := &cvmapplyproto.ZiyanCvmApplyInitTaskUpdateReq{
		Status:  cvt.ValToPtr(status),
		Message: message,
		EndAt:   now.Format(constant.DateTimeLayout),
	}

	if taskId != "" {
		update.TaskID = taskId
		update.TaskLink = taskUrl
	}

	if err := model.Operation().InitRecord().UpdateInitRecord(kt, filter, update); err != nil {
		logs.Errorf("failed to update init record, suborderId: %s, err: %v, status: %d, ip: %s, rid: %s",
			suborderId, err, status, ip, kt.Rid)
		return err
	}

	return nil
}

// CreateDeliverRecord create resource apply deliver record
func CreateDeliverRecord(kt *kit.Kit, info *types.DeviceInfo) error {
	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", info.SubOrderId), tools.RuleEqual("ip", info.Ip))
	cnt, err := model.Operation().DeliverRecord().CountDeliverRecord(kt, filter)
	if err != nil {
		logs.Errorf("failed to count deliver record, subOrderID: %s, err: %v, rid: %s", info.SubOrderId, err, kt.Rid)
		return err
	}
	if cnt > 0 {
		return nil
	}

	now := time.Now()
	record := &types.DeliverRecord{
		SubOrderId:       info.SubOrderId,
		Ip:               info.Ip,
		AssetId:          info.AssetId,
		Status:           types.DeliverStatusHandling,
		Message:          "handling",
		Deliverer:        info.Deliverer,
		GenerateTaskId:   info.GenerateTaskId,
		GenerateTaskLink: info.GenerateTaskLink,
		InitTaskId:       info.InitTaskId,
		InitTaskLink:     info.InitTaskLink,
		IsManualMatched:  info.IsManualMatched,
		CreateAt:         now,
		UpdateAt:         now,
		StartAt:          now,
	}
	if err = model.Operation().DeliverRecord().CreateDeliverRecord(kt, record); err != nil {
		logs.Errorf("failed to create deliver record, subOrderID: %s, err: %v, rid: %s", info.SubOrderId, err, kt.Rid)
		return err
	}

	return nil
}

// UpdateDeliverRecord update resource apply deliver record
func UpdateDeliverRecord(kt *kit.Kit, info *types.DeviceInfo, message string,
	status types.DeliverStepStatus) error {

	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", info.SubOrderId), tools.RuleEqual("ip", info.Ip))

	now := time.Now()
	update := &cvmapplyproto.ZiyanCvmDeliverRecordUpdateReq{
		Status:  cvt.ValToPtr(status),
		Message: message,
		EndAt:   now.Format(constant.DateTimeLayout),
	}

	if err := model.Operation().DeliverRecord().UpdateDeliverRecord(kt, filter, update); err != nil {
		logs.Errorf("failed to update deliver record, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// GetDeliverRecord get resource apply deliver record
func GetDeliverRecord(kt *kit.Kit, subOrderId string, ip string, assetId string) (
	*types.DeliverRecord, error) {

	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", subOrderId),
		tools.RuleEqual("ip", ip),
		tools.RuleEqual("asset_id", assetId),
	)

	record, err := model.Operation().DeliverRecord().GetDeliverRecord(kt, filter)
	if err != nil {
		logs.Errorf("failed to get deliver record, ip: %s, assetId: %s, err: %v, rid: %s", ip, assetId, err, kt.Rid)
		return nil, err
	}

	return record, nil
}

// GetInitRecords get init records
func GetInitRecords(kt *kit.Kit, subOrderId string) ([]*types.InitRecord, error) {
	records := make([]*types.InitRecord, 0)
	startIndex := uint32(0)
	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", subOrderId))
	for {
		page := &core.BasePage{
			Start: startIndex,
			Limit: constant.BatchOperationMaxLimit,
		}

		record, err := model.Operation().InitRecord().FindManyInitRecord(kt, filter, page)
		if err != nil {
			logs.Errorf("failed to get init record, err: %v, subOrderId: %s, rid: %s", err, subOrderId, kt.Rid)
			return nil, err
		}
		records = append(records, record...)
		if len(record) < constant.BatchOperationMaxLimit {
			break
		}
		startIndex += constant.BatchOperationMaxLimit
	}

	return records, nil
}

// GetInitRecord get init record by ip
func GetInitRecord(kt *kit.Kit, subOrderId string, ip string) (
	*types.InitRecord, error) {

	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", subOrderId), tools.RuleEqual("ip", ip))
	record, err := model.Operation().InitRecord().GetInitRecord(kt, filter)
	if err != nil {
		logs.Errorf("failed to get init record, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return record, nil
}
