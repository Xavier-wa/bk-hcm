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

// Package generator generate task
package generator

import (
	"fmt"
	"strconv"

	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/querybuilder"
)

// matchPM match pm devices
func (g *Generator) matchPM(kt *kit.Kit, order *types.ApplyOrder, existDevices []*types.DeviceInfo) error {
	replicas := order.TotalNum - uint(len(existDevices))

	// 1. init generate record
	generateId, err := g.initGenerateRecord(kt, order.ResourceType, order.SubOrderId, replicas, false)
	if err != nil {
		logs.Errorf("failed to match pm when init generate record, err: %v, order id: %s", err, order.SubOrderId)
		return fmt.Errorf("failed to match pm, err: %v, order id: %s", err, order.SubOrderId)
	}
	// 2. get match devices
	candidates, err := g.getMatchDevice(order, replicas)
	if err != nil {
		// update generate record status to Done
		if errRecord := g.UpdateGenerateRecord(kt, order, generateId,
			types.GenerateStatusFailed, err.Error(), "", nil); errRecord != nil {
			logs.Errorf("failed to match pm when update generate record, order id: %s, err: %v, rid: %s",
				order.SubOrderId, errRecord, kt.Rid)
			return fmt.Errorf("failed to match pm, order id: %s, err: %v", order.SubOrderId, errRecord)
		}

		return err
	}

	if len(candidates) == 0 {
		// update generate record status to Done
		msg := "match no devices"
		if errRecord := g.UpdateGenerateRecord(kt, order, generateId,
			types.GenerateStatusFailed, msg, "", nil); errRecord != nil {
			logs.Errorf("failed to match pm when update generate record, order id: %s, err: %v, rid: %s",
				order.SubOrderId, errRecord, kt.Rid)
			return fmt.Errorf("failed to match pm, order id: %s, err: %v", order.SubOrderId, errRecord)
		}

		return fmt.Errorf("match no devices, order id: %s", order.SubOrderId)
	}

	// 3. update generate record status to handling
	if err = g.UpdateGenerateRecord(kt, order, generateId, types.GenerateStatusHandling,
		"handling", "", nil); err != nil {
		logs.Errorf("failed to match pm when update generate record, order id: %s, err: %v, rid: %s",
			order.SubOrderId, err, kt.Rid)
		return fmt.Errorf("failed to match pm, order id: %s, err: %v", order.SubOrderId, err)
	}

	// TODO: check whether device is locked by other orders
	deviceList := make([]*types.DeviceInfo, 0)
	successIps := make([]string, 0)
	for _, host := range candidates {
		deviceList = append(deviceList, &types.DeviceInfo{
			Ip:        host.Ip,
			AssetId:   host.AssetId,
			Deliverer: "icr",
		})
		successIps = append(successIps, host.Ip)
	}

	// 4. save matched pm instances info
	if err := g.createGeneratedDevices(kt, order, generateId, deviceList); err != nil {
		logs.Errorf("failed to update generated device, err: %v, order id: %s", err, order.SubOrderId)
		return fmt.Errorf("failed to update generated device, err: %v, order id: %s", err, order.SubOrderId)
	}

	// 5. update generate record status to success
	if err = g.UpdateGenerateRecord(kt, order, generateId, types.GenerateStatusSuccess,
		"success", "", successIps); err != nil {
		logs.Errorf("failed to match pm when update generate record, err: %v, order id: %s, rid: %s",
			err, order.SubOrderId, kt.Rid)
		return fmt.Errorf("failed to match pm, err: %v, order id: %s", err, order.SubOrderId)
	}
	return nil
}

// getMatchDevice get resource apply match devices
func (g *Generator) getMatchDevice(order *types.ApplyOrder, replicas uint) ([]*types.MatchDevice, error) {
	candidates, err := g.listHostFromPool(order)
	if err != nil {
		logs.Errorf("failed to get cc host info, err: %v, order id: %s", err, order.SubOrderId)
		return nil, err
	}

	matchedDevices := make([]*types.MatchDevice, 0)
	for _, host := range candidates {
		rackId, err := strconv.Atoi(host.RackId)
		if err != nil {
			logs.Warnf("failed to convert host %d rack_id %s to int", host.BkHostID, host.RackId)
			rackId = 0
		}

		device := &types.MatchDevice{
			BkHostId:     host.BkHostID,
			AssetId:      host.BkAssetID,
			Ip:           host.GetUniqIp(),
			OuterIp:      host.BkHostOuterIP,
			Isp:          host.BkIpOerName,
			DeviceType:   host.SvrDeviceClass,
			OsType:       host.BkOSName,
			Region:       host.BkZoneName,
			Zone:         host.SubZone,
			Module:       host.ModuleName,
			Equipment:    int64(rackId),
			IdcUnit:      host.IdcUnitName,
			IdcLogicArea: host.LogicDomain,
			RaidType:     host.RaidName,
			InputTime:    host.SvrInputTime,
			MatchScore:   1.0,
			MatchTag:     true,
		}
		matchedDevices = append(matchedDevices, device)

		if uint(len(matchedDevices)) >= replicas {
			break
		}
	}

	return matchedDevices, nil
}

// listHostFromPool list filtered hosts from resource pool
func (g *Generator) listHostFromPool(order *types.ApplyOrder) ([]*cmdb.Host, error) {
	rule := querybuilder.CombinedRule{
		Condition: querybuilder.ConditionAnd,
		Rules:     make([]querybuilder.Rule, 0),
	}

	if order.Spec != nil {
		if len(order.Spec.Region) != 0 {
			rule.Rules = append(rule.Rules, querybuilder.AtomRule{
				Field:    "bk_zone_name",
				Operator: querybuilder.OperatorEqual,
				Value:    order.Spec.Region,
			})
		}
		if len(order.Spec.Zone) != 0 {
			rule.Rules = append(rule.Rules, querybuilder.AtomRule{
				Field:    "sub_zone",
				Operator: querybuilder.OperatorEqual,
				Value:    order.Spec.Zone,
			})
		}
		if len(order.Spec.DeviceType) != 0 {
			rule.Rules = append(rule.Rules, querybuilder.AtomRule{
				Field:    "svr_device_class",
				Operator: querybuilder.OperatorEqual,
				Value:    order.Spec.DeviceType,
			})
		}
		if len(order.Spec.OsType) != 0 {
			rule.Rules = append(rule.Rules, querybuilder.AtomRule{
				Field:    "bk_os_name",
				Operator: querybuilder.OperatorEqual,
				Value:    order.Spec.OsType,
			})
		}
		if len(order.Spec.RaidType) != 0 {
			rule.Rules = append(rule.Rules, querybuilder.AtomRule{
				Field:    "raid_name",
				Operator: querybuilder.OperatorEqual,
				Value:    order.Spec.RaidType,
			})
		}
		if len(order.Spec.Isp) != 0 {
			rule.Rules = append(rule.Rules, querybuilder.AtomRule{
				Field:    "bk_ip_oper_name",
				Operator: querybuilder.OperatorEqual,
				Value:    order.Spec.Isp,
			})
		}
	}
	req := &cmdb.ListBizHostParams{
		BizID:       931,
		BkModuleIDs: []int64{239149},
		Fields: []string{
			"bk_host_id",
			"bk_asset_id",
			"bk_host_innerip",
			"bk_host_outerip",
			// 外网运营商
			"bk_ip_oper_name",
			// 机型
			"svr_device_class",
			"bk_os_name",
			// 地域
			"bk_zone_name",
			// 可用区
			"sub_zone",
			"module_name",
			// 机架号，string类型
			"rack_id",
			"idc_unit_name",
			// 逻辑区域
			"logic_domain",
			"raid_name",
			"svr_input_time",
		},
		Page: &cmdb.BasePage{
			Start: 0,
			Limit: pkg.BKMaxInstanceLimit,
		},
	}
	if len(rule.Rules) > 0 {
		req.HostPropertyFilter = &cmdb.QueryFilter{
			Rule: rule,
		}
	}

	resp, err := g.cc.ListBizHost(kit.New(), req)
	if err != nil {
		logs.Errorf("failed to get cc host info, err: %v, order id: %s", err, order.SubOrderId)
		return nil, err
	}

	hosts := make([]*cmdb.Host, 0)
	for _, host := range resp.Info {
		hosts = append(hosts, cvt.ValToPtr(host))
	}

	return hosts, nil
}
