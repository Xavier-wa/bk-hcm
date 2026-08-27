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

package billprepaid

import (
	billcore "hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"

	"github.com/shopspring/decimal"
)

// prepaidAdjustmentStat 单个预付费主单的调账聚合结果，用于派生核算状态与累计核算金额。
// 分子分母都只统计 type=increase 的条目，调减条目不计入。
type prepaidAdjustmentStat struct {
	// IncreaseNum 调增条目总数，即核算派生的分母
	IncreaseNum int
	// PushedIncreaseNum 已推送的调增条目数，即核算派生的分子
	PushedIncreaseNum int
	// AccountedCost 已推送调增条目的分摊金额之和
	AccountedCost decimal.Decimal
	// AccountedRMBCost 已推送调增条目分摊金额之和的人民币值
	AccountedRMBCost decimal.Decimal
}

// statAdjustmentFields 聚合口径实际读到的列，与 statAdjustmentBySourceID 用到的字段一一对应。
// 改动 statAdjustmentBySourceID 的读取字段时必须同步这里，否则新读的字段会恒为零值。
var statAdjustmentFields = []string{"source_id", "type", "push_status", "cost", "rmb_cost"}

// statPrepaidAdjustment 按主单 ID 聚合调账条目，得到各主单的核算派生输入。
func (b *billPrepaidSvc) statPrepaidAdjustment(kt *kit.Kit, sourceIDs []string) (
	map[string]*prepaidAdjustmentStat, error) {

	items, err := b.listPrepaidAdjustment(kt, sourceIDs, statAdjustmentFields...)
	if err != nil {
		return nil, err
	}

	return statAdjustmentBySourceID(items), nil
}

// statAdjustmentBySourceID 按 source_id 聚合调账条目，只统计调增条目。
func statAdjustmentBySourceID(items []*billcore.AdjustmentItem) map[string]*prepaidAdjustmentStat {
	statMap := make(map[string]*prepaidAdjustmentStat, len(items))
	for _, item := range items {
		if item.Type != enumor.BillAdjustmentIncrease {
			continue
		}

		stat, ok := statMap[item.SourceID]
		if !ok {
			stat = &prepaidAdjustmentStat{AccountedCost: decimal.Zero, AccountedRMBCost: decimal.Zero}
			statMap[item.SourceID] = stat
		}

		stat.IncreaseNum++
		if item.PushStatus != enumor.BillAdjustmentPushStatusPushed {
			continue
		}
		stat.PushedIncreaseNum++
		stat.AccountedCost = stat.AccountedCost.Add(item.Cost)
		stat.AccountedRMBCost = stat.AccountedRMBCost.Add(item.RMBCost)
	}

	return statMap
}
