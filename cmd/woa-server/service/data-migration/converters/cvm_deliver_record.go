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
	Register(&CvmDeliverRecordConverter{})
}

// CvmDeliverRecordConverter 设备交付任务记录转换器（cr_DeliverRecord -> ziyan_cvm_deliver_record）
type CvmDeliverRecordConverter struct{}

// GetName 获取转换器名称
func (c *CvmDeliverRecordConverter) GetName() string {
	return table.ZiyanCvmDeliverRecordTable + "_converter"
}

// NewSourceDataSlice 创建源数据切片
func (c *CvmDeliverRecordConverter) NewSourceDataSlice() interface{} {
	return &[]*tasktypes.DeliverRecord{}
}

// ConvertToCreate 转换为创建请求
func (c *CvmDeliverRecordConverter) ConvertToCreate(source interface{}) (interface{}, error) {
	deliverRecord, ok := source.(*tasktypes.DeliverRecord)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *tasktypes.DeliverRecord, got: %T", source)
	}

	return &cvmapplyproto.ZiyanCvmDeliverRecordCreateReq{
		SuborderID:       deliverRecord.SubOrderId,
		IP:               deliverRecord.Ip,
		AssetID:          deliverRecord.AssetId,
		Status:           deliverRecord.Status,
		Message:          deliverRecord.Message,
		Deliverer:        deliverRecord.Deliverer,
		GenerateTaskID:   deliverRecord.GenerateTaskId,
		GenerateTaskLink: deliverRecord.GenerateTaskLink,
		InitTaskID:       deliverRecord.InitTaskId,
		InitTaskLink:     deliverRecord.InitTaskLink,
		IsManualMatched:  deliverRecord.IsManualMatched,
		StartAt:          deliverRecord.StartAt.Format(constant.TimeStdFormat),
		EndAt:            deliverRecord.EndAt.Format(constant.TimeStdFormat),
		CreatedAt:        types.Time(deliverRecord.CreateAt.In(time.Local).Format(constant.TimeStdFormat)),
		UpdatedAt:        types.Time(deliverRecord.UpdateAt.In(time.Local).Format(constant.TimeStdFormat)),
	}, nil
}

// ConvertToUpdate 转换为更新请求
func (c *CvmDeliverRecordConverter) ConvertToUpdate(source interface{}, target interface{}) (interface{}, error) {
	deliverRecord, ok := source.(*tasktypes.DeliverRecord)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *tasktypes.DeliverRecord, got: %T", source)
	}

	// 获取目标数据的ID（更新时必需）
	record, ok := target.(*cvmapplytable.ZiyanCvmDeliverRecord)
	if !ok {
		return nil, fmt.Errorf("target data type mismatch, "+
			"expected *cvmapplytable.ZiyanCvmDeliverRecord, got: %T", target)
	}

	return &cvmapplyproto.ZiyanCvmDeliverRecordUpdateReq{
		ID:               record.ID, // 从目标数据中获取ID（主键）
		SuborderID:       deliverRecord.SubOrderId,
		IP:               deliverRecord.Ip,
		AssetID:          deliverRecord.AssetId,
		Status:           cvt.ValToPtr(deliverRecord.Status),
		Message:          deliverRecord.Message,
		Deliverer:        deliverRecord.Deliverer,
		GenerateTaskID:   deliverRecord.GenerateTaskId,
		GenerateTaskLink: deliverRecord.GenerateTaskLink,
		InitTaskID:       deliverRecord.InitTaskId,
		InitTaskLink:     deliverRecord.InitTaskLink,
		IsManualMatched:  cvt.ValToPtr(deliverRecord.IsManualMatched),
		StartAt:          deliverRecord.StartAt.Format(constant.TimeStdFormat),
		EndAt:            deliverRecord.EndAt.Format(constant.TimeStdFormat),
	}, nil
}

// ExtractPrimaryKey 提取主键值（复合主键：suborder_id + ip）
func (c *CvmDeliverRecordConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
	// 尝试作为源数据类型处理
	if deliverRecord, ok := data.(*tasktypes.DeliverRecord); ok {
		return map[string]interface{}{
			"suborder_id": deliverRecord.SubOrderId,
			"ip":          deliverRecord.Ip,
		}, nil
	}

	// 尝试作为目标数据类型处理
	if record, ok := data.(*cvmapplytable.ZiyanCvmDeliverRecord); ok {
		return map[string]interface{}{
			"suborder_id": record.SuborderID,
			"ip":          record.IP,
		}, nil
	}

	return nil, fmt.Errorf("data type mismatch, expected *tasktypes.DeliverRecord or "+
		"*cvmapplytable.ZiyanCvmDeliverRecord, got: %T", data)
}

// CompareData 对比两个数据是否一致
func (c *CvmDeliverRecordConverter) CompareData(source, target interface{}, fields []string) (bool, []string) {
	deliverRecord, ok := source.(*tasktypes.DeliverRecord)
	if !ok {
		return false, []string{"source_type_mismatch"}
	}

	record, ok := target.(*cvmapplytable.ZiyanCvmDeliverRecord)
	if !ok {
		return false, []string{"target_type_mismatch"}
	}

	diffFields := make([]string, 0)
	for _, field := range fields {
		if c.compareField(deliverRecord, record, field) {
			diffFields = append(diffFields, field)
		}
	}

	return len(diffFields) == 0, diffFields
}

// compareField 对比单个字段，返回true表示不一致
func (c *CvmDeliverRecordConverter) compareField(deliverRecord *tasktypes.DeliverRecord,
	record *cvmapplytable.ZiyanCvmDeliverRecord, field string) bool {

	switch field {
	case "status":
		return deliverRecord.Status != cvt.PtrToVal(record.Status)
	case "message":
		return deliverRecord.Message != record.Message
	case "generate_task_id":
		return deliverRecord.GenerateTaskId != record.GenerateTaskID
	case "generate_task_link":
		return deliverRecord.GenerateTaskLink != record.GenerateTaskLink
	case "init_task_id":
		return deliverRecord.InitTaskId != record.InitTaskID
	case "init_task_link":
		return deliverRecord.InitTaskLink != record.InitTaskLink
	case "is_manual_matched":
		return deliverRecord.IsManualMatched != cvt.PtrToVal(record.IsManualMatched)
	case "deliverer":
		return deliverRecord.Deliverer != record.Deliverer
	default:
		return false
	}
}
