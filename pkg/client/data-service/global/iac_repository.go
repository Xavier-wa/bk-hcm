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

// IacRepositoryClient data-service iac_repository API 客户端。
type IacRepositoryClient struct {
	client rest.ClientInterface
}

// NewIacRepositoryClient 创建 IacRepositoryClient。
func NewIacRepositoryClient(client rest.ClientInterface) *IacRepositoryClient {
	return &IacRepositoryClient{client: client}
}

// BatchCreate 批量创建 iac_repository 记录。
func (c *IacRepositoryClient) BatchCreate(kt *kit.Kit, req *dataiac.IacRepositoryBatchCreateReq) (
	*core.BatchCreateResult, error) {

	return common.Request[dataiac.IacRepositoryBatchCreateReq, core.BatchCreateResult](c.client, rest.POST, kt, req,
		"/iac_repository/batch/create")
}

// BatchUpdate 批量更新 iac_repository 记录。
func (c *IacRepositoryClient) BatchUpdate(kt *kit.Kit, req *dataiac.IacRepositoryBatchUpdateReq) error {
	return common.RequestNoResp[dataiac.IacRepositoryBatchUpdateReq](c.client, rest.PATCH, kt, req,
		"/iac_repository/batch/update")
}

// BatchDelete 批量删除 iac_repository 记录。
func (c *IacRepositoryClient) BatchDelete(kt *kit.Kit, req *dataiac.IacRepositoryBatchDeleteReq) error {
	return common.RequestNoResp[dataiac.IacRepositoryBatchDeleteReq](c.client, rest.DELETE, kt, req,
		"/iac_repository/batch")
}

// List 列表查询 iac_repository 记录。
func (c *IacRepositoryClient) List(kt *kit.Kit, req *dataiac.IacRepositoryListReq) (
	*dataiac.IacRepositoryListResult, error) {

	return common.Request[dataiac.IacRepositoryListReq, dataiac.IacRepositoryListResult](c.client, rest.POST, kt, req,
		"/iac_repository/list")
}
