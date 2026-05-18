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

// Package matcher provides ...
package matcher

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"hcm/cmd/woa-server/logics/config"
	"hcm/cmd/woa-server/logics/plan"
	rollingserver "hcm/cmd/woa-server/logics/rolling-server"
	"hcm/cmd/woa-server/logics/task/informer"
	"hcm/cmd/woa-server/logics/task/scheduler/record"
	"hcm/cmd/woa-server/logics/task/sops"
	model "hcm/cmd/woa-server/model/task"
	cfgtype "hcm/cmd/woa-server/types/config"
	types "hcm/cmd/woa-server/types/task"
	"hcm/pkg/api/core"
	cvmapplyproto "hcm/pkg/api/data-service/cvm-apply"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/thirdparty"
	"hcm/pkg/thirdparty/api-gateway/bkchatapi"
	"hcm/pkg/thirdparty/api-gateway/cmdb"
	"hcm/pkg/thirdparty/api-gateway/cmsi"
	"hcm/pkg/thirdparty/api-gateway/sopsapi"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/maps"
	"hcm/pkg/tools/slice"
	toolsutil "hcm/pkg/tools/util"
	"hcm/pkg/tools/utils/wait"
	"hcm/pkg/tools/uuid"

	"golang.org/x/sync/errgroup"
)

// Matcher matches devices for apply order
type Matcher struct {
	rsLogics     rollingserver.Logics
	planLogics   plan.Logics
	configLogics config.Logics
	informer     informer.Interface
	sops         sopsapi.SopsClientInterface
	sopsOpt      cc.SopsCli
	cc           cmdb.Client
	bkchat       bkchatapi.BkChatClientInterface
	cmsiClient   cmsi.Client
	ctx          context.Context
	kt           *kit.Kit
}

// New create a matcher
func New(ctx context.Context, rsLogics rollingserver.Logics, thirdCli *thirdparty.Client, cmdbCli cmdb.Client,
	clientConf cc.ClientConfig, informer informer.Interface, planLogics plan.Logics, configLogics config.Logics,
	cmsiCli cmsi.Client) (*Matcher, error) {

	matcher := &Matcher{
		rsLogics:     rsLogics,
		planLogics:   planLogics,
		configLogics: configLogics,
		informer:     informer,
		sops:         thirdCli.Sops,
		sopsOpt:      clientConf.Sops,
		cc:           cmdbCli,
		bkchat:       thirdCli.BkChat,
		cmsiClient:   cmsiCli,
		ctx:          ctx,
		kt:           &kit.Kit{Ctx: ctx, Rid: uuid.UUID()},
	}

	// TODO: get worker num from config
	go matcher.Run(60)

	return matcher, nil
}

// Run starts matcher workers
func (m *Matcher) Run(workers int) {
	for i := 0; i < workers; i++ {
		go wait.Until(m.runWorker, time.Second, m.ctx)
	}

	select {
	case <-m.ctx.Done():
		logs.Infof("matcher exits")
	}
}

// runWorker deals with apply order match task
func (m *Matcher) runWorker() error {
	// Check if informer is available (only available on master node)
	if m.informer == nil {
		logs.Warnf("task scheduler informer matcher is not available")
		time.Sleep(time.Second)
		return nil
	}

	generateInformer := m.informer.Generate()
	if generateInformer == nil {
		logs.Warnf("task scheduler generate informer is not available")
		time.Sleep(time.Second)
		return nil
	}

	generateID, err := generateInformer.Pop()
	if err != nil {
		return err
	}
	if len(generateID) == 0 {
		logs.Warnf("shutdown to deal generate informer, for get generate id from informer")
		time.Sleep(time.Second)
		return nil
	}

	// 为匹配任务创建后台操作的 kt
	kt := core.NewBackendKit()

	// get generate record
	generateRecord, err := m.GetGenerateRecord(kt, generateID)
	if err != nil {
		logs.Errorf("failed to get generate record by id: %d, err: %v, rid: %s", generateID, err, kt.Rid)
		return err
	}

	// check generate record status
	if generateRecord.Status != types.GenerateStatusSuccess {
		logs.Infof("generate record %s is not done yet, need not match, subOrderID: %s, status: %d, rid: %s",
			generateID, generateRecord.SubOrderId, generateRecord.Status, kt.Rid)
		return nil
	}

	// check generate record matched or not
	if generateRecord.IsMatched == true {
		logs.Infof("generate record %s is matched, need not match again, subOrderID: %s, rid: %s",
			generateID, generateRecord.SubOrderId, kt.Rid)
		return nil
	}

	// deal match device
	if err = m.matchHandler(kt, generateRecord); err != nil {
		logs.Errorf("failed to match device, order id: %s, err: %v, rid: %s", generateRecord.SubOrderId, err, kt.Rid)
		return err
	}

	logs.Infof("match done, generate id: %s, order id: %s, rid: %s", generateID, generateRecord.SubOrderId, kt.Rid)

	return nil
}

// FinalApplyStep after deliver device, check order result to regenerate device or reinit
func (m *Matcher) FinalApplyStep(kt *kit.Kit, genRecord *types.GenerateRecord, order *types.ApplyOrder) error {
	// CVM生产（管理员）的申请单，不需要设置IsMatched（只有走完初始化、交付流程的单据，才需要设置IsMatched）
	if order.ProductType != enumor.ProductTypeAdmin {
		// set generate record matched
		if err := m.setGenerateRecordMatched(kt, genRecord.GenerateId); err != nil {
			logs.Errorf("failed to update generate record, err: %v, schedule id: %s, rid: %s",
				err, genRecord.GenerateId, kt.Rid)
			return err
		}
	}

	// update apply order status
	if err := m.UpdateApplyOrderStatus(kt, order); err != nil {
		logs.Errorf("failed to update apply order status, order id: %s, err: %v, rid: %s", genRecord.SubOrderId, err,
			kt.Rid)
		return err
	}

	// 采购到资源池的子单，不需要通知以及更新采购资源池子单状态等逻辑
	if order.Source == enumor.ApplyTicketSrcPurchaseToResPool {
		return nil
	}

	// send ticket done notification
	if err := m.notifyApplyDone(kt, order.OrderId); err != nil {
		logs.Warnf("failed to send apply done notification, order id: %s, err: %v, rid: %s",
			genRecord.SubOrderId, err, kt.Rid)
	}

	if err := m.checkAndNotifyDelivery(kt, order.OrderId); err != nil {
		logs.Warnf("check delivery notification failed, orderId: %d, err: %v, rid: %s", order.OrderId, err, kt.Rid)
	}

	if err := m.updatePurchaseToResPoolSuborderRunning(kt, order); err != nil {
		logs.Errorf("failed to update purchase to resource pool suborder running, err: %v, orderId: %s, rid: %s",
			err, order.OrderId, kt.Rid)
		return err
	}

	return nil
}

// matchHandler apply order match handler
func (m *Matcher) matchHandler(kt *kit.Kit, genRecord *types.GenerateRecord) error {
	// get apply order by key
	applyOrder, err := m.getApplyOrder(kt, genRecord.SubOrderId)
	if err != nil {
		logs.Errorf("get apply order by key %s failed, err: %v, rid: %s", genRecord.SubOrderId, err, kt.Rid)
		return err
	}

	// check order status
	if applyOrder.Status != types.ApplyStatusMatching && applyOrder.Status != types.ApplyStatusGracefulTerminate {
		logs.Infof("apply order %s cannot match for status not Matching, generateID: %s, status: %s, rid: %s",
			genRecord.SubOrderId, genRecord.GenerateId, applyOrder.Status, kt.Rid)
		return fmt.Errorf("apply order %s cannot match for status not Matching, status: %s", genRecord.SubOrderId,
			applyOrder.Status)
	}

	// CVM生产（管理员）-不需要走初始化、交付流程
	if applyOrder.ProductType == enumor.ProductTypeAdmin {
		return m.FinalApplyStep(kt, genRecord, applyOrder)
	}

	// match device
	if err = m.matchDevice(kt, applyOrder, genRecord.GenerateId); err != nil {
		logs.Errorf("failed to match device, order id: %s, generateID: %s, err: %v, rid: %s", genRecord.SubOrderId,
			genRecord.GenerateId, err, kt.Rid)
		return err
	}

	return m.FinalApplyStep(kt, genRecord, applyOrder)
}

// getApplyOrder gets apply order from db by order id
func (m *Matcher) getApplyOrder(kt *kit.Kit, orderId string) (*types.ApplyOrder, error) {
	applyFilter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", orderId),
	)
	order, err := model.Operation().ApplyOrder().GetApplyOrder(kt, applyFilter)
	if err != nil {
		logs.Errorf("failed to get apply order by id: %s", orderId)
		return nil, err
	}

	return order, nil
}

func (m *Matcher) updateSuspendSteps(kt *kit.Kit, order *types.ApplyOrder) error {
	now := time.Now()
	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", order.SubOrderId),
		tools.RuleEqual("step_name", types.StepNameGenerate),
	)
	update := &cvmapplyproto.ZiyanCvmApplyStepUpdateReq{
		Status:  cvt.ValToPtr(types.StepStatusFailed),
		EndAt:   now.Format(constant.DateTimeLayout),
		Message: "can not get generateId, unknown generate status, check YunTi to find if devices are generated",
	}

	if err := model.Operation().ApplyStep().UpdateApplyStep(kt, filter, update); err != nil {
		logs.Errorf("failed to update apply 生产 step status to apply status failed, suborderId: %s, err: %v, rid: %s",
			order.SubOrderId, err, kt.Rid)
		return err
	}
	return nil
}

func (m *Matcher) updateGenerateFailed(kt *kit.Kit, generateID string) error {
	filter := tools.ExpressionAnd(tools.RuleEqual("generate_id", generateID))
	update := &cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq{
		Status: cvt.ValToPtr(types.GenerateStatusFailed),
		Message: cvt.ValToPtr("can not get generateId, unknown generate status, check YunTi to find " +
			"if devices are generated"),
	}

	if err := model.Operation().GenerateRecord().UpdateGenerateRecord(kt, filter, update); err != nil {
		logs.Errorf("failed to update generate record to failed, generateID: %s, err: %v, rid: %s",
			generateID, err, kt.Rid)
		return err
	}

	return nil
}

// UpdateApplyOrderStatus update apply order status
func (m *Matcher) UpdateApplyOrderStatus(kt *kit.Kit, order *types.ApplyOrder) error {
	// 1. get unreleased devices from db
	devices, err := m.GetUnreleasedDevice(kt, order.SubOrderId)
	if err != nil {
		logs.Errorf("failed to get unreleased device, order id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return err
	}

	// 2. calculate apply order status by total and matched count
	var diskType enumor.DiskType
	if order.Spec != nil {
		diskType = order.Spec.DiskType
	}
	deviceTypeCountMap, deliverGroupCntMap := m.calDeviceTypeCountMap(devices, diskType)
	matchedCnt := calMatchCnt(devices)

	genRecords, err := m.GetOrderGenRecords(kt, order.SubOrderId)
	if err != nil {
		logs.Errorf("failed to get generate records, order id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return err
	}

	hasGenRecordMatching := false
	isSuspend := false
	suspendCnt := 0

	for _, recordItem := range genRecords {
		if recordItem.Status == types.GenerateStatusInit || recordItem.Status == types.GenerateStatusHandling ||
			recordItem.Status == types.GenerateStatusSuccess && !recordItem.IsMatched {
			hasGenRecordMatching = true
		}

		if recordItem.Status == types.GenerateStatusSuspend {
			isSuspend = true
			suspendCnt += int(recordItem.TotalNum)
			logs.Infof("generate failed, unknown if generate interface was called, task_id not obtained, "+
				"check machines, rid: %s", kt.Rid)
			if err = m.updateGenerateFailed(kt, recordItem.GenerateId); err != nil {
				logs.Errorf("failed to update generate status to failed, suborderId: %s, err: %v, rid: %s",
					order.SubOrderId, err, kt.Rid)
			}
		}
	}

	pendingCnt, status, stage := m.calcApplyOrderStatus(order.ResourceType, matchedCnt, order.TotalNum,
		hasGenRecordMatching)

	if isSuspend && suspendCnt+matchedCnt >= int(order.TotalNum) {
		status = types.ApplyStatusTerminate
		stage = types.TicketStageSuspend
		if err = m.updateSuspendSteps(kt, order); err != nil {
			logs.Errorf("failed to update suspend steps, suborderId: %s, err: %v, rid: %s",
				order.SubOrderId, err, kt.Rid)
		}
	}

	if order.RequireType.IsNeedQuotaManage() {
		appliedTypes := []enumor.AppliedType{enumor.NormalAppliedType, enumor.ResourcePoolAppliedType}

		if err = m.rsLogics.UpdateSubOrderRollingDeliveredCore(kt, order.BkBizId, order.SubOrderId, appliedTypes,
			deviceTypeCountMap); err != nil {
			logs.Errorf("update rolling delivered cpu field failed, err: %v, suborder_id: %s, bizID: %d, "+
				"deviceTypeCountMap: %v, rid: %s", err, order.SubOrderId, order.BkBizId, deviceTypeCountMap, kt.Rid)
			return err
		}
	}

	// 3. do update apply order status
	err = m.updateApplyOrderToDb(kt, order, matchedCnt, pendingCnt, stage, status, deliverGroupCntMap)
	if err != nil {
		return err
	}

	return nil
}

func (m *Matcher) calcApplyOrderStatus(resType types.ResourceType, matchedCnt int, totalNum uint,
	hasGenRecordMatching bool) (int, types.ApplyStatus, types.TicketStage) {

	pendingCnt := 0
	status := types.ApplyStatusDone
	stage := types.TicketStageDone
	if matchedCnt < int(totalNum) {
		pendingCnt = int(totalNum) - matchedCnt
		// TODO 临时，升降配order不进入matchedSome，直接失败
		if resType == types.ResourceTypeUpgradeCvm {
			stage = types.TicketStageSuspend
			status = types.ApplyStatusTerminate
			return pendingCnt, status, stage
		}

		// do not set status to MATCHED_SOME if there are matching tasks
		status = types.ApplyStatusMatchedSome
		if hasGenRecordMatching {
			status = types.ApplyStatusMatching
		}
		stage = types.TicketStageRunning
	}

	return pendingCnt, status, stage
}

// calMatchCnt calculate matched count
func calMatchCnt(devices []*types.DeviceInfo) int {
	matchedCnt := 0
	for _, device := range devices {
		if !device.IsDelivered {
			continue
		}
		matchedCnt++
	}

	return matchedCnt
}

// getRegionList get region list by zone list
func (m *Matcher) getRegionList(kt *kit.Kit, zoneList []string) ([]*cfgtype.Zone, error) {
	req := &cfgtype.GetZoneParam{}
	// if input is empty list, return all zone info
	if len(zoneList) > 0 {
		req.Zone = zoneList
	}
	zoneResp, err := m.configLogics.Zone().GetZone(kt, req)
	if err != nil {
		return nil, err
	}

	return zoneResp.Info, nil
}

// calDeviceTypeCountMap calculate matched count
func (m *Matcher) calDeviceTypeCountMap(devices []*types.DeviceInfo, diskType enumor.DiskType) (
	map[string]int, map[types.DeliveredCVMKey]int) {

	deviceTypeCountMap := make(map[string]int)
	deliverGroupCntMap := make(map[types.DeliveredCVMKey]int)

	for _, device := range devices {
		if !device.IsDelivered {
			continue
		}

		if _, ok := deviceTypeCountMap[device.DeviceType]; !ok {
			deviceTypeCountMap[device.DeviceType] = 0
		}
		deviceTypeCountMap[device.DeviceType]++

		deliveredKey := types.DeliveredCVMKey{
			DeviceType: device.DeviceType,
			Region:     device.CloudRegion,
			Zone:       device.CloudZone,
			DiskType:   diskType,
		}

		if _, ok := deliverGroupCntMap[deliveredKey]; !ok {
			deliverGroupCntMap[deliveredKey] = 0
		}
		deliverGroupCntMap[deliveredKey]++
	}
	return deviceTypeCountMap, deliverGroupCntMap
}

// updateApplyOrderToDb update apply order status to db
func (m *Matcher) updateApplyOrderToDb(kt *kit.Kit, order *types.ApplyOrder, matchedCnt int, pendingCnt int,
	stage types.TicketStage, status types.ApplyStatus, deviceTypeCountMap map[types.DeliveredCVMKey]int) error {

	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", order.SubOrderId))
	update := &cvmapplyproto.ZiyanCvmApplySuborderUpdateReq{
		SuccessNum: cvt.ValToPtr(uint(matchedCnt)),
		PendingNum: cvt.ValToPtr(uint(pendingCnt)),
		Stage:      stage,
		Status:     status,
	}
	// 记录交付核数，用于预测扣除
	if order.ResourceType == types.ResourceTypeCvm ||
		order.ResourceType == types.ResourceTypeUpgradeCvm {
		sum, verifyGroups, err := m.GetCpuCoreSum(kt, deviceTypeCountMap)
		if err != nil {
			logs.Errorf("get cpu core failed, err: %v, deviceTypeCountMap: %v, rid: %s",
				err, deviceTypeCountMap, kt.Rid)
			return err
		}
		update.DeliveredCore = cvt.ValToPtr(uint(sum))
		planExpendGroups := slice.Map(verifyGroups, func(t plan.VerifyResPlanElemV2) types.PlanExpendGroup {
			return types.PlanExpendGroup{
				DeviceType: t.DeviceType,
				Region:     t.RegionID,
				Zone:       t.ZoneID,
				CPUCore:    t.CpuCore,
			}
		})
		planExpendGroupsJSON, err := model.MarshalToJsonField(planExpendGroups)
		if err != nil {
			logs.Errorf("marshal plan expend group failed, subOrderID: %s, err: %v, planExpendGroups: %v, rid: %s",
				order.SubOrderId, err, planExpendGroups, kt.Rid)
			return err
		}
		update.PlanExpendGroup = cvt.ValToPtr(planExpendGroupsJSON)

		// 为该子订单匹配CVM资源预测单并生成预测变更记录
		if err = m.planLogics.AddMatchedPlanDemandExpendLogs(kt, order.BkBizId, order, verifyGroups); err != nil {
			logs.Errorf("failed to add matched plan demand expend logs, subOrderID: %s, err: %v, subOrder: %+v",
				order.SubOrderId, err, cvt.PtrToVal(order))
			return err
		}
	}

	if err := model.Operation().ApplyOrder().UpdateApplyOrder(kt, filter, update); err != nil {
		logs.Errorf("failed to update apply order, id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return err
	}
	return nil
}

// GetCpuCoreSum 获取机型对应的cpu核数之和，以及按照机型类型、region、zone分组的核数之和
func (m *Matcher) GetCpuCoreSum(kt *kit.Kit, deviceTypeCountMap map[types.DeliveredCVMKey]int) (
	int64, []plan.VerifyResPlanElemV2, error) {

	deviceTypesMap := make(map[string]interface{})
	for deliverGroup := range deviceTypeCountMap {
		deviceTypesMap[deliverGroup.DeviceType] = nil
	}
	deviceTypes := maps.Keys(deviceTypesMap)
	deviceTypeInfoMap, err := m.configLogics.Device().ListCvmInstanceInfoByDeviceTypes(kt, deviceTypes)
	if err != nil {
		logs.Errorf("get cvm instance info by device type failed, err: %v, device_types: %v, rid: %s",
			err, deviceTypes, kt.Rid)
		return 0, nil, err
	}

	var deliveredCore int64
	verifyGroupMap := make(map[plan.VerifyResPlanElemV2]int64)
	for deliverGroup, count := range deviceTypeCountMap {
		deviceTypeInfo, ok := deviceTypeInfoMap[deliverGroup.DeviceType]
		if !ok {
			logs.Errorf("can not find device_type, type: %s, rid: %s", deliverGroup.DeviceType, kt.Rid)
			return 0, nil, fmt.Errorf("can not find device_type, type: %s", deliverGroup.DeviceType)
		}
		deliveredCore += deviceTypeInfo.CPUAmount * int64(count)
		verifyGroupKey := plan.VerifyResPlanElemV2{
			DeviceType: deliverGroup.DeviceType,
			RegionID:   deliverGroup.Region,
			ZoneID:     deliverGroup.Zone,
		}
		verifyGroupMap[verifyGroupKey] += deliveredCore
	}

	verifyGroups := make([]plan.VerifyResPlanElemV2, 0, len(verifyGroupMap))
	for key, val := range verifyGroupMap {
		verifyGroups = append(verifyGroups, plan.VerifyResPlanElemV2{
			DeviceType: key.DeviceType,
			RegionID:   key.RegionID,
			ZoneID:     key.ZoneID,
			CpuCore:    val,
		})
	}

	return deliveredCore, verifyGroups, nil
}

// GetGenerateRecord gets generate record from db by generate id
func (m *Matcher) GetGenerateRecord(kt *kit.Kit, generateID string) (*types.GenerateRecord, error) {
	filter := tools.ExpressionAnd(tools.RuleEqual("generate_id", generateID))
	recordInfo, err := model.Operation().GenerateRecord().GetGenerateRecord(kt, filter)
	if err != nil {
		logs.Errorf("failed to get generate record by id: %s, err: %v, rid: %s", generateID, err, kt.Rid)
		return nil, err
	}

	return recordInfo, nil
}

// GetOrderGenRecords gets all generate records related to given order
func (m *Matcher) GetOrderGenRecords(kt *kit.Kit, suborderID string) ([]*types.GenerateRecord, error) {
	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", suborderID))
	records, err := model.Operation().GenerateRecord().FindManyGenerateRecord(kt, filter, nil)
	if err != nil {
		logs.Errorf("failed to get generate record by order id: %s, err: %+v, rid: %s", suborderID, err, kt.Rid)
		return nil, err
	}

	return records, nil
}

// setGenerateRecordMatched set generate record matched
func (m *Matcher) setGenerateRecordMatched(kt *kit.Kit, generateID string) error {
	filter := tools.ExpressionAnd(tools.RuleEqual("generate_id", generateID))
	update := &cvmapplyproto.ZiyanCvmGenerateRecordUpdateReq{
		IsMatched: cvt.ValToPtr(true),
	}
	if err := model.Operation().GenerateRecord().UpdateGenerateRecord(kt, filter, update); err != nil {
		logs.Errorf("failed to update generate record, generate id: %s, update: %+v, err: %v, rid: %s",
			generateID, update, err, kt.Rid)
		return err
	}

	return nil
}

// InitDevices start init devices
func (m *Matcher) InitDevices(kt *kit.Kit, order *types.ApplyOrder, unreleased []*types.DeviceInfo) (
	[]*types.DeviceInfo, error) {

	// start init step
	if err := record.StartStep(kt, order.SubOrderId, types.StepNameInit); err != nil {
		logs.Errorf("failed to start init step, order id: %s, err: %v", order.SubOrderId, err)
		return nil, err
	}

	successDeviceMap, errMap := m.ProcessInitStep(kt, unreleased)
	if len(errMap) > 0 {
		// todo 暂时和原逻辑保持一致，这里err不做处理，ProcessInitStep内已经有打印错误日志
	}

	// update init step
	if err := record.UpdateInitStep(kt, order.SubOrderId, order.TotalNum); err != nil {
		logs.Errorf("failed to update init step, subOrderID: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return nil, err
	}

	return maps.Values(successDeviceMap), nil
}

// DeliverDevices deliver devices to business
func (m *Matcher) DeliverDevices(kt *kit.Kit, order *types.ApplyOrder, observeDevices []*types.DeviceInfo) error {
	// start deliver step
	if err := record.StartStep(kt, order.SubOrderId, types.StepNameDeliver); err != nil {
		logs.Errorf("failed to start deliver step, order id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return err
	}

	// deliver devices to business
	// TODO: batch processing
	for _, device := range observeDevices {
		if err := m.DeliverDevice(kt, device, order); err != nil {
			logs.Errorf("failed to deliver device, subOrderId: %s, ip: %s, err: %v, rid: %s", order.SubOrderId,
				device.Ip, err, kt.Rid)
			continue
		}
	}

	// update deliver step
	if err := record.UpdateDeliverStep(kt, order.SubOrderId, order.TotalNum); err != nil {
		logs.Errorf("failed to update init step, subOrderId: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return err
	}

	return nil
}

// ProcessInitStep process init step
func (m *Matcher) ProcessInitStep(kt *kit.Kit, devices []*types.DeviceInfo) (map[int]*types.DeviceInfo, map[int]error) {
	maxRetry := 3
	errMap := make(map[int]error)
	deviceInitMsgMap := make(map[int]*types.DeviceInitMsg)
	successDeviceMap := make(map[int]*types.DeviceInfo)
	eg := errgroup.Group{}
	eg.SetLimit(10)
	var lock sync.Mutex

	// 1. 创建主机初始化任务
	for idx, device := range devices {
		curDevice := device
		curIdx := idx
		if curDevice.IsInited {
			successDeviceMap[curIdx] = curDevice
			logs.Infof("host %s is initialized, need not init, rid: %s", curDevice.Ip, kt.Rid)
			continue
		}
		eg.Go(func() error {
			var err error
			var initMsg *types.DeviceInitMsg
			for try := 0; try < maxRetry; try++ {
				if initMsg, err = m.initDevice(kt, curDevice); err != nil {
					logs.Errorf("failed to init device, will retry in 60s, ip: %s, err: %v, rid: %s",
						curDevice.Ip, err, kt.Rid)
					// 从yunti同步给公司cmdb, 到cc去同步公司cmdb信息，拿到ip，有时候会有1分钟内的延迟，所以这里sleep1分钟
					time.Sleep(time.Minute)
					continue
				}
				break
			}
			lock.Lock()
			defer lock.Unlock()
			if err != nil {
				errMap[curIdx] = err
				return nil
			}
			deviceInitMsgMap[curIdx] = initMsg
			return nil
		})
	}
	_ = eg.Wait()

	// 2. 检查主机初始化任务是否执行完成
	for idx, msg := range deviceInitMsgMap {
		curMsg := msg
		curIdx := idx
		eg.Go(func() error {
			err := m.CheckSopsUpdate(kt, curMsg.BizID, curMsg.Device, curMsg.JobUrl, curMsg.JobID)
			lock.Lock()
			defer lock.Unlock()
			if err != nil {
				logs.Errorf("failed to check sops task, ip: %s, err: %v, rid: %s", curMsg.Device.Ip, err, kt.Rid)
				errMap[curIdx] = err
				return nil
			}
			successDeviceMap[curIdx] = curMsg.Device
			return nil
		})
	}
	_ = eg.Wait()

	return successDeviceMap, errMap
}

// matchDevice deal match device tasks
func (m *Matcher) matchDevice(kt *kit.Kit, order *types.ApplyOrder, generateID string) error {
	// 1. get unreleased devices from db
	unreleased, err := m.getGeneratedDevice(kt, generateID, order.SubOrderId)
	if err != nil {
		logs.Errorf("failed to get unreleased device, order id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return err
	}

	observeDevices, err := m.InitDevices(kt, order, unreleased)

	if order.EnableDiskCheck {
		observeDevices, err = m.RunDiskCheck(kt, order, observeDevices)
		if err != nil {
			logs.Errorf("failed to run disk check task, order id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
			return err
		}
	}

	return m.DeliverDevices(kt, order, observeDevices)
}

// getGeneratedDevice gets generated devices bindings to generate record
func (m *Matcher) getGeneratedDevice(kt *kit.Kit, generateID string, subOrderID string) ([]*types.DeviceInfo, error) {
	filter := tools.ExpressionAnd(
		tools.RuleEqual("suborder_id", subOrderID),
		tools.RuleEqual("generate_id", generateID),
	)
	devices, err := model.Operation().DeviceInfo().GetDeviceInfo(kt, filter)
	if err != nil {
		logs.Errorf("failed to get binding devices to generate id: %s, subOrderID: %s, err: %v, rid: %s",
			generateID, subOrderID, err, kt.Rid)
		return nil, err
	}

	return devices, nil
}

// GetUnreleasedDevice gets unreleased devices bindings to current apply order
func (m *Matcher) GetUnreleasedDevice(kt *kit.Kit, subOrderID string) ([]*types.DeviceInfo, error) {
	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", subOrderID))
	devices, err := model.Operation().DeviceInfo().GetDeviceInfo(kt, filter)
	if err != nil {
		logs.Errorf("failed to get binding devices to order %s, err: %v, rid: %s", subOrderID, err, kt.Rid)
		return nil, err
	}

	return devices, nil
}

// initDevice executes device initialization task
func (m *Matcher) initDevice(kt *kit.Kit, info *types.DeviceInfo) (*types.DeviceInitMsg, error) {
	if info.IsInited {
		logs.Infof("host %s is initialized, need not init, subOrderID: %s, rid: %s", info.Ip, info.SubOrderId, kt.Rid)
		return &types.DeviceInitMsg{Device: info}, nil
	}

	// 获取并验证主机信息，返回bkBizID信息
	bkBizID, initRecord, hostInfo, err := m.validateDeviceForInit(kt, info)
	if err != nil {
		return nil, err
	}

	// 检查是否有进行中的初始化任务
	if initRecord != nil && initRecord.Status == types.InitStatusHandling {
		logs.Infof("init device host is initialing, need not init, subOrderID: %s, ip: %s, rid: %s",
			info.SubOrderId, info.Ip, kt.Rid)
		return &types.DeviceInitMsg{Device: info, JobUrl: initRecord.TaskLink, JobID: initRecord.TaskId,
			BizID: bkBizID}, nil
	}

	// create init record
	created, err := record.CreateInitRecord(kt, info.SubOrderId, info.Ip)
	if err != nil {
		logs.Errorf("create init task record failed, subOrderID: %s, ip: %s, err: %v, rid: %s",
			info.SubOrderId, info.Ip, err, kt.Rid)
		return nil, fmt.Errorf("host %s failed to initialize, err: %v", info.Ip, err)
	}

	// 记录已存在：可能是上次失败重试，也可能是其他协程并发抢先创建，仅当当前状态为 Failed 时才继续重试发起新的 sops 任务；
	// 其余状态(Init/Handling/Success)说明已有其他协程在处理或已完成，直接复用现有任务信息返回，避免重复创建 sops 任务。
	if !created {
		cur := initRecord
		if cur == nil {
			// race: validate 时还没有记录，但 CreateInitRecord 时已存在，重新读取最新状态
			cur, err = record.GetInitRecord(kt, info.SubOrderId, info.Ip)
			if err != nil {
				logs.Errorf("failed to get init record after duplicated create, subOrderID: %s, ip: %s, err: %v, "+
					"rid: %s", info.SubOrderId, info.Ip, err, kt.Rid)
				return nil, fmt.Errorf("host %s failed to initialize, err: %v", info.Ip, err)
			}
		}
		if cur.Status != types.InitStatusFailed {
			logs.Infof("init record already exists, skip creating new sops task, subOrderID: %s, ip: %s, status: %d, "+
				"rid: %s", info.SubOrderId, info.Ip, cur.Status, kt.Rid)
			return &types.DeviceInitMsg{Device: info, JobUrl: cur.TaskLink, JobID: cur.TaskId, BizID: bkBizID}, nil
		}
	}

	// 创建初始化任务（新建场景 或 已存在 Failed 重试场景）
	return m.createInitTask(kt, info, bkBizID, hostInfo)
}

// validateDeviceForInit 验证设备是否可以进行初始化，获取主机信息和业务ID
func (m *Matcher) validateDeviceForInit(kt *kit.Kit, info *types.DeviceInfo) (
	int64, *types.InitRecord, *cmdb.Host, error) {

	// 检查是否有进行中的初始化任务
	initRecord, err := record.GetInitRecord(kt, info.SubOrderId, info.Ip)
	if err != nil && errf.Error(err).Code != errf.RecordNotFound {
		logs.Errorf("failed to get init record, err: %v, subOrderID: %s, ip: %s, rid: %s",
			err, info.SubOrderId, info.Ip, kt.Rid)
		return 0, nil, nil, err
	}

	// 根据IP获取主机信息
	hostInfo, err := m.cc.GetHostInfoByIP(kt, info.Ip, 0)
	if err != nil {
		logs.Errorf("sops:process:check:matcher:ieod init, get host info by ip failed, ip: %s, infoBkBizID: %d, "+
			"err: %v, rid: %s", info.Ip, info.BkBizId, err, kt.Rid)
		return 0, nil, nil, err
	}

	// 检查固资号是否一致
	if hostInfo.BkAssetID != info.AssetId {
		logs.Errorf("sops:process:check:matcher:ieod init, asset id not match, infoBkBizID: %d, ip: %s, "+
			"deviceAssetID: %s, hostAssetID: %s, rid: %s", info.BkBizId, info.Ip, info.AssetId,
			hostInfo.BkAssetID, kt.Rid)
		return 0, nil, nil, errf.Newf(errf.InvalidParameter, "asset id not match, ip: %s, deviceAssetID: %s, "+
			"hostAssetID: %s", info.Ip, info.AssetId, hostInfo.BkAssetID)
	}

	// 根据bkHostID去cmdb获取bkBizID
	bkBizID, err := m.getHostBizID(kt, info, hostInfo.BkHostID)
	if err != nil {
		return 0, nil, nil, err
	}

	// 检查bkBizID是否为资源池业务
	if bkBizID != enumor.ResourcePoolBiz {
		logs.Errorf("sops:process:check:matcher:ieod init, biz id not match, infoBkBizID: %d, bkBizID: %d, ip: %s, "+
			"deviceAssetID: %s, rid: %s", info.BkBizId, bkBizID, info.Ip, info.AssetId, kt.Rid)
		return 0, nil, nil, errf.Newf(errf.InvalidParameter, "biz id not match, ip: %s, infoBkBizID: %d, bkBizID: %d",
			info.Ip, info.BkBizId, bkBizID)
	}

	return bkBizID, initRecord, hostInfo, nil
}

// getHostBizID 根据主机ID获取业务ID
func (m *Matcher) getHostBizID(kt *kit.Kit, info *types.DeviceInfo, bkHostID int64) (int64, error) {
	bkBizIDs, err := m.cc.GetHostBizIds(kt, []int64{bkHostID})
	if err != nil {
		logs.Errorf("sops:process:check:matcher:ieod init, get host info by host id failed, ip: %s, infoBkBizID: %d, "+
			"bkHostID: %d, err: %v, rid: %s", info.Ip, info.BkBizId, bkHostID, err, kt.Rid)
		return 0, err
	}
	bkBizID, ok := bkBizIDs[bkHostID]
	if !ok {
		logs.Errorf("can not find biz id by host id: %d, rid: %s", bkHostID, kt.Rid)
		return 0, fmt.Errorf("can not find biz id by host id: %d", bkHostID)
	}
	return bkBizID, nil
}

// createInitTask 创建设备初始化任务
func (m *Matcher) createInitTask(kt *kit.Kit, info *types.DeviceInfo, bkBizID int64,
	hostInfo *cmdb.Host) (*types.DeviceInitMsg, error) {

	// 1. create job
	jobId, jobUrl, err := sops.CreateInitSopsTask(kt, m.sops, info.Ip, m.sopsOpt.DevnetIP, bkBizID, hostInfo.BkOsType,
		info.SubOrderId)
	if err != nil {
		logs.Errorf("sops:process:check:matcher:ieod init device, host %s failed to initialize, infoBkBizID: %d, "+
			"bkBizID: %d, bkHostID: %d, err: %v, rid: %s", info.Ip, info.BkBizId, bkBizID, info.BkHostId, err, kt.Rid)
		// update init record
		errRecord := record.UpdateInitRecord(kt, info.SubOrderId, info.Ip, "", "",
			err.Error(), types.InitStatusFailed)
		if errRecord != nil {
			logs.Errorf("update init record failed, host ip: %s, bkBidID: %d, bkHostID: %d, err: %v",
				info.Ip, info.BkBizId, info.BkHostId, errRecord)
			return nil, fmt.Errorf("update init record failed, host ip: %s, err: %v", info.Ip, errRecord)
		}
		return nil, fmt.Errorf("host %s failed to initialize, err: %v", info.Ip, err)
	}

	jobIDStr := strconv.FormatInt(jobId, 10)
	// update init record
	errRecord := record.UpdateInitRecord(kt, info.SubOrderId, info.Ip, jobIDStr, jobUrl, "handling",
		types.InitStatusHandling)
	if errRecord != nil {
		logs.Warnf("host %s failed to update initialize record, jobID: %d, jobUrl: %s, bkBizID: %d, err: %v, rid: %s",
			info.Ip, jobId, jobUrl, bkBizID, errRecord, kt.Rid)
	}

	return &types.DeviceInitMsg{Device: info, JobUrl: jobUrl, JobID: jobIDStr, BizID: bkBizID}, nil
}

// CheckSopsUpdate 检查sops任务状态并更新
func (m *Matcher) CheckSopsUpdate(kt *kit.Kit, bkBizID int64, info *types.DeviceInfo, jobUrl string,
	jobIDStr string) error {

	// 1. get job status
	jobId, err := strconv.ParseInt(jobIDStr, 10, 64)
	if err != nil {
		logs.Errorf("can not get jobId by jobIDStr, jobIDStr: %s, err: %v, rid: %s", jobIDStr, err, kt.Rid)
		return fmt.Errorf("can not get jobId by jobIDStr, jobIDStr: %s", jobIDStr)
	}

	if _, err = sops.CheckTaskStatus(kt, m.sops, jobId, bkBizID); err != nil {
		logs.Infof("sops:process:check:matcher:ieod init device, host %s failed to initialize, jobID: %d, "+
			"jobUrl: %s, bkBizID: %d, err: %v, rid: %s", info.Ip, jobId, jobUrl, bkBizID, err, kt.Rid)
		// update init record
		errRecord := record.UpdateInitRecord(kt, info.SubOrderId, info.Ip, jobIDStr, jobUrl,
			err.Error(), types.InitStatusFailed)
		if errRecord != nil {
			logs.Errorf("host %s failed to initialize, bkBizID: %d, jobID: %d, jobUrl: %s, err: %v, rid: %s",
				info.Ip, bkBizID, jobId, jobUrl, errRecord, kt.Rid)
			return fmt.Errorf("host %s failed to initialize, err: %v", info.Ip, errRecord)
		}
		return fmt.Errorf("host %s failed to initialize, jobID: %d, err: %v", info.Ip, jobId, err)
	}

	// 2. update device status
	info.InitTaskId = strconv.FormatInt(jobId, 10)
	info.InitTaskLink = jobUrl
	if err = m.SetDeviceInited(kt, info); err != nil {
		logs.Errorf("host %s failed to initialize, jobID: %d, jobUrl: %s, err: %v, rid: %s",
			info.Ip, jobId, jobUrl, err, kt.Rid)
		return fmt.Errorf("host %s failed to initialize, jobID: %d, jobUrl: %s, err: %v", info.Ip, jobId, jobUrl, err)
	}

	// update init record
	if err = record.UpdateInitRecord(kt, info.SubOrderId, info.Ip, jobIDStr, jobUrl, "success",
		types.InitStatusSuccess); err != nil {
		logs.Errorf("host %s failed to initialize, bkBizID: %d, jobId: %d, jobUrl: %s, err: %v, rid: %s",
			info.Ip, bkBizID, jobId, jobUrl, err, kt.Rid)
		return fmt.Errorf("host %s failed to initialize, jobID: %d, jobUrl: %s, err: %v", info.Ip, jobId, jobUrl, err)
	}
	return nil
}

// checkDeviceDisk executes device disk check task
func (m *Matcher) checkDeviceDisk(info *types.DeviceInfo) error {
	if info.IsDiskChecked {
		logs.Infof("host %s is disk-checked, need not disk check", info.Ip)
		return nil
	}

	return nil
}

// DeliverDevice delivers device to business
func (m *Matcher) DeliverDevice(kt *kit.Kit, info *types.DeviceInfo, order *types.ApplyOrder) error {
	if info.IsDelivered {
		logs.Infof("host %s is delivered, need not deliver, subOrderID: %s", info.Ip, order.SubOrderId)
		return nil
	}

	// create deliver record
	if err := record.CreateDeliverRecord(kt, info); err != nil {
		logs.Errorf("failed to deliver device, ip: %s, subOrderID: %s, err: %v, rid: %s",
			info.Ip, order.SubOrderId, err, kt.Rid)
		return fmt.Errorf("failed to deliver device, ip: %s, err: %v", info.Ip, err)
	}
	// 1. set host module and host operator
	if err := m.transferHostAndSetOperator(info, order); err != nil {
		logs.Errorf("failed to deliver device, ip: %s, subOrderID: %s, err: %v, rid: %s",
			info.Ip, order.SubOrderId, err, kt.Rid)
		// update deliver record
		if errRecord := record.UpdateDeliverRecord(kt, info, err.Error(),
			types.DeliverStatusFailed); errRecord != nil {
			logs.Errorf("failed to deliver device, ip: %s, subOrderID: %s, err: %v, rid: %s",
				info.Ip, order.SubOrderId, err, kt.Rid)
			return fmt.Errorf("failed to deliver device, ip: %s, err: %v", info.Ip, err)
		}
		return fmt.Errorf("failed to deliver device, ip: %s, err: %v", info.Ip, err)
	}
	// 2. update device status
	if err := m.SetDeviceDelivered(kt, info); err != nil {
		logs.Errorf("failed to deliver device, ip: %s, subOrderID: %s, err: %v, rid: %s",
			info.Ip, order.SubOrderId, err, kt.Rid)
		return fmt.Errorf("failed to deliver device, ip: %s, err: %v", info.Ip, err)
	}

	// update deliver record
	if err := record.UpdateDeliverRecord(kt, info, "success",
		types.DeliverStatusSuccess); err != nil {
		logs.Errorf("failed to deliver device, ip: %s, subOrderID: %s, err: %v, rid: %s",
			info.Ip, order.SubOrderId, err, kt.Rid)
		return fmt.Errorf("failed to deliver device, ip: %s, err: %v", info.Ip, err)
	}

	return nil
}

// setDeviceChecked set device checked flag
func (m *Matcher) setDeviceChecked(kt *kit.Kit, info *types.DeviceInfo) error {
	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", info.SubOrderId), tools.RuleEqual("ip", info.Ip))
	update := &cvmapplyproto.ZiyanCvmDeviceInfoUpdateReq{
		IsChecked: cvt.ValToPtr(true),
	}
	if err := model.Operation().DeviceInfo().UpdateDeviceInfo(kt, filter, update); err != nil {
		logs.Errorf("failed to update device checked flag, ip: %s, err: %v", info.Ip, err)
		return err
	}

	info.IsChecked = true

	return nil
}

// SetDeviceInited set device inited flag
func (m *Matcher) SetDeviceInited(kt *kit.Kit, info *types.DeviceInfo) error {
	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", info.SubOrderId), tools.RuleEqual("ip", info.Ip))
	update := &cvmapplyproto.ZiyanCvmDeviceInfoUpdateReq{
		IsInited:     cvt.ValToPtr(true),
		InitTaskID:   info.InitTaskId,
		InitTaskLink: info.InitTaskLink,
	}

	if err := model.Operation().DeviceInfo().UpdateDeviceInfo(kt, filter, update); err != nil {
		logs.Errorf("failed to update device inited flag, ip: %s, err: %v, rid: %s", info.Ip, err, kt.Rid)
		return err
	}

	info.IsInited = true

	return nil
}

// SetDeviceDelivered set device delivered flag
func (m *Matcher) SetDeviceDelivered(kt *kit.Kit, info *types.DeviceInfo) error {
	filter := tools.ExpressionAnd(tools.RuleEqual("suborder_id", info.SubOrderId), tools.RuleEqual("ip", info.Ip))
	update := &cvmapplyproto.ZiyanCvmDeviceInfoUpdateReq{
		IsDelivered: cvt.ValToPtr(true),
	}

	if err := model.Operation().DeviceInfo().UpdateDeviceInfo(kt, filter, update); err != nil {
		logs.Errorf("failed to update device delivered flag, ip: %s, err: %v, rid: %s", info.Ip, err, kt.Rid)
		return err
	}

	return nil
}

func (m *Matcher) notifyApplyDone(kt *kit.Kit, orderId uint64) error {
	// check if all apply suborders done
	applyFilter := tools.ExpressionAnd(
		tools.RuleEqual("order_id", orderId),
		tools.RuleNotEqual("status", types.ApplyStatusDone),
		tools.RuleNotEqual("source", enumor.ApplyTicketSrcPurchaseToResPool),
	)
	cnt, err := model.Operation().ApplyOrder().CountApplyOrder(kt, applyFilter)
	if err != nil {
		return err
	}
	if cnt > 0 {
		// exist suborder not done, need not notify
		return nil
	}

	filterTicket := tools.ExpressionAnd(tools.RuleEqual("order_id", orderId))

	ticket, err := model.Operation().ApplyTicket().GetApplyTicket(kt, filterTicket)
	if err != nil {
		return nil
	}

	// TODO: add verification after front end set enable notice by default
	/*
		if !ticket.EnableNotice {
			// need not notify
			return nil
		}
	*/

	users := []string{ticket.User}
	users = append(users, ticket.Follower...)
	users = toolsutil.StrArrayUnique(users)
	noticeFmt := m.bkchat.GetNoticeFmt()
	bizName := m.getBizName(ticket.BkBizId)
	requireName := ticket.RequireType.GetName()
	createTime := ticket.CreateAt.Local().Format(constant.DateTimeLayout)
	if ticket.CreateAt.Location() == time.UTC {
		location, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			logs.Warnf("scheduler:logics:bkchat:notifyApplyDone:failed, orderId: %d, err: %v, createAt: %+v",
				orderId, err, ticket.CreateAt)
			return err
		}
		createTime = ticket.CreateAt.In(location).Format(constant.DateTimeLayout)
	}
	resType := types.ResourceTypeCvm
	if len(ticket.Suborders) > 0 && ticket.Suborders[0] != nil {
		resType = ticket.Suborders[0].ResourceType
	}
	bkHcmURL := cc.WoaServer().BkHcmURL
	content := fmt.Sprintf(noticeFmt, orderId, orderId, ticket.User, bizName, requireName, createTime, ticket.Remark,
		bkHcmURL, ticket.OrderId, ticket.BkBizId, ticket.BkBizId, resType)

	for _, user := range users {
		resp, err := m.bkchat.SendApplyDoneMsg(nil, nil, user, content)
		if err != nil {
			logs.Warnf("scheduler:logics:bkchat:notifyApplyDone:failed, failed to send bkchat message, err: %v", err)
			continue
		}
		if resp.Code != 0 {
			logs.Warnf("scheduler:logics:bkchat:notifyApplyDone:failed, failed to send bkchat message, "+
				"code: %d, msg: %s", resp.Code, resp.Msg)
			continue
		}
	}

	return nil
}

// checkAndNotifyDelivery 检查并触发邮件通知
func (m *Matcher) checkAndNotifyDelivery(kt *kit.Kit, orderId uint64) error {
	// 检查所有子单是否都已完成
	applyFilter := tools.ExpressionAnd(
		tools.RuleEqual("order_id", orderId),
		tools.RuleNotEqual("status", types.ApplyStatusDone),
		tools.RuleNotEqual("source", enumor.ApplyTicketSrcPurchaseToResPool),
	)
	cnt, err := model.Operation().ApplyOrder().CountApplyOrder(kt, applyFilter)
	if err != nil {
		logs.Errorf("count apply order failed, orderId: %d, err: %v, rid: %s", orderId, err, kt.Rid)
		return err
	}
	if cnt > 0 {
		// 还有子单未完成，主单未完成，不触发通知
		logs.Infof("skip notify: main order not done, orderId:%d, not_done:%d, rid: %s", orderId, cnt, kt.Rid)
		return nil
	}

	// 检查是否有已交付的设备
	devices, err := m.getDeliveryDevices(kt, int64(orderId))
	if err != nil {
		logs.Errorf("failed to get delivery devices, orderId: %d, err: %v, rid: %s", orderId, err, kt.Rid)
		return err
	}

	if len(devices) == 0 {
		// 没有已交付的设备，不触发通知
		logs.Infof("skip notify: no delivered devices, orderId:%d, rid: %s", orderId, kt.Rid)
		return nil
	}

	// 获取申请票据
	filterTicket := tools.ExpressionAnd(tools.RuleEqual("order_id", orderId))

	ticket, err := model.Operation().ApplyTicket().GetApplyTicket(kt, filterTicket)
	if err != nil {
		logs.Errorf("failed to get apply ticket, orderId: %d, err: %v, rid: %s", orderId, err, kt.Rid)
		return err
	}
	if ticket == nil {
		logs.Warnf("apply ticket not found, orderId: %d, rid: %s", orderId, kt.Rid)
		return nil
	}

	// 主单已完成，且有已交付的设备，触发邮件通知
	if err := m.sendDeliveryNotification(kt, int64(orderId), ticket, devices); err != nil {
		logs.Errorf("failed to send delivery notification, orderId: %d, err: %v, rid: %s", orderId, err, kt.Rid)
		return err
	}

	return nil
}

// sendDeliveryNotification 发送主机申请交付通知
func (m *Matcher) sendDeliveryNotification(kt *kit.Kit, orderId int64, ticket *types.ApplyTicket,
	devices []*types.DeviceInfo) error {

	if len(devices) == 0 {
		logs.Warnf("skip notify: no delivered devices, orderId:%d, rid: %s", orderId, kt.Rid)
		return nil
	}

	// 当前仅通知提单人
	receivers := []string{ticket.User}

	if len(receivers) == 0 {
		logs.Warnf("skip notify: no receivers, orderId:%d, user:%s, rid: %s", orderId, ticket.User, kt.Rid)
		return nil
	}

	// 发送邮件通知
	emailErr := m.sendDeliveryEmailNotification(kt, ticket, devices, receivers)
	if emailErr != nil {
		logs.Errorf("send email failed, orderId:%d, err:%v, rid: %s", orderId, emailErr, kt.Rid)
	}

	// 发送企业微信通知
	wecomErr := m.sendDeliveryWeComNotification(kt, ticket, receivers)
	if wecomErr != nil {
		logs.Errorf("send wecom failed, orderId:%d, err:%v, rid: %s", orderId, wecomErr, kt.Rid)
	}

	if emailErr != nil && wecomErr != nil {
		return fmt.Errorf("email and wecom notifications failed for orderId:%d", orderId)
	}
	if emailErr != nil {
		return emailErr
	}
	if wecomErr != nil {
		return wecomErr
	}

	return nil
}

// sendDeliveryEmailNotification 发送交付邮件通知
func (m *Matcher) sendDeliveryEmailNotification(kt *kit.Kit, ticket *types.ApplyTicket, devices []*types.DeviceInfo,
	receivers []string) error {
	// 生成邮件内容
	title, content, err := m.generateDeliveryEmailContent(kt, ticket, devices)
	if err != nil {
		return err
	}

	// 发送邮件
	mail := &cmsi.CmsiMail{
		Title:            title,
		Content:          content,
		ReceiverUserName: strings.Join(receivers, ","),
	}

	if err := m.cmsiClient.SendMail(kt, mail); err != nil {
		return err
	}

	return nil
}

// getDeliveryDevices 获取交付设备信息
func (m *Matcher) getDeliveryDevices(kt *kit.Kit, orderId int64) ([]*types.DeviceInfo, error) {
	filter := tools.ExpressionAnd(tools.RuleEqual("order_id", orderId), tools.RuleEqual("is_delivered", true))

	devices, err := model.Operation().DeviceInfo().FindManyDeviceInfo(kt, filter, nil)
	if err != nil {
		logs.Errorf("failed to query delivery devices, orderId: %d, err: %v, rid: %s", orderId, err, kt.Rid)
		return nil, err
	}

	return devices, nil
}

// sendDeliveryWeComNotification 发送交付企业微信通知
func (m *Matcher) sendDeliveryWeComNotification(kt *kit.Kit, ticket *types.ApplyTicket, receivers []string) error {
	// 生成企业微信通知内容
	content, err := m.generateDeliveryWeComContent(kt, ticket)
	if err != nil {
		return err
	}

	// 发送给所有接收者
	failedUsers := make([]string, 0)
	for _, user := range receivers {
		resp, err := m.bkchat.SendApplyDoneMsg(kt.Ctx, kt.Header(), user, content)
		if err != nil {
			logs.Warnf("failed to send delivery WeCom notification, orderId: %d, user: %s, err: %v, rid: %s",
				ticket.OrderId, user, err, kt.Rid)
			failedUsers = append(failedUsers, user)
			continue
		}
		if resp.Code != 0 {
			logs.Errorf("failed to send delivery WeCom notification, orderId: %d, user: %s, code: %d, msg: %s, rid: %s",
				ticket.OrderId, user, resp.Code, resp.Msg, kt.Rid)
			failedUsers = append(failedUsers, user)
			continue
		}
	}

	if len(failedUsers) > 0 {
		return fmt.Errorf("failed to send delivery WeCom notification to users: %v", failedUsers)
	}

	return nil
}

// generateDeliveryWeComContent 生成交付企业微信通知内容
func (m *Matcher) generateDeliveryWeComContent(kt *kit.Kit, ticket *types.ApplyTicket) (string, error) {
	bizName := m.getBizName(ticket.BkBizId)
	createTime := ticket.CreateAt.Local().Format(constant.DateTimeLayout)
	if locName := cc.WoaServer().LocalTimezone; locName != "" {
		if location, err := time.LoadLocation(locName); err != nil {
			logs.Warnf("get location time zone: %s failed, err: %v, rid: %s", locName, err, kt.Rid)
		} else {
			createTime = ticket.CreateAt.In(location).Format(constant.DateTimeLayout)
		}
	}
	requireType := ticket.RequireType.GetName()
	bkHcmURL := cc.WoaServer().BkHcmURL
	content := fmt.Sprintf(constant.HostDeliveryNoticeWeComContentTemplate,
		ticket.OrderId, ticket.OrderId, ticket.User, bizName, requireType, createTime, ticket.Remark,
		bkHcmURL, ticket.OrderId, ticket.BkBizId, ticket.BkBizId)
	return content, nil
}

// generateDeliveryEmailContent 生成交付邮件通知内容
func (m *Matcher) generateDeliveryEmailContent(kt *kit.Kit, ticket *types.ApplyTicket,
	devices []*types.DeviceInfo) (string, string, error) {
	// 邮件标题
	title := fmt.Sprintf(constant.HostDeliveryNoticeTitle, m.getBizName(ticket.BkBizId), ticket.OrderId)
	// 查询子单所属园区
	regionMap := m.fetchRegionMapBySubOrder(kt, ticket.OrderId)
	// 构建表格内容
	tableContent := buildDeliveryEmailTable(regionMap, devices)

	// 整体内容
	bkHcmURL := cc.WoaServer().BkHcmURL
	createTime := ticket.CreateAt.Local().Format(constant.DateTimeLayout)
	if locName := cc.WoaServer().LocalTimezone; locName != "" {
		if location, err := time.LoadLocation(locName); err != nil {
			logs.Warnf("get location time zone: %s failed, err: %v, rid: %s", locName, err, kt.Rid)
		} else {
			createTime = ticket.CreateAt.In(location).Format(constant.DateTimeLayout)
		}
	}
	// 生成设备视角链接的 Base64 编码参数
	deviceRulesEncoded, err := encodeHostApplyDeviceRules(int64(ticket.OrderId))
	if err != nil {
		logs.Errorf("failed to encode device rules, orderId: %d, err: %v, rid: %s", ticket.OrderId, err, kt.Rid)
		return "", "", err
	}

	content := fmt.Sprintf(constant.HostDeliveryNoticeEmailContentTemplate,
		title, bkHcmURL, bkHcmURL, title, m.getBizName(ticket.BkBizId), createTime, len(devices),
		int64(ticket.OrderId), tableContent, bkHcmURL, ticket.BkBizId, deviceRulesEncoded)

	return title, content, nil
}

func (m *Matcher) fetchRegionMapBySubOrder(kt *kit.Kit, orderID uint64) map[string]string {
	result := make(map[string]string)
	suborderFilter := tools.ExpressionAnd(tools.RuleEqual("order_id", orderID))
	orders, err := model.Operation().ApplyOrder().FindManyApplyOrder(kt, suborderFilter, nil)
	if err != nil {
		logs.Warnf("failed to query apply orders for region info, orderId: %d, err: %v", orderID, err)
		return result
	}
	for _, order := range orders {
		if order.Spec != nil && order.Spec.Region != "" {
			result[order.SubOrderId] = order.Spec.Region
		}
	}
	return result
}

func buildDeliveryEmailTable(regionMap map[string]string, devices []*types.DeviceInfo) string {
	type comboKey struct {
		deviceType string
		region     string
		zone       string
	}
	type comboStat struct {
		totalCount  int
		parentCount int
	}

	stats := make(map[comboKey]*comboStat)
	for _, device := range devices {
		key := comboKey{
			deviceType: device.DeviceType,
			region:     regionMap[device.SubOrderId],
			zone:       device.ZoneName,
		}
		if stats[key] == nil {
			stats[key] = &comboStat{}
		}
		stats[key].totalCount++
		if device.OwnerIP != "" {
			stats[key].parentCount++
		}
	}

	keys := make([]comboKey, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].deviceType != keys[j].deviceType {
			return keys[i].deviceType < keys[j].deviceType
		}
		if keys[i].region != keys[j].region {
			return keys[i].region < keys[j].region
		}
		return keys[i].zone < keys[j].zone
	})

	var builder strings.Builder
	for _, key := range keys {
		stat := stats[key]
		parentCountStr := "-"
		if stat.parentCount == stat.totalCount {
			parentCountStr = fmt.Sprintf("%d", stat.parentCount)
		}
		builder.WriteString(fmt.Sprintf(constant.HostDeliveryNoticeEmailTableTemplate,
			key.deviceType, stat.totalCount, key.region, key.zone, parentCountStr))
	}
	return builder.String()
}

// encodeHostApplyDeviceRules 生成设备视角链接的 Base64 编码参数
func encodeHostApplyDeviceRules(orderId int64) (string, error) {
	deviceRulesJSON := fmt.Sprintf(`{"orderId":%d,"bkUsername":[],"dateRange":[]}`, orderId)
	// 先进行 URL 编码
	urlEncoded := url.QueryEscape(deviceRulesJSON)
	// 再进行 Base64 编码
	deviceRulesEncoded := base64.StdEncoding.EncodeToString([]byte(urlEncoded))
	return deviceRulesEncoded, nil
}

// RunDiskCheck 执行磁盘检查
func (m *Matcher) RunDiskCheck(kt *kit.Kit, order *types.ApplyOrder, devices []*types.DeviceInfo) (
	[]*types.DeviceInfo, error) {

	// start init step
	if err := record.StartStep(kt, order.SubOrderId, types.StepNameDiskCheck); err != nil {
		logs.Errorf("failed to start init step, order id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return nil, err
	}

	mutex := sync.Mutex{}
	wg := sync.WaitGroup{}
	errs := make([]error, 0)
	observeDevices := make([]*types.DeviceInfo, 0)
	appendError := func(err error) {
		mutex.Lock()
		defer mutex.Unlock()
		errs = append(errs, err)
	}
	appendDevice := func(device *types.DeviceInfo) {
		mutex.Lock()
		defer mutex.Unlock()
		observeDevices = append(observeDevices, device)
	}
	for _, device := range devices {
		wg.Add(1)
		go func(device *types.DeviceInfo) {
			defer wg.Done()

			// check device disk
			maxRetry := 3
			var err error = nil
			for try := 0; try < maxRetry; try++ {
				if err = m.checkDeviceDisk(device); err != nil {
					logs.Errorf("failed to check device disk, will retry in 60s, ip: %s, err: %v, rid: %s",
						device.Ip, err, kt.Rid)
					time.Sleep(180 * time.Second)
					continue
				}
				break
			}

			if err != nil {
				appendError(err)
			} else {
				appendDevice(device)
			}
		}(device)
	}
	wg.Wait()

	// update disk check step
	if err := record.UpdateDiskCheckStep(kt, order.SubOrderId, order.TotalNum); err != nil {
		logs.Errorf("failed to update init step, order id: %s, err: %v, rid: %s", order.SubOrderId, err, kt.Rid)
		return nil, err
	}

	return observeDevices, nil
}

func (m *Matcher) updatePurchaseToResPoolSuborderRunning(kt *kit.Kit, order *types.ApplyOrder) error {
	if order.Source == enumor.ApplyTicketSrcPurchaseToResPool {
		return nil
	}

	orderId := order.OrderId
	applyFilter := tools.ExpressionAnd(
		tools.RuleEqual("order_id", orderId),
		tools.RuleNotEqual("status", types.ApplyStatusDone),
		tools.RuleNotEqual("source", enumor.ApplyTicketSrcPurchaseToResPool),
	)
	cnt, err := model.Operation().ApplyOrder().CountApplyOrder(kt, applyFilter)
	if err != nil {
		logs.Errorf("count apply order failed, orderId: %d, err: %v, rid: %s", orderId, err, kt.Rid)
		return err
	}
	if cnt > 0 {
		logs.Infof("skip update purchase to resource pool suborder running: main order not done, orderId: %d, "+
			"not_done: %d, rid: %s", orderId, cnt, kt.Rid)
		return nil
	}

	applyFilter = tools.ExpressionAnd(
		tools.RuleEqual("order_id", orderId),
		tools.RuleEqual("stage", types.TicketStageUncommit),
		tools.RuleEqual("source", enumor.ApplyTicketSrcPurchaseToResPool),
	)
	cnt, err = model.Operation().ApplyOrder().CountApplyOrder(kt, applyFilter)
	if err != nil {
		logs.Errorf("count apply order failed, orderId: %d, err: %v, rid: %s", orderId, err, kt.Rid)
		return err
	}
	if cnt == 0 {
		return nil
	}

	update := &cvmapplyproto.ZiyanCvmApplySuborderUpdateReq{
		Stage: types.TicketStageRunning,
	}
	if err = model.Operation().ApplyOrder().UpdateApplyOrder(kt, applyFilter, update); err != nil {
		logs.Errorf("failed to update purchase to resource pool suborder running, err: %v, orderId: %d, rid: %s",
			err, orderId, kt.Rid)
		return err
	}

	return nil
}
