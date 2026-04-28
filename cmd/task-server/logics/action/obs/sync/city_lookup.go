/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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

package sync

import (
	actcli "hcm/cmd/task-server/logics/action/cli"
	"hcm/pkg/api/core"
	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// loadRegionCityMap loads region-to-city mappings for a specific vendor from data-service.
// Returns a map of region -> city_id.
func loadRegionCityMap(kt *kit.Kit, vendor enumor.Vendor) (map[string]int32, error) {
	regionCityMap := make(map[string]int32)

	req := &dsbill.BillRegionCityRelListReq{
		Filter: tools.ExpressionAnd(tools.RuleEqual("vendor", vendor)),
		Page:   core.NewDefaultBasePage(),
	}
	for {
		result, err := actcli.GetDataService().Global.Bill.ListBillRegionCityRel(kt, req)
		if err != nil {
			logs.Errorf("load region city map for vendor %s failed, err: %v, rid: %s", vendor, err, kt.Rid)
			return nil, err
		}

		for _, item := range result.Details {
			regionCityMap[item.Region] = item.CityId
		}

		if len(result.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	logs.Infof("loaded %d region-city mappings for vendor %s, rid: %s", len(regionCityMap), vendor, kt.Rid)
	return regionCityMap, nil
}

// lookupCityID looks up the city ID for a given region. Falls back to default city IDs based on account site.
func lookupCityID(kt *kit.Kit, regionCityMap map[string]int32, region string, isChina bool) int32 {
	if cityID, ok := regionCityMap[region]; ok {
		return cityID
	}

	logs.Warnf("no city mapping found for region %s, using default city id, isChina: %t, rid: %s", region, isChina,
		kt.Rid)
	if isChina {
		return constant.OBSDefaultCityIDChina
	}
	return constant.OBSDefaultCityIDOverseas
}
