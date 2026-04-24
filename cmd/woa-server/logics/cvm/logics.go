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

package cvm

import (
	"encoding/json"
	"fmt"
	"strconv"

	"hcm/cmd/woa-server/logics/config"
	rollingserver "hcm/cmd/woa-server/logics/rolling-server"
	taskLogics "hcm/cmd/woa-server/logics/task"
	"hcm/cmd/woa-server/logics/task/scheduler"
	model "hcm/cmd/woa-server/model/cvm"
	types "hcm/cmd/woa-server/types/cvm"
	rstypes "hcm/cmd/woa-server/types/rolling-server"
	taskTypes "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/thirdparty"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/thirdparty/cvmapi"
	"hcm/pkg/tools/metadata"
	"hcm/pkg/tools/times"
)

// Logics provides management interface for operations of model and instance and related resources like association
type Logics interface {
	// CreateCvmProductApplyOrder creates cvm product apply order
	CreateCvmProductApplyOrder(kt *kit.Kit, param *types.CvmCreateReq) (*types.CvmCreateResult, error)
	// GetApplyOrderById get cvm apply order info
	GetApplyOrderById(kt *kit.Kit, param *types.CvmOrderReq) (*types.CvmOrderResult, error)
	// GetApplyOrder get cvm apply order info
	GetApplyOrder(kt *kit.Kit, param *types.GetApplyParam) (*types.CvmOrderResult, error)
	// GetApplyDevice get cvm apply order launched instances
	GetApplyDevice(kt *kit.Kit, param *types.CvmDeviceReq) (*types.CvmDeviceResult, error)
	// GetCapacity get cvm apply capacity
	GetCapacity(kt *kit.Kit, param *types.CvmCapacityReq) (*types.CvmCapacityResult, error)
}

type logics struct {
	cvm            cvmapi.CVMClientInterface
	cliConf        cc.ClientConfig
	confLogic      config.Logics
	cmdbCli        cmdb.Client
	rsLogic        rollingserver.Logics
	taskLogic      taskLogics.Logics
	schedulerLogic scheduler.Interface
	client         *client.ClientSet
}

// New create a logics manager
func New(thirdCli *thirdparty.Client, cliConf cc.ClientConfig, confLogic config.Logics,
	cmdbCli cmdb.Client, rsLogic rollingserver.Logics, taskLogic taskLogics.Logics,
	schedulerLogic scheduler.Interface, client *client.ClientSet) Logics {

	return &logics{
		cvm:            thirdCli.CVM,
		confLogic:      confLogic,
		cliConf:        cliConf,
		cmdbCli:        cmdbCli,
		rsLogic:        rsLogic,
		taskLogic:      taskLogic,
		schedulerLogic: schedulerLogic,
		client:         client,
	}
}

func (l *logics) processingOrderByRequireType(kt *kit.Kit, order *taskTypes.ApplyOrder) error {
	switch order.RequireType {
	case enumor.RequireTypeRollServer:
		canApply, reason, err := l.rsLogic.CanApplyHost(kt, order.BkBizId, order.TotalNum, enumor.CvmProduceAppliedType)
		if err != nil {
			logs.Errorf("determine can apply rolling server host failed, err: %v, bizID: %s, total: %d, rid: %s",
				err, order.BkBizId, order.TotalNum, kt.Rid)
			return err
		}
		if !canApply {
			logs.Errorf("can not apply host, order: %+v, reason: %s, rid: %s", *order, reason, kt.Rid)
			return fmt.Errorf("%s", reason)
		}

		data := rstypes.CreateAppliedRecordData{
			BizID:       order.BkBizId,
			OrderID:     order.OrderId,
			SubOrderID:  strconv.FormatUint(order.OrderId, 10),
			DeviceType:  order.Spec.DeviceType,
			Count:       int(order.TotalNum),
			AppliedType: enumor.CvmProduceAppliedType,
			RequireType: order.RequireType,
		}

		if err = l.rsLogic.CreateAppliedRecord(kt, []rstypes.CreateAppliedRecordData{data}); err != nil {
			logs.Errorf("create rolling applied record failed, err: %v, order: %+v, rid: %s", err, *order, kt.Rid)
			return err
		}

	case enumor.RequireTypeSpringResPool:
		configChargeType, err := l.confLogic.SpringResPool().GetChargeType(kt, order.BkBizId)
		if err != nil {
			logs.Errorf("failed to get spring res pool charge type config, bizID: %d, err: %v, rid: %s",
				order.BkBizId, err, kt.Rid)
			return err
		}

		userChargeType := order.Spec.ChargeType
		if userChargeType != configChargeType {
			logs.Errorf("charge type mismatch for spring res pool, bizID: %d, user: %s, config: %s, rid: %s",
				order.BkBizId, userChargeType, configChargeType, kt.Rid)
			return fmt.Errorf("charge type mismatch: user selected %s, but config requires %s, please adjust",
				userChargeType, configChargeType)
		}

	default:
		return nil
	}

	return nil
}

// GetApplyOrderById get cvm apply order info by order id
func (l *logics) GetApplyOrderById(kt *kit.Kit, param *types.CvmOrderReq) (*types.CvmOrderResult, error) {
	filter := map[string]interface{}{
		"order_id": param.OrderId,
	}

	page := metadata.BasePage{
		Start: 0,
		Limit: 1,
	}

	insts, err := model.Operation().ApplyOrder().FindManyApplyOrder(kt.Ctx, page, filter)
	if err != nil {
		return nil, err
	}

	rst := &types.CvmOrderResult{
		Info: insts,
	}

	return rst, nil
}

// GetApplyOrder get cvm apply order info
func (l *logics) GetApplyOrder(kt *kit.Kit, param *types.GetApplyParam) (*types.CvmOrderResult, error) {
	filterExpr := param.GetFilterExpression()
	suborderReq := &cvmapplyproto.ZiyanCvmApplySuborderListReq{
		Filter: filterExpr,
		Page:   param.Page,
	}

	suborderResp, err := l.client.DataService().TCloudZiyan.ZiyanCvmApplySuborder.List(kt.Ctx, kt.Header(), suborderReq)
	if err != nil {
		logs.Errorf("failed to list apply suborder, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	if param.Page.Count {
		return &types.CvmOrderResult{Count: int64(suborderResp.Count), Info: make([]*types.ApplyOrder, 0)}, nil
	}

	var generateRecords map[string]*cvmapplytable.ZiyanCvmGenerateRecord
	if len(suborderResp.Details) > 0 {
		suborderIDs := make([]string, 0, len(suborderResp.Details))
		for _, suborder := range suborderResp.Details {
			suborderIDs = append(suborderIDs, suborder.SuborderID)
		}

		generateFilter := &filter.Expression{
			Op: filter.And,
			Rules: []filter.RuleFactory{
				&filter.AtomRule{
					Field: "suborder_id",
					Op:    filter.In.Factory(),
					Value: suborderIDs,
				},
			},
		}

		generateRecords = make(map[string]*cvmapplytable.ZiyanCvmGenerateRecord)
		start := uint32(0)

		for {
			generateReq := &cvmapplyproto.ZiyanCvmGenerateRecordListReq{
				Filter: generateFilter,
				Page: &core.BasePage{
					Count: false,
					Start: start,
					Limit: core.DefaultMaxPageLimit,
				},
			}

			generateResp, err := l.client.DataService().TCloudZiyan.ZiyanCvmGenerateRecord.List(kt.Ctx, kt.Header(),
				generateReq)
			if err != nil {
				logs.Errorf("failed to list generate records, err: %v, rid: %s", err, kt.Rid)
				return nil, err
			}

			for _, record := range generateResp.Details {
				if _, exists := generateRecords[record.SuborderID]; exists {
					return nil, fmt.Errorf("suborder %s has multiple generate records, rid: %s",
						record.SuborderID, kt.Rid)
				}
				generateRecords[record.SuborderID] = record
			}

			if len(generateResp.Details) < int(core.DefaultMaxPageLimit) {
				break
			}

			start += uint32(core.DefaultMaxPageLimit)
		}
	}
	applyList, err := convertApplySuborderList(kt, suborderResp.Details, generateRecords)
	if err != nil {
		return nil, err
	}

	return &types.CvmOrderResult{
		Count: int64(suborderResp.Count),
		Info:  applyList,
	}, nil
}

// convertApplySuborderList convert apply suborder list to apply order list
func convertApplySuborderList(kt *kit.Kit, details []*cvmapplytable.ZiyanCvmApplySuborder,
	generateRecords map[string]*cvmapplytable.ZiyanCvmGenerateRecord) ([]*types.ApplyOrder, error) {

	if len(details) == 0 {
		return make([]*types.ApplyOrder, 0), nil
	}

	rst := make([]*types.ApplyOrder, 0, len(details))
	for _, item := range details {
		var record *cvmapplytable.ZiyanCvmGenerateRecord
		if generateRecords != nil {
			record = generateRecords[item.SuborderID]
		}
		detailItem, err := convertApplySuborder(kt, item, record)
		if err != nil {
			return nil, err
		}
		rst = append(rst, detailItem)
	}
	return rst, nil
}

// convertApplySuborder convert apply suborder
func convertApplySuborder(kt *kit.Kit, item *cvmapplytable.ZiyanCvmApplySuborder,
	record *cvmapplytable.ZiyanCvmGenerateRecord) (*types.ApplyOrder, error) {

	if item == nil {
		return nil, nil
	}

	spec, err := buildOrderSpec(kt, item)
	if err != nil {
		logs.Errorf("failed to build order spec, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	message, taskID, taskLink, status, total, success, failed, pending := extractGenerateRecordInfo(record)

	createdAt, err := times.ParseTypesTime(item.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at failed: %w", err)
	}
	updatedAt, err := times.ParseTypesTime(item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at failed: %w", err)
	}

	return &types.ApplyOrder{
		OrderId:     item.OrderID,
		BkBizId:     item.BkBizID,
		User:        item.BkUsername,
		RequireType: int64(item.RequireType),
		Remark:      item.Remark,
		Spec:        spec,
		Status:      status,
		Message:     message,
		TaskId:      taskID,
		TaskLink:    taskLink,
		Total:       total,
		SuccessNum:  success,
		FailedNum:   failed,
		PendingNum:  pending,
		CreateAt:    createdAt,
		UpdateAt:    updatedAt,
	}, nil
}

// buildOrderSpec builds OrderSpec from apply suborder item
func buildOrderSpec(kt *kit.Kit, item *cvmapplytable.ZiyanCvmApplySuborder) (*types.OrderSpec, error) {
	spec := &types.OrderSpec{
		Region:            item.Region,
		Zone:              item.Zone,
		DeviceType:        item.DeviceType,
		ImageId:           item.ImageID,
		DiskSize:          item.DiskSize,
		DiskType:          item.DiskType,
		NetworkType:       item.NetworkType,
		Vpc:               item.Vpc,
		Subnet:            item.Subnet,
		ChargeType:        item.ChargeType,
		ChargeMonths:      item.ChargeMonths,
		InheritInstanceId: item.InheritInstanceID,
		BkAssetID:         item.BkAssetID,
	}

	if err := unmarshalSystemDisk(kt, string(item.SystemDisk), spec); err != nil {
		logs.Errorf("failed to unmarshal system disk, data: %s, rid: %s", string(item.SystemDisk), kt.Rid)
		return nil, err
	}

	if err := unmarshalDataDisk(kt, string(item.DataDisk), spec); err != nil {
		logs.Errorf("failed to unmarshal data disk, data: %s, rid: %s", string(item.DataDisk), kt.Rid)
		return nil, err
	}

	return spec, nil
}

// unmarshalSystemDisk unmarshal system disk JSON data
func unmarshalSystemDisk(kt *kit.Kit, systemDiskStr string, spec *types.OrderSpec) error {
	if len(systemDiskStr) > 0 {
		if err := json.Unmarshal([]byte(systemDiskStr), &spec.SystemDisk); err != nil {
			return fmt.Errorf("failed to unmarshal system disk, data: %s,err: %w,rid: %s", systemDiskStr,
				err, kt.Rid)
		}
	}
	return nil
}

// unmarshalDataDisk unmarshal data disk JSON data
func unmarshalDataDisk(kt *kit.Kit, dataDiskStr string, spec *types.OrderSpec) error {
	if len(dataDiskStr) > 0 {
		if err := json.Unmarshal([]byte(dataDiskStr), &spec.DataDisk); err != nil {
			return fmt.Errorf("failed to unmarshal data disk, data: %s,  err: %w,rid: %s", dataDiskStr,
				err, kt.Rid)
		}
	}
	return nil
}

// extractGenerateRecordInfo extracts information from generate records
func extractGenerateRecordInfo(record *cvmapplytable.ZiyanCvmGenerateRecord) (
	message string, taskID string, taskLink string, status types.ApplyStatus, total uint, success uint, failed uint,
	pending uint) {

	if record == nil {
		return
	}

	message = record.Message
	taskID = record.TaskID
	taskLink = record.TaskLink

	if record.Status != nil {
		status = convertGenerateStatusToApplyStatus(*record.Status)
	}
	if record.TotalNum != nil {
		total = *record.TotalNum
	}
	if record.SuccessNum != nil {
		success = *record.SuccessNum
	}
	if record.TotalNum != nil && record.SuccessNum != nil && record.Status != nil {
		failed, pending = calculateFailedAndPending(*record.Status, *record.TotalNum, *record.SuccessNum)
	}

	return
}

// calculateFailedAndPending calculates failed and pending counts based on status
func calculateFailedAndPending(status taskTypes.GenerateStepStatus, totalNum, successNum uint) (failed, pending uint) {
	switch status {
	case taskTypes.GenerateStatusFailed:
		failed = totalNum - successNum
	case taskTypes.GenerateStatusSuccess:
		// failed and pending are already 0
	default:
		pending = totalNum - successNum
	}
	return
}

// convertGenerateStatusToApplyStatus 将生成状态转换为申请状态
func convertGenerateStatusToApplyStatus(status taskTypes.GenerateStepStatus) types.ApplyStatus {
	switch status {
	case taskTypes.GenerateStatusSuccess:
		return types.ApplyStatusSuccess
	case taskTypes.GenerateStatusHandling:
		return types.ApplyStatusRunning
	case taskTypes.GenerateStatusFailed:
		return types.ApplyStatusFailed
	case taskTypes.GenerateStatusSuspend:
		return types.RecycleStatusPaused
	default:
		return types.ApplyStatusInit
	}
}

// GetApplyDevice get cvm apply order launched instances
func (l *logics) GetApplyDevice(kt *kit.Kit, param *types.CvmDeviceReq) (*types.CvmDeviceResult, error) {
	expr := &filter.Expression{
		Op: filter.And,
		Rules: []filter.RuleFactory{
			&filter.AtomRule{Field: "order_id", Op: filter.Equal.Factory(), Value: param.OrderId},
		},
	}

	insts := make([]*types.CvmInfo, 0)
	start := uint32(0)
	for {
		req := &cvmapplyproto.ZiyanCvmDeviceInfoListReq{
			Filter: expr,
			Page: &core.BasePage{
				Count: false,
				Start: start,
				Limit: core.DefaultMaxPageLimit,
			},
		}
		result, err := l.client.DataService().TCloudZiyan.ZiyanCvmDeviceInfo.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			logs.Errorf("failed to list cvm device info, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		cvmDeviceList, err := convertCvmDeviceInfoList(kt, result.Details)
		if err != nil {
			logs.Errorf("failed to convert cvm device info list, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		insts = append(insts, cvmDeviceList...)
		if len(result.Details) < int(core.DefaultMaxPageLimit) {
			break
		}
		start += uint32(len(result.Details))
	}

	rst := &types.CvmDeviceResult{
		Count: int64(len(insts)),
		Info:  insts,
	}

	return rst, nil
}

// convertCvmDeviceInfoList convert cvm device info  list
func convertCvmDeviceInfoList(kt *kit.Kit, details []*cvmapplytable.ZiyanCvmDeviceInfo) ([]*types.CvmInfo, error) {
	if len(details) == 0 {
		return make([]*types.CvmInfo, 0), nil
	}

	rst := make([]*types.CvmInfo, 0, len(details))
	for _, item := range details {
		detailItem, err := convertCvmDeviceInfo(kt, item)
		if err != nil {
			logs.Errorf("failed to convert cvm device info, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		rst = append(rst, detailItem)
	}
	return rst, nil
}

// convertCvmDeviceInfo convert cvm device info
func convertCvmDeviceInfo(kt *kit.Kit, item *cvmapplytable.ZiyanCvmDeviceInfo) (*types.CvmInfo, error) {
	if item == nil {
		return nil, nil
	}

	updatedAt, err := times.ParseTypesTime(item.UpdatedAt)
	if err != nil {
		logs.Errorf("failed to parse updated_at, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return &types.CvmInfo{
		OrderId:   uint64(item.OrderID),
		CvmTaskId: item.GenerateTaskID,
		AssetId:   item.AssetID,
		Ip:        item.IP,
		UpdateAt:  updatedAt,
	}, nil
}

// GetCapacity get cvm apply capacity
func (l *logics) GetCapacity(kt *kit.Kit, param *types.CvmCapacityReq) (*types.CvmCapacityResult, error) {
	req := &cvmapi.CapacityReq{
		ReqMeta: cvmapi.ReqMeta{
			Id:      cvmapi.CvmId,
			JsonRpc: cvmapi.CvmJsonRpc,
			Method:  cvmapi.CvmLaunchMethod,
		},
		Params: &cvmapi.CapacityParam{
			DeptId:       cvmapi.CvmDeptId,
			Business3Id:  int(param.BkBizId),
			CloudCampus:  param.Zone,
			InstanceType: param.DeviceType,
			VpcId:        param.VpcId,
			SubnetId:     param.SubnetId,
		},
	}

	// set project name
	req.Params.ProjectName = string(enumor.RequireType(param.RequireType).ToObsProject())

	resp, err := l.cvm.QueryCvmCapacity(nil, nil, req)
	if err != nil {
		logs.Errorf("scheduler:logics:cvm:capacity:failed, failed to get cvm apply capacity, err: %v, rid: %s", err,
			kt.Rid)
		return nil, err
	}

	// TODO: support return multiple vpc and subnet capacity
	rst := &types.CvmCapacityResult{
		Count: 1,
		Info:  make([]*types.CapacityItem, 0),
	}

	capacityItem := &types.CapacityItem{
		Region:   param.Region,
		Zone:     param.Zone,
		VpcId:    param.VpcId,
		SubnetId: param.SubnetId,
		MaxNum:   resp.Result.MaxNum,
		MaxInfo:  make([]*types.CapacityInfo, 0),
	}

	for _, info := range resp.Result.MaxInfo {
		capacityItem.MaxInfo = append(capacityItem.MaxInfo, &types.CapacityInfo{
			Key:   info.Key,
			Value: info.Value,
		})
	}

	rst.Info = append(rst.Info, capacityItem)
	return rst, nil
}
