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
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/table"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/dal/table/types"
	cvt "hcm/pkg/tools/converter"
)

func init() {
	Register(&CvmDeviceInfoConverter{})
}

// CvmDeviceInfoConverter 设备交付记录转换器（cr_DeviceInfo -> ziyan_cvm_device_info）
type CvmDeviceInfoConverter struct{}

// mongoDeviceInfo Mongo源表结构兼容类型（兼容generate_id为int/string）
type mongoDeviceInfo struct {
	OrderId          uint64                 `json:"order_id" bson:"order_id"`
	SubOrderId       string                 `json:"suborder_id" bson:"suborder_id"`
	GenerateId       interface{}            `json:"generate_id" bson:"generate_id"`
	BkBizId          int                    `json:"bk_biz_id" bson:"bk_biz_id"`
	User             string                 `json:"bk_username" bson:"bk_username"`
	BkHostId         int64                  `json:"bk_host_id" bson:"bk_host_id"`
	Ip               string                 `json:"ip" bson:"ip"`
	AssetId          string                 `json:"asset_id" bson:"asset_id"`
	InstanceID       string                 `json:"instance_id" bson:"instance_id"`
	RequireType      enumor.RequireType     `json:"require_type" bson:"require_type"`
	ResourceType     tasktypes.ResourceType `json:"resource_type" bson:"resource_type"`
	DeviceType       string                 `json:"device_type" bson:"device_type"`
	Description      string                 `json:"description" bson:"description"`
	Remark           string                 `json:"remark" bson:"remark"`
	ZoneName         string                 `json:"zone_name" bson:"zone_name"`
	ZoneID           int                    `json:"zone_id" bson:"zone_id"`
	ModuleName       string                 `json:"module_name" bson:"module_name"`
	Equipment        string                 `json:"rack_id" bson:"rack_id"`
	IsMatched        bool                   `json:"is_matched" bson:"is_matched"`
	IsInited         bool                   `json:"is_inited" bson:"is_inited"`
	IsDelivered      bool                   `json:"is_delivered" bson:"is_delivered"`
	Deliverer        string                 `json:"deliverer" bson:"deliverer"`
	GenerateTaskId   string                 `json:"generate_task_id" bson:"generate_task_id"`
	GenerateTaskLink string                 `json:"generate_task_link" bson:"generate_task_link"`
	InitTaskId       string                 `json:"init_task_id" bson:"init_task_id"`
	InitTaskLink     string                 `json:"init_task_link" bson:"init_task_link"`
	CreateAt         time.Time              `json:"create_at" bson:"create_at"`
	UpdateAt         time.Time              `json:"update_at" bson:"update_at"`
}

// GetName 获取转换器名称
func (c *CvmDeviceInfoConverter) GetName() string {
	return table.ZiyanCvmDeviceInfoTable + "_converter"
}

// NewSourceDataSlice 创建源数据切片
func (c *CvmDeviceInfoConverter) NewSourceDataSlice() interface{} {
	return &[]*mongoDeviceInfo{}
}

// ConvertToCreate 转换为创建请求
func (c *CvmDeviceInfoConverter) ConvertToCreate(source interface{}) (interface{}, error) {
	deviceInfo, ok := source.(*mongoDeviceInfo)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *mongoDeviceInfo, got: %T", source)
	}

	generateID, err := normalizeGenerateID(deviceInfo.GenerateId)
	if err != nil {
		return nil, fmt.Errorf("normalize generate_id failed: %w", err)
	}

	return &cvmapplyproto.ZiyanCvmDeviceInfoCreateReq{
		OrderID:          int64(deviceInfo.OrderId),
		SuborderID:       deviceInfo.SubOrderId,
		GenerateID:       generateID,
		BkBizID:          int64(deviceInfo.BkBizId),
		BkUsername:       deviceInfo.User,
		IP:               deviceInfo.Ip,
		AssetID:          deviceInfo.AssetId,
		RequireType:      deviceInfo.RequireType,
		ResourceType:     deviceInfo.ResourceType,
		DeviceType:       deviceInfo.DeviceType,
		ZoneName:         deviceInfo.ZoneName,
		ZoneID:           int64(deviceInfo.ZoneID),
		ModuleName:       deviceInfo.ModuleName,
		RackID:           deviceInfo.Equipment,
		IsMatched:        deviceInfo.IsMatched,
		IsInited:         deviceInfo.IsInited,
		IsDelivered:      deviceInfo.IsDelivered,
		Deliverer:        deviceInfo.Deliverer,
		GenerateTaskID:   deviceInfo.GenerateTaskId,
		GenerateTaskLink: deviceInfo.GenerateTaskLink,
		InitTaskID:       deviceInfo.InitTaskId,
		InitTaskLink:     deviceInfo.InitTaskLink,
		CreatedAt:        types.Time(deviceInfo.CreateAt.In(time.Local).Format(constant.TimeStdFormat)),
		UpdatedAt:        types.Time(deviceInfo.UpdateAt.In(time.Local).Format(constant.TimeStdFormat)),
	}, nil
}

// ConvertToUpdate 转换为更新请求
func (c *CvmDeviceInfoConverter) ConvertToUpdate(source interface{}, target interface{}) (interface{}, error) {
	deviceInfo, ok := source.(*mongoDeviceInfo)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *mongoDeviceInfo, got: %T", source)
	}

	generateID, err := normalizeGenerateID(deviceInfo.GenerateId)
	if err != nil {
		return nil, fmt.Errorf("normalize generate_id failed: %w", err)
	}

	// 获取目标数据的ID（更新时必需）
	device, ok := target.(*cvmapplytable.ZiyanCvmDeviceInfo)
	if !ok {
		return nil, fmt.Errorf("target data type mismatch, expected *cvmapplytable.ZiyanCvmDeviceInfo, got: %T", target)
	}

	return &cvmapplyproto.ZiyanCvmDeviceInfoUpdateReq{
		ID:               device.ID,
		OrderID:          int64(deviceInfo.OrderId),
		GenerateID:       cvt.ValToPtr(generateID),
		BkBizID:          int64(deviceInfo.BkBizId),
		BkUsername:       deviceInfo.User,
		BkHostID:         cvt.ValToPtr(deviceInfo.BkHostId),
		IP:               deviceInfo.Ip,
		AssetID:          deviceInfo.AssetId,
		InstanceID:       deviceInfo.InstanceID,
		RequireType:      cvt.ValToPtr(deviceInfo.RequireType),
		ResourceType:     cvt.ValToPtr(deviceInfo.ResourceType),
		DeviceType:       deviceInfo.DeviceType,
		Description:      deviceInfo.Description,
		Remark:           deviceInfo.Remark,
		ZoneName:         deviceInfo.ZoneName,
		ZoneID:           cvt.ValToPtr(int64(deviceInfo.ZoneID)),
		ModuleName:       deviceInfo.ModuleName,
		RackID:           deviceInfo.Equipment,
		IsMatched:        cvt.ValToPtr(deviceInfo.IsMatched),
		IsInited:         cvt.ValToPtr(deviceInfo.IsInited),
		IsDelivered:      cvt.ValToPtr(deviceInfo.IsDelivered),
		Deliverer:        deviceInfo.Deliverer,
		GenerateTaskID:   deviceInfo.GenerateTaskId,
		GenerateTaskLink: deviceInfo.GenerateTaskLink,
		InitTaskID:       deviceInfo.InitTaskId,
		InitTaskLink:     deviceInfo.InitTaskLink,
	}, nil
}

// ExtractPrimaryKey 提取主键值（复合主键：order_id + suborder_id + generate_id + asset_id）
func (c *CvmDeviceInfoConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
	// 尝试作为源数据类型处理
	if deviceInfo, ok := data.(*mongoDeviceInfo); ok {
		generateID, err := normalizeGenerateID(deviceInfo.GenerateId)
		if err != nil {
			return nil, fmt.Errorf("normalize generate_id failed: %w", err)
		}

		return map[string]interface{}{
			"order_id":    deviceInfo.OrderId,
			"suborder_id": deviceInfo.SubOrderId,
			"generate_id": generateID,
			"ip":          deviceInfo.Ip,
			"asset_id":    deviceInfo.AssetId,
		}, nil
	}

	// 尝试作为目标数据类型处理
	if device, ok := data.(*cvmapplytable.ZiyanCvmDeviceInfo); ok {
		return map[string]interface{}{
			"order_id":    device.OrderID,
			"suborder_id": device.SuborderID,
			"generate_id": device.GenerateID,
			"ip":          device.IP,
			"asset_id":    device.AssetID,
		}, nil
	}

	return nil, fmt.Errorf("data type mismatch, expected *mongoDeviceInfo or "+
		"*cvmapplytable.ZiyanCvmDeviceInfo, got: %T", data)
}

// CompareData 对比两个数据是否一致
func (c *CvmDeviceInfoConverter) CompareData(source, target interface{}, fields []string) (bool, []string) {
	deviceInfo, ok := source.(*mongoDeviceInfo)
	if !ok {
		return false, []string{"source_type_mismatch"}
	}

	device, ok := target.(*cvmapplytable.ZiyanCvmDeviceInfo)
	if !ok {
		return false, []string{"target_type_mismatch"}
	}

	diffFields := make([]string, 0)
	for _, field := range fields {
		if c.compareField(deviceInfo, device, field) {
			diffFields = append(diffFields, field)
		}
	}

	return len(diffFields) == 0, diffFields
}

// compareField 对比单个字段，返回true表示不一致
func (c *CvmDeviceInfoConverter) compareField(deviceInfo *mongoDeviceInfo,
	device *cvmapplytable.ZiyanCvmDeviceInfo, field string) bool {

	switch field {
	case "ip":
		return deviceInfo.Ip != device.IP
	case "asset_id":
		return deviceInfo.AssetId != device.AssetID
	case "is_matched":
		return deviceInfo.IsMatched != cvt.PtrToVal(device.IsMatched)
	case "is_inited":
		return deviceInfo.IsInited != cvt.PtrToVal(device.IsInited)
	case "is_delivered":
		return deviceInfo.IsDelivered != cvt.PtrToVal(device.IsDelivered)
	case "generate_task_id":
		return deviceInfo.GenerateTaskId != device.GenerateTaskID
	case "generate_task_link":
		return deviceInfo.GenerateTaskLink != device.GenerateTaskLink
	case "init_task_id":
		return deviceInfo.InitTaskId != device.InitTaskID
	case "init_task_link":
		return deviceInfo.InitTaskLink != device.InitTaskLink
	case "device_type":
		return deviceInfo.DeviceType != device.DeviceType
	case "resource_type":
		return deviceInfo.ResourceType != device.ResourceType
	case "deliverer":
		return deviceInfo.Deliverer != device.Deliverer
	default:
		return false
	}
}
