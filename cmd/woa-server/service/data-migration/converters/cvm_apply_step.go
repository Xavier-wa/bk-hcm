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

	tasktypes "hcm/cmd/woa-server/types/task"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/dal/table"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/dal/table/types"
	cvt "hcm/pkg/tools/converter"
)

func init() {
	Register(&CvmApplyStepConverter{})
}

// CvmApplyStepConverter 申请单步骤记录转换器（cr_ApplyStep -> ziyan_cvm_apply_step）
type CvmApplyStepConverter struct{}

// GetName 获取转换器名称
func (c *CvmApplyStepConverter) GetName() string {
	return table.ZiyanCvmApplyStepTable + "_converter"
}

// NewSourceDataSlice 创建源数据切片
func (c *CvmApplyStepConverter) NewSourceDataSlice() interface{} {
	return &[]*tasktypes.ApplyStep{}
}

// ConvertToCreate 转换为创建请求
func (c *CvmApplyStepConverter) ConvertToCreate(source interface{}) (interface{}, error) {
	applyStep, ok := source.(*tasktypes.ApplyStep)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *tasktypes.ApplyStep, got: %T", source)
	}

	return &cvmapplyproto.ZiyanCvmApplyStepCreateReq{
		SuborderID: applyStep.SubOrderId,
		StepID:     applyStep.StepId,
		StepName:   applyStep.StepName,
		Status:     applyStep.Status,
		Message:    applyStep.Message,
		TotalNum:   applyStep.TotalNum,
		SuccessNum: applyStep.SuccessNum,
		FailedNum:  applyStep.FailedNum,
		RunningNum: applyStep.RunningNum,
		StartAt:    applyStep.StartAt.Format(constant.TimeStdFormat),
		EndAt:      applyStep.EndAt.Format(constant.TimeStdFormat),
		CreatedAt:  types.Time(applyStep.CreateAt.In(time.Local).Format(constant.TimeStdFormat)),
		UpdatedAt:  types.Time(applyStep.UpdateAt.In(time.Local).Format(constant.TimeStdFormat)),
	}, nil
}

// ConvertToUpdate 转换为更新请求
func (c *CvmApplyStepConverter) ConvertToUpdate(source interface{}, target interface{}) (interface{}, error) {
	applyStep, ok := source.(*tasktypes.ApplyStep)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *tasktypes.ApplyStep, got: %T", source)
	}

	// 获取目标数据的ID（更新时必需）
	step, ok := target.(*cvmapplytable.ZiyanCvmApplyStep)
	if !ok {
		return nil, fmt.Errorf("target data type mismatch, expected *cvmapplytable.ZiyanCvmApplyStep, got: %T", target)
	}

	return &cvmapplyproto.ZiyanCvmApplyStepUpdateReq{
		ID:         step.ID, // 从目标数据中获取ID（主键）
		SuborderID: applyStep.SubOrderId,
		StepID:     cvt.ValToPtr(applyStep.StepId),
		StepName:   applyStep.StepName,
		Status:     cvt.ValToPtr(applyStep.Status),
		Message:    applyStep.Message,
		TotalNum:   cvt.ValToPtr(applyStep.TotalNum),
		SuccessNum: cvt.ValToPtr(applyStep.SuccessNum),
		FailedNum:  cvt.ValToPtr(applyStep.FailedNum),
		RunningNum: cvt.ValToPtr(applyStep.RunningNum),
		StartAt:    applyStep.StartAt.Format(constant.TimeStdFormat),
		EndAt:      applyStep.EndAt.Format(constant.TimeStdFormat),
	}, nil
}

// ExtractPrimaryKey 提取主键值（复合主键：suborder_id + step_name）
func (c *CvmApplyStepConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
	// 尝试作为源数据类型处理
	if applyStep, ok := data.(*tasktypes.ApplyStep); ok {
		return map[string]interface{}{
			"suborder_id": applyStep.SubOrderId,
			"step_name":   applyStep.StepName,
		}, nil
	}

	// 尝试作为目标数据类型处理
	if step, ok := data.(*cvmapplytable.ZiyanCvmApplyStep); ok {
		return map[string]interface{}{
			"suborder_id": step.SuborderID,
			"step_name":   step.StepName,
		}, nil
	}

	return nil, fmt.Errorf("data type mismatch, expected *tasktypes.ApplyStep or "+
		"*cvmapplytable.ZiyanCvmApplyStep, got: %T", data)
}

// CompareData 对比两个数据是否一致
func (c *CvmApplyStepConverter) CompareData(source, target interface{}, fields []string) (bool, []string) {
	applyStep, ok := source.(*tasktypes.ApplyStep)
	if !ok {
		return false, []string{"source_type_mismatch"}
	}

	step, ok := target.(*cvmapplytable.ZiyanCvmApplyStep)
	if !ok {
		return false, []string{"target_type_mismatch"}
	}

	diffFields := make([]string, 0)
	for _, field := range fields {
		if c.compareField(applyStep, step, field) {
			diffFields = append(diffFields, field)
		}
	}

	return len(diffFields) == 0, diffFields
}

// compareField 对比单个字段，返回true表示不一致
func (c *CvmApplyStepConverter) compareField(applyStep *tasktypes.ApplyStep,
	step *cvmapplytable.ZiyanCvmApplyStep, field string) bool {

	switch field {
	case "status":
		return applyStep.Status != cvt.PtrToVal(step.Status)
	case "total_num":
		return applyStep.TotalNum != cvt.PtrToVal(step.TotalNum)
	case "success_num":
		return applyStep.SuccessNum != cvt.PtrToVal(step.SuccessNum)
	case "failed_num":
		return applyStep.FailedNum != cvt.PtrToVal(step.FailedNum)
	case "running_num":
		return applyStep.RunningNum != cvt.PtrToVal(step.RunningNum)
	default:
		return false
	}
}
