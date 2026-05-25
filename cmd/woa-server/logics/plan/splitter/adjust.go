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

package splitter

import (
	"time"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"
)

// SplitAdjustTicket split res plan adjust ticket to sub ticket.
// The virtualDeptID parameter is forwarded to prepareAddSubTickets to gate transfer pool logic.
func (s *SubTicketSplitter) SplitAdjustTicket(kt *kit.Kit, ticketID string, virtualDeptID int64,
	demands rpt.ResPlanDemands, planProductName, opProductName string) error {

	// 1. 无需考虑转移的预测，单独创建子单
	// 包含非跨年延期类调整、关键属性未产生变化（技术分类、项目类型、总核数）的调整
	remainDemands := s.getDemandsWithoutTransfer(kt, demands)

	// 2. 将剩余需求拆分为调减和调增两部分
	addDemands, delDemands := s.splitAdjustDemandsToAddAndDelete(remainDemands)

	// 3. 调减逻辑
	err := s.prepareDeleteSubTickets(kt, ticketID, virtualDeptID, enumor.RPTicketTypeDelete, delDemands,
		planProductName, opProductName)
	if err != nil {
		logs.Errorf("failed to prepare delete sub tickets, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	// 4. 调增逻辑, cvmAddDemands 为排除纯 CBS 需求后剩余的 CVM 追加需求
	canTransfer, cvmAddDemands, err := s.prepareAddSubTickets(kt, ticketID, virtualDeptID, addDemands)
	if err != nil {
		logs.Errorf("failed to prepare add sub tickets, err: %v, rid: %s", err, kt.Rid)
		return err
	}
	// 4.1. 如果追加发现无可转移预测，直接将所有调增需求合并为一个追加子单
	if !canTransfer {
		s.adjSplitGroupDemands[enumor.RPTicketTypeAdd] = append(s.adjSplitGroupDemands[enumor.RPTicketTypeAdd],
			cvt.SliceToPtr(cvmAddDemands)...)
	}

	// 5. 将所有 adjSplitGroupDemands 中的非转移、延期需求全部合并到adjust子单
	delGroup := make([]enumor.RPTicketType, 0)
	adjustItems := make([]*rpt.ResPlanDemand, 0)
	for groupK, items := range s.adjSplitGroupDemands {
		if groupK.CanMerged() {
			adjustItems = append(adjustItems, items...)
			delGroup = append(delGroup, groupK)
		}
	}
	s.adjSplitGroupDemands[enumor.RPTicketTypeAdjust] = append(s.adjSplitGroupDemands[enumor.RPTicketTypeAdjust],
		adjustItems...)
	for _, delGroupK := range delGroup {
		delete(s.adjSplitGroupDemands, delGroupK)
	}

	// 6. 对所有拆分后的变更需求，整理并创建出N个子单
	err = s.createSubTicket(kt, ticketID, demands, enumor.RPTicketTypeAdjust)
	if err != nil {
		logs.Errorf("failed to create sub ticket, err: %v, rid: %s", err, kt.Rid)
		return err
	}

	return nil
}

// getDemandsWithoutTransfer 从预测需求中拆分出无需考虑转移的需求，以 delay 类型存入 adjSplitGroupDemands 备用
// 并将剩余的需要考虑转移的需求返回继续常规拆分。
// 注意：跨年延期需求（原始和更新后的期望交付时间属于不同需求年）不放入延期组，
// 而是返回继续走调减/调增拆分逻辑，以确保额度计算正确。
func (s *SubTicketSplitter) getDemandsWithoutTransfer(kt *kit.Kit, allDemands rpt.ResPlanDemands) rpt.ResPlanDemands {

	remainDemands := make([]rpt.ResPlanDemand, 0, len(allDemands))
	for _, demand := range allDemands {
		// 非调整类型需求，排除
		if demand.Original == nil || demand.Updated == nil {
			remainDemands = append(remainDemands, demand)
			continue
		}

		// 调整类型需求中的延期需求
		if demand.Updated.ExpectTime != demand.Original.ExpectTime {
			// 判断是否为跨年延期：使用 GetDemandYearMonth 获取需求年进行对比
			originalTime, err1 := time.Parse(constant.DateLayout, demand.Original.ExpectTime)
			updatedTime, err2 := time.Parse(constant.DateLayout, demand.Updated.ExpectTime)
			if err1 != nil || err2 != nil {
				logs.Warnf("failed to parse demand time, demand_id: %s, original: %s, updated: %s, "+
					"err1: %v, err2: %v, rid: %s", demand.Original.DemandID,
					demand.Original.ExpectTime, demand.Updated.ExpectTime, err1, err2, kt.Rid)
				remainDemands = append(remainDemands, demand)
				continue
			}

			originalYear, _, err1 := s.demandTime.GetDemandYearMonth(kt, originalTime)
			updatedYear, _, err2 := s.demandTime.GetDemandYearMonth(kt, updatedTime)
			if err1 != nil || err2 != nil {
				logs.Warnf("failed to get demand year month, demand_id: %s, original: %s, updated: %s, "+
					"err1: %v, err2: %v, rid: %s", demand.Original.DemandID,
					demand.Original.ExpectTime, demand.Updated.ExpectTime, err1, err2, kt.Rid)
				remainDemands = append(remainDemands, demand)
				continue
			}

			// 跨年延期，不放入延期组，走普通调整逻辑
			if originalYear != updatedYear {
				remainDemands = append(remainDemands, demand)
				continue
			}

			// 非跨年延期，放入延期组
			delayDemand := demand.Clone()
			s.adjSplitGroupDemands[enumor.RPTicketTypeDelay] = append(s.adjSplitGroupDemands[enumor.RPTicketTypeDelay],
				delayDemand)
			continue
		}

		// 关键属性未产生变化（技术分类、项目类型、总核数）的调整
		if demand.Updated.Cvm.TechnicalClass == demand.Original.Cvm.TechnicalClass &&
			demand.Updated.ObsProject == demand.Original.ObsProject &&
			demand.Updated.Cvm.CpuCore == demand.Original.Cvm.CpuCore {
			delayDemand := demand.Clone()
			s.adjSplitGroupDemands[enumor.RPTicketTypeDelay] = append(s.adjSplitGroupDemands[enumor.RPTicketTypeDelay],
				delayDemand)
			continue
		}

		// 剩余调整类型需求
		remainDemands = append(remainDemands, demand)
	}

	return remainDemands
}

// splitAdjustDemandsToAddAndDelete 将调整类需求拆分为新增和删除两部分
func (s *SubTicketSplitter) splitAdjustDemandsToAddAndDelete(demands rpt.ResPlanDemands) (
	addDemands rpt.ResPlanDemands, delDemands rpt.ResPlanDemands) {

	addDemands = make([]rpt.ResPlanDemand, 0, len(demands))
	delDemands = make([]rpt.ResPlanDemand, 0, len(demands))
	for _, demand := range demands {
		if demand.Original == nil {
			addDemands = append(addDemands, demand)
			continue
		}

		if demand.Updated == nil {
			delDemands = append(delDemands, demand)
			continue
		}

		// 分别clone删除和追加的部分
		delDemands = append(delDemands, cvt.PtrToVal(demand.CloneOriginal()))
		addDemands = append(addDemands, cvt.PtrToVal(demand.CloneUpdated()))
	}

	return
}
