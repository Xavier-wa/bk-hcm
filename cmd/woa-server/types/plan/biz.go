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

// Package plan ...
package plan

// BizOrgRel is GetBizOrgRel result.
type BizOrgRel struct {
	BkBizID         int64  `json:"bk_biz_id"`
	BkBizName       string `json:"bk_biz_name"`
	OpProductID     int64  `json:"op_product_id"`
	OpProductName   string `json:"op_product_name"`
	PlanProductID   int64  `json:"plan_product_id"`
	PlanProductName string `json:"plan_product_name"`
	VirtualDeptID   int64  `json:"virtual_dept_id"`
	VirtualDeptName string `json:"virtual_dept_name"`
}
