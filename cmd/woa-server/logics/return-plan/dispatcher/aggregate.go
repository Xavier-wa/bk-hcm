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

	"hcm/pkg/criteria/enumor"
	tablerst "hcm/pkg/dal/table/return-plan/return-plan-sub-ticket"
)

// determineTicketStatus 由子单终态构成推导主单状态（调用前须确保子单均已终结）。
// 已失效(invalid)子单为重试时被取代的历史记录，不参与状态推导。
// 失败(failed/terminated)优先级高于驳回(rejected/revoked)：
// 全 done→done / 全失败→failed / 全驳回→rejected / 含失败→partial_failed / 含驳回→partial_rejected。
func determineTicketStatus(subTickets []tablerst.ReturnPlanSubTicketTable) enumor.ReturnPlanTicketStatus {
	var total, done, failed, rejected int
	for i := range subTickets {
		switch subTickets[i].Status {
		case enumor.ReturnPlanSubTicketStatusDone:
			done++
			total++
		case enumor.ReturnPlanSubTicketStatusFailed, enumor.ReturnPlanSubTicketStatusTerminated:
			failed++
			total++
		case enumor.ReturnPlanSubTicketStatusRejected, enumor.ReturnPlanSubTicketStatusRevoked:
			rejected++
			total++
		}
	}

	if total == 0 {
		return enumor.ReturnPlanTicketStatusAuditing
	}

	switch {
	case done == total:
		return enumor.ReturnPlanTicketStatusDone
	case failed == total:
		return enumor.ReturnPlanTicketStatusFailed
	case rejected == total:
		return enumor.ReturnPlanTicketStatusRejected
	case failed > 0:
		return enumor.ReturnPlanTicketStatusPartialFailed
	case rejected > 0:
		return enumor.ReturnPlanTicketStatusPartialRejected
	default:
		return enumor.ReturnPlanTicketStatusAuditing
	}
}

// aggregateFailMessage 收集未成功子单的 message，按 [子单ID/资源池] 原因 逐行拼接，超长截断。
func aggregateFailMessage(subTickets []tablerst.ReturnPlanSubTicketTable) string {
	lines := make([]string, 0, len(subTickets))
	for i := range subTickets {
		s := subTickets[i]
		// 成功与已失效(重试被取代)的子单不计入失败原因
		if s.Status == enumor.ReturnPlanSubTicketStatusDone ||
			s.Status == enumor.ReturnPlanSubTicketStatusInvalid || s.Message == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("[%s/%s] %s", s.ID, s.ResPoolName, s.Message))
	}
	return truncateMessage(strings.Join(lines, "\n"), maxTicketMessageLen)
}

// truncateMessage 按 rune 截断到 max 长度，避免截断多字节字符。
func truncateMessage(msg string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(msg)
	if len(runes) <= max {
		return msg
	}
	return string(runes[:max])
}
