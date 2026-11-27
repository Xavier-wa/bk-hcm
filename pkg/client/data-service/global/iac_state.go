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

// IacStateClient data-service iac_state API 客户端。
type IacStateClient struct {
	client rest.ClientInterface
}

// NewIacStateClient 创建 IacStateClient。
func NewIacStateClient(client rest.ClientInterface) *IacStateClient {
	return &IacStateClient{client: client}
}

// BatchCreate 批量创建 iac_state 记录。
func (c *IacStateClient) BatchCreate(kt *kit.Kit, req *dataiac.IacStateBatchCreateReq) (
	*core.BatchCreateResult, error) {

	return common.Request[dataiac.IacStateBatchCreateReq, core.BatchCreateResult](c.client, rest.POST, kt, req,
		"/iac_state/batch/create")
}

// BatchUpdate 批量更新 iac_state 记录。
func (c *IacStateClient) BatchUpdate(kt *kit.Kit, req *dataiac.IacStateBatchUpdateReq) error {
	return common.RequestNoResp[dataiac.IacStateBatchUpdateReq](c.client, rest.PATCH, kt, req,
		"/iac_state/batch/update")
}

// BatchDelete 批量删除 iac_state 记录。
func (c *IacStateClient) BatchDelete(kt *kit.Kit, req *dataiac.IacStateBatchDeleteReq) error {
	return common.RequestNoResp[dataiac.IacStateBatchDeleteReq](c.client, rest.DELETE, kt, req, "/iac_state/batch")
}

// List 列表查询 iac_state 记录。
func (c *IacStateClient) List(kt *kit.Kit, req *dataiac.IacStateListReq) (*dataiac.IacStateListResult, error) {
	return common.Request[dataiac.IacStateListReq, dataiac.IacStateListResult](c.client, rest.POST, kt, req,
		"/iac_state/list")
}
