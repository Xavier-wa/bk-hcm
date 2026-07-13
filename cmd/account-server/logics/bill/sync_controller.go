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

package bill

import (
	"encoding/json"
	"fmt"
	"time"

	syncaction "hcm/cmd/task-server/logics/action/obs/sync"
	"hcm/pkg/api/core"
	billcore "hcm/pkg/api/core/bill"
	"hcm/pkg/api/data-service/bill"
	dsbillapi "hcm/pkg/api/data-service/bill"
	taskserver "hcm/pkg/api/task-server"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/runtime/filter"
	"hcm/pkg/serviced"
	"hcm/pkg/thirdparty/obs"
	"hcm/pkg/tools/converter"

	"github.com/shopspring/decimal"
)

const (
	stateNew     = "new"
	stateSyncing = "syncing"
	stateSynced  = "synced"

	batchSize = uint64(50000)
)

// SyncRecordDetailItem ...
type SyncRecordDetailItem struct {
	RootAccountID string `json:"root_account_id"`
	MainAccountID string `json:"main_account_id"`
	Vendor        string `json:"vendor"`
	BillYear      int    `json:"bill_year"`
	BillMonth     int    `json:"bill_month"`
	ProductID     int64  `json:"product_id"`
	Total         uint64 `json:"total"`
	CurrentIndex  uint64 `json:"current_index"`
	BatchSize     uint64 `json:"batch_size"`
	FlowID        string `json:"flow_id"`
	State         string `json:"state"`
}

// NewSyncController create new sync controller
func NewSyncController(opt *SyncControllerOption) (*SyncController, error) {
	if opt.Client == nil {
		return nil, fmt.Errorf("client cannot be empty")
	}
	if opt.Sd == nil {
		return nil, fmt.Errorf("servicediscovery cannot be empty")
	}
	return &SyncController{
		Client: opt.Client,
		Sd:     opt.Sd,
		obs:    opt.Obs,
	}, nil
}

// SyncControllerOption option for sync controller
type SyncControllerOption struct {
	Client *client.ClientSet
	Sd     serviced.ServiceDiscover
	Obs    obs.Client
}

// SyncController bill sync controller
type SyncController struct {
	Client *client.ClientSet
	Sd     serviced.ServiceDiscover
	obs    obs.Client
}

// Run controller
func (sc *SyncController) Run() {
	go sc.syncLoop(getInternalKit())
}

func (sc *SyncController) syncLoop(kt *kit.Kit) {
	if sc.Sd.IsMaster() {
		sc.doSync(kt.NewSubKit())
	}

	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			if sc.Sd.IsMaster() {
				sc.doSync(kt.NewSubKit())
			}
		case <-kt.Ctx.Done():
			logs.Infof("sync record controller context done, rid: %s", kt.Rid)
			return
		}
	}
}

func (sc *SyncController) doSync(kt *kit.Kit) {
	pendingSyncRecordList, err := sc.listSyncingRecord(kt)
	if err != nil {
		logs.Errorf("list syncing record failed, err %s", err.Error())
		return
	}
	for _, record := range pendingSyncRecordList {
		if err := sc.handleSyncRecord(kt, record); err != nil {
			logs.Errorf("handle sync record of vendor %s %d-%d failed, err %v, rid: %s",
				record.Vendor, record.BillYear, record.BillMonth, err, kt.Rid)
			continue
		}
	}
}

func (sc *SyncController) listSyncingRecord(kt *kit.Kit) ([]*billcore.SyncRecord, error) {
	expressions := []*filter.AtomRule{
		tools.RuleIn("state", []enumor.BillSyncState{
			enumor.BillSyncRecordStateNew,
			enumor.BillSyncRecordStateSyncingBillItem,
			enumor.BillSyncRecordStateSyncingAdjustment,
		}),
	}
	pendingSyncRecordList, err := sc.Client.DataService().Global.Bill.ListBillSyncRecord(kt, &core.ListReq{
		Filter: tools.ExpressionAnd(expressions...),
		Page:   core.NewDefaultBasePage(),
	})
	if err != nil {
		logs.Errorf("list pending bill sync record failed, err %s, rid: %s", err.Error(), kt.Rid)
		return nil, err
	}
	return pendingSyncRecordList.Details, nil
}

func (sc *SyncController) handleSyncRecord(kt *kit.Kit, syncRecord *billcore.SyncRecord) error {
	if syncRecord.State == enumor.BillSyncRecordStateNew || len(syncRecord.Detail) == 0 {
		return sc.initSyncItem(kt, syncRecord)
	}
	itemList, err := sc.getItemListFromDetail(kt, syncRecord)
	if err != nil {
		return err
	}
	if len(itemList) == 0 {
		return sc.initSyncItem(kt, syncRecord)
	}

	allSynced, err := sc.advanceSyncItems(kt, syncRecord, itemList)
	if err != nil {
		return err
	}
	// 仍有未同步完成的 item，等待下一个 tick 继续推进
	if !allSynced {
		return nil
	}
	// all bill item synced, handle adjustment
	adjustmentSynced, err := sc.handleAdjustment(kt, syncRecord)
	if err != nil {
		logs.Errorf("failed to handle obs adjustment sync record, rid: %s", kt.Rid)
		return err
	}
	if !adjustmentSynced {
		// wait
		return nil
	}

	if err := sc.notifyObs(kt, syncRecord); err != nil {
		logs.Errorf("fail notify obs for bill item synced, err: %v, rid: %s", err, kt.Rid)
		return err
	}
	if err := sc.Client.DataService().Global.Bill.UpdateBillSyncRecord(kt, &bill.BillSyncRecordUpdateReq{
		ID:    syncRecord.ID,
		State: enumor.BillSyncRecordStateSynced,
	}); err != nil {
		logs.Warnf("update bill sync record state to synced failed, err %s, rid: %s", err.Error(), kt.Rid)
		return err
	}

	return nil
}

// advanceSyncItems 按并发度推进 sync record 内的 item，返回是否全部已同步完成。
func (sc *SyncController) advanceSyncItems(kt *kit.Kit, syncRecord *billcore.SyncRecord,
	itemList []*SyncRecordDetailItem) (bool, error) {

	// 并发度：同一 sync record 内同时处于 syncing 状态的 item 数量上限。
	// 名额释放不追求实时，已完成的 item 留待下个 tick 重新统计时再腾出名额
	maxConcurrency := converter.PtrToVal(cc.AccountServer().Controller.ObsSyncConcurrency)
	syncingCount := 0
	for _, item := range itemList {
		if item.State == stateSyncing {
			syncingCount++
		}
	}

	changed := false
	allSynced := true
	for index, item := range itemList {
		if item.State == stateSynced {
			continue
		}
		allSynced = false
		// 启动 item 同步，受并发度限制，达到上限则跳过，留待后续 tick
		if item.State == stateNew && syncingCount >= maxConcurrency {
			continue
		}
		beforeState := item.State
		afterItem, err := sc.handleSyncRecordDetailItem(kt, item)
		if err != nil {
			return false, err
		}
		itemList[index] = afterItem
		changed = true
		if beforeState == stateNew && afterItem.State == stateSyncing {
			syncingCount++
		}
	}

	if !changed {
		return allSynced, nil
	}
	if err := sc.updateSyncRecordDetail(kt, syncRecord, itemList); err != nil {
		return false, err
	}
	return allSynced, nil
}

// updateSyncRecordDetail 将 item 列表回写到 sync record 的 detail 字段。
func (sc *SyncController) updateSyncRecordDetail(kt *kit.Kit, syncRecord *billcore.SyncRecord,
	itemList []*SyncRecordDetailItem) error {

	newDetailData, err := json.Marshal(itemList)
	if err != nil {
		return err
	}
	req := &bill.BillSyncRecordUpdateReq{ID: syncRecord.ID, Detail: newDetailData}
	if err := sc.Client.DataService().Global.Bill.UpdateBillSyncRecord(kt, req); err != nil {
		logs.Errorf("update bill sync record detail failed, err: %s, record: %s, rid: %s",
			err, syncRecord.ID, kt.Rid)
		return err
	}
	return nil
}

func (sc *SyncController) notifyObs(kt *kit.Kit, syncRecord *billcore.SyncRecord) error {

	var obsAccountType obs.OBSAccountType
	var costColumn string
	var totalCount uint64
	var sum = decimal.Zero
	switch syncRecord.Vendor {
	case enumor.Aws:
		obsAccountType = obs.AccountTypeAws
		costColumn = "cost"
		// OBS 侧要求aws 使用外币金额
		sum = syncRecord.Cost
	case enumor.HuaWei:
		obsAccountType = obs.AccountTypeHuawei
		costColumn = "real_cost"
		sum = syncRecord.RMBCost
	case enumor.Gcp:
		obsAccountType = obs.AccountTypeGCP
		costColumn = "cost"
		sum = syncRecord.RMBCost

	default:
		return fmt.Errorf("unsupport vendor %s for obs notify repull", syncRecord.Vendor)
	}

	records, err := sc.getItemListFromDetail(kt, syncRecord)
	if err != nil {
		logs.Errorf("fail to unmarshal sync detail, err %s, rid: %s", err, kt.Rid)
		return err
	}
	for _, record := range records {
		totalCount += record.Total
	}

	// 通知 OBS 同步完成
	err = sc.obs.NotifyRePull(kt, &obs.NotifyObsPullReq{
		// 组装为OBS侧要求的格式, 202406
		YearMonth: int64(syncRecord.BillYear*100 + syncRecord.BillMonth),
		AccountInfoList: []obs.AccountInfo{{
			AccountType: obsAccountType,
			Total:       totalCount,
			Column:      costColumn,
			SumColValue: sum,
		}},
	})
	if err != nil {
		logs.Errorf("fail to notify obs to repull bill, err: %v, vendor: %s, year: %d, month: %d, sum: %s, count: %d,"+
			"col: %s, rid: %v", err, syncRecord.Vendor, syncRecord.BillYear, syncRecord.BillMonth, sum.String(),
			totalCount, costColumn, kt.Rid)
		return err
	}
	logs.Infof("notify obs to repull bill done, vendor: %s, year: %d, month: %d, sum: %s, count: %d, col: %s, rid: %s",
		syncRecord.Vendor, syncRecord.BillYear, syncRecord.BillMonth, sum.String(), totalCount, costColumn, kt.Rid)
	return nil
}

func (sc *SyncController) initSyncItem(kt *kit.Kit, syncRecord *billcore.SyncRecord) error {
	expressions := []*filter.AtomRule{
		tools.RuleEqual("vendor", syncRecord.Vendor),
		tools.RuleEqual("bill_year", syncRecord.BillYear),
		tools.RuleEqual("bill_month", syncRecord.BillMonth),
	}
	result, err := sc.Client.DataService().Global.Bill.ListBillSummaryMain(kt, &bill.BillSummaryMainListReq{
		Filter: tools.ExpressionAnd(expressions...),
		Page: &core.BasePage{
			Count: true,
		},
	})
	if err != nil {
		logs.Warnf("count all summary main failed, err %s, rid %s", err.Error(), kt.Rid)
		return err
	}
	var mainSummaryList []*bill.BillSummaryMain
	for offset := uint64(0); offset < result.Count; offset = offset + uint64(core.DefaultMaxPageLimit) {
		tmpResult, err := sc.Client.DataService().Global.Bill.ListBillSummaryMain(kt, &dsbillapi.BillSummaryMainListReq{
			Filter: tools.ExpressionAnd(expressions...),
			Page: &core.BasePage{
				Start: uint32(offset),
				Limit: core.DefaultMaxPageLimit,
			},
		})
		if err != nil {
			logs.Warnf("list all summary main failed, err %s, rid %s", err.Error(), kt.Rid)
			return err
		}
		mainSummaryList = append(mainSummaryList, tmpResult.Details...)
	}
	var itemList []*SyncRecordDetailItem
	for _, mainSummary := range mainSummaryList {
		itemList = append(itemList, &SyncRecordDetailItem{
			RootAccountID: mainSummary.RootAccountID,
			MainAccountID: mainSummary.MainAccountID,
			BillYear:      mainSummary.BillYear,
			BillMonth:     mainSummary.BillMonth,
			Vendor:        string(mainSummary.Vendor),
			ProductID:     mainSummary.ProductID,
			Total:         0,
			CurrentIndex:  0,
			BatchSize:     batchSize,
			FlowID:        "",
			State:         stateNew,
		})
	}
	newDetailData, err := json.Marshal(itemList)
	if err != nil {
		return err
	}
	req := &bill.BillSyncRecordUpdateReq{
		ID:     syncRecord.ID,
		State:  enumor.BillSyncRecordStateSyncingBillItem,
		Detail: newDetailData,
	}
	if err := sc.Client.DataService().Global.Bill.UpdateBillSyncRecord(kt, req); err != nil {
		logs.Errorf("update bill sync record detail failed, err: %s, record: %s rid: %s", err, syncRecord.ID, kt.Rid)
		return err
	}
	logs.Infof("init sync record for vendor %s with %d main account", syncRecord.Vendor, len(itemList))
	return nil
}

func (sc *SyncController) getItemListFromDetail(
	kt *kit.Kit, syncRecord *billcore.SyncRecord) ([]*SyncRecordDetailItem, error) {

	var itemList []*SyncRecordDetailItem
	if err := json.Unmarshal([]byte(syncRecord.Detail), &itemList); err != nil {
		logs.Warnf("decode sync record detail %s failed, err %s, rid: %s", syncRecord.Detail, err.Error(), kt.Rid)
		return nil, fmt.Errorf("decode sync record detail %s failed, err %s", syncRecord.Detail, err.Error())
	}
	return itemList, nil
}

func (sc *SyncController) handleSyncRecordDetailItem(kt *kit.Kit, syncRecordItem *SyncRecordDetailItem) (
	*SyncRecordDetailItem, error) {

	switch syncRecordItem.State {
	case stateNew:
		return sc.setTotal(kt, syncRecordItem)
	case stateSyncing:
		return sc.doSubSyncTask(kt, syncRecordItem)
	case stateSynced:
		return syncRecordItem, nil
	default:
		return nil, fmt.Errorf("invalid item state %s", stateSynced)
	}
}

func (sc *SyncController) setTotal(kt *kit.Kit, syncRecordItem *SyncRecordDetailItem) (*SyncRecordDetailItem, error) {
	flt := tools.ExpressionAnd(
		tools.RuleEqual("root_account_id", syncRecordItem.RootAccountID),
		tools.RuleEqual("main_account_id", syncRecordItem.MainAccountID),
		tools.RuleEqual("bill_year", syncRecordItem.BillYear),
		tools.RuleEqual("bill_month", syncRecordItem.BillMonth),
	)
	comOpt := &bill.ItemCommonOpt{
		Vendor: enumor.Vendor(syncRecordItem.Vendor),
		Year:   syncRecordItem.BillYear,
		Month:  syncRecordItem.BillMonth,
	}
	listReq := &bill.BillItemListReq{
		ItemCommonOpt: comOpt,
		ListReq:       &core.ListReq{Filter: flt, Page: core.NewCountPage()},
	}
	result, err := sc.Client.DataService().Global.Bill.ListBillItem(kt, listReq)
	if err != nil {
		logs.Warnf("count bill item for %s %s %d %d failed, err %s, rid: %s",
			syncRecordItem.RootAccountID, syncRecordItem.MainAccountID,
			syncRecordItem.BillYear, syncRecordItem.BillMonth, err.Error(), kt.Rid)
		return nil, err
	}
	syncRecordItem.Total = result.Count
	syncRecordItem.State = stateSyncing
	return syncRecordItem, nil
}

func (sc *SyncController) doSubSyncTask(kt *kit.Kit, syncRecordItem *SyncRecordDetailItem) (
	*SyncRecordDetailItem, error) {

	if len(syncRecordItem.FlowID) == 0 {
		// create custom sync flow
		id, err := sc.createSyncBillItemFlow(kt, syncRecordItem)
		if err != nil {
			return nil, err
		}
		syncRecordItem.FlowID = id
		syncRecordItem.State = stateSyncing
		return syncRecordItem, nil
	}
	flow, err := sc.Client.TaskServer().GetFlow(kt, syncRecordItem.FlowID)
	if err != nil {
		logs.Warnf("get sync flow %s failed, err %s, rid: %s", syncRecordItem.FlowID, err.Error(), kt.Rid)
		return nil, err
	}
	if flow.State == enumor.FlowSuccess {
		if syncRecordItem.CurrentIndex+syncRecordItem.BatchSize > syncRecordItem.Total {
			syncRecordItem.State = stateSynced
			return syncRecordItem, nil
		}
		syncRecordItem.FlowID = ""
		syncRecordItem.CurrentIndex = syncRecordItem.CurrentIndex + syncRecordItem.BatchSize
		syncRecordItem.State = stateSyncing
		return syncRecordItem, nil
	} else if flow.State == enumor.FlowFailed {

		id, err := sc.createSyncBillItemFlow(kt, syncRecordItem)
		if err != nil {
			return nil, err
		}
		syncRecordItem.FlowID = id
		syncRecordItem.State = stateSyncing
		return syncRecordItem, nil
	}
	return syncRecordItem, nil
}

func (sc *SyncController) createSyncBillItemFlow(kt *kit.Kit, syncDetail *SyncRecordDetailItem) (string, error) {

	memo := fmt.Sprintf("obs :%s %s/%s %d-%d", syncDetail.Vendor, syncDetail.RootAccountID, syncDetail.MainAccountID,
		syncDetail.BillYear, syncDetail.BillMonth)
	flowReq := &taskserver.AddCustomFlowReq{
		Name: enumor.FlowObsSyncBillItem,
		Memo: memo,
		Tasks: []taskserver.CustomFlowTask{
			syncaction.BuildSyncTask(
				syncDetail.RootAccountID,
				syncDetail.MainAccountID, enumor.Vendor(syncDetail.Vendor),
				syncDetail.BillYear, syncDetail.BillMonth,
				syncDetail.CurrentIndex, syncDetail.BatchSize),
		},
	}
	result, err := sc.Client.TaskServer().CreateCustomFlow(kt, flowReq)
	if err != nil {
		logs.Errorf("create obs bill item sync task for %v failed, err: %s, rid: %s", syncDetail, err.Error(), kt.Rid)
		return "", err
	}
	return result.ID, nil
}

func (sc *SyncController) createSyncAdjustmentFlow(kt *kit.Kit, record *billcore.SyncRecord) (string, error) {

	memo := fmt.Sprintf("obs adjustment:%s %d-%d", record.Vendor, record.BillYear, record.BillMonth)
	flowReq := &taskserver.AddCustomFlowReq{
		Name: enumor.FlowObsSyncAdjustment,
		Memo: memo,
		Tasks: []taskserver.CustomFlowTask{
			syncaction.BuildSyncAdjustmentTask(record.Vendor, record.BillYear, record.BillMonth),
		},
	}
	result, err := sc.Client.TaskServer().CreateCustomFlow(kt, flowReq)
	if err != nil {
		logs.Errorf("create obs adjustment sync task for %s %d-%02d failed, err: %v, rid: %s",
			record.Vendor, record.BillYear, record.BillMonth, err, kt.Rid)
		return "", err
	}
	return result.ID, nil
}

func (sc *SyncController) handleAdjustment(kt *kit.Kit, record *billcore.SyncRecord) (synced bool, err error) {
	flowID := record.AdjustmentFlowID
	if len(flowID) == 0 {
		return sc.resetAdjustmentFlowId(kt, record)
	}

	flow, err := sc.Client.TaskServer().GetFlow(kt, flowID)
	if err != nil {
		logs.Errorf("get obs adjustment sync flow %s failed, err: %s, rid: %s", flowID, err, kt.Rid)
		return false, err
	}
	switch flow.State {
	case enumor.FlowCancel, enumor.FlowFailed:
		// retry
		return sc.resetAdjustmentFlowId(kt, record)
	case enumor.FlowSuccess:
		// 	success
		return true, nil
	default:
		// wait
		return false, nil
	}
}

func (sc *SyncController) resetAdjustmentFlowId(kt *kit.Kit, record *billcore.SyncRecord) (bool, error) {
	flowID, err := sc.createSyncAdjustmentFlow(kt, record)
	if err != nil {
		return false, err
	}
	record.AdjustmentFlowID = flowID
	req := &bill.BillSyncRecordUpdateReq{
		ID:               record.ID,
		State:            enumor.BillSyncRecordStateSyncingAdjustment,
		AdjustmentFlowID: flowID}
	if err := sc.Client.DataService().Global.Bill.UpdateBillSyncRecord(kt, req); err != nil {
		logs.Errorf("set obs adjustment sync record flow id failed, err: %v, rid: %s", err, kt.Rid)
		return false, err
	}
	return false, nil
}
