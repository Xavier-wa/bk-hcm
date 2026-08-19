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

// Package rollingserver ...
package rollingserver

import (
	"runtime/debug"
	"time"

	"hcm/cmd/woa-server/dal/task/table"
	"hcm/cmd/woa-server/logics/biz"
	"hcm/cmd/woa-server/logics/config"
	"hcm/cmd/woa-server/storage/driver/mongodb"
	rolling_server "hcm/cmd/woa-server/types/rolling-server"
	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	rsproto "hcm/pkg/api/data-service/rolling-server"
	woaserver "hcm/pkg/api/woa-server"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	rstable "hcm/pkg/dal/table/rolling-server"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"
	"hcm/pkg/thirdparty"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/thirdparty/api-gateway/cmsi"
)

// Logics provides management interface for rolling server.
type Logics interface {
	GetGlobalQuotaConfig(kt *kit.Kit) (*rstable.RollingGlobalConfigTable, error)
	ListQuotaOffsetAdjustRecords(kt *kit.Kit, offsetConfigIDs []string, page *core.BasePage) (
		*rolling_server.ListQuotaOffsetsAdjustRecordsResp, error)
	ListBizsWithExistQuota(kt *kit.Kit, req *rolling_server.ListBizsWithExistQuotaReq) (
		*rolling_server.ListBizsWithExistQuotaResp, error)
	// ListBizQuotaConfigs 获取批量业务的额度信息列表，管理员使用
	ListBizQuotaConfigs(kt *kit.Kit, req *rolling_server.ListBizQuotaConfigsReq) (
		*rolling_server.ListBizQuotaConfigsResp, error)
	// ListBizBizQuotaConfigs 获取当前业务自身的额度信息，业务使用
	ListBizBizQuotaConfigs(kt *kit.Kit, bkBizID int64, req *rolling_server.ListBizBizQuotaConfigsReq) (
		*rolling_server.ListBizQuotaConfigsResp, error)
	// CreateBizQuotaConfigs 批量给业务生成当月的滚服基础额度，幂等可重复执行
	CreateBizQuotaConfigs(kt *kit.Kit, req *rolling_server.CreateBizQuotaConfigsReq) (
		*rolling_server.CreateBizQuotaConfigsResp, error)
	// CreateBizQuotaConfigsForAllBiz 批量给所有业务生成当月的滚服基础额度，此时额度从全局配置表获取
	CreateBizQuotaConfigsForAllBiz(kt *kit.Kit, quotaMonth rolling_server.QuotaMonth) (
		*rolling_server.CreateBizQuotaConfigsResp, error)
	AdjustQuotaOffsetConfigs(kt *kit.Kit, bkBizIDs []int64, adjustMonth rolling_server.AdjustMonthRange,
		quotaOffset int64) (*rolling_server.AdjustQuotaOffsetsResp, error)
	// BatchCreateQuotaOffsetConfigAudit 配额修改时创建审计记录
	BatchCreateQuotaOffsetConfigAudit(kt *kit.Kit, effectIDs []string, quotaOffset int64) error

	SyncBills(kt *kit.Kit, req *rolling_server.RollingBillSyncReq) error
	// GetCpuCoreSummary 查询滚服已交付、已退还的CPU核心数概览信息
	GetCpuCoreSummary(kt *kit.Kit, req *rolling_server.CpuCoreSummaryReq) (*rsproto.RollingCpuCoreSummaryItem, error)
	IsResPoolBiz(kt *kit.Kit, bizID int64) (bool, error)
	CanApplyHost(kt *kit.Kit, bizID int64, appliedCount uint, appliedType enumor.AppliedType) (bool, string, error)
	CreateAppliedRecord(kt *kit.Kit, createArr []rolling_server.CreateAppliedRecordData) error
	UpdateSubOrderRollingDeliveredCore(kt *kit.Kit, bizID int64, subOrderID string, appliedTypes []enumor.AppliedType,
		deviceTypeCountMap map[string]int) error
	ReduceRollingCvmProdAppliedRecord(kt *kit.Kit, devices []*types.MatchDeviceBrief) error
	GetCpuCoreSum(kt *kit.Kit, deviceTypeCountMap map[string]int) (int64, error)
	// CalSplitRecycleHosts 计算并匹配指定时间范围指定业务的主机Host
	CalSplitRecycleHosts(kt *kit.Kit, bkBizID int64, hosts []*table.RecycleHost, recycleTypeSeq []table.RecycleType,
		allBizReturnedCpuCore, globalQuota int64) ([]*table.RecycleHost, int64, error)
	// InsertReturnedHostMatched 插入需要退还的主机匹配记录
	InsertReturnedHostMatched(kt *kit.Kit, bkBizID int64, orderID uint64, subOrderID string, hosts []*table.RecycleHost,
		status enumor.ReturnedStatus) error
	// UpdateReturnedStatusBySubOrderID 根据回收子订单ID更新滚服回收的状态
	UpdateReturnedStatusBySubOrderID(kt *kit.Kit, bkBizID int64, subOrderID string,
		updateLocked enumor.ReturnedStatus) error
	// ListReturnedRecordsBySubOrderID 根据回收子订单ID查询滚服回收列表
	ListReturnedRecordsBySubOrderID(kt *kit.Kit, bkBizID int64, subOrderID string) (
		[]*rstable.RollingReturnedRecord, error)
	// GetCurrentMonthAllReturnedCpuCore 获取当月所有业务回收的CPU总核心数
	GetCurrentMonthAllReturnedCpuCore(kt *kit.Kit) (int64, error)
	// GetRollingGlobalQuota 查询系统配置的全局总额度
	GetRollingGlobalQuota(kt *kit.Kit) (int64, error)
	// CheckReturnedStatusBySubOrderID 校验回收订单是否有滚服剩余额度
	CheckReturnedStatusBySubOrderID(kt *kit.Kit, orders []*table.RecycleOrder) error
	// PushReturnNotifications 推送归还到期提醒通知
	PushReturnNotifications(kt *kit.Kit, bizIDs []int64, extraReceivers []string) error
	// ListInheritedHosts 查询指定业务、指定地域、**多个**机型族下可继承的固资候选，返回以机型族为 key 的结果
	ListInheritedHosts(kt *kit.Kit, bkBizID int64, region string, deviceFamilies []string) (
		map[string][]*woaserver.InheritedHost, error)
}

// logics rolling server logics.
type logics struct {
	sd           serviced.State
	client       *client.ClientSet
	cmdbClient   cmdb.Client
	configLogics config.Logics
	bizLogics    biz.Logics
	cmsiClient   cmsi.Client
	thirdCli     *thirdparty.Client
}

// New creates rolling server logics instance.
func New(sd serviced.State, client *client.ClientSet, cmdbCli cmdb.Client, thirdCli *thirdparty.Client,
	bizLogic biz.Logics, cmsiCli cmsi.Client, configLogics config.Logics) (Logics, error) {

	rsLogics := &logics{
		sd:           sd,
		client:       client,
		cmdbClient:   cmdbCli,
		configLogics: configLogics,
		bizLogics:    bizLogic,
		cmsiClient:   cmsiCli,
		thirdCli:     thirdCli,
	}

	go rsLogics.run()

	return rsLogics, nil
}

func (l *logics) run() {
	// 启动后需等待一段时间，mongo等服务初始化完成后才能开始定时任务 TODO 用统一的任务调度模块来执行定时任务，确保在初始化之后
	for {
		if mongodb.Client() == nil {
			logs.Warnf("mongodb client is not ready, wait seconds to retry")
			time.Sleep(constant.IntervalWaitTaskStart)
			continue
		}
		break
	}

	loc, err := time.LoadLocation(cc.WoaServer().LocalTimezone)
	if err != nil {
		logs.Warnf("%s: load location: %s failed: %v", constant.RollingServerReturnNotificationPushFailed,
			cc.WoaServer().LocalTimezone, err)
		loc = time.UTC
	}

	if cc.WoaServer().RollingServer.SyncBill {
		go l.syncBillsPeriodically()
	}

	go l.createBaseQuotaConfigPeriodically(loc)

	// 滚服归还到期提醒通知
	go func() {
		if !cc.WoaServer().RollingServer.ReturnNotification.Enable {
			return
		}

		defer func() {
			if r := recover(); r != nil {
				logs.Errorf("%s: panic: %v\n%s", constant.RollingServerReturnNotificationPushFailed, r, debug.Stack())
			}
		}()

		l.pushReturnNotificationsPeriodically(loc)
	}()
}
