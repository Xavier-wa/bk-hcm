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

// Package bkbotapproval bk审批助手
package bkbotapproval

import (
	"encoding/json"

	"hcm/pkg/criteria/validator"
)

// SendMessageTplReq send message tpl req.
type SendMessageTplReq struct {
	Title     string          `json:"title" validate:"required"`                   // 标题，markdown格式
	Approvers []string        `json:"approvers" validate:"required,min=1,max=100"` // 审批人，不可以为空
	Receiver  []string        `json:"receiver" validate:"required,min=1,max=100"`
	Summary   string          `json:"summary" validate:"required"` // 具体内容，markdown格式
	Actions   []MessageAction `json:"actions" validate:"required,max=20"`
}

// MessageAction message action
type MessageAction struct {
	Name  string      `json:"name" validate:"required"`   // 按钮名
	Color ButtonColor `json:"color" validate:"omitempty"` // 颜色 red/green
	// 回调地址，是bkapi网关的话，需要给approvalbot开启权限
	CallbackURL string `json:"callback_url" validate:"omitempty"`
	// 回调地址只能是POST，返回数据中一定要有如下结构
	// {"data":{"response_msg":"这是点击按钮后回显的信息","response_color":"green"}}
	CallbackData json.RawMessage `json:"callback_data" validate:"omitempty"` // 发送给回调地址的数据
}

// CvmApplyModifyConfirmCallbackData cvm apply modify confirm callback data
type CvmApplyModifyConfirmCallbackData struct {
	BkBizID    int64      `json:"bk_biz_id" validate:"required"`
	Action     ActionType `json:"action" validate:"required"`
	SuborderID string     `json:"suborder_id" validate:"required"`
	ModifyID   string     `json:"modify_id" validate:"required"`
}

// Validate validate
func (p *SendMessageTplReq) Validate() error {
	return validator.Validate.Struct(p)
}

// ButtonColor define button color
type ButtonColor string

const (
	// GreenButtonColor 按钮颜色-绿色
	GreenButtonColor ButtonColor = "green"
	// RedButtonColor 按钮颜色-红色
	RedButtonColor ButtonColor = "red"
)

// ActionType define action type
type ActionType string

const (
	// ApproveActionType 同意
	ApproveActionType ActionType = "APPROVE"
	// RejectActionType 拒绝
	RejectActionType ActionType = "REJECT"
)
