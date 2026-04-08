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

// TicketStage 资源申请工单阶段
type TicketStage string

const (
	// TicketStageUncommit 未提交
	TicketStageUncommit TicketStage = "UNCOMMIT"
	// TicketStageAudit 待审核
	TicketStageAudit TicketStage = "AUDIT"
	// TicketStageTerminate 终止
	TicketStageTerminate TicketStage = "TERMINATE"
	// TicketStageRunning 备货中
	TicketStageRunning TicketStage = "RUNNING"
	// TicketStageSuspend 备货异常
	TicketStageSuspend TicketStage = "SUSPEND"
	// TicketStageDone 完成
	TicketStageDone TicketStage = "DONE"
	// TicketStageConfirming 修改需求后-待用户确认
	TicketStageConfirming TicketStage = "CONFIRMING"
)

// Validate TicketStage.
func (t TicketStage) Validate() error {
	switch t {
	case TicketStageUncommit, TicketStageAudit, TicketStageTerminate,
		TicketStageRunning, TicketStageSuspend, TicketStageDone, TicketStageConfirming:
	default:
		return fmt.Errorf("unsupported ticket stage: %s", t)
	}

	return nil
}

// ApplyStatus 申请状态
type ApplyStatus string

const (
	// ApplyStatusWaitForMatch 待匹配（初始状态）
	ApplyStatusWaitForMatch ApplyStatus = "WAIT"
	// ApplyStatusMatching 匹配执行中
	ApplyStatusMatching ApplyStatus = "MATCHING"
	// ApplyStatusMatchedSome 已完成部分资源匹配
	ApplyStatusMatchedSome ApplyStatus = "MATCHED_SOME"
	// ApplyStatusPaused 已暂停
	ApplyStatusPaused ApplyStatus = "PAUSED"
	// ApplyStatusDone 已完成
	ApplyStatusDone ApplyStatus = "DONE"
	// ApplyStatusTerminate 终止
	ApplyStatusTerminate ApplyStatus = "TERMINATE"
	// ApplyStatusGracefulTerminate 比起 ApplyStatusTerminate，将不再发起重试，但是后续的流程仍会继续流转
	ApplyStatusGracefulTerminate ApplyStatus = "GRACEFUL_TERMINATE"
	// ApplyStatusConfirming 修改需求后-待用户确认
	ApplyStatusConfirming ApplyStatus = "CONFIRMING"
)

// Validate ApplyStatus.
func (a ApplyStatus) Validate() error {
	switch a {
	case ApplyStatusWaitForMatch, ApplyStatusMatching, ApplyStatusMatchedSome,
		ApplyStatusPaused, ApplyStatusDone, ApplyStatusTerminate,
		ApplyStatusGracefulTerminate, ApplyStatusConfirming:
	default:
		return fmt.Errorf("unsupported apply status: %s", a)
	}

	return nil
}

// ResourceType 资源类型
type ResourceType string

// ResourceType 资源类型
const (
	ResourceTypePm          ResourceType = "IDCPM"
	ResourceTypeCvm         ResourceType = "QCLOUDCVM"
	ResourceTypeIdcDvm      ResourceType = "IDCDVM"
	ResourceTypeQcloudDvm   ResourceType = "QCLOUDDVM"
	ResourceTypePool        ResourceType = "POOL"
	ResourceTypeOthers      ResourceType = "OTHERS"
	ResourceTypeUnsupported ResourceType = "UNSUPPORTED"
	// ResourceTypeUpgradeCvm cvm升降配
	ResourceTypeUpgradeCvm ResourceType = "UPGRADECVM"
)

// Validate ResourceType.
func (r ResourceType) Validate() error {
	switch r {
	case ResourceTypePm, ResourceTypeCvm, ResourceTypeIdcDvm, ResourceTypeQcloudDvm,
		ResourceTypePool, ResourceTypeOthers, ResourceTypeUnsupported, ResourceTypeUpgradeCvm:
	default:
		return fmt.Errorf("unsupported resource type: %s", r)
	}

	return nil
}

// StepStatusType 步骤状态
type StepStatusType int

// StepStatusType 步骤状态
const (
	StepStatusInit     StepStatusType = -1
	StepStatusSuccess  StepStatusType = 0
	StepStatusHandling StepStatusType = 1
	StepStatusFailed   StepStatusType = 2
)

// Validate StepStatusType.
func (s StepStatusType) Validate() error {
	switch s {
	case StepStatusInit, StepStatusSuccess, StepStatusHandling, StepStatusFailed:
	default:
		return fmt.Errorf("unsupported step status type: %d", s)
	}

	return nil
}

// InitStepStatus 初始化步骤状态
type InitStepStatus int

// InitStepStatus 初始化步骤状态
const (
	InitStatusInit     InitStepStatus = -1
	InitStatusSuccess  InitStepStatus = 0
	InitStatusHandling InitStepStatus = 1
	InitStatusFailed   InitStepStatus = 2
)

// Validate InitStepStatus.
func (i InitStepStatus) Validate() error {
	switch i {
	case InitStatusInit, InitStatusSuccess, InitStatusHandling, InitStatusFailed:
	default:
		return fmt.Errorf("unsupported init step status: %d", i)
	}

	return nil
}

// DeliverStepStatus 交付步骤状态
type DeliverStepStatus int

// DeliverStepStatus 交付步骤状态
const (
	DeliverStatusInit     DeliverStepStatus = -1
	DeliverStatusSuccess  DeliverStepStatus = 0
	DeliverStatusHandling DeliverStepStatus = 1
	DeliverStatusFailed   DeliverStepStatus = 2
)

// Validate DeliverStepStatus.
func (d DeliverStepStatus) Validate() error {
	switch d {
	case DeliverStatusInit, DeliverStatusSuccess, DeliverStatusHandling, DeliverStatusFailed:
	default:
		return fmt.Errorf("unsupported deliver step status: %d", d)
	}

	return nil
}

// GenerateStepStatus 生产步骤状态
type GenerateStepStatus int

// GenerateStepStatus 生产步骤状态
const (
	GenerateStatusInit     GenerateStepStatus = -1
	GenerateStatusSuccess  GenerateStepStatus = 0
	GenerateStatusHandling GenerateStepStatus = 1
	GenerateStatusFailed   GenerateStepStatus = 2
	// GenerateStatusSuspend 分区生产订单，未拿到机器生产单据id时状态，更新后此生产订单不会进入再生产
	GenerateStatusSuspend GenerateStepStatus = 3
)

// Validate GenerateStepStatus.
func (g GenerateStepStatus) Validate() error {
	switch g {
	case GenerateStatusInit, GenerateStatusSuccess, GenerateStatusHandling,
		GenerateStatusFailed, GenerateStatusSuspend:
	default:
		return fmt.Errorf("unsupported generate step status: %d", g)
	}

	return nil
}
