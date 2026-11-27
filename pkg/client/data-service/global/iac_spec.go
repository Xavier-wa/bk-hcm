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

package global

import (
	"hcm/pkg/api/core"
	dataiac "hcm/pkg/api/data-service/iac"
	"hcm/pkg/client/common"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
)

// IacSpecClient data-service iac_spec API 客户端。
type IacSpecClient struct {
	client rest.ClientInterface
}

// NewIacSpecClient 创建 IacSpecClient。
func NewIacSpecClient(client rest.ClientInterface) *IacSpecClient {
	return &IacSpecClient{client: client}
}

// BatchCreate 批量创建 iac_spec 记录。
func (c *IacSpecClient) BatchCreate(kt *kit.Kit, req *dataiac.IacSpecBatchCreateReq) (*core.BatchCreateResult, error) {
	return common.Request[dataiac.IacSpecBatchCreateReq, core.BatchCreateResult](c.client, rest.POST, kt, req,
		"/iac_spec/batch/create")
}

// BatchUpdate 批量更新 iac_spec 记录。
func (c *IacSpecClient) BatchUpdate(kt *kit.Kit, req *dataiac.IacSpecBatchUpdateReq) error {
	return common.RequestNoResp[dataiac.IacSpecBatchUpdateReq](c.client, rest.PATCH, kt, req, "/iac_spec/batch/update")
}

// BatchDelete 批量删除 iac_spec 记录。
func (c *IacSpecClient) BatchDelete(kt *kit.Kit, req *dataiac.IacSpecBatchDeleteReq) error {
	return common.RequestNoResp[dataiac.IacSpecBatchDeleteReq](c.client, rest.DELETE, kt, req, "/iac_spec/batch")
}

// List 列表查询 iac_spec 记录。
func (c *IacSpecClient) List(kt *kit.Kit, req *dataiac.IacSpecListReq) (*dataiac.IacSpecListResult, error) {
	return common.Request[dataiac.IacSpecListReq, dataiac.IacSpecListResult](c.client, rest.POST, kt, req,
		"/iac_spec/list")
}
