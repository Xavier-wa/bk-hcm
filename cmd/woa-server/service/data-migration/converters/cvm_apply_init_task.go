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
	Register(&CvmApplyInitTaskConverter{})
}

// CvmApplyInitTaskConverter 初始化任务记录转换器（cr_InitRecord -> ziyan_cvm_apply_init_task）
type CvmApplyInitTaskConverter struct{}

// GetName 获取转换器名称
func (c *CvmApplyInitTaskConverter) GetName() string {
	return table.ZiyanCvmApplyInitTaskTable + "_converter"
}

// NewSourceDataSlice 创建源数据切片
func (c *CvmApplyInitTaskConverter) NewSourceDataSlice() interface{} {
	return &[]*tasktypes.InitRecord{}
}

// ConvertToCreate 转换为创建请求
func (c *CvmApplyInitTaskConverter) ConvertToCreate(source interface{}) (interface{}, error) {
	initRecord, ok := source.(*tasktypes.InitRecord)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *tasktypes.InitRecord, got: %T", source)
	}

	return &cvmapplyproto.ZiyanCvmApplyInitTaskCreateReq{
		SuborderID: initRecord.SubOrderId,
		IP:         initRecord.Ip,
		TaskID:     initRecord.TaskId,
		TaskLink:   initRecord.TaskLink,
		Status:     initRecord.Status,
		Message:    initRecord.Message,
		StartAt:    initRecord.StartAt.Format(constant.TimeStdFormat),
		EndAt:      initRecord.EndAt.Format(constant.TimeStdFormat),
		CreatedAt:  types.Time(initRecord.CreateAt.In(time.Local).Format(constant.TimeStdFormat)),
		UpdatedAt:  types.Time(initRecord.UpdateAt.In(time.Local).Format(constant.TimeStdFormat)),
	}, nil
}

// ConvertToUpdate 转换为更新请求
func (c *CvmApplyInitTaskConverter) ConvertToUpdate(source interface{}, target interface{}) (interface{}, error) {
	initRecord, ok := source.(*tasktypes.InitRecord)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *tasktypes.InitRecord, got: %T", source)
	}

	// 获取目标数据的ID（更新时必需）
	task, ok := target.(*cvmapplytable.ZiyanCvmApplyInitTask)
	if !ok {
		return nil, fmt.Errorf("target data type mismatch, "+
			"expected *cvmapplytable.ZiyanCvmApplyInitTask, got: %T", target)
	}

	return &cvmapplyproto.ZiyanCvmApplyInitTaskUpdateReq{
		ID:         task.ID, // 从目标数据中获取ID（主键）
		SuborderID: initRecord.SubOrderId,
		IP:         initRecord.Ip,
		TaskID:     initRecord.TaskId,
		TaskLink:   initRecord.TaskLink,
		Status:     cvt.ValToPtr(initRecord.Status),
		Message:    initRecord.Message,
		StartAt:    initRecord.StartAt.Format(constant.TimeStdFormat),
		EndAt:      initRecord.EndAt.Format(constant.TimeStdFormat),
	}, nil
}

// ExtractPrimaryKey 提取主键值（复合主键：suborder_id + ip）
func (c *CvmApplyInitTaskConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
	// 尝试作为源数据类型处理
	if initRecord, ok := data.(*tasktypes.InitRecord); ok {
		return map[string]interface{}{
			"suborder_id": initRecord.SubOrderId,
			"ip":          initRecord.Ip,
			"task_id":     initRecord.TaskId,
		}, nil
	}

	// 尝试作为目标数据类型处理
	if task, ok := data.(*cvmapplytable.ZiyanCvmApplyInitTask); ok {
		return map[string]interface{}{
			"suborder_id": task.SuborderID,
			"ip":          task.IP,
			"task_id":     task.TaskID,
		}, nil
	}

	return nil, fmt.Errorf("data type mismatch, expected *tasktypes.InitRecord or "+
		"*cvmapplytable.ZiyanCvmApplyInitTask, got: %T", data)
}

// CompareData 对比两个数据是否一致
func (c *CvmApplyInitTaskConverter) CompareData(source, target interface{}, fields []string) (bool, []string) {
	initRecord, ok := source.(*tasktypes.InitRecord)
	if !ok {
		return false, []string{"source_type_mismatch"}
	}

	task, ok := target.(*cvmapplytable.ZiyanCvmApplyInitTask)
	if !ok {
		return false, []string{"target_type_mismatch"}
	}

	diffFields := make([]string, 0)
	for _, field := range fields {
		if c.compareField(initRecord, task, field) {
			diffFields = append(diffFields, field)
		}
	}

	return len(diffFields) == 0, diffFields
}

// compareField 对比单个字段，返回true表示不一致
func (c *CvmApplyInitTaskConverter) compareField(initRecord *tasktypes.InitRecord,
	task *cvmapplytable.ZiyanCvmApplyInitTask, field string) bool {

	switch field {
	case "suborder_id":
		return initRecord.SubOrderId != task.SuborderID
	case "ip":
		return initRecord.Ip != task.IP
	case "status":
		return initRecord.Status != cvt.PtrToVal(task.Status)
	case "message":
		return initRecord.Message != task.Message
	case "task_id":
		return initRecord.TaskId != task.TaskID
	case "task_link":
		return initRecord.TaskLink != task.TaskLink
	case "start_at":
		return initRecord.StartAt.Format(constant.TimeStdFormat) != task.StartAt
	case "end_at":
		return initRecord.EndAt.Format(constant.TimeStdFormat) != task.EndAt
	default:
		return false
	}
}
