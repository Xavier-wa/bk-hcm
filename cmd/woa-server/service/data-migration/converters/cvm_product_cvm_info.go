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

	cvmtypes "hcm/cmd/woa-server/types/cvm"
	tasktypes "hcm/cmd/woa-server/types/task"
	"hcm/pkg"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/criteria/constant"
	cvmapplytable "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/dal/table/types"
)

// init 注册转换器
func init() {
	Register(&CvmInfoConverter{})
}

// CvmInfoConverter cr_CvmInfo转换器
type CvmInfoConverter struct{}

// GetName 获取转换器名称
func (c *CvmInfoConverter) GetName() string {
	return pkg.BKTableNameCvmInfo + "_converter"
}

// NewSourceDataSlice 创建源数据切片
func (c *CvmInfoConverter) NewSourceDataSlice() interface{} {
	return &[]*cvmtypes.CvmInfo{}
}

// ConvertToCreate 转换为创建请求
func (c *CvmInfoConverter) ConvertToCreate(source interface{}) (interface{}, error) {
	cvmInfo, ok := source.(*cvmtypes.CvmInfo)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *cvmtypes.CvmInfo, got: %T", source)
	}

	req := &cvmapplyproto.ZiyanCvmDeviceInfoCreateReq{
		OrderID:        int64(cvmInfo.OrderId),
		SuborderID:     fmt.Sprintf("%d-1", cvmInfo.OrderId), // 默认子单ID格式
		BkBizID:        tasktypes.ResourceOperationService,
		IP:             cvmInfo.Ip,
		AssetID:        cvmInfo.AssetId,
		IsDelivered:    true,
		GenerateTaskID: cvmInfo.CvmTaskId,
		CreatedAt:      types.Time(cvmInfo.UpdateAt.In(time.Local).Format(constant.TimeStdFormat)),
		UpdatedAt:      types.Time(cvmInfo.UpdateAt.In(time.Local).Format(constant.TimeStdFormat)),
	}

	return req, nil
}

// ConvertToUpdate 转换为更新请求
func (c *CvmInfoConverter) ConvertToUpdate(source interface{}, target interface{}) (interface{}, error) {
	cvmInfo, ok := source.(*cvmtypes.CvmInfo)
	if !ok {
		return nil, fmt.Errorf("source data type mismatch, expected *cvmtypes.CvmInfo, got: %T", source)
	}

	deviceInfo, ok := target.(*cvmapplytable.ZiyanCvmDeviceInfo)
	if !ok {
		return nil, fmt.Errorf("target data type mismatch, expected *cvmapplytable.ZiyanCvmDeviceInfo, got: %T", target)
	}

	req := &cvmapplyproto.ZiyanCvmDeviceInfoUpdateReq{
		ID:             deviceInfo.ID,
		OrderID:        int64(cvmInfo.OrderId),
		SuborderID:     deviceInfo.SuborderID,
		IP:             cvmInfo.Ip,
		AssetID:        cvmInfo.AssetId,
		GenerateTaskID: cvmInfo.CvmTaskId,
	}

	return req, nil
}

// ExtractPrimaryKey 提取主键（复合主键：order_id + ip + asset_id）
func (c *CvmInfoConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
	// 尝试作为源数据类型处理
	if cvmInfo, ok := data.(*cvmtypes.CvmInfo); ok {
		return map[string]interface{}{
			"order_id": int64(cvmInfo.OrderId),
			"ip":       cvmInfo.Ip,
			"asset_id": cvmInfo.AssetId,
		}, nil
	}

	// 尝试作为目标数据类型处理
	if deviceInfo, ok := data.(*cvmapplytable.ZiyanCvmDeviceInfo); ok {
		return map[string]interface{}{
			"order_id": deviceInfo.OrderID,
			"ip":       deviceInfo.IP,
			"asset_id": deviceInfo.AssetID,
		}, nil
	}

	return nil, fmt.Errorf("data type mismatch, expected *cvmtypes.CvmInfo or "+
		"*cvmapplytable.ZiyanCvmDeviceInfo, got: %T", data)
}

// CompareData 比较数据
func (c *CvmInfoConverter) CompareData(source interface{}, target interface{}, fields []string) (bool, []string) {
	cvmInfo, ok := source.(*cvmtypes.CvmInfo)
	if !ok {
		return false, []string{"source_type_mismatch"}
	}

	deviceInfo, ok := target.(*cvmapplytable.ZiyanCvmDeviceInfo)
	if !ok {
		return false, []string{"target_type_mismatch"}
	}

	diffFields := make([]string, 0)
	for _, field := range fields {
		if c.compareField(cvmInfo, deviceInfo, field) {
			diffFields = append(diffFields, field)
		}
	}

	return len(diffFields) == 0, diffFields
}

// compareField 比较单个字段
func (c *CvmInfoConverter) compareField(cvmInfo *cvmtypes.CvmInfo,
	deviceInfo *cvmapplytable.ZiyanCvmDeviceInfo, field string) bool {

	switch field {
	case "order_id":
		return int64(cvmInfo.OrderId) != deviceInfo.OrderID
	case "ip":
		return cvmInfo.Ip != deviceInfo.IP
	case "asset_id":
		return cvmInfo.AssetId != deviceInfo.AssetID
	default:
		return false
	}
}
