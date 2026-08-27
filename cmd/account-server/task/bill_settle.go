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

package task

import (
	"time"

	"hcm/pkg/api/core"
	dsbill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/cc"
	"hcm/pkg/client"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	croncore "hcm/pkg/cron/core"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"
	"hcm/pkg/tools/slice"
)

// BillSettleTask 定账任务：把已越过锁定时点账期的记录由 unsettled 置为 settled。
type BillSettleTask struct {
	clientSet *client.ClientSet
	sd        serviced.State
}

// NewBillSettleTask 创建定账任务实例。
func NewBillSettleTask(clientSet *client.ClientSet, sd serviced.State) (croncore.Task, error) {
	return &BillSettleTask{clientSet: clientSet, sd: sd}, nil
}

// Name 返回任务名。
func (t *BillSettleTask) Name() string {
	return string(enumor.CronTaskBillSettle)
}

// Next 返回下次执行时间，由 account_server.yaml 的 billSettle.intervalMinute 控制。
func (t *BillSettleTask) Next() (time.Time, error) {
	interval := cc.AccountServer().BillSettle.IntervalMinute

	return time.Now().Add(time.Duration(interval) * time.Minute), nil
}

// GetURL 返回手动触发接口的子路径。
func (t *BillSettleTask) GetURL() string {
	return "/bills/settle/run"
}

// Do 执行 cron 调度的定账。
func (t *BillSettleTask) Do(kt *kit.Kit) error {
	if t.sd == nil || !t.sd.IsMaster() {
		logs.V(5).Infof("current node is not master, skip bill settle task, rid: %s", kt.Rid)
		return nil
	}

	return t.RunOnce(kt)
}

// RunOnce 执行一次定账主流程。
func (t *BillSettleTask) RunOnce(kt *kit.Kit) error {
	settleCfg := cc.AccountServer().BillSettle
	periods := lockedPeriods(time.Now(), settleCfg.LockDay, settleCfg.LookbackMonth)
	if len(periods) == 0 {
		logs.Infof("no locked bill period to settle, rid: %s", kt.Rid)
		return nil
	}

	logs.Infof("start bill settle task, locked_period_num: %d, rid: %s", len(periods), kt.Rid)

	prepaidNum, err := t.settlePrepaidItems(kt, periods)
	if err != nil {
		return err
	}

	adjustmentNum, err := t.settleAdjustmentItems(kt, periods)
	if err != nil {
		return err
	}

	logs.Infof("bill settle task success, prepaid_num: %d, adjustment_num: %d, rid: %s",
		prepaidNum, adjustmentNum, kt.Rid)

	return nil
}

// settlePrepaidItems 按订单账期定账预付费主表，只捞 unsettled，settled 单向不可逆不重复更新。
func (t *BillSettleTask) settlePrepaidItems(kt *kit.Kit, periods []billPeriod) (int, error) {
	total := 0
	for _, period := range periods {
		ids, err := t.listUnsettledPrepaidIDs(kt, period)
		if err != nil {
			return total, err
		}
		if len(ids) == 0 {
			continue
		}

		for _, batch := range slice.Split(ids, constant.BatchOperationMaxLimit) {
			updateReq := &dsbill.PrepaidItemUpdateReq{IDs: batch, SettleState: enumor.BillSettleStateSettled}
			if err = t.clientSet.DataService().Global.Bill.UpdateBillPrepaidItem(kt, updateReq); err != nil {
				logs.Errorf("settle prepaid item failed, err: %v, period: %d-%02d, rid: %s",
					err, period.Year, period.Month, kt.Rid)
				return total, err
			}
			total += len(batch)
		}

		logs.Infof("settle prepaid item success, period: %d-%02d, num: %d, rid: %s",
			period.Year, period.Month, len(ids), kt.Rid)
	}

	return total, nil
}

// listUnsettledPrepaidIDs 分页拉取指定订单账期下所有未定账的预付费主单 ID。
func (t *BillSettleTask) listUnsettledPrepaidIDs(kt *kit.Kit, period billPeriod) ([]string, error) {
	ids := make([]string, 0)
	for start := uint32(0); ; start += uint32(core.DefaultMaxPageLimit) {
		listReq := &dsbill.PrepaidItemListReq{
			Filter: tools.ExpressionAnd(
				tools.RuleEqual("order_year", period.Year),
				tools.RuleEqual("order_month", period.Month),
				tools.RuleEqual("settle_state", enumor.BillSettleStateUnsettled),
			),
			Page:   &core.BasePage{Start: start, Limit: core.DefaultMaxPageLimit},
			Fields: []string{"id"},
		}
		resp, err := t.clientSet.DataService().Global.Bill.ListBillPrepaidItem(kt, listReq)
		if err != nil {
			logs.Errorf("list unsettled prepaid item failed, err: %v, period: %d-%02d, rid: %s",
				err, period.Year, period.Month, kt.Rid)
			return nil, err
		}
		for _, item := range resp.Details {
			ids = append(ids, item.ID)
		}
		if len(resp.Details) < int(core.DefaultMaxPageLimit) {
			break
		}
	}

	return ids, nil
}

// settleAdjustmentItems 按调账各自的账单账期定账，与主表判定相互独立。
func (t *BillSettleTask) settleAdjustmentItems(kt *kit.Kit, periods []billPeriod) (int, error) {
	total := 0
	for _, period := range periods {
		ids, err := t.listUnsettledAdjustmentIDs(kt, period)
		if err != nil {
			return total, err
		}
		if len(ids) == 0 {
			continue
		}

		for _, batch := range slice.Split(ids, constant.BatchOperationMaxLimit) {
			updateReq := &dsbill.BillAdjustmentItemStateUpdateReq{
				Filter:      tools.ContainersExpression("id", batch),
				SettleState: enumor.BillSettleStateSettled,
			}
			err = t.clientSet.DataService().Global.Bill.BatchUpdateBillAdjustmentItemState(kt, updateReq)
			if err != nil {
				logs.Errorf("settle adjustment item failed, err: %v, period: %d-%02d, rid: %s",
					err, period.Year, period.Month, kt.Rid)
				return total, err
			}
			total += len(batch)
		}

		logs.Infof("settle adjustment item success, period: %d-%02d, num: %d, rid: %s",
			period.Year, period.Month, len(ids), kt.Rid)
	}

	return total, nil
}

// listUnsettledAdjustmentIDs 分页拉取指定账单账期下所有未定账的调账 ID。
// 只捞 state=confirmed：定账即冻结，「账已定不可再改」的前提是这笔账已经确认过，
// 人工录入但仍待确认的调账不参与定账，否则会被定账守卫永久锁死而无法确认。
// 预付费派生调账落库即 confirmed，本条件不影响预付费链路。
func (t *BillSettleTask) listUnsettledAdjustmentIDs(kt *kit.Kit, period billPeriod) ([]string, error) {
	ids := make([]string, 0)
	for start := uint32(0); ; start += uint32(core.DefaultMaxPageLimit) {
		listReq := &dsbill.BillAdjustmentItemListReq{
			Filter: tools.ExpressionAnd(
				tools.RuleEqual("bill_year", period.Year),
				tools.RuleEqual("bill_month", period.Month),
				tools.RuleEqual("settle_state", enumor.BillSettleStateUnsettled),
				tools.RuleEqual("state", enumor.BillAdjustmentStateConfirmed),
			),
			Page:   &core.BasePage{Start: start, Limit: core.DefaultMaxPageLimit},
			Fields: []string{"id"},
		}
		resp, err := t.clientSet.DataService().Global.Bill.ListBillAdjustmentItem(kt, listReq)
		if err != nil {
			logs.Errorf("list unsettled adjustment item failed, err: %v, period: %d-%02d, rid: %s",
				err, period.Year, period.Month, kt.Rid)
			return nil, err
		}
		for _, item := range resp.Details {
			ids = append(ids, item.ID)
		}
		if len(resp.Details) < int(core.DefaultMaxPageLimit) {
			break
		}
	}

	return ids, nil
}
