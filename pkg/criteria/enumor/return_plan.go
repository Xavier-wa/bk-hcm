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

package enumor

import "fmt"

// ReturnPlanTicketType is return plan ticket type.
type ReturnPlanTicketType string

const (
	// ReturnPlanTicketTypeAdd 新增
	ReturnPlanTicketTypeAdd ReturnPlanTicketType = "add"
	// ReturnPlanTicketTypeAdjust 调整
	ReturnPlanTicketTypeAdjust ReturnPlanTicketType = "adjust"
	// ReturnPlanTicketTypeCancel 取消
	ReturnPlanTicketTypeCancel ReturnPlanTicketType = "cancel"
)

// Validate ReturnPlanTicketType.
func (t ReturnPlanTicketType) Validate() error {
	switch t {
	case ReturnPlanTicketTypeAdd, ReturnPlanTicketTypeAdjust, ReturnPlanTicketTypeCancel:
	default:
		return fmt.Errorf("unsupported return plan ticket type: %s", t)
	}
	return nil
}

// returnPlanTicketTypeNameMap records ReturnPlanTicketType's name.
var returnPlanTicketTypeNameMap = map[ReturnPlanTicketType]string{
	ReturnPlanTicketTypeAdd:    "新增",
	ReturnPlanTicketTypeAdjust: "调整",
	ReturnPlanTicketTypeCancel: "取消",
}

// Name return ReturnPlanTicketType's name.
func (t ReturnPlanTicketType) Name() string {
	return returnPlanTicketTypeNameMap[t]
}

// GetReturnPlanTicketTypeMembers get ReturnPlanTicketType's members for root tickets.
func GetReturnPlanTicketTypeMembers() []ReturnPlanTicketType {
	return []ReturnPlanTicketType{
		ReturnPlanTicketTypeAdd,
		ReturnPlanTicketTypeAdjust,
		ReturnPlanTicketTypeCancel,
	}
}

// ReturnPlanTicketStatus is return plan ticket status.
type ReturnPlanTicketStatus string

const (
	// ReturnPlanTicketStatusInit 待处理
	ReturnPlanTicketStatusInit ReturnPlanTicketStatus = "init"
	// ReturnPlanTicketStatusAuditing 处理中
	ReturnPlanTicketStatusAuditing ReturnPlanTicketStatus = "auditing"
	// ReturnPlanTicketStatusRejected 审批驳回（CRP 审批流全部驳回）
	ReturnPlanTicketStatusRejected ReturnPlanTicketStatus = "rejected"
	// ReturnPlanTicketStatusPartialRejected 部分驳回（CRP 审批流部分子单驳回）
	ReturnPlanTicketStatusPartialRejected ReturnPlanTicketStatus = "partial_rejected"
	// ReturnPlanTicketStatusRevoked 已撤销（CRP 审批流撤销）
	ReturnPlanTicketStatusRevoked ReturnPlanTicketStatus = "revoked"
	// ReturnPlanTicketStatusDone 成功
	ReturnPlanTicketStatusDone ReturnPlanTicketStatus = "done"
	// ReturnPlanTicketStatusFailed 全部失败
	ReturnPlanTicketStatusFailed ReturnPlanTicketStatus = "failed"
	// ReturnPlanTicketStatusPartialFailed 部分失败
	ReturnPlanTicketStatusPartialFailed ReturnPlanTicketStatus = "partial_failed"
	// ReturnPlanTicketStatusTerminated 已终止
	ReturnPlanTicketStatusTerminated ReturnPlanTicketStatus = "terminated"
)

// Validate ReturnPlanTicketStatus.
func (s ReturnPlanTicketStatus) Validate() error {
	switch s {
	case ReturnPlanTicketStatusInit:
	case ReturnPlanTicketStatusAuditing:
	case ReturnPlanTicketStatusRejected:
	case ReturnPlanTicketStatusPartialRejected:
	case ReturnPlanTicketStatusRevoked:
	case ReturnPlanTicketStatusDone:
	case ReturnPlanTicketStatusFailed:
	case ReturnPlanTicketStatusPartialFailed:
	case ReturnPlanTicketStatusTerminated:
	default:
		return fmt.Errorf("unsupported return plan ticket status: %s", s)
	}
	return nil
}

// IsUnfinished returns whether the ticket status is unfinished.
func (s ReturnPlanTicketStatus) IsUnfinished() bool {
	switch s {
	case ReturnPlanTicketStatusInit:
	case ReturnPlanTicketStatusAuditing:
	default:
		return false
	}
	return true
}

// CanTerminate returns true if the ticket is in a failed/rejected state that can be terminated.
// Auditing/init are unfinished states and cannot be terminated.
func (s ReturnPlanTicketStatus) CanTerminate() bool {
	switch s {
	case ReturnPlanTicketStatusRejected:
	case ReturnPlanTicketStatusPartialRejected:
	case ReturnPlanTicketStatusFailed:
	case ReturnPlanTicketStatusPartialFailed:
	default:
		return false
	}
	return true
}

// returnPlanTicketStatusNameMap records ReturnPlanTicketStatus's name.
var returnPlanTicketStatusNameMap = map[ReturnPlanTicketStatus]string{
	ReturnPlanTicketStatusInit:            "待处理",
	ReturnPlanTicketStatusAuditing:        "处理中",
	ReturnPlanTicketStatusRejected:        "审批驳回",
	ReturnPlanTicketStatusPartialRejected: "部分驳回",
	ReturnPlanTicketStatusRevoked:         "已撤销",
	ReturnPlanTicketStatusDone:            "成功",
	ReturnPlanTicketStatusFailed:          "失败",
	ReturnPlanTicketStatusPartialFailed:   "部分失败",
	ReturnPlanTicketStatusTerminated:      "已终止",
}

// Name return ReturnPlanTicketStatus's name.
func (s ReturnPlanTicketStatus) Name() string {
	return returnPlanTicketStatusNameMap[s]
}

// GetReturnPlanTicketStatusMembers get ReturnPlanTicketStatus's members.
func GetReturnPlanTicketStatusMembers() []ReturnPlanTicketStatus {
	return []ReturnPlanTicketStatus{
		ReturnPlanTicketStatusInit,
		ReturnPlanTicketStatusAuditing,
		ReturnPlanTicketStatusRejected,
		ReturnPlanTicketStatusPartialRejected,
		ReturnPlanTicketStatusRevoked,
		ReturnPlanTicketStatusDone,
		ReturnPlanTicketStatusFailed,
		ReturnPlanTicketStatusPartialFailed,
		ReturnPlanTicketStatusTerminated,
	}
}

// ReturnPlanSubTicketStatus is return plan sub ticket status.
type ReturnPlanSubTicketStatus string

const (
	// ReturnPlanSubTicketStatusInit 待处理
	ReturnPlanSubTicketStatusInit ReturnPlanSubTicketStatus = "init"
	// ReturnPlanSubTicketStatusAuditing 处理中
	ReturnPlanSubTicketStatusAuditing ReturnPlanSubTicketStatus = "auditing"
	// ReturnPlanSubTicketStatusRejected 审批驳回（CRP 审批流驳回）
	ReturnPlanSubTicketStatusRejected ReturnPlanSubTicketStatus = "rejected"
	// ReturnPlanSubTicketStatusRevoked 已撤销（CRP 审批流撤销）
	ReturnPlanSubTicketStatusRevoked ReturnPlanSubTicketStatus = "revoked"
	// ReturnPlanSubTicketStatusInvalid 已失效
	ReturnPlanSubTicketStatusInvalid ReturnPlanSubTicketStatus = "invalid"
	// ReturnPlanSubTicketStatusDone 成功
	ReturnPlanSubTicketStatusDone ReturnPlanSubTicketStatus = "done"
	// ReturnPlanSubTicketStatusFailed 失败
	ReturnPlanSubTicketStatusFailed ReturnPlanSubTicketStatus = "failed"
	// ReturnPlanSubTicketStatusTerminated 已终止
	ReturnPlanSubTicketStatusTerminated ReturnPlanSubTicketStatus = "terminated"
)

// Validate ReturnPlanSubTicketStatus.
func (s ReturnPlanSubTicketStatus) Validate() error {
	switch s {
	case ReturnPlanSubTicketStatusInit:
	case ReturnPlanSubTicketStatusAuditing:
	case ReturnPlanSubTicketStatusRejected:
	case ReturnPlanSubTicketStatusRevoked:
	case ReturnPlanSubTicketStatusInvalid:
	case ReturnPlanSubTicketStatusDone:
	case ReturnPlanSubTicketStatusFailed:
	case ReturnPlanSubTicketStatusTerminated:
	default:
		return fmt.Errorf("unsupported return plan sub ticket status: %s", s)
	}
	return nil
}

// IsUnfinished return true if ReturnPlanSubTicketStatus is unfinished.
func (s ReturnPlanSubTicketStatus) IsUnfinished() bool {
	switch s {
	case ReturnPlanSubTicketStatusInit:
	case ReturnPlanSubTicketStatusAuditing:
	default:
		return false
	}
	return true
}

// returnPlanSubTicketStatusNameMap records ReturnPlanSubTicketStatus's name.
var returnPlanSubTicketStatusNameMap = map[ReturnPlanSubTicketStatus]string{
	ReturnPlanSubTicketStatusInit:       "待处理",
	ReturnPlanSubTicketStatusAuditing:   "处理中",
	ReturnPlanSubTicketStatusRejected:   "审批驳回",
	ReturnPlanSubTicketStatusRevoked:    "已撤销",
	ReturnPlanSubTicketStatusInvalid:    "已失效",
	ReturnPlanSubTicketStatusDone:       "成功",
	ReturnPlanSubTicketStatusFailed:     "失败",
	ReturnPlanSubTicketStatusTerminated: "已终止",
}

// Name return ReturnPlanSubTicketStatus's name.
func (s ReturnPlanSubTicketStatus) Name() string {
	return returnPlanSubTicketStatusNameMap[s]
}

// GetReturnPlanSubTicketStatusMembers get ReturnPlanSubTicketStatus's members.
func GetReturnPlanSubTicketStatusMembers() []ReturnPlanSubTicketStatus {
	return []ReturnPlanSubTicketStatus{
		ReturnPlanSubTicketStatusInit,
		ReturnPlanSubTicketStatusAuditing,
		ReturnPlanSubTicketStatusRejected,
		ReturnPlanSubTicketStatusRevoked,
		ReturnPlanSubTicketStatusInvalid,
		ReturnPlanSubTicketStatusDone,
		ReturnPlanSubTicketStatusFailed,
		ReturnPlanSubTicketStatusTerminated,
	}
}

// ReturnPlanOrderStatus is CRP return plan order status from queryOrderDetail.
// 与资源预测 PlanOrderStatus 语义不同，禁止复用。
type ReturnPlanOrderStatus int

const (
	// ReturnPlanOrderStatusDraft 草稿，待提交
	ReturnPlanOrderStatusDraft ReturnPlanOrderStatus = 0
	// ReturnPlanOrderStatusResTeamApprove 资源团队审批
	ReturnPlanOrderStatusResTeamApprove ReturnPlanOrderStatus = 1
	// ReturnPlanOrderStatusDeptAdminApprove 部门管理员审批
	ReturnPlanOrderStatusDeptAdminApprove ReturnPlanOrderStatus = 2
	// ReturnPlanOrderStatusFinished 审批结束（成功终态）
	ReturnPlanOrderStatusFinished ReturnPlanOrderStatus = 3
	// ReturnPlanOrderStatusRejected 审批驳回
	ReturnPlanOrderStatusRejected ReturnPlanOrderStatus = 4
)

// Validate ReturnPlanOrderStatus.
func (s ReturnPlanOrderStatus) Validate() error {
	switch s {
	case ReturnPlanOrderStatusDraft, ReturnPlanOrderStatusResTeamApprove,
		ReturnPlanOrderStatusDeptAdminApprove, ReturnPlanOrderStatusFinished,
		ReturnPlanOrderStatusRejected:
	default:
		return fmt.Errorf("unsupported return plan order status: %d", s)
	}
	return nil
}

// returnPlanOrderStatusNameMap records ReturnPlanOrderStatus's name.
var returnPlanOrderStatusNameMap = map[ReturnPlanOrderStatus]string{
	ReturnPlanOrderStatusDraft:            "待提交",
	ReturnPlanOrderStatusResTeamApprove:   "资源团队审批",
	ReturnPlanOrderStatusDeptAdminApprove: "部门管理员审批",
	ReturnPlanOrderStatusFinished:         "审批结束",
	ReturnPlanOrderStatusRejected:         "审批驳回",
}

// Name return ReturnPlanOrderStatus's name.
func (s ReturnPlanOrderStatus) Name() string {
	if name, ok := returnPlanOrderStatusNameMap[s]; ok {
		return name
	}
	return fmt.Sprintf("unknown(%d)", s)
}

// GetReturnPlanOrderStatusMembers get ReturnPlanOrderStatus's members.
func GetReturnPlanOrderStatusMembers() []ReturnPlanOrderStatus {
	return []ReturnPlanOrderStatus{
		ReturnPlanOrderStatusDraft,
		ReturnPlanOrderStatusResTeamApprove,
		ReturnPlanOrderStatusDeptAdminApprove,
		ReturnPlanOrderStatusFinished,
		ReturnPlanOrderStatusRejected,
	}
}
