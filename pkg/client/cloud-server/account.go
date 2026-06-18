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

package cloudserver

import (
	protocloud "hcm/pkg/api/data-service/cloud"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
)

// AccountClient account related client
type AccountClient struct {
	client rest.ClientInterface
}

// NewAccountClient return account client instance.
func NewAccountClient(client rest.ClientInterface) *AccountClient {
	return &AccountClient{
		client: client,
	}
}

// ListByUsageBizID lists resource accounts associated with the given biz ID.
// It calls GET /accounts/bizs/{bk_biz_id}?account_type={account_type} on cloud-server.
// No bk_ticket is required; the internal backend kit is sufficient.
func (c AccountClient) ListByUsageBizID(kt *kit.Kit, bkBizID int64, accountType enumor.AccountType) (
	[]*protocloud.AccountBizRelWithAccount, error) {

	resp := new(protocloud.AccountBizRelWithAccountListResp)

	err := c.client.Get().
		WithContext(kt.Ctx).
		SubResourcef("/accounts/bizs/%d", bkBizID).
		WithParam("account_type", string(accountType)).
		WithHeaders(kt.Header()).
		Do().
		Into(resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != errf.OK {
		return nil, errf.New(resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// Sync 账号同步
func (c AccountClient) Sync(kt *kit.Kit, accountID string) error {
	resp := new(rest.BaseResp)

	err := c.client.Post().
		WithContext(kt.Ctx).
		Body(struct{}{}).
		SubResourcef("/accounts/%s/sync", accountID).
		WithHeaders(kt.Header()).
		Do().
		Into(resp)
	if err != nil {
		return err
	}

	if resp.Code != errf.OK {
		return errf.New(resp.Code, resp.Message)
	}

	return nil
}
