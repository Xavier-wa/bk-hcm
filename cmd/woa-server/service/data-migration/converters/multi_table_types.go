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
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
)

// MultiTableCreateRequest 多表创建请求包装类型
// 用于一次性创建多个目标表的记录（例如同时创建 suborder 和 generate_record）
type MultiTableCreateRequest struct {
	// SuborderRequest 子单创建请求
	SuborderRequest *cvmapplyproto.ZiyanCvmApplySuborderCreateReq
	// GenerateRecordRequest 生产记录创建请求
	GenerateRecordRequest *cvmapplyproto.ZiyanCvmGenerateRecordCreateReq
}

// MultiTableUpdateRequest 多表更新请求包装类型
// 用于一次性更新多个目标表的记录
type MultiTableUpdateRequest struct {
	// SuborderRequest 子单更新请求
	SuborderRequest *cvmapplyproto.ZiyanCvmApplySuborderUpdateReq
	// GenerateRecordRequest 生产记录更新请求
	GenerateRecordRequest *cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq
}
