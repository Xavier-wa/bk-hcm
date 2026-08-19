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

// Package tenant ...
package tenant

import (
	"hcm/pkg/api/core"
	dataservice "hcm/pkg/client/data-service"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/tools/slice"
)

// ListAllTenantID list all tenant_id from data-service
func ListAllTenantID(kt *kit.Kit, ds *dataservice.Client) ([]string, error) {
	tenantIDs := make([]string, 0)

	listReq := &core.ListReq{
		Filter: tools.EqualExpression("status", enumor.TenantEnable),
		Page:   core.NewDefaultBasePage(),
	}

	for {
		res, err := ds.Global.Tenant.List(kt, listReq)
		if err != nil {
			logs.Errorf("list tenant failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}

		for _, item := range res.Details {
			tenantIDs = append(tenantIDs, item.TenantID)
		}

		if len(res.Details) < int(listReq.Page.Limit) {
			break
		}

		listReq.Page.Start += uint32(listReq.Page.Limit)
	}

	return tenantIDs, nil
}

// CheckTenantExist 校验租户存在且处于启用状态。
// 适用于调用方在请求中显式指定目标租户的场景，此时目标租户与请求身份租户无关，需要单独校验。
func CheckTenantExist(kt *kit.Kit, ds *dataservice.Client, tenantID string) error {
	tenantIDs, err := ListAllTenantID(kt, ds)
	if err != nil {
		return err
	}

	if !slice.IsItemInSlice(tenantIDs, tenantID) {
		logs.Errorf("tenant not found, tenant: %s, rid: %s", tenantID, kt.Rid)
		return errf.Newf(errf.InvalidParameter, "tenant not found: %s", tenantID)
	}

	return nil
}
