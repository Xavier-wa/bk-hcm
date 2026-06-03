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
	"encoding/json"
	"fmt"
	"time"

	tasktable "hcm/cmd/woa-server/dal/task/table"
	"hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/table"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/dal/table/types"
	"hcm/pkg/tools/converter"
)

// init 注册转换器
func init() {
	Register(&CvmModifyRecordConverter{})
}

// CvmModifyRecordConverter CVM变更记录转换器
type CvmModifyRecordConverter struct{}

// mongoModifyRecord Mongo源表结构（兼容id为int64/string）
type mongoModifyRecord struct {
	ID         interface{}                  `json:"id" bson:"id"`
	SuborderID string                       `json:"suborder_id" bson:"suborder_id"`
	User       string                       `json:"bk_username" bson:"bk_username"`
	Details    *tasktable.ModifyDetail      `json:"details" bson:"details"`
	CreatedAt  time.Time                    `json:"created_at" bson:"create_at"`
	UpdatedAt  time.Time                    `json:"updated_at" bson:"update_at"`
	Status     enumor.CvmModifyRecordStatus `json:"status" bson:"status"`
	Approver   string                       `json:"approver" bson:"approver"`
}

// normalizeModifyRecordID 兼容Mongo中id为int64/string
func normalizeModifyRecordID(id interface{}) (string, error) {
	switch v := id.(type) {
	case string:
		if v == "" {
			return "", fmt.Errorf("id is empty string")
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
		return "", fmt.Errorf("unsupported id type: %T", id)
	}
}

// normalizeSourceRecord 将源数据统一转换为 tasktable.ModifyRecord
func (c *CvmModifyRecordConverter) normalizeSourceRecord(source interface{}) (*tasktable.ModifyRecord, error) {
	if record, ok := source.(*tasktable.ModifyRecord); ok {
		return record, nil
	}

	if record, ok := source.(*mongoModifyRecord); ok {
		id, err := normalizeModifyRecordID(record.ID)
		if err != nil {
			return nil, fmt.Errorf("normalize id failed: %w", err)
		}

		return &tasktable.ModifyRecord{
			ID:         id,
			SuborderID: record.SuborderID,
			User:       record.User,
			Details:    record.Details,
			CreatedAt:  record.CreatedAt,
			UpdatedAt:  record.UpdatedAt,
			Status:     record.Status,
			Approver:   record.Approver,
		}, nil

	}

	return nil, fmt.Errorf("source data type mismatch, expected *tasktable.ModifyRecord or *mongoModifyRecord, "+
		"got: %T", source)
}

// GetName 获取转换器名称
func (c *CvmModifyRecordConverter) GetName() string {
	return table.ZiyanCvmModifyRecordTable + "_converter"
}

// NewSourceDataSlice 创建源数据切片
func (c *CvmModifyRecordConverter) NewSourceDataSlice() interface{} {
	return &[]*mongoModifyRecord{}
}

// ConvertToCreate 转换为创建请求
func (c *CvmModifyRecordConverter) ConvertToCreate(source interface{}) (interface{}, error) {
	record, err := c.normalizeSourceRecord(source)
	if err != nil {
		return nil, err
	}

	// 序列化 data_disk 和 zones
	preDataDiskJSON, err := convertToJsonField(record.Details.PreData.DataDisk)
	if err != nil {
		return nil, fmt.Errorf("serialize pre_data_disk failed: %w", err)
	}

	preZonesJSON, err := convertStringSliceToJsonField(record.Details.PreData.Zones)
	if err != nil {
		return nil, fmt.Errorf("serialize pre_zones failed: %w", err)
	}

	curDataDiskJSON, err := convertToJsonField(record.Details.CurData.DataDisk)
	if err != nil {
		return nil, fmt.Errorf("serialize cur_data_disk failed: %w", err)
	}

	curZonesJSON, err := convertStringSliceToJsonField(record.Details.CurData.Zones)
	if err != nil {
		return nil, fmt.Errorf("serialize cur_zones failed: %w", err)
	}

	req := &cvmapply.ZiyanCvmModifyRecordCreateReq{
		ID:         record.ID,
		SuborderID: record.SuborderID,
		BkUsername: record.User,
		// Pre data fields
		PreTotalNum:       converter.ValToPtr(record.Details.PreData.TotalNum),
		PreReplicas:       converter.ValToPtr(record.Details.PreData.Replicas),
		PreRegion:         record.Details.PreData.Region,
		PreZone:           record.Details.PreData.Zone,
		PreDeviceType:     record.Details.PreData.DeviceType,
		PreImageID:        record.Details.PreData.ImageId,
		PreDiskSize:       converter.ValToPtr(int(record.Details.PreData.DiskSize)),
		PreDiskType:       record.Details.PreData.DiskType,
		PreNetworkType:    record.Details.PreData.NetworkType,
		PreVpc:            record.Details.PreData.Vpc,
		PreSubnet:         record.Details.PreData.Subnet,
		PreSystemDiskType: record.Details.PreData.SystemDisk.DiskType,
		PreSystemDiskSize: converter.ValToPtr(int(record.Details.PreData.SystemDisk.DiskSize)),
		PreSystemDiskNum:  converter.ValToPtr(int(record.Details.PreData.SystemDisk.DiskNum)),
		PreDataDisk:       preDataDiskJSON,
		PreZones:          preZonesJSON,
		PreResAssign:      record.Details.PreData.ResAssign,
		// Cur data fields
		CurTotalNum:       converter.ValToPtr(record.Details.CurData.TotalNum),
		CurReplicas:       converter.ValToPtr(record.Details.CurData.Replicas),
		CurRegion:         record.Details.CurData.Region,
		CurZone:           record.Details.CurData.Zone,
		CurDeviceType:     record.Details.CurData.DeviceType,
		CurImageID:        record.Details.CurData.ImageId,
		CurDiskSize:       converter.ValToPtr(int(record.Details.CurData.DiskSize)),
		CurDiskType:       record.Details.CurData.DiskType,
		CurNetworkType:    record.Details.CurData.NetworkType,
		CurVpc:            record.Details.CurData.Vpc,
		CurSubnet:         record.Details.CurData.Subnet,
		CurSystemDiskType: record.Details.CurData.SystemDisk.DiskType,
		CurSystemDiskSize: converter.ValToPtr(int(record.Details.CurData.SystemDisk.DiskSize)),
		CurSystemDiskNum:  converter.ValToPtr(int(record.Details.CurData.SystemDisk.DiskNum)),
		CurDataDisk:       curDataDiskJSON,
		CurZones:          curZonesJSON,
		CurResAssign:      record.Details.CurData.ResAssign,
		Status:            record.Status,
		Approver:          record.Approver,
		CreatedAt:         types.Time(record.CreatedAt.In(time.Local).Format(constant.TimeStdFormat)),
		UpdatedAt:         types.Time(record.UpdatedAt.In(time.Local).Format(constant.TimeStdFormat)),
	}

	return req, nil
}

// ConvertToUpdate 转换为更新请求
func convertStringSliceToJsonField(data []string) (types.JsonField, error) {
	if data == nil {
		return types.JsonField("[]"), nil
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return types.JsonField("[]"), err
	}
	return types.JsonField(jsonData), nil
}

func (c *CvmModifyRecordConverter) ConvertToUpdate(source interface{}, target interface{}) (interface{}, error) {
	record, err := c.normalizeSourceRecord(source)
	if err != nil {
		return nil, err
	}

	existingRecord, ok := target.(*cvmapplytable.ZiyanCvmModifyRecord)
	if !ok {
		return nil, fmt.Errorf("target data type mismatch, "+
			"expected *cvmapplytable.ZiyanCvmModifyRecord, got: %T", target)
	}

	// 序列化 data_disk 和 zones
	preDataDiskJSON, err := convertToJsonField(record.Details.PreData.DataDisk)
	if err != nil {
		return nil, fmt.Errorf("serialize pre_data_disk failed: %w", err)
	}

	preZonesJSON, err := convertStringSliceToJsonField(record.Details.PreData.Zones)
	if err != nil {
		return nil, fmt.Errorf("serialize pre_zones failed: %w", err)
	}

	curDataDiskJSON, err := convertToJsonField(record.Details.CurData.DataDisk)
	if err != nil {
		return nil, fmt.Errorf("serialize cur_data_disk failed: %w", err)
	}

	curZonesJSON, err := convertStringSliceToJsonField(record.Details.CurData.Zones)
	if err != nil {
		return nil, fmt.Errorf("serialize cur_zones failed: %w", err)
	}

	req := &cvmapply.ZiyanCvmModifyRecordUpdateReq{
		ID:         existingRecord.ID,
		SuborderID: converter.ValToPtr(record.SuborderID),
		BkUsername: converter.ValToPtr(record.User),
		// Pre data fields
		PreTotalNum:          converter.ValToPtr(record.Details.PreData.TotalNum),
		PreReplicas:          converter.ValToPtr(record.Details.PreData.Replicas),
		PreRegion:            converter.ValToPtr(record.Details.PreData.Region),
		PreZone:              converter.ValToPtr(record.Details.PreData.Zone),
		PreDeviceType:        converter.ValToPtr(record.Details.PreData.DeviceType),
		PreImageID:           converter.ValToPtr(record.Details.PreData.ImageId),
		PreDiskSize:          converter.ValToPtr(int(record.Details.PreData.DiskSize)),
		PreDiskType:          converter.ValToPtr(record.Details.PreData.DiskType),
		PreNetworkType:       converter.ValToPtr(record.Details.PreData.NetworkType),
		PreVpc:               converter.ValToPtr(record.Details.PreData.Vpc),
		PreSubnet:            converter.ValToPtr(record.Details.PreData.Subnet),
		PreSystemDiskType:    converter.ValToPtr(record.Details.PreData.SystemDisk.DiskType),
		PreSystemDiskSize:    converter.ValToPtr(int(record.Details.PreData.SystemDisk.DiskSize)),
		PreSystemDiskNum:     converter.ValToPtr(int(record.Details.PreData.SystemDisk.DiskNum)),
		PreDataDisk:          converter.ValToPtr(preDataDiskJSON),
		PreZones:             converter.ValToPtr(preZonesJSON),
		PreResAssign:         converter.ValToPtr(record.Details.PreData.ResAssign),
		PreBkAssetID:         converter.ValToPtr(record.Details.PreData.BkAssetID),
		PreInheritInstanceID: converter.ValToPtr(record.Details.PreData.InheritInstanceID),
		// Cur data fields
		CurTotalNum:          converter.ValToPtr(record.Details.CurData.TotalNum),
		CurReplicas:          converter.ValToPtr(record.Details.CurData.Replicas),
		CurRegion:            converter.ValToPtr(record.Details.CurData.Region),
		CurZone:              converter.ValToPtr(record.Details.CurData.Zone),
		CurDeviceType:        converter.ValToPtr(record.Details.CurData.DeviceType),
		CurImageID:           converter.ValToPtr(record.Details.CurData.ImageId),
		CurDiskSize:          converter.ValToPtr(int(record.Details.CurData.DiskSize)),
		CurDiskType:          converter.ValToPtr(record.Details.CurData.DiskType),
		CurNetworkType:       converter.ValToPtr(record.Details.CurData.NetworkType),
		CurVpc:               converter.ValToPtr(record.Details.CurData.Vpc),
		CurSubnet:            converter.ValToPtr(record.Details.CurData.Subnet),
		CurSystemDiskType:    converter.ValToPtr(record.Details.CurData.SystemDisk.DiskType),
		CurSystemDiskSize:    converter.ValToPtr(int(record.Details.CurData.SystemDisk.DiskSize)),
		CurSystemDiskNum:     converter.ValToPtr(int(record.Details.CurData.SystemDisk.DiskNum)),
		CurDataDisk:          converter.ValToPtr(curDataDiskJSON),
		CurZones:             converter.ValToPtr(curZonesJSON),
		CurResAssign:         converter.ValToPtr(record.Details.CurData.ResAssign),
		CurBkAssetID:         converter.ValToPtr(record.Details.CurData.BkAssetID),
		CurInheritInstanceID: converter.ValToPtr(record.Details.CurData.InheritInstanceID),
		Status:               converter.ValToPtr(record.Status),
		Approver:             converter.ValToPtr(record.Approver),
	}

	return req, nil
}

// ExtractPrimaryKey 提取主键
func (c *CvmModifyRecordConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
	if modifyRecord, ok := data.(*mongoModifyRecord); ok {
		id, err := normalizeModifyRecordID(modifyRecord.ID)
		if err != nil {
			return nil, fmt.Errorf("normalize id failed: %w", err)
		}
		return map[string]interface{}{
			"id": id,
		}, nil
	}

	if modifyRecord, ok := data.(*tasktable.ModifyRecord); ok {
		return map[string]interface{}{
			"id": modifyRecord.ID,
		}, nil
	}

	if record, ok := data.(*cvmapplytable.ZiyanCvmModifyRecord); ok {
		return map[string]interface{}{
			"id": record.ID,
		}, nil
	}

	return nil, fmt.Errorf("data type mismatch, expected *mongoModifyRecord, *tasktable.ModifyRecord or "+
		"*cvmapplytable.ZiyanCvmModifyRecord, got: %T", data)
}

// CompareData 比较数据
func (c *CvmModifyRecordConverter) CompareData(source interface{}, target interface{}, fields []string) (
	bool, []string) {

	sourceRecord, err := c.normalizeSourceRecord(source)
	if err != nil {
		return false, []string{"source_type_mismatch"}
	}

	targetRecord, ok := target.(*cvmapplytable.ZiyanCvmModifyRecord)

	if !ok {
		return false, []string{"target_type_mismatch"}
	}

	diffFields := make([]string, 0)
	// 检查指定字段是否有变化
	for _, field := range fields {
		if c.compareField(sourceRecord, targetRecord, field) {
			diffFields = append(diffFields, field)
		}
	}

	return len(diffFields) == 0, diffFields
}

// compareField 比较单个字段
func (c *CvmModifyRecordConverter) compareField(record *tasktable.ModifyRecord,
	existingRecord *cvmapplytable.ZiyanCvmModifyRecord, field string) bool {

	// 比较基础字段
	if c.compareBasicField(record, existingRecord, field) {
		return true
	}

	// 比较 pre 数据字段
	if c.comparePreDataField(record, existingRecord, field) {
		return true
	}

	// 比较 cur 数据字段
	if c.compareCurDataField(record, existingRecord, field) {
		return true
	}

	return false
}

// compareBasicField 比较基础字段
func (c *CvmModifyRecordConverter) compareBasicField(record *tasktable.ModifyRecord,
	existingRecord *cvmapplytable.ZiyanCvmModifyRecord, field string) bool {

	switch field {
	case "status":
		return record.Status != existingRecord.Status
	case "approver":
		return record.Approver != converter.PtrToVal(existingRecord.Approver)
	default:
		return false
	}
}

// comparePreDataField 比较 pre 数据字段
func (c *CvmModifyRecordConverter) comparePreDataField(record *tasktable.ModifyRecord,
	existingRecord *cvmapplytable.ZiyanCvmModifyRecord, field string) bool {

	switch field {
	case "pre_total_num":
		return existingRecord.PreTotalNum == nil || *existingRecord.PreTotalNum != record.Details.PreData.TotalNum
	case "pre_replicas":
		return existingRecord.PreReplicas == nil || *existingRecord.PreReplicas != record.Details.PreData.Replicas
	case "pre_region":
		return record.Details.PreData.Region != converter.PtrToVal(existingRecord.PreRegion)
	case "pre_zone":
		return record.Details.PreData.Zone != converter.PtrToVal(existingRecord.PreZone)
	case "pre_device_type":
		return record.Details.PreData.DeviceType != converter.PtrToVal(existingRecord.PreDeviceType)
	case "pre_image_id":
		return record.Details.PreData.ImageId != converter.PtrToVal(existingRecord.PreImageID)
	case "pre_vpc":
		return record.Details.PreData.Vpc != converter.PtrToVal(existingRecord.PreVpc)
	case "pre_subnet":
		return record.Details.PreData.Subnet != converter.PtrToVal(existingRecord.PreSubnet)
	case "pre_res_assign":
		return record.Details.PreData.ResAssign != converter.PtrToVal(existingRecord.PreResAssign)
	default:
		return false
	}
}

// compareCurDataField 比较 cur 数据字段
func (c *CvmModifyRecordConverter) compareCurDataField(record *tasktable.ModifyRecord,
	existingRecord *cvmapplytable.ZiyanCvmModifyRecord, field string) bool {

	switch field {
	case "cur_total_num":
		return existingRecord.CurTotalNum == nil || *existingRecord.CurTotalNum != record.Details.CurData.TotalNum
	case "cur_replicas":
		return existingRecord.CurReplicas == nil || *existingRecord.CurReplicas != record.Details.CurData.Replicas
	case "cur_region":
		return record.Details.CurData.Region != converter.PtrToVal(existingRecord.CurRegion)
	case "cur_zone":
		return record.Details.CurData.Zone != converter.PtrToVal(existingRecord.CurZone)
	case "cur_device_type":
		return record.Details.CurData.DeviceType != converter.PtrToVal(existingRecord.CurDeviceType)
	case "cur_image_id":
		return record.Details.CurData.ImageId != converter.PtrToVal(existingRecord.CurImageID)
	case "cur_vpc":
		return record.Details.CurData.Vpc != converter.PtrToVal(existingRecord.CurVpc)
	case "cur_subnet":
		return record.Details.CurData.Subnet != converter.PtrToVal(existingRecord.CurSubnet)
	case "cur_res_assign":
		return record.Details.CurData.ResAssign != converter.PtrToVal(existingRecord.CurResAssign)
	default:
		return false
	}
}
