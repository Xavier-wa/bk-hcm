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

// Package rollingserver ...
package rollingserver

import (
	"sync"
	"time"

	"hcm/pkg/api/core"
	protocloud "hcm/pkg/api/data-service/cloud"
	woaserver "hcm/pkg/api/woa-server"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/thirdparty/cvmapi"
	"hcm/pkg/tools/querybuilder"
	"hcm/pkg/tools/slice"

	"golang.org/x/sync/errgroup"
)

// ListInheritedHosts 并发查询多个机型族下可继承的固资候选，返回以机型族为 key 的结果。
// 机型族之间相互独立，按 RsInheritedHostQueryConcurrency 限制并发；任一机型族查询失败则整体失败。
func (l *logics) ListInheritedHosts(kt *kit.Kit, bkBizID int64, region string, deviceFamilies []string) (
	map[string][]*woaserver.InheritedHost, error) {

	result := make(map[string][]*woaserver.InheritedHost, len(deviceFamilies))
	if len(deviceFamilies) == 0 {
		return result, nil
	}

	var lock sync.Mutex
	eg := errgroup.Group{}
	eg.SetLimit(constant.RsInheritedHostQueryConcurrency)
	for _, deviceFamily := range slice.Unique(deviceFamilies) {
		eg.Go(func() error {
			hosts, err := l.listFamilyInheritedHosts(kt, bkBizID, region, deviceFamily)
			if err != nil {
				return err
			}

			lock.Lock()
			defer lock.Unlock()
			result[deviceFamily] = hosts
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		logs.Errorf("list inherited hosts failed, err: %v, bkBizID: %d, region: %s, deviceFamilies: %v, rid: %s",
			err, bkBizID, region, deviceFamilies, kt.Rid)
		return nil, err
	}

	return result, nil
}

// listFamilyInheritedHosts 查询单个机型族下可继承的固资候选。
func (l *logics) listFamilyInheritedHosts(kt *kit.Kit, bkBizID int64, region string, deviceFamily string) (
	[]*woaserver.InheritedHost, error) {

	deviceTypes, err := l.listCommonDeviceTypesByFamily(kt, deviceFamily)
	if err != nil {
		logs.Errorf("list common device types of device family failed, err: %v, deviceFamily: %s, rid: %s", err,
			deviceFamily, kt.Rid)
		return nil, err
	}

	// 机型族在 device_type 表中不存在（或该族无通用机型）时视同该族无候选
	if len(deviceTypes) == 0 {
		logs.Warnf("no common device type found for device family: %s, bkBizID: %d, region: %s, rid: %s",
			deviceFamily, bkBizID, region, kt.Rid)
		return make([]*woaserver.InheritedHost, 0), nil
	}

	hosts, err := l.listInheritedHostsFromCmdb(kt, bkBizID, region, deviceTypes)
	if err != nil {
		logs.Errorf("list inherited hosts from cmdb failed, err: %v, bkBizID: %d, region: %s, deviceFamily: %s, "+
			"rid: %s", err, bkBizID, region, deviceFamily, kt.Rid)
		return nil, err
	}

	return hosts, nil
}

// listCommonDeviceTypesByFamily 经 device_type 表把机型族展开成该族的通用机型名单。
// 专用机型（SpecialType）不可用于滚服继承，由 DB 过滤直接排除。
func (l *logics) listCommonDeviceTypesByFamily(kt *kit.Kit, deviceFamily string) ([]string, error) {
	req := &protocloud.DistinctDeviceTypeListReq{
		ListReq: core.ListReq{
			Filter: tools.ExpressionAnd(
				tools.RuleEqual("vendor", enumor.TCloudZiyan),
				tools.RuleEqual("device_family", deviceFamily),
				tools.RuleEqual("device_type_class", cvmapi.CommonType),
			),
			Page: core.NewDefaultBasePage(),
		},
	}

	deviceTypes := make([]string, 0)
	for {
		result, err := l.configLogics.Device().ListDistinctDeviceType(kt, req)
		if err != nil {
			logs.Errorf("list distinct device type failed, err: %v, deviceFamily: %s, rid: %s", err, deviceFamily,
				kt.Rid)
			return nil, err
		}

		for _, detail := range result.Details {
			deviceTypes = append(deviceTypes, detail.DeviceType)
		}

		if len(result.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	return deviceTypes, nil
}

// listInheritedHostsFromCmdb 按滚服继承口径查询 CMDB 主机。
func (l *logics) listInheritedHostsFromCmdb(kt *kit.Kit, bkBizID int64, region string, deviceTypes []string) (
	[]*woaserver.InheritedHost, error) {

	rule := querybuilder.CombinedRule{
		Condition: querybuilder.ConditionAnd,
		Rules: []querybuilder.Rule{
			querybuilder.AtomRule{
				Field:    "dept_name",
				Operator: querybuilder.OperatorEqual,
				Value:    constant.IEGDeptName,
			},
			querybuilder.AtomRule{
				Field:    "bk_cloud_region",
				Operator: querybuilder.OperatorEqual,
				Value:    region,
			},
			querybuilder.AtomRule{
				Field:    "bk_svr_device_cls_name",
				Operator: querybuilder.OperatorIn,
				Value:    deviceTypes,
			},
			querybuilder.AtomRule{
				Field:    "instance_charge_type",
				Operator: querybuilder.OperatorNotEqual,
				Value:    "",
			},
		},
	}

	req := &cmdb.ListBizHostParams{
		BizID:              bkBizID,
		HostPropertyFilter: &cmdb.QueryFilter{Rule: rule},
		Fields: []string{"bk_asset_id", "bk_host_innerip", "bk_cloud_inst_id", "bk_svr_device_cls_name",
			"instance_charge_type", "billing_start_time", "billing_expire_time", "bk_cloud_region", "dept_name"},
		Page: &cmdb.BasePage{
			Start: 0,
			Limit: constant.RsInheritedHostReturnLimit,
			Sort:  "billing_start_time",
		},
	}

	resp, err := l.cmdbClient.ListBizHost(kt, req)
	if err != nil {
		logs.Errorf("list biz host from cmdb failed, err: %v, req: %+v, rid: %s", err, req, kt.Rid)
		return nil, err
	}
	if resp == nil {
		return make([]*woaserver.InheritedHost, 0), nil
	}

	hosts := make([]*woaserver.InheritedHost, 0, len(resp.Info))
	for _, host := range resp.Info {
		hosts = append(hosts, &woaserver.InheritedHost{
			AssetID:            host.BkAssetID,
			InnerIP:            host.BkHostInnerIP,
			CloudInstID:        host.BkCloudInstID,
			DeviceType:         host.SvrDeviceClassName,
			InstanceChargeType: host.InstanceChargeType,
			BillingStartTime:   host.BillingStartTime,
			BillingExpireTime:  host.BillingExpireTime,
			ChargeMonths:       CalcRemainMonths(time.Now(), host.BillingExpireTime),
		})
	}

	return hosts, nil
}

// CalcRemainMonths 计算 from 到 expire 之间的剩余月数，用于继承固资的续费月数（charge_months）。
// 「不足一月补一月」：from 加上算出的月数后仍早于 expire 时再补一月。
func CalcRemainMonths(from, expire time.Time) int {
	// 总月数 = 年份差 * 12 + 月份差
	totalMonths := (expire.Year()-from.Year())*12 + int(expire.Month()-from.Month())

	// 到期时间的日大于起始时间的日，则添加一个月
	if expire.Day() > from.Day() {
		totalMonths++
	}

	if from.AddDate(0, totalMonths, 0).Before(expire) {
		totalMonths++
	}

	return totalMonths
}
