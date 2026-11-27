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

// IacSpecNodeClient data-service iac_spec_node API 客户端。
type IacSpecNodeClient struct {
	client rest.ClientInterface
}

// NewIacSpecNodeClient 创建 IacSpecNodeClient。
func NewIacSpecNodeClient(client rest.ClientInterface) *IacSpecNodeClient {
	return &IacSpecNodeClient{client: client}
}

// BatchCreate 批量创建 iac_spec_node 记录。
func (c *IacSpecNodeClient) BatchCreate(kt *kit.Kit, req *dataiac.IacSpecNodeBatchCreateReq) (
	*core.BatchCreateResult, error) {

	return common.Request[dataiac.IacSpecNodeBatchCreateReq, core.BatchCreateResult](c.client, rest.POST, kt, req,
		"/iac_spec_node/batch/create")
}

// BatchUpdate 批量更新 iac_spec_node 记录。
func (c *IacSpecNodeClient) BatchUpdate(kt *kit.Kit, req *dataiac.IacSpecNodeBatchUpdateReq) error {
	return common.RequestNoResp[dataiac.IacSpecNodeBatchUpdateReq](c.client, rest.PATCH, kt, req,
		"/iac_spec_node/batch/update")
}

// BatchDelete 批量删除 iac_spec_node 记录。
func (c *IacSpecNodeClient) BatchDelete(kt *kit.Kit, req *dataiac.IacSpecNodeBatchDeleteReq) error {
	return common.RequestNoResp[dataiac.IacSpecNodeBatchDeleteReq](c.client, rest.DELETE, kt, req,
		"/iac_spec_node/batch")
}

// List 列表查询 iac_spec_node 记录。
func (c *IacSpecNodeClient) List(kt *kit.Kit, req *dataiac.IacSpecNodeListReq) (*dataiac.IacSpecNodeListResult, error) {
	return common.Request[dataiac.IacSpecNodeListReq, dataiac.IacSpecNodeListResult](c.client, rest.POST, kt, req,
		"/iac_spec_node/list")
}
