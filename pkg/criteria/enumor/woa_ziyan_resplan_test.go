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
)

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
