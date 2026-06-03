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
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"
)

func init() {
	Register(&CvmApplySuborderConverter{})
}

// CvmApplySuborderConverter CVM申请单子单转换器
type CvmApplySuborderConverter struct{}

// GetName 获取转换器名称
func (c *CvmApplySuborderConverter) GetName() string {
	return table.ZiyanCvmApplySuborderTable + "_converter"
}

// NewSourceDataSlice 创建源数据切片
func (c *CvmApplySuborderConverter) NewSourceDataSlice() interface{} {
	return &[]*tasktypes.ApplyOrder{}
}

// ConvertToCreate 转换为创建请求
func (c *CvmApplySuborderConverter) ConvertToCreate(source interface{}) (interface{}, error) {
	applyOrder, ok := source.(*tasktypes.ApplyOrder)
	if !ok {
		return nil, fmt.Errorf("source type mismatch, expected *tasktypes.ApplyOrder")
	}

	jsonFields, err := c.convertApplyOrderJSONFields(applyOrder)
	if err != nil {
		return nil, err
	}

	return c.buildCreateRequest(applyOrder, jsonFields), nil
}

// applyOrderJSONFields 存储转换后的JSON字段
type applyOrderJSONFields struct {
	follower        types.JsonField
	systemDisk      types.JsonField
	dataDisk        types.JsonField
	zones           types.JsonField
	failedZoneIds   types.JsonField
	upgradeCvmList  types.JsonField
	planExpendGroup types.JsonField
}

// convertApplyOrderJSONFields 转换ApplyOrder中的JSON字段
func (c *CvmApplySuborderConverter) convertApplyOrderJSONFields(
	applyOrder *tasktypes.ApplyOrder) (*applyOrderJSONFields, error) {

	fields := &applyOrderJSONFields{}
	var err error

	// 转换follower为JSON
	fields.follower, err = convertToJsonField(applyOrder.Follower)
	if err != nil {
		logs.Errorf("convert follower to JSON failed: %v", err)
		fields.follower = types.JsonField("[]")
	}

	// 转换系统盘
	if applyOrder.Spec != nil {
		fields.systemDisk, err = convertToJsonField(applyOrder.Spec.SystemDisk)
		if err != nil {
			return nil, fmt.Errorf("convert system_disk to JSON failed: %v", err)
		}
	}

	// 转换数据盘
	if applyOrder.Spec != nil {
		fields.dataDisk, err = convertToJsonField(applyOrder.Spec.DataDisk)
		if err != nil {
			logs.Errorf("convert data_disk to JSON failed: %v", err)
			fields.dataDisk = types.JsonField("[]")
		}
	}

	// 转换可用区列表
	if applyOrder.Spec != nil {
		fields.zones, err = convertToJsonField(applyOrder.Spec.Zones)
		if err != nil {
			return nil, fmt.Errorf("convert zones to JSON failed: %v", err)
		}
	}

	if applyOrder.Spec != nil {
		fields.failedZoneIds, err = convertToJsonField(applyOrder.Spec.FailedZoneIDs)
		if err != nil {
			return nil, fmt.Errorf("convert failed_zone_ids to JSON failed: %v", err)
		}
	}

	if applyOrder.UpgradeCVMList != nil {
		fields.upgradeCvmList, err = convertToJsonField(applyOrder.UpgradeCVMList)
		if err != nil {
			return nil, fmt.Errorf("convert upgrade_cvm_list to JSON failed: %v", err)
		}
	}

	if applyOrder.PlanExpendGroup != nil {
		fields.planExpendGroup, err = convertToJsonField(applyOrder.PlanExpendGroup)
		if err != nil {
			return nil, fmt.Errorf("convert plan_expend_group to JSON failed: %v", err)
		}
	}

	return fields, nil
}

// buildCreateRequest 构建创建请求对象
func (c *CvmApplySuborderConverter) buildCreateRequest(
	applyOrder *tasktypes.ApplyOrder, fields *applyOrderJSONFields) *cvmapplyproto.ZiyanCvmApplySuborderCreateReq {

	req := &cvmapplyproto.ZiyanCvmApplySuborderCreateReq{
		SuborderID:        applyOrder.SubOrderId,
		OrderID:           applyOrder.OrderId,
		BkBizID:           applyOrder.BkBizId,
		BkUsername:        applyOrder.User,
		Follower:          fields.follower,
		Auditor:           applyOrder.Auditor,
		Source:            applyOrder.Source,
		ProductType:       enumor.ProductTypeBusiness,
		RequireType:       applyOrder.RequireType,
		ExpectTime:        applyOrder.ExpectTime,
		ResourceType:      applyOrder.ResourceType,
		AntiAffinityLevel: applyOrder.AntiAffinityLevel,
		EnableDiskCheck:   cvt.ValToPtr(applyOrder.EnableDiskCheck),
		ObsProject:        applyOrder.ObsProject,
		Description:       applyOrder.Description,
		Remark:            applyOrder.Remark,
		FailedZoneIds:     fields.failedZoneIds,
		SystemDisk:        fields.systemDisk,
		DataDisk:          fields.dataDisk,
		Zones:             fields.zones,
		UpgradeCvmList:    fields.upgradeCvmList,
		Stage:             applyOrder.Stage,
		Status:            applyOrder.Status,
		RetryTime:         applyOrder.RetryTime,
		ModifyTime:        applyOrder.ModifyTime,
		AppliedCore:       applyOrder.AppliedCore,
		DeliveredCore:     applyOrder.DeliveredCore,
		PlanExpendGroup:   fields.planExpendGroup,
		OriginNum:         applyOrder.OriginNum,
		TotalNum:          applyOrder.TotalNum,
		SuccessNum:        applyOrder.SuccessNum,
		PendingNum:        applyOrder.PendingNum,
		CreatedAt:         types.Time(applyOrder.CreateAt.In(time.Local).Format(constant.TimeStdFormat)),
		UpdatedAt:         types.Time(applyOrder.UpdateAt.In(time.Local).Format(constant.TimeStdFormat)),
	}

	if applyOrder.Spec != nil {
		req.Region = applyOrder.Spec.Region
		req.Zone = applyOrder.Spec.Zone
		req.DeviceGroup = applyOrder.Spec.DeviceGroup
		req.DeviceSize = applyOrder.Spec.DeviceSize
		req.DeviceType = applyOrder.Spec.DeviceType
		req.ImageID = applyOrder.Spec.ImageId
		req.Image = applyOrder.Spec.Image
		req.DiskSize = applyOrder.Spec.DiskSize
		req.DiskType = applyOrder.Spec.DiskType
		req.NetworkType = applyOrder.Spec.NetworkType
		req.Vpc = applyOrder.Spec.Vpc
		req.Subnet = applyOrder.Spec.Subnet
		req.OsType = applyOrder.Spec.OsType
		req.RaidType = applyOrder.Spec.RaidType
		req.Isp = applyOrder.Spec.Isp
		req.ChargeType = applyOrder.Spec.ChargeType
		req.ChargeMonths = applyOrder.Spec.ChargeMonths
		req.InheritInstanceID = applyOrder.Spec.InheritInstanceId
		req.ResAssign = applyOrder.Spec.ResAssign
	}

	return req
}

// ConvertToUpdate 转换为更新请求
func (c *CvmApplySuborderConverter) ConvertToUpdate(source, target interface{}) (interface{}, error) {
	applyOrder, ok := source.(*tasktypes.ApplyOrder)
	if !ok {
		return nil, fmt.Errorf("source type mismatch, expected *tasktypes.ApplyOrder")
	}

	_, ok = target.(*cvmapplytable.ZiyanCvmApplySuborder)
	if !ok {
		return nil, fmt.Errorf("target type mismatch, expected *cvmapplytable.ZiyanCvmApplySuborder")
	}

	jsonFields := c.convertApplyOrderJSONFieldsForUpdate(applyOrder)
	return c.buildUpdateRequest(applyOrder, jsonFields), nil
}

// convertApplyOrderJSONFieldsForUpdate 转换ApplyOrder中的JSON字段（用于更新）
func (c *CvmApplySuborderConverter) convertApplyOrderJSONFieldsForUpdate(
	applyOrder *tasktypes.ApplyOrder) *applyOrderJSONFields {

	fields := &applyOrderJSONFields{}

	// 转换各个JSON字段，忽略错误
	fields.follower, _ = convertToJsonField(applyOrder.Follower)
	if applyOrder.Spec != nil {
		fields.systemDisk, _ = convertToJsonField(applyOrder.Spec.SystemDisk)
		fields.dataDisk, _ = convertToJsonField(applyOrder.Spec.DataDisk)
		fields.zones, _ = convertToJsonField(applyOrder.Spec.Zones)
	}
	fields.upgradeCvmList, _ = convertToJsonField(applyOrder.UpgradeCVMList)
	fields.planExpendGroup, _ = convertToJsonField(applyOrder.PlanExpendGroup)

	return fields
}

// buildUpdateRequest 构建更新请求对象
func (c *CvmApplySuborderConverter) buildUpdateRequest(
	applyOrder *tasktypes.ApplyOrder, fields *applyOrderJSONFields) *cvmapplyproto.ZiyanCvmApplySuborderUpdateReq {

	req := &cvmapplyproto.ZiyanCvmApplySuborderUpdateReq{
		SuborderID:        applyOrder.SubOrderId,
		OrderID:           applyOrder.OrderId,
		BkUsername:        applyOrder.User,
		Follower:          cvt.ValToPtr(fields.follower),
		Auditor:           applyOrder.Auditor,
		Source:            applyOrder.Source,
		ProductType:       enumor.ProductTypeBusiness,
		RequireType:       applyOrder.RequireType,
		ExpectTime:        cvt.ValToPtr(applyOrder.ExpectTime),
		ResourceType:      applyOrder.ResourceType,
		AntiAffinityLevel: applyOrder.AntiAffinityLevel,
		EnableDiskCheck:   cvt.ValToPtr(applyOrder.EnableDiskCheck),
		ObsProject:        applyOrder.ObsProject,
		Description:       applyOrder.Description,
		Remark:            applyOrder.Remark,
		SystemDisk:        cvt.ValToPtr(fields.systemDisk),
		DataDisk:          cvt.ValToPtr(fields.dataDisk),
		Zones:             cvt.ValToPtr(fields.zones),
		UpgradeCvmList:    cvt.ValToPtr(fields.upgradeCvmList),
		Stage:             applyOrder.Stage,
		Status:            applyOrder.Status,
		RetryTime:         cvt.ValToPtr(applyOrder.RetryTime),
		ModifyTime:        cvt.ValToPtr(applyOrder.ModifyTime),
		AppliedCore:       cvt.ValToPtr(applyOrder.AppliedCore),
		DeliveredCore:     cvt.ValToPtr(applyOrder.DeliveredCore),
		PlanExpendGroup:   cvt.ValToPtr(fields.planExpendGroup),
		OriginNum:         cvt.ValToPtr(applyOrder.OriginNum),
		TotalNum:          cvt.ValToPtr(applyOrder.TotalNum),
		SuccessNum:        cvt.ValToPtr(applyOrder.SuccessNum),
		PendingNum:        cvt.ValToPtr(applyOrder.PendingNum),
	}

	if applyOrder.Spec != nil {
		req.Region = applyOrder.Spec.Region
		req.Zone = applyOrder.Spec.Zone
		req.DeviceGroup = applyOrder.Spec.DeviceGroup
		req.DeviceSize = applyOrder.Spec.DeviceSize
		req.DeviceType = applyOrder.Spec.DeviceType
		req.ImageID = applyOrder.Spec.ImageId
		req.Image = applyOrder.Spec.Image
		req.DiskSize = cvt.ValToPtr(applyOrder.Spec.DiskSize)
		req.DiskType = applyOrder.Spec.DiskType
		req.NetworkType = applyOrder.Spec.NetworkType
		req.Vpc = cvt.ValToPtr(applyOrder.Spec.Vpc)
		req.Subnet = cvt.ValToPtr(applyOrder.Spec.Subnet)
		req.OsType = applyOrder.Spec.OsType
		req.RaidType = applyOrder.Spec.RaidType
		req.Isp = applyOrder.Spec.Isp
		req.ChargeType = applyOrder.Spec.ChargeType
		req.ChargeMonths = cvt.ValToPtr(applyOrder.Spec.ChargeMonths)
		req.InheritInstanceID = applyOrder.Spec.InheritInstanceId
		req.ResAssign = cvt.ValToPtr(applyOrder.Spec.ResAssign)
	}

	return req
}

// ExtractPrimaryKey 提取主键值
func (c *CvmApplySuborderConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
	// 尝试作为源数据类型处理
	if applyOrder, ok := data.(*tasktypes.ApplyOrder); ok {
		return applyOrder.SubOrderId, nil
	}

	// 尝试作为目标数据类型处理
	if suborder, ok := data.(*cvmapplytable.ZiyanCvmApplySuborder); ok {
		return suborder.SuborderID, nil
	}

	return nil, fmt.Errorf("data type mismatch, expected *tasktypes.ApplyOrder or "+
		"*cvmapplytable.ZiyanCvmApplySuborder, got: %T", data)
}

// CompareData 对比两个数据是否一致
func (c *CvmApplySuborderConverter) CompareData(source, target interface{}, fields []string) (bool, []string) {
	applyOrder, ok := source.(*tasktypes.ApplyOrder)
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

// compareField 对比单个字段，返回true表示不一致
func (c *CvmApplySuborderConverter) compareField(applyOrder *tasktypes.ApplyOrder,
	suborder *cvmapplytable.ZiyanCvmApplySuborder, field string) bool {

	switch field {
	case "stage":
		return string(applyOrder.Stage) != string(suborder.Stage)
	case "status":
		return string(applyOrder.Status) != string(suborder.Status)
	case "total_num":
		return applyOrder.TotalNum != cvt.PtrToVal(suborder.TotalNum)
	case "success_num":
		return applyOrder.SuccessNum != cvt.PtrToVal(suborder.SuccessNum)
	case "pending_num":
		return applyOrder.PendingNum != cvt.PtrToVal(suborder.PendingNum)
	case "applied_core":
		return applyOrder.AppliedCore != cvt.PtrToVal(suborder.AppliedCore)
	case "delivered_core":
		return applyOrder.DeliveredCore != cvt.PtrToVal(suborder.DeliveredCore)
	case "device_type":
		if applyOrder.Spec == nil {
			return suborder.DeviceType != ""
		}
		return applyOrder.Spec.DeviceType != suborder.DeviceType
	case "region":
		if applyOrder.Spec == nil {
			return suborder.Region != ""
		}
		return applyOrder.Spec.Region != suborder.Region
	case "zone":
		if applyOrder.Spec == nil {
			return suborder.Zone != ""
		}
		return applyOrder.Spec.Zone != suborder.Zone
	case "image_id":
		if applyOrder.Spec == nil {
			return suborder.ImageID != ""
		}
		return applyOrder.Spec.ImageId != suborder.ImageID
	case "charge_type":
		if applyOrder.Spec == nil {
			return suborder.ChargeType != ""
		}
		return applyOrder.Spec.ChargeType != suborder.ChargeType
	case "charge_months":
		if applyOrder.Spec == nil {
			return suborder.ChargeMonths != 0
		}
		return applyOrder.Spec.ChargeMonths != suborder.ChargeMonths
	default:
		return false
	}
}
