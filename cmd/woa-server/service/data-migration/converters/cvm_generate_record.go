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

// mongoGenerateRecord Mongo源表结构兼容类型（兼容generate_id为int/string）
type mongoGenerateRecord struct {
	SubOrderId      string                       `json:"suborder_id" bson:"suborder_id"`
	GenerateId      interface{}                  `json:"generate_id" bson:"generate_id"`
	GenerateType    string                       `json:"generate_type" bson:"generate_type"`
	TaskId          string                       `json:"task_id" bson:"task_id"`
	TaskLink        string                       `json:"task_link" bson:"task_link"`
	RequestInfo     string                       `json:"request_info" bson:"request_info"`
	Status          tasktypes.GenerateStepStatus `json:"status" bson:"status"`
	IsMatched       bool                         `json:"is_matched" bson:"is_matched"`
	Message         string                       `json:"message" bson:"message"`
	TotalNum        uint                         `json:"total_num" bson:"total_num"`
	SuccessNum      uint                         `json:"success_num" bson:"success_num"`
	SuccessList     []string                     `json:"success_list" bson:"success_list"`
	CreateAt        time.Time                    `json:"create_at" bson:"create_at"`
	UpdateAt        time.Time                    `json:"update_at" bson:"update_at"`
	StartAt         time.Time                    `json:"start_at" bson:"start_at"`
	EndAt           time.Time                    `json:"end_at" bson:"end_at"`
	IsManualMatched bool                         `json:"is_manual_matched" bson:"is_manual_matched"` // 是否手工匹配
}

// GetName 获取转换器名称
func (c *CvmGenerateRecordConverter) GetName() string {
	return table.ZiyanCvmGenerateRecordTable + "_converter"
}

// NewSourceDataSlice 创建源数据切片
func (c *CvmGenerateRecordConverter) NewSourceDataSlice() interface{} {
	return &[]*mongoGenerateRecord{}
}

// ConvertToCreate 转换为创建请求
func (c *CvmGenerateRecordConverter) ConvertToCreate(source interface{}) (interface{}, error) {
	generateRecord, ok := source.(*mongoGenerateRecord)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *mongoGenerateRecord, got: %T", source)
	}

	generateID, err := normalizeGenerateID(generateRecord.GenerateId)
	if err != nil {
		return nil, fmt.Errorf("normalize generate_id failed: %w", err)
	}

	// 转换SuccessList为JSON
	successListJSON, err := convertToJsonField(generateRecord.SuccessList)
	if err != nil {
		logs.Errorf("convert success_list to JSON failed: %v", err)
		successListJSON = types.JsonField("[]")
	}

	return &cvmapplyproto.ZiyanCvmGenerateRecordCreateReq{
		GenerateID:   generateID,
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
	generateRecord, ok := source.(*mongoGenerateRecord)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *mongoGenerateRecord, got: %T", source)
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
		GenerateType: cvt.ValToPtr(generateRecord.GenerateType),
		TaskID:       cvt.ValToPtr(generateRecord.TaskId),
		TaskLink:     cvt.ValToPtr(generateRecord.TaskLink),
		RequestInfo:  generateRecord.RequestInfo,
		Status:       cvt.ValToPtr(generateRecord.Status),
		IsMatched:    cvt.ValToPtr(generateRecord.IsMatched),
		Message:      cvt.ValToPtr(generateRecord.Message),
		TotalNum:     cvt.ValToPtr(generateRecord.TotalNum),
		SuccessNum:   cvt.ValToPtr(generateRecord.SuccessNum),
		SuccessList:  cvt.ValToPtr(successListJSON),
		StartAt:      cvt.ValToPtr(generateRecord.StartAt.Format(constant.TimeStdFormat)),
		EndAt:        cvt.ValToPtr(generateRecord.EndAt.Format(constant.TimeStdFormat)),
	}, nil
}

// ExtractPrimaryKey 提取主键值（generate_id）
func (c *CvmGenerateRecordConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
	// 尝试作为源数据类型处理（MongoDB中GenerateId可能是int/string）
	if generateRecord, ok := data.(*mongoGenerateRecord); ok {
		return normalizeGenerateID(generateRecord.GenerateId)
	}

	// 尝试作为目标数据类型处理（MySQL中是string）
	if record, ok := data.(*cvmapplytable.ZiyanCvmGenerateRecord); ok {
		return record.GenerateID, nil
	}

	return nil, fmt.Errorf("data type mismatch, expected *mongoGenerateRecord or "+
		"*cvmapplytable.ZiyanCvmGenerateRecord, got: %T", data)
}

// CompareData 对比两个数据是否一致
func (c *CvmGenerateRecordConverter) CompareData(source, target interface{}, fields []string) (bool, []string) {
	generateRecord, ok := source.(*mongoGenerateRecord)
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
func (c *CvmGenerateRecordConverter) compareField(generateRecord *mongoGenerateRecord,
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
func (c *CvmGenerateRecordConverter) compareSuccessList(generateRecord *mongoGenerateRecord,
	record *cvmapplytable.ZiyanCvmGenerateRecord) bool {

	successListJSON, _ := convertToJsonField(generateRecord.SuccessList)
	return string(successListJSON) != string(record.SuccessList)
}

// normalizeGenerateID 统一将Mongo中的generate_id转换为MySQL所需字符串
func normalizeGenerateID(val interface{}) (string, error) {
	switch v := val.(type) {
	case string:
		if v == "" {
			return "", fmt.Errorf("generate_id is empty")
		}
		return v, nil
	case int:
		return fmt.Sprintf("%d", v), nil
	case int32:
		return fmt.Sprintf("%d", v), nil
	case int64:
		return fmt.Sprintf("%d", v), nil
	case uint:
		return fmt.Sprintf("%d", v), nil
	case uint32:
		return fmt.Sprintf("%d", v), nil
	case uint64:
		return fmt.Sprintf("%d", v), nil
	default:
		return "", fmt.Errorf("unsupported generate_id type: %T", val)
	}
}
