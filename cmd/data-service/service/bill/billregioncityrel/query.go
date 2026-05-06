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

package billregioncityrel

import (
	"hcm/pkg/api/core"
	corebill "hcm/pkg/api/core/bill"
	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/types"
	tablebill "hcm/pkg/dal/table/bill"
	"hcm/pkg/rest"
)

// ListBillRegionCityRel lists account bill region city rel records.
func (svc *service) ListBillRegionCityRel(cts *rest.Contexts) (interface{}, error) {
	req := new(dsbill.BillRegionCityRelListReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	opt := &types.ListOption{
		Filter: req.Filter,
		Page:   req.Page,
		Fields: req.Fields,
	}

	data, err := svc.dao.AccountBillRegionCityRel().List(cts.Kit, opt)
	if err != nil {
		return nil, err
	}

	details := make([]*corebill.AccountBillRegionCityRel, 0, len(data.Details))
	for i := range data.Details {
		details = append(details, convRegionCityRel(&data.Details[i]))
	}

	return &dsbill.BillRegionCityRelListResult{Details: details, Count: data.Count}, nil
}

func convRegionCityRel(m *tablebill.AccountBillRegionCityRel) *corebill.AccountBillRegionCityRel {
	return &corebill.AccountBillRegionCityRel{
		ID:     m.ID,
		Region: m.Region,
		Vendor: m.Vendor,
		CityId: m.CityID,
		Revision: core.Revision{
			Creator:   m.Creator,
			Reviser:   m.Reviser,
			CreatedAt: m.CreatedAt.String(),
			UpdatedAt: m.UpdatedAt.String(),
		},
	}
}
