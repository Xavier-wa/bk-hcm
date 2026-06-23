/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package generator

import (
	"math"

	cfgtype "hcm/cmd/woa-server/types/config"
	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/cvmapi"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"
)

// QueryCapacity queries real-time CVM capacity for the given zone/vpc/subnet.
// When disableUpsertDB is true the call is read-only: no capacity snapshot is written to the DB,
// which is required for pre-check paths that must not have side-effects.
//
// Return value: zone → MaxNum map. For small-amount green-channel require types MaxNum is
// always math.MaxInt (capacity is considered unlimited).
func (g *Generator) QueryCapacity(kt *kit.Kit, order *types.ApplyOrder, zone, vpc, subnet string,
	orderZones []string, disableUpsertDB bool) (map[string]int64, error) {

	// Small-amount green channel: skip real capacity query.
	if order.RequireType.NotNeedVerifyCapacity() {
		if len(zone) > 0 && zone != cvmapi.CvmSeparateCampus {
			return map[string]int64{zone: math.MaxInt}, nil
		}
		if zone == cvmapi.CvmSeparateCampus && len(orderZones) > 0 {
			return slice.FuncToMap(orderZones, func(z string) (string, int64) { return z, math.MaxInt }), nil
		}
		return map[string]int64{}, nil
	}

	param := &cfgtype.GetCapacityParam{
		RequireType:      order.RequireType,
		DeviceType:       order.Spec.DeviceType,
		Region:           order.Spec.Region,
		Zone:             zone,
		Vpc:              vpc,
		Subnet:           subnet,
		IgnorePrediction: !order.RequireType.NeedVerifyResPlan(),
		BizID:            order.BkBizId,
		DisableUpsertDB:  disableUpsertDB,
	}
	if len(order.Spec.ChargeType) > 0 {
		param.ChargeType = order.Spec.ChargeType
	}

	rst, err := g.configLogics.Capacity().GetCapacity(kt, param)
	if err != nil {
		logs.Errorf("failed to query cvm capacity, err: %v, param: %+v, rid: %s",
			err, cvt.PtrToVal(param), kt.Rid)
		return nil, err
	}

	zoneCapacity := make(map[string]int64, len(rst.Info))
	for _, capInfo := range rst.Info {
		if capInfo != nil {
			zoneCapacity[capInfo.Zone] = capInfo.MaxNum
		}
	}

	return zoneCapacity, nil
}
