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

// Package applyrecommend provides logic for offline apply recommend stats.
package applyrecommend

import (
	"time"

	"hcm/pkg/api/core"
	dataproto "hcm/pkg/api/data-service"
	protocloud "hcm/pkg/api/data-service/cloud/zone"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/dal/dao/tools"
	cvmapply "hcm/pkg/dal/table/cvm-apply"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/tools/maps"
	"hcm/pkg/tools/slice"
)

// Logics holds the apply recommend logics.
type Logics struct {
	client *client.ClientSet
}

// NewLogics creates a new apply recommend Logics instance.
func NewLogics(client *client.ClientSet) *Logics {
	return &Logics{client: client}
}

// GenerateRecommend executes the full apply recommend offline stats pipeline.
func (l *Logics) GenerateRecommend(kt *kit.Kit) error {
	startTime := time.Now()
	cfg := cc.WoaServer().ApplyRecommend

	userCounts, bizCounts, err := l.collectCounts(kt, cfg.LookbackDays)
	if err != nil {
		return err
	}

	userRecommends := AggregateUser(userCounts, cfg.MaxRows)
	bizRecommends := AggregateBiz(bizCounts, cfg.MaxRows)

	if err = l.writeUserRecommends(kt, userRecommends); err != nil {
		return err
	}

	if err = l.writeBizRecommends(kt, bizRecommends); err != nil {
		return err
	}

	if err = l.cleanupExpired(kt, startTime); err != nil {
		return err
	}

	logs.Infof("generate recommend success, rid: %s", kt.Rid)
	return nil
}

// collectCounts paginates through ziyan_cvm_device_info and builds count maps.
func (l *Logics) collectCounts(kt *kit.Kit, lookbackDays int) (map[userCountKey]int, map[bizCountKey]int, error) {
	userCounts := make(map[userCountKey]int)
	bizCounts := make(map[bizCountKey]int)

	since := time.Now().AddDate(0, 0, -lookbackDays).Format(constant.TimeStdFormat)
	expr := tools.ExpressionAnd(tools.RuleEqual("is_delivered", true), tools.RuleGreaterThanEqual("updated_at", since))
	req := &cvmapplyproto.ZiyanCvmDeviceInfoListReq{
		Filter: expr,
		Page:   core.NewDefaultBasePage(),
	}
	devices := make([]*cvmapply.ZiyanCvmDeviceInfo, 0)
	for {
		resp, err := l.client.DataService().TCloudZiyan.ZiyanCvmDeviceInfo.List(kt.Ctx, kt.Header(), req)
		if err != nil {
			logs.Errorf("list ziyan cvm device info failed, err: %v, rid: %s", err, kt.Rid)
			return nil, nil, err
		}
		devices = append(devices, resp.Details...)
		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	devices, err := l.fillOutCloudRegion(kt, devices)
	if err != nil {
		logs.Errorf("fill out cloud region failed, err: %v, rid: %s", err, kt.Rid)
		return nil, nil, err
	}

	for _, item := range devices {
		if item.CloudRegion == "" {
			logs.Errorf("cloud region is empty, id: %s, rid: %s", item.ID, kt.Rid)
			continue
		}

		uk := userCountKey{
			BkBizID:     item.BkBizID,
			BkUsername:  item.BkUsername,
			RequireType: item.RequireType,
			Region:      item.CloudRegion,
			DeviceType:  item.DeviceType,
		}
		userCounts[uk]++

		bk := bizCountKey{
			BkBizID:     item.BkBizID,
			RequireType: item.RequireType,
			Region:      item.CloudRegion,
			DeviceType:  item.DeviceType,
		}
		bizCounts[bk]++
	}

	return userCounts, bizCounts, nil
}

func (l *Logics) fillOutCloudRegion(kt *kit.Kit, devices []*cvmapply.ZiyanCvmDeviceInfo) (
	[]*cvmapply.ZiyanCvmDeviceInfo, error) {

	zoneMap := make(map[string]struct{})
	zoneNameMap := make(map[string]struct{})
	for _, item := range devices {
		if item.CloudRegion != "" {
			continue
		}
		if item.CloudZone != "" {
			zoneMap[item.CloudZone] = struct{}{}
			continue
		}
		if item.ZoneName != "" {
			zoneNameMap[item.ZoneName] = struct{}{}
		}
	}
	if len(zoneMap) == 0 && len(zoneNameMap) == 0 {
		return devices, nil
	}

	rules := make([]*filter.AtomRule, 0)
	for _, batch := range slice.Split(maps.Keys(zoneMap), int(filter.DefaultMaxInLimit)) {
		rules = append(rules, tools.RuleIn("name", batch))
	}
	for _, batch := range slice.Split(maps.Keys(zoneNameMap), int(filter.DefaultMaxInLimit)) {
		rules = append(rules, tools.RuleJsonIn("extension.logic_campus_name", batch))
	}
	req := &protocloud.ZoneListReq{
		Filter: tools.ExpressionOr(rules...),
		Page:   core.NewDefaultBasePage(),
	}

	zoneRegionMap := make(map[string]string)
	zoneNameRegionMap := make(map[string]string)
	for {
		resp, err := l.client.DataService().TCloudZiyan.Zone.ListZoneExt(kt, req)
		if err != nil {
			logs.Errorf("list zone failed, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		for _, item := range resp.Details {
			zoneRegionMap[item.Name] = item.Region
			if item.Extension != nil && item.Extension.LogicCampusName != "" {
				zoneNameRegionMap[item.Extension.LogicCampusName] = item.Region
			}
		}

		if len(resp.Details) < int(req.Page.Limit) {
			break
		}
		req.Page.Start += uint32(req.Page.Limit)
	}

	for i, item := range devices {
		if item.CloudRegion != "" {
			continue
		}
		if item.CloudZone != "" {
			if region, ok := zoneRegionMap[item.CloudZone]; ok {
				devices[i].CloudRegion = region
				continue
			}
		}
		if item.ZoneName != "" {
			if region, ok := zoneNameRegionMap[item.ZoneName]; ok {
				devices[i].CloudRegion = region
				continue
			}
		}
	}

	return devices, nil
}

// writeUserRecommends writes user recommend data: Delete + BatchCreate per biz.
func (l *Logics) writeUserRecommends(kt *kit.Kit,
	userRecommends map[bizUserKey][]cvmapplyproto.ZiyanCvmApplyUserRecommendCreateReq) error {

	for key, items := range userRecommends {
		delReq := &dataproto.BatchDeleteReq{
			Filter: tools.ExpressionAnd(tools.RuleEqual("bk_biz_id", key.BkBizID),
				tools.RuleEqual("bk_username", key.BkUsername)),
		}
		err := l.client.DataService().TCloudZiyan.ZiyanCvmApplyUserRecommend.BatchDelete(kt.Ctx, kt.Header(), delReq)
		if err != nil {
			logs.Errorf("delete user recommend failed, err: %v, bizID: %d, user: %s, rid: %s", err, key.BkBizID,
				key.BkUsername, kt.Rid)
			return err
		}

		createReq := &cvmapplyproto.BatchCreateZiyanCvmApplyUserRecommendReq{Items: items}
		if _, err = l.client.DataService().TCloudZiyan.ZiyanCvmApplyUserRecommend.BatchCreate(kt.Ctx, kt.Header(),
			createReq); err != nil {
			logs.Errorf("create user recommend failed, err: %v, bizID: %d, user: %s, rid: %s", err, key.BkBizID,
				key.BkUsername, kt.Rid)
			return err
		}
	}

	return nil
}

// writeBizRecommends writes biz recommend data: Delete + BatchCreate per biz.
func (l *Logics) writeBizRecommends(kt *kit.Kit,
	bizRecommends map[int64][]cvmapplyproto.ZiyanCvmApplyBizRecommendCreateReq) error {

	for bizID, items := range bizRecommends {
		delReq := &dataproto.BatchDeleteReq{
			Filter: tools.EqualExpression("bk_biz_id", bizID),
		}
		if err := l.client.DataService().TCloudZiyan.ZiyanCvmApplyBizRecommend.BatchDelete(
			kt.Ctx, kt.Header(), delReq); err != nil {
			logs.Errorf("delete biz recommend for biz %d failed, err: %v, rid: %s", bizID, err, kt.Rid)
			return err
		}

		createReq := &cvmapplyproto.BatchCreateZiyanCvmApplyBizRecommendReq{Items: items}
		if _, err := l.client.DataService().TCloudZiyan.ZiyanCvmApplyBizRecommend.BatchCreate(
			kt.Ctx, kt.Header(), createReq); err != nil {
			logs.Errorf("create biz recommend for biz %d failed, err: %v, rid: %s", bizID, err, kt.Rid)
			return err
		}
	}

	return nil
}

// cleanupExpired deletes expired records from both recommend tables.
func (l *Logics) cleanupExpired(kt *kit.Kit, startTime time.Time) error {
	startTimeStr := startTime.Format(constant.TimeStdFormat)

	if err := l.cleanupExpiredUserRecommend(kt, startTimeStr); err != nil {
		return err
	}
	return l.cleanupExpiredBizRecommend(kt, startTimeStr)
}

func (l *Logics) cleanupExpiredUserRecommend(kt *kit.Kit, startTimeStr string) error {
	listReq := &cvmapplyproto.ZiyanCvmApplyUserRecommendListReq{
		Filter: tools.ExpressionAnd(tools.RuleLessThan("updated_at", startTimeStr)),
		Page:   core.NewDefaultBasePage(),
		Fields: []string{"id"},
	}

	for {
		resp, err := l.client.DataService().TCloudZiyan.ZiyanCvmApplyUserRecommend.List(kt.Ctx, kt.Header(), listReq)
		if err != nil {
			logs.Errorf("list expired user recommend failed, err: %v, rid: %s", err, kt.Rid)
			return err
		}
		if len(resp.Details) == 0 {
			break
		}

		ids := make([]string, len(resp.Details))
		for i, item := range resp.Details {
			ids[i] = item.ID
		}
		delReq := &dataproto.BatchDeleteReq{Filter: tools.ContainersExpression("id", ids)}
		err = l.client.DataService().TCloudZiyan.ZiyanCvmApplyUserRecommend.BatchDelete(kt.Ctx, kt.Header(), delReq)
		if err != nil {
			logs.Errorf("delete expired user recommend failed, err: %v, rid: %s", err, kt.Rid)
			return err
		}

		if len(resp.Details) < int(listReq.Page.Limit) {
			break
		}
	}

	return nil
}

func (l *Logics) cleanupExpiredBizRecommend(kt *kit.Kit, startTimeStr string) error {
	listReq := &cvmapplyproto.ZiyanCvmApplyBizRecommendListReq{
		Filter: tools.ExpressionAnd(tools.RuleLessThan("updated_at", startTimeStr)),
		Page:   core.NewDefaultBasePage(),
		Fields: []string{"id"},
	}

	for {
		resp, err := l.client.DataService().TCloudZiyan.ZiyanCvmApplyBizRecommend.List(kt.Ctx, kt.Header(), listReq)
		if err != nil {
			logs.Errorf("list expired biz recommend failed, err: %v, rid: %s", err, kt.Rid)
			return err
		}
		if len(resp.Details) == 0 {
			break
		}

		ids := make([]string, len(resp.Details))
		for i, item := range resp.Details {
			ids[i] = item.ID
		}
		delReq := &dataproto.BatchDeleteReq{Filter: tools.ContainersExpression("id", ids)}
		err = l.client.DataService().TCloudZiyan.ZiyanCvmApplyBizRecommend.BatchDelete(kt.Ctx, kt.Header(), delReq)
		if err != nil {
			logs.Errorf("delete expired biz recommend failed, err: %v, rid: %s", err, kt.Rid)
			return err
		}

		if len(resp.Details) < int(listReq.Page.Limit) {
			break
		}
	}

	return nil
}
