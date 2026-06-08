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

package dispatcher

import (
	"fmt"
	"strings"

	"hcm/cmd/woa-server/logics/plan/demand-time"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

// autoApproveCheckResult 自动过单条件检查结果
type autoApproveCheckResult struct {
	// CanAutoApprove 是否可自动过单
	CanAutoApprove bool
	// Reason 原因说明，不满足条件时包含详细原因
	Reason string
	// TotalCPUCores 整单CPU核心数
	TotalCPUCores int64
	// TotalCBSSizeGB 整单CBS容量（单位：GB）
	TotalCBSSizeGB int64
}

// checkPredictionAutoApprove 检查预测单是否满足自动过单条件
// 前置条件：只有"追加"类型的需求单才允许自动过单
// 四个条件必须全部满足：
// 0. 不包含非今年的预测需求
// 1. 机型全部为标准型（DeviceFamily == "标准型"）；纯磁盘单（CVM 为空）跳过此校验
// 2. 整单CPU ≤ 1500 核
// 3. 整单CBS ≤ 45TB（46080GB）
func checkPredictionAutoApprove(kt *kit.Kit, demands rpt.ResPlanDemands) *autoApproveCheckResult {
	result := &autoApproveCheckResult{CanAutoApprove: true}
	var reasons []string

	// 条件0：检查是否包含非今年的预测需求
	if demandtime.ContainsNonCurrentYearDemand(demands) {
		result.CanAutoApprove = false
		reasons = append(reasons, "包含非今年的预测需求")
	}

	nonStandardFamilies := make(map[string]struct{})

	// 遍历需求，检查需求类型和机型，统计资源
	for _, demand := range demands {
		var demandReasons []string
		result, demandReasons = processDemand(demand, result, nonStandardFamilies)
		reasons = append(reasons, demandReasons...)
		// 已确定不能自动过单，无需继续遍历
		if !result.CanAutoApprove {
			break
		}
	}

	// 条件2：检查CPU核心数是否超出阈值
	if result.TotalCPUCores > constant.AutoApproveCPUCoreThreshold {
		result.CanAutoApprove = false
		reasons = append(reasons, fmt.Sprintf("CPU核心数超出阈值: %d > %d",
			result.TotalCPUCores, constant.AutoApproveCPUCoreThreshold))
	}
	// 条件3：检查CBS容量是否超出阈值
	if result.TotalCBSSizeGB > constant.AutoApproveCBSSizeThreshold {
		result.CanAutoApprove = false
		reasons = append(reasons, fmt.Sprintf("CBS容量超出阈值: %dGB > %dGB",
			result.TotalCBSSizeGB, constant.AutoApproveCBSSizeThreshold))
	}

	// 组装原因说明
	if len(reasons) > 0 {
		result.Reason = strings.Join(reasons, "; ")
	} else if result.CanAutoApprove {
		result.Reason = "满足自动过单条件"
	}

	logs.Infof("auto approve check result: can_auto_approve=%v, reason=%s, cpu=%d, cbs=%dGB, rid: %s",
		result.CanAutoApprove, result.Reason, result.TotalCPUCores, result.TotalCBSSizeGB, kt.Rid)

	return result
}

// processDemand 处理单个需求，返回更新后的结果和原因列表
func processDemand(demand rpt.ResPlanDemand, result *autoApproveCheckResult,
	nonStandardFamilies map[string]struct{}) (*autoApproveCheckResult, []string) {

	// 检查需求类型：只有"追加"类型才允许自动过单
	if demand.Original != nil {
		result.CanAutoApprove = false
		if demand.Updated == nil {
			return result, []string{"包含删除类型需求"}
		}
		return result, []string{"包含变更类型需求"}
	}

	if demand.Updated == nil {
		return result, nil
	}

	// 统计资源
	result.TotalCPUCores += demand.Updated.Cvm.CpuCore
	result.TotalCBSSizeGB += demand.Updated.Cbs.DiskSize

	// 纯磁盘单（CVM 为空）跳过机型校验
	if demand.Updated.Cvm.IsEmpty() {
		return result, nil
	}

	// 条件1：检查机型是否为标准型
	var reasons []string
	family := demand.Updated.Cvm.DeviceFamily
	if family != string(enumor.DeviceFamilyStandard) {
		if _, exists := nonStandardFamilies[family]; !exists {
			nonStandardFamilies[family] = struct{}{}
			result.CanAutoApprove = false
			if family == "" {
				reasons = append(reasons, "包含未指定机型族的需求")
			} else {
				reasons = append(reasons, fmt.Sprintf("包含非标准机型族: %s", family))
			}
		}
	}

	return result, reasons
}
