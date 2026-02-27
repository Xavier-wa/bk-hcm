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
	Register(&CvmApplyOrderConverter{})
}

// CvmApplyOrderConverter CVM申请单主单转换器
type CvmApplyOrderConverter struct{}

// GetName 获取转换器名称
func (c *CvmApplyOrderConverter) GetName() string {
	return table.ZiyanCvmApplyOrderTable + "_converter"
}

// NewSourceDataSlice 创建源数据切片
func (c *CvmApplyOrderConverter) NewSourceDataSlice() interface{} {
	return &[]*tasktypes.ApplyTicket{}
}

// ConvertToCreate 转换为创建请求
func (c *CvmApplyOrderConverter) ConvertToCreate(source interface{}) (interface{}, error) {
	ticket, ok := source.(*tasktypes.ApplyTicket)
	if !ok {
		return nil, fmt.Errorf("source type mismatch, expected *tasktypes.ApplyTicket")
	}

	// 转换follower为JSON
	followerJSON, err := convertToJsonField(ticket.Follower)
	if err != nil {
		logs.Errorf("convert follower to JSON failed: %v", err)
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyOrderCreateReq{
		OrderID:      ticket.OrderId,
		ProductType:  enumor.ProductTypeBusiness, // 固定为“业务生产”
		ItsmTicketID: ticket.ItsmTicketId,
		Stage:        ticket.Stage,
		BkBizID:      ticket.BkBizId,
		BkUsername:   ticket.User,
		Follower:     followerJSON,
		EnableNotice: ticket.EnableNotice,
		RequireType:  ticket.RequireType,
		ExpectTime:   ticket.ExpectTime,
		Remark:       ticket.Remark,
		Suborders:    ticket.Suborders,
		CreatedAt:    types.Time(ticket.CreateAt.In(time.Local).Format(constant.TimeStdFormat)),
		UpdatedAt:    types.Time(ticket.UpdateAt.In(time.Local).Format(constant.TimeStdFormat)),
	}, nil
}

// ConvertToUpdate 转换为更新请求
func (c *CvmApplyOrderConverter) ConvertToUpdate(source, target interface{}) (interface{}, error) {
	ticket, ok := source.(*tasktypes.ApplyTicket)
	if !ok {
		return nil, fmt.Errorf("source type mismatch, expected *tasktypes.ApplyTicket")
	}

	existingOrder, ok := target.(*cvmapplytable.ZiyanCvmApplyOrder)
	if !ok {
		return nil, fmt.Errorf("target type mismatch, expected *cvmapplytable.ZiyanCvmApplyOrder")
	}

	// 转换follower为JSON
	followerJSON, err := convertToJsonField(ticket.Follower)
	if err != nil {
		logs.Errorf("convert follower to JSON failed: %v", err)
		return nil, err
	}

	return &cvmapplyproto.ZiyanCvmApplyOrderUpdateReq{
		OrderID:      existingOrder.OrderID,
		ProductType:  enumor.ProductTypeBusiness, // 固定为“业务生产”
		ItsmTicketID: ticket.ItsmTicketId,
		Stage:        ticket.Stage,
		BkUsername:   ticket.User,
		Follower:     followerJSON,
		EnableNotice: cvt.ValToPtr(ticket.EnableNotice),
		RequireType:  ticket.RequireType,
		ExpectTime:   cvt.ValToPtr(ticket.ExpectTime),
		Remark:       ticket.Remark,
		Suborders:    ticket.Suborders,
	}, nil
}

// ExtractPrimaryKey 提取主键值
func (c *CvmApplyOrderConverter) ExtractPrimaryKey(data interface{}) (interface{}, error) {
	// 尝试作为源数据类型处理
	if ticket, ok := data.(*tasktypes.ApplyTicket); ok {
		return ticket.OrderId, nil
	}

	// 尝试作为目标数据类型处理
	if order, ok := data.(*cvmapplytable.ZiyanCvmApplyOrder); ok {
		return order.OrderID, nil
	}

	return nil, fmt.Errorf("data type mismatch, expected *tasktypes.ApplyTicket or "+
		"*cvmapplytable.ZiyanCvmApplyOrder, got: %T", data)
}

// CompareData 对比两个数据是否一致
func (c *CvmApplyOrderConverter) CompareData(source, target interface{}, fields []string) (bool, []string) {
	ticket, ok := source.(*tasktypes.ApplyTicket)
	if !ok {
		return false, []string{"source_type_mismatch"}
	}

	order, ok := target.(*cvmapplytable.ZiyanCvmApplyOrder)
	if !ok {
		return false, []string{"target_type_mismatch"}
	}

	diffFields := make([]string, 0)
	for _, field := range fields {
		if c.compareField(ticket, order, field) {
			diffFields = append(diffFields, field)
		}
	}

	return len(diffFields) == 0, diffFields
}

// compareField 对比单个字段，返回true表示不一致
func (c *CvmApplyOrderConverter) compareField(ticket *tasktypes.ApplyTicket,
	order *cvmapplytable.ZiyanCvmApplyOrder, field string) bool {

	switch field {
	case "stage":
		return string(ticket.Stage) != string(order.Stage)
	case "require_type":
		return ticket.RequireType != order.RequireType
	case "remark":
		return ticket.Remark != order.Remark
	case "total_num":
		return c.compareTotalNum(ticket, order)
	case "expect_time":
		return ticket.ExpectTime != order.ExpectTime
	default:
		return false
	}
}

// compareTotalNum 对比total_num字段
func (c *CvmApplyOrderConverter) compareTotalNum(ticket *tasktypes.ApplyTicket,
	order *cvmapplytable.ZiyanCvmApplyOrder) bool {

	totalNum := len(ticket.Suborders)
	var suborders []tasktypes.Suborder
	if err := json.Unmarshal([]byte(order.Suborders), &suborders); err == nil {
		return totalNum != len(suborders)
	}
	return false
}

// 辅助函数

// convertToJsonField 转换为JsonField
func convertToJsonField(data interface{}) (types.JsonField, error) {
	if data == nil {
		return types.JsonField("[]"), nil
	}

	// 如果已经是JsonField，直接返回
	if jf, ok := data.(types.JsonField); ok {
		return jf, nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return types.JsonField("[]"), err
	}

	return types.JsonField(jsonData), nil
}
