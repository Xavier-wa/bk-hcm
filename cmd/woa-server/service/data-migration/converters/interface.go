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

// Package converters provides data converters for different table types
package converters

// DataConverter 数据转换器接口（所有表转换器都实现此接口）
type DataConverter interface {
	// GetName 获取转换器名称
	GetName() string

	// NewSourceDataSlice 创建源数据切片（用于MongoDB查询结果反序列化）
	// 返回一个指向正确类型切片的指针，例如 *[]*tasktypes.ApplyTicket
	NewSourceDataSlice() interface{}

	// ConvertToCreate 转换为创建请求（MongoDB -> MySQL Create Request）
	ConvertToCreate(source interface{}) (interface{}, error)

	// ConvertToUpdate 转换为更新请求（MongoDB -> MySQL Update Request）
	ConvertToUpdate(source interface{}, target interface{}) (interface{}, error)

	// ExtractPrimaryKey 提取主键值（从源数据或目标数据中提取）
	// 单字段主键返回单个值（如 "12345"），复合主键返回map[string]interface{}（如 {"suborder_id": "xxx", "step_name": "yyy"}）
	ExtractPrimaryKey(data interface{}) (interface{}, error)

	// CompareData 对比两个数据是否一致
	// 返回：是否相等，差异字段列表
	CompareData(source interface{}, target interface{}, fields []string) (bool, []string)
}
