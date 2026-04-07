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

package ziyan

import (
	"hcm/pkg/api/core"
	dsproto "hcm/pkg/api/data-service/dissolve"
	"hcm/pkg/client/common"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
)

// DissolveClient is data service dissolve recycle host api client.
type DissolveClient struct {
	client rest.ClientInterface
}

// NewDissolveClient create a new dissolve api client.
func NewDissolveClient(client rest.ClientInterface) *DissolveClient {
	return &DissolveClient{
		client: client,
	}
}

// BatchCreateRecycleHost batch create dissolve recycle host records.
func (d *DissolveClient) BatchCreateRecycleHost(kt *kit.Kit, req *dsproto.BatchCreateRecycleHostReq) (
	*core.BatchCreateResult, error) {

	return common.Request[dsproto.BatchCreateRecycleHostReq, core.BatchCreateResult](
		d.client, rest.POST, kt, req, "/dissolve/recycle_hosts/batch/create")
}

// ListRecycleHost list dissolve recycle hosts.
func (d *DissolveClient) ListRecycleHost(kt *kit.Kit, req *dsproto.RecycleHostListReq) (
	*dsproto.RecycleHostListResult, error) {

	return common.Request[dsproto.RecycleHostListReq, dsproto.RecycleHostListResult](
		d.client, rest.POST, kt, req, "/dissolve/recycle_hosts/list")
}

// BatchUpdateRecycleHost batch update dissolve recycle host records.
func (d *DissolveClient) BatchUpdateRecycleHost(kt *kit.Kit, req *dsproto.BatchUpdateRecycleHostReq) error {
	return common.RequestNoResp[dsproto.BatchUpdateRecycleHostReq](
		d.client, rest.PATCH, kt, req, "/dissolve/recycle_hosts/batch")
}

// BatchDeleteRecycleHost batch delete dissolve recycle host records.
func (d *DissolveClient) BatchDeleteRecycleHost(kt *kit.Kit, req *dsproto.BatchDeleteRecycleHostReq) error {
	return common.RequestNoResp[dsproto.BatchDeleteRecycleHostReq](
		d.client, rest.DELETE, kt, req, "/dissolve/recycle_hosts/batch")
}
