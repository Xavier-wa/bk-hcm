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
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"
)

func init() {
	Register(&CvmGenerateRecordConverter{})
}

// CvmGenerateRecordConverter 生产任务记录转换器（cr_GenerateRecord -> ziyan_cvm_generate_record）
type CvmGenerateRecordConverter struct{}

// GetName 获取转换器名称
func (c *CvmGenerateRecordConverter) GetName() string {
	return table.ZiyanCvmGenerateRecordTable + "_converter"
}

// NewSourceDataSlice 创建源数据切片
func (c *CvmGenerateRecordConverter) NewSourceDataSlice() interface{} {
	return &[]*tasktypes.GenerateRecord{}
}

// ConvertToCreate 转换为创建请求
func (c *CvmGenerateRecordConverter) ConvertToCreate(source interface{}) (interface{}, error) {
	generateRecord, ok := source.(*tasktypes.GenerateRecord)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *tasktypes.GenerateRecord, got: %T", source)
	}

	// 转换SuccessList为JSON
	successListJSON, err := convertToJsonField(generateRecord.SuccessList)
	if err != nil {
		logs.Errorf("convert success_list to JSON failed: %v", err)
		successListJSON = types.JsonField("[]")
	}

	return &cvmapplyproto.ZiyanCvmGenerateRecordCreateReq{
		GenerateID:   fmt.Sprintf("%d", generateRecord.GenerateId),
		SuborderID:   generateRecord.SubOrderId,
		GenerateType: generateRecord.GenerateType,
		TaskID:       generateRecord.TaskId,
		TaskLink:     generateRecord.TaskLink,
		RequestInfo:  generateRecord.RequestInfo,
		Status:       generateRecord.Status,
		IsMatched:    generateRecord.IsMatched,
		Message:      generateRecord.Message,
		TotalNum:     generateRecord.TotalNum,
		SuccessNum:   generateRecord.SuccessNum,
		SuccessList:  successListJSON,
		StartAt:      generateRecord.StartAt.Format(constant.TimeStdFormat),
		EndAt:        generateRecord.EndAt.Format(constant.TimeStdFormat),
		CreatedAt:    types.Time(generateRecord.CreateAt.In(time.Local).Format(constant.TimeStdFormat)),
		UpdatedAt:    types.Time(generateRecord.UpdateAt.In(time.Local).Format(constant.TimeStdFormat)),
	}, nil
}

// ConvertToUpdate 转换为更新请求
func (c *CvmGenerateRecordConverter) ConvertToUpdate(source interface{}, target interface{}) (interface{}, error) {
	generateRecord, ok := source.(*tasktypes.GenerateRecord)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *tasktypes.GenerateRecord, got: %T", source)
	}

	// 获取目标数据的GenerateID（更新时必需）
	record, ok := target.(*cvmapplytable.ZiyanCvmGenerateRecord)
	if !ok {
		return nil, fmt.Errorf("target data type mismatch, "+
			"expected *cvmapplytable.ZiyanCvmGenerateRecord, got: %T", target)
	}

	// 转换SuccessList为JSON
	successListJSON, err := convertToJsonField(generateRecord.SuccessList)
	if err != nil {
		logs.Errorf("convert success_list to JSON failed: %v", err)
		successListJSON = types.JsonField("[]")
	}

	return &cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq{
		GenerateID:   record.GenerateID,
		SuborderID:   generateRecord.SubOrderId,
		GenerateType: generateRecord.GenerateType,
		TaskID:       generateRecord.TaskId,
		TaskLink:     generateRecord.TaskLink,
		RequestInfo:  generateRecord.RequestInfo,
		Status:       cvt.ValToPtr(generateRecord.Status),
		IsMatched:    cvt.ValToPtr(generateRecord.IsMatched),
		Message:      generateRecord.Message,
		TotalNum:     cvt.ValToPtr(generateRecord.TotalNum),
		SuccessNum:   cvt.ValToPtr(generateRecord.SuccessNum),
		SuccessList:  successListJSON,
		StartAt:      generateRecord.StartAt.Format(constant.TimeStdFormat),
		EndAt:        generateRecord.EndAt.Format(constant.TimeStdFormat),
	}, nil
}

// ExtractPrimaryKey 提取主键值（generate_id）
func (c *CvmGenerateRecordConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
	// 尝试作为源数据类型处理（MongoDB中GenerateId是uint64）
	if generateRecord, ok := data.(*tasktypes.GenerateRecord); ok {
		// MongoDB中是uint64，需要转换为string
		return fmt.Sprintf("%d", generateRecord.GenerateId), nil
	}

	// 尝试作为目标数据类型处理（MySQL中是string）
	if record, ok := data.(*cvmapplytable.ZiyanCvmGenerateRecord); ok {
		return record.GenerateID, nil
	}

	return nil, fmt.Errorf("data type mismatch, expected *tasktypes.GenerateRecord or "+
		"*cvmapplytable.ZiyanCvmGenerateRecord, got: %T", data)
}

// CompareData 对比两个数据是否一致
func (c *CvmGenerateRecordConverter) CompareData(source, target interface{}, fields []string) (bool, []string) {
	generateRecord, ok := source.(*tasktypes.GenerateRecord)
	if !ok {
		return false, []string{"source_type_mismatch"}
	}

	record, ok := target.(*cvmapplytable.ZiyanCvmGenerateRecord)
	if !ok {
		return false, []string{"target_type_mismatch"}
	}

	diffFields := make([]string, 0)
	for _, field := range fields {
		if c.compareField(generateRecord, record, field) {
			diffFields = append(diffFields, field)
		}
	}

	return len(diffFields) == 0, diffFields
}

// compareField 对比单个字段，返回true表示不一致
func (c *CvmGenerateRecordConverter) compareField(generateRecord *tasktypes.GenerateRecord,
	record *cvmapplytable.ZiyanCvmGenerateRecord, field string) bool {

	switch field {
	case "status":
		return generateRecord.Status != cvt.PtrToVal(record.Status)
	case "is_matched":
		return generateRecord.IsMatched != cvt.PtrToVal(record.IsMatched)
	case "total_num":
		return generateRecord.TotalNum != cvt.PtrToVal(record.TotalNum)
	case "success_num":
		return generateRecord.SuccessNum != cvt.PtrToVal(record.SuccessNum)
	case "success_list":
		return c.compareSuccessList(generateRecord, record)
	default:
		return false
	}
}

// compareSuccessList 对比success_list字段
func (c *CvmGenerateRecordConverter) compareSuccessList(generateRecord *tasktypes.GenerateRecord,
	record *cvmapplytable.ZiyanCvmGenerateRecord) bool {

	successListJSON, _ := convertToJsonField(generateRecord.SuccessList)
	return string(successListJSON) != string(record.SuccessList)
}
