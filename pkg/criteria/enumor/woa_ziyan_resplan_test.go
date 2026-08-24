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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateRootTicketType 覆盖主单类型校验：add/adjust/delete/budget_declare 合法，其余非法。
func TestValidateRootTicketType(t *testing.T) {
	tests := []struct {
		name    string
		typ     RPTicketType
		wantErr bool
	}{
		{"add ok", RPTicketTypeAdd, false},
		{"adjust ok", RPTicketTypeAdjust, false},
		{"delete ok", RPTicketTypeDelete, false},
		{"budget_declare ok", RPTicketTypeBudgetDeclare, false},
		{"delay not root", RPTicketTypeDelay, true},
		{"transfer not root", RPTicketTypeTransfer, true},
		{"empty invalid", RPTicketType(""), true},
		{"unknown invalid", RPTicketType("unknown"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.typ.ValidateRootTicketType()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// TestRPTicketTypeBudgetDeclareName 验证预算申报展示名。
func TestRPTicketTypeBudgetDeclareName(t *testing.T) {
	assert.Equal(t, "预算申报", RPTicketTypeBudgetDeclare.Name())
}

// TestGetRPTicketTypeMembersContainsBudgetDeclare 验证 meta/列表主单类型成员含 budget_declare。
func TestGetRPTicketTypeMembersContainsBudgetDeclare(t *testing.T) {
	members := GetRPTicketTypeMembers()
	assert.Contains(t, members, RPTicketTypeBudgetDeclare)
	assert.Contains(t, members, RPTicketTypeAdd)
	assert.Contains(t, members, RPTicketTypeAdjust)
	assert.Contains(t, members, RPTicketTypeDelete)
}

// TestValidateResPlan 校验只认形态、不卡当前时间窗口；非法形态失败。
func TestValidateResPlan(t *testing.T) {
	tests := []struct {
		name    string
		obs     ObsProject
		wantErr bool
	}{
		{"常规项目", ObsProjectNormal, false},
		{"滚服项目", ObsProjectRollServer, false},
		{"改造复用", ObsProjectReuse, false},
		{"轻量云徙", ObsProjectMigrate, false},
		{"短租项目", ObsProjectShortLease, false},
		{"窗口外春保", ObsProject("2099春节保障"), false},
		{"窗口外裁撤", ObsProject("2098机房裁撤"), false},
		{"错写春保", ObsProject("2029春保"), true},
		{"未知文案", ObsProject("foobar"), true},
		{"空值", ObsProject(""), true},
		{"年份不足四位", ObsProject("99春节保障"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.obs.ValidateResPlan()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// TestGetObsProjectMembersForResPlanYearWindow 下拉枚举仍按当前时间窗口，不含远期年份。
func TestGetObsProjectMembersForResPlanYearWindow(t *testing.T) {
	members := GetObsProjectMembersForResPlan()
	assert.NotEmpty(t, members)
	assert.Contains(t, members, getSpringObsProjectForResPlan()[0])
	assert.NotContains(t, members, ObsProject("2099春节保障"))
	assert.NotContains(t, members, ObsProject("2098机房裁撤"))
}

// TestValidateAndSubTicketMembersExcludeBudgetDeclare 子单 Validate/成员集合不含 budget_declare。
func TestValidateAndSubTicketMembersExcludeBudgetDeclare(t *testing.T) {
	assert.Error(t, RPTicketTypeBudgetDeclare.Validate())
	assert.NotContains(t, GetPRSubTicketTypeMembers(), RPTicketTypeBudgetDeclare)
}

// TestResPlanReviewStatusValidate 验证评审状态的合法性校验，中长期预测为合法状态。
func TestResPlanReviewStatusValidate(t *testing.T) {
	tests := []struct {
		name    string
		status  ResPlanReviewStatus
		wantErr bool
	}{
		{"已评审", ResPlanReviewStatusPass, false},
		{"待评审", ResPlanReviewStatusPending, false},
		{"中长期预测", ResPlanReviewStatusMediumLongTerm, false},
		{"未知状态", ResPlanReviewStatus("未知状态"), true},
		{"空字符串", ResPlanReviewStatus(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.status.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestResPlanReviewStatusIsUnreviewed 验证中长期预测按未评审（待评审）逻辑处理。
func TestResPlanReviewStatusIsUnreviewed(t *testing.T) {
	tests := []struct {
		name   string
		status ResPlanReviewStatus
		want   bool
	}{
		{"已评审非未评审", ResPlanReviewStatusPass, false},
		{"待评审为未评审", ResPlanReviewStatusPending, true},
		{"中长期预测按未评审处理", ResPlanReviewStatusMediumLongTerm, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsUnreviewed(); got != tt.want {
				t.Errorf("IsUnreviewed() = %v, want %v", got, tt.want)
			}
		})
	}
}
