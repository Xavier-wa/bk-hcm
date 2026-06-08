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

package converters

import (
	"fmt"
	"time"

	tasktable "hcm/cmd/woa-server/dal/task/table"
	cvmtypes "hcm/cmd/woa-server/types/cvm"
	tasktypes "hcm/cmd/woa-server/types/task"
	"hcm/pkg"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/dal/table/types"
	cvt "hcm/pkg/tools/converter"
)

// init 注册转换器
func init() {
	Register(&CvmCvmApplyOrderConverter{})
}

// CvmCvmApplyOrderConverter cr_CvmApplyOrder转换器
type CvmCvmApplyOrderConverter struct{}

// GetName 获取转换器名称
func (c *CvmCvmApplyOrderConverter) GetName() string {
	return pkg.BKTableNameCvmApplyOrder + "_converter"
}

// NewSourceDataSlice 创建源数据切片
func (c *CvmCvmApplyOrderConverter) NewSourceDataSlice() interface{} {
	return &[]*cvmtypes.ApplyOrder{}
}

// ConvertToCreate 转换为创建请求（同时创建子单和生产记录）
func (c *CvmCvmApplyOrderConverter) ConvertToCreate(source interface{}) (interface{}, error) {
	applyOrder, ok := source.(*cvmtypes.ApplyOrder)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *cvmtypes.ApplyOrder, got: %T", source)
	}

	if applyOrder.Spec == nil {
		return nil, fmt.Errorf("applyOrder.Spec is nil for order_id: %d", applyOrder.OrderId)
	}

	// 序列化 system_disk 和 data_disk
	systemDiskJSON, err := convertToJsonField(applyOrder.Spec.SystemDisk)
	if err != nil {
		return nil, fmt.Errorf("serialize system_disk failed: %w", err)
	}
	dataDiskJSON, err := convertToJsonField(applyOrder.Spec.DataDisk)
	if err != nil {
		return nil, fmt.Errorf("serialize data_disk failed: %w", err)
	}

	// 构建子单ID（使用 "{order_id}-1" 格式）
	suborderID := fmt.Sprintf("%d-1", applyOrder.OrderId)
	stage, status := convertCvmApplyStageStatus(applyOrder.Status)

	// 构建子单创建请求
	suborderReq := &cvmapplyproto.ZiyanCvmApplySuborderCreateReq{
		SuborderID:        suborderID,
		OrderID:           applyOrder.OrderId,
		BkBizID:           applyOrder.BkBizId,
		BkUsername:        applyOrder.User,
		Source:            enumor.ApplyTicketSrcBusiness,
		ProductType:       enumor.ProductTypeAdmin,
		RequireType:       enumor.RequireType(applyOrder.RequireType),
		ResourceType:      tasktypes.ResourceTypeCvm,
		Stage:             stage,
		Status:            status,
		TotalNum:          applyOrder.Total,
		SuccessNum:        applyOrder.SuccessNum,
		PendingNum:        applyOrder.PendingNum,
		FailedNum:         applyOrder.FailedNum,
		Region:            applyOrder.Spec.Region,
		Zone:              applyOrder.Spec.Zone,
		ImageID:           applyOrder.Spec.ImageId,
		DeviceType:        applyOrder.Spec.DeviceType,
		DiskSize:          applyOrder.Spec.DiskSize,
		DiskType:          applyOrder.Spec.DiskType,
		NetworkType:       applyOrder.Spec.NetworkType,
		Vpc:               applyOrder.Spec.Vpc,
		Subnet:            applyOrder.Spec.Subnet,
		ChargeType:        applyOrder.Spec.ChargeType,
		ChargeMonths:      applyOrder.Spec.ChargeMonths,
		InheritInstanceID: applyOrder.Spec.InheritInstanceId,
		SystemDisk:        systemDiskJSON,
		DataDisk:          dataDiskJSON,
		Remark:            applyOrder.Remark,
		CreatedAt:         types.Time(applyOrder.CreateAt.In(time.Local).Format(constant.TimeStdFormat)),
		UpdatedAt:         types.Time(applyOrder.UpdateAt.In(time.Local).Format(constant.TimeStdFormat)),
	}

	// 构建生产记录创建请求
	generateRecordReq := &cvmapplyproto.ZiyanCvmGenerateRecordCreateReq{
		SuborderID:   suborderID,
		GenerateType: string(tasktable.ResourceTypeCvm),
		TaskID:       applyOrder.TaskId,
		TaskLink:     applyOrder.TaskLink,
		Status:       convertCvmApplyStatusToGenerateStatus(applyOrder.Status),
		Message:      applyOrder.Message,
		TotalNum:     applyOrder.Total,
		SuccessNum:   applyOrder.SuccessNum,
		CreatedAt:    types.Time(applyOrder.CreateAt.In(time.Local).Format(constant.TimeStdFormat)),
		UpdatedAt:    types.Time(applyOrder.UpdateAt.In(time.Local).Format(constant.TimeStdFormat)),
	}

	// 返回多表创建请求
	return &MultiTableCreateRequest{
		SuborderRequest:       suborderReq,
		GenerateRecordRequest: generateRecordReq,
	}, nil
}

// ConvertToUpdate 转换为更新请求（同时更新子单和生产记录）
func (c *CvmCvmApplyOrderConverter) ConvertToUpdate(source interface{}, target interface{}) (interface{}, error) {
	applyOrder, ok := source.(*cvmtypes.ApplyOrder)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *cvmtypes.ApplyOrder, got: %T", source)
	}

	suborder, ok := target.(*cvmapplytable.ZiyanCvmApplySuborder)
	if !ok {
		return nil, fmt.Errorf("target data type mismatch, "+
			"expected *cvmapplytable.ZiyanCvmApplySuborder, got: %T", target)
	}

	if applyOrder.Spec == nil {
		return nil, fmt.Errorf("applyOrder.Spec is nil for order_id: %d", applyOrder.OrderId)
	}

	// 序列化 system_disk 和 data_disk
	systemDiskJSON, err := convertToJsonField(applyOrder.Spec.SystemDisk)
	if err != nil {
		return nil, fmt.Errorf("serialize system_disk failed: %w", err)
	}
	dataDiskJSON, err := convertToJsonField(applyOrder.Spec.DataDisk)
	if err != nil {
		return nil, fmt.Errorf("serialize data_disk failed: %w", err)
	}
	stage, status := convertCvmApplyStageStatus(applyOrder.Status)

	// 构建子单更新请求
	suborderUpdateReq := &cvmapplyproto.ZiyanCvmApplySuborderUpdateReq{
		SuborderID:        suborder.SuborderID,
		OrderID:           applyOrder.OrderId,
		BkUsername:        applyOrder.User,
		RequireType:       enumor.RequireType(applyOrder.RequireType),
		Stage:             stage,
		Status:            status,
		TotalNum:          cvt.ValToPtr(applyOrder.Total),
		SuccessNum:        cvt.ValToPtr(applyOrder.SuccessNum),
		PendingNum:        cvt.ValToPtr(applyOrder.PendingNum),
		FailedNum:         cvt.ValToPtr(applyOrder.FailedNum),
		Region:            applyOrder.Spec.Region,
		Zone:              applyOrder.Spec.Zone,
		ImageID:           applyOrder.Spec.ImageId,
		DeviceType:        applyOrder.Spec.DeviceType,
		DiskSize:          cvt.ValToPtr(applyOrder.Spec.DiskSize),
		DiskType:          applyOrder.Spec.DiskType,
		NetworkType:       applyOrder.Spec.NetworkType,
		Vpc:               cvt.ValToPtr(applyOrder.Spec.Vpc),
		Subnet:            cvt.ValToPtr(applyOrder.Spec.Subnet),
		ChargeType:        applyOrder.Spec.ChargeType,
		ChargeMonths:      cvt.ValToPtr(applyOrder.Spec.ChargeMonths),
		InheritInstanceID: applyOrder.Spec.InheritInstanceId,
		SystemDisk:        cvt.ValToPtr(systemDiskJSON),
		DataDisk:          cvt.ValToPtr(dataDiskJSON),
		Remark:            applyOrder.Remark,
	}

	// 构建生产记录更新请求（GenerateID 会在 API 路由器层面根据 suborder_id 查询后填充）
	generateRecordUpdateReq := &cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq{
		GenerateID: "", // 占位符，会在 handleMultiTableUpdate 中查询并填充真实的 generate_id
		SuborderID: suborder.SuborderID,
		TaskID:     cvt.ValToPtr(applyOrder.TaskId),
		TaskLink:   cvt.ValToPtr(applyOrder.TaskLink),
		Status:     cvt.ValToPtr(convertCvmApplyStatusToGenerateStatus(applyOrder.Status)),
		Message:    cvt.ValToPtr(applyOrder.Message),
		TotalNum:   cvt.ValToPtr(applyOrder.Total),
		SuccessNum: cvt.ValToPtr(applyOrder.SuccessNum),
	}

	// 返回多表更新请求
	return &MultiTableUpdateRequest{
		SuborderRequest:       suborderUpdateReq,
		GenerateRecordRequest: generateRecordUpdateReq,
	}, nil
}

// ExtractPrimaryKey 提取主键
func (c *CvmCvmApplyOrderConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
	// 尝试作为源数据类型处理
	if applyOrder, ok := data.(*cvmtypes.ApplyOrder); ok {
		return map[string]interface{}{
			"suborder_id": fmt.Sprintf("%d-1", applyOrder.OrderId),
		}, nil
	}

	// 尝试作为目标数据类型处理
	if suborder, ok := data.(*cvmapplytable.ZiyanCvmApplySuborder); ok {
		return map[string]interface{}{
			"suborder_id": suborder.SuborderID,
		}, nil
	}

	return nil, fmt.Errorf("data type mismatch, expected *cvmtypes.ApplyOrder or "+
		"*cvmapplytable.ZiyanCvmApplySuborder, got: %T", data)
}

// CompareData 比较数据
func (c *CvmCvmApplyOrderConverter) CompareData(source interface{}, target interface{}, fields []string) (bool, []string) {
	applyOrder, ok := source.(*cvmtypes.ApplyOrder)
	if !ok {
		return false, []string{"source_type_mismatch"}
	}

	suborder, ok := target.(*cvmapplytable.ZiyanCvmApplySuborder)
	if !ok {
		return false, []string{"target_type_mismatch"}
	}

	diffFields := make([]string, 0)
	for _, field := range fields {
		if c.compareField(applyOrder, suborder, field) {
			diffFields = append(diffFields, field)
		}
	}

	return len(diffFields) == 0, diffFields
}

// compareField 比较单个字段
func (c *CvmCvmApplyOrderConverter) compareField(applyOrder *cvmtypes.ApplyOrder,
	suborder *cvmapplytable.ZiyanCvmApplySuborder, field string) bool {

	if applyOrder.Spec == nil {
		return false
	}

	stage, status := convertCvmApplyStageStatus(applyOrder.Status)
	switch field {
	case "stage":
		return stage != suborder.Stage
	case "status":
		return status != suborder.Status
	case "total_num":
		return applyOrder.Total != cvt.PtrToVal(suborder.TotalNum)
	case "success_num":
		return applyOrder.SuccessNum != cvt.PtrToVal(suborder.SuccessNum)
	case "pending_num":
		return applyOrder.PendingNum != cvt.PtrToVal(suborder.PendingNum)
	case "failed_num":
		return applyOrder.FailedNum != cvt.PtrToVal(suborder.FailedNum)
	case "region":
		return applyOrder.Spec.Region != suborder.Region
	case "zone":
		return applyOrder.Spec.Zone != suborder.Zone
	case "image_id":
		return applyOrder.Spec.ImageId != suborder.ImageID
	case "device_type":
		return applyOrder.Spec.DeviceType != suborder.DeviceType
	case "charge_type":
		return applyOrder.Spec.ChargeType != suborder.ChargeType
	case "charge_months":
		return applyOrder.Spec.ChargeMonths != suborder.ChargeMonths
	default:
		return false
	}
}

// convertCvmApplyStageStatus 转换申请阶段和状态
func convertCvmApplyStageStatus(status cvmtypes.ApplyStatus) (tasktypes.TicketStage, tasktypes.ApplyStatus) {
	switch status {
	case cvmtypes.ApplyStatusInit:
		return tasktypes.TicketStageUncommit, tasktypes.ApplyStatusWaitForMatch
	case cvmtypes.ApplyStatusRunning:
		return tasktypes.TicketStageRunning, tasktypes.ApplyStatusMatching
	case cvmtypes.RecycleStatusPaused:
		return tasktypes.TicketStageSuspend, tasktypes.ApplyStatusPaused
	case cvmtypes.RecycleStatusDone:
		return tasktypes.TicketStageDone, tasktypes.ApplyStatusDone
	case cvmtypes.ApplyStatusSuccess:
		return tasktypes.TicketStageDone, tasktypes.ApplyStatusDone
	case cvmtypes.ApplyStatusFailed:
		return tasktypes.TicketStageSuspend, tasktypes.ApplyStatusTerminate
	default:
		return tasktypes.TicketStageUncommit, tasktypes.ApplyStatusWaitForMatch
	}
}

// convertCvmApplyStatusToGenerateStatus 转换申请状态到生产记录状态
func convertCvmApplyStatusToGenerateStatus(status cvmtypes.ApplyStatus) tasktypes.GenerateStepStatus {
	switch status {
	case cvmtypes.ApplyStatusInit:
		return tasktypes.GenerateStatusInit
	case cvmtypes.ApplyStatusRunning:
		return tasktypes.GenerateStatusHandling
	case cvmtypes.RecycleStatusPaused:
		return tasktypes.GenerateStatusSuspend
	case cvmtypes.RecycleStatusDone:
		return tasktypes.GenerateStatusSuccess
	case cvmtypes.ApplyStatusSuccess:
		return tasktypes.GenerateStatusSuccess
	case cvmtypes.ApplyStatusFailed:
		return tasktypes.GenerateStatusFailed
	default:
		return tasktypes.GenerateStatusInit
	}
}
