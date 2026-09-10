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

package task

import (
	"fmt"
	"time"

	"hcm/cmd/woa-server/logics/plan"
	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/cc"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	croncore "hcm/pkg/cron/core"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/serviced"
)

// RecycleReturnResPlanReportTask is the task for recycle return res plan report.
type RecycleReturnResPlanReportTask struct {
	planLogics plan.Logics
	sd         serviced.State
	reportCfg  cc.ResPlanRecycleReturnResPlanReport
}

// NewRecycleReturnResPlanReportTask create a new recycle return res plan report task.
func NewRecycleReturnResPlanReportTask(planLogics plan.Logics, sd serviced.State,
	reportCfg cc.ResPlanRecycleReturnResPlanReport) (croncore.Task, error) {
	return &RecycleReturnResPlanReportTask{
		planLogics: planLogics,
		sd:         sd,
		reportCfg:  reportCfg,
	}, nil
}

// Name return the name of the task.
func (d *RecycleReturnResPlanReportTask) Name() string {
	return string(enumor.CronTaskRecycleReturnResPlanReport)
}

// Next return the next time to run the task. The task runs at 8:00 on every Monday.
func (d *RecycleReturnResPlanReportTask) Next() (time.Time, error) {
	loc, err := time.LoadLocation(cc.WoaServer().LocalTimezone)
	if err != nil {
		logs.Warnf("load timezone failed: %v, use UTC", err)
		loc = time.UTC
	}

	now := time.Now().In(loc)
	next := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, loc)
	// 顺延到下一个尚未到达的周一 08:00
	for next.Weekday() != time.Monday || !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next, nil
}

// Do execute the task with default last week range.
func (d *RecycleReturnResPlanReportTask) Do(kt *kit.Kit) error {
	if !d.reportCfg.Enable {
		logs.V(5).Infof("recycle return res plan report is disabled, skip, rid: %s", kt.Rid)
		return nil
	}

	// 仅 master 节点执行，避免多节点重复发送
	if d.sd == nil || !d.sd.IsMaster() {
		logs.V(5).Infof("current node is not master, skip recycle return res plan report, rid: %s", kt.Rid)
		return nil
	}

	return d.run(kt, "", "")
}

// DoWithRange execute the task with optional custom start/end range.
func (d *RecycleReturnResPlanReportTask) DoWithRange(kt *kit.Kit, startStr, endStr string) (
	*ptypes.PushRecycleReturnResPlanReportResp, error) {

	return d.planLogics.PushRecycleReturnResPlanReport(kt, startStr, endStr)
}

func (d *RecycleReturnResPlanReportTask) run(kt *kit.Kit, startStr, endStr string) error {
	// start/end 传空默认取上周一 00:00:00 ~ 上周日 23:59:59
	resp, err := d.DoWithRange(kt, startStr, endStr)
	if err != nil {
		logs.Errorf("%s: run recycle return res plan report failed, err: %v, rid: %s",
			constant.RecycleReturnResPlanReportFailed, err, kt.Rid)
		return err
	}
	if !resp.Success {
		logs.Errorf("%s: run recycle return res plan report failed, msg: %s, rid: %s",
			constant.RecycleReturnResPlanReportFailed, resp.Message, kt.Rid)
		return fmt.Errorf("run recycle return res plan report failed: %s", resp.Message)
	}

	return nil
}

// GetURL get the url of the task, require every task to have external api in service.
func (d *RecycleReturnResPlanReportTask) GetURL() string {
	return "/plans/resources/recycle_return_res_plan_reports/push"
}
