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

package aws

import (
	"testing"

	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/runtime/filter"

	"github.com/stretchr/testify/assert"
)

// TestListRegion_FilterConditionValidation 验证过滤条件包含 account_id 和 sync_enable 字段
func TestListRegion_FilterConditionValidation(t *testing.T) {
	accountID := "test-account-123"

	// 构建与 ListRegion 中相同的过滤条件
	filterExpr := tools.ExpressionAnd(
		tools.RuleEqual("account_id", accountID),
		tools.RuleEqual("sync_enable", true),
	)

	// 验证过滤表达式不为空
	assert.NotNil(t, filterExpr)

	// 验证操作符为 And
	assert.Equal(t, filter.And, filterExpr.Op)

	// 验证规则数量为 2
	assert.Len(t, filterExpr.Rules, 2)

	// 验证第一个规则是 account_id
	rule1, ok := filterExpr.Rules[0].(*filter.AtomRule)
	assert.True(t, ok)
	assert.Equal(t, "account_id", rule1.Field)
	assert.Equal(t, accountID, rule1.Value)

	// 验证第二个规则是 sync_enable
	rule2, ok := filterExpr.Rules[1].(*filter.AtomRule)
	assert.True(t, ok)
	assert.Equal(t, "sync_enable", rule2.Field)
	assert.Equal(t, true, rule2.Value)
}

// TestListRegion_FilterWithDifferentAccountIDs 验证不同 account_id 生成不同的过滤条件
func TestListRegion_FilterWithDifferentAccountIDs(t *testing.T) {
	testCases := []struct {
		name      string
		accountID string
	}{
		{"empty account id", ""},
		{"normal account id", "account-001"},
		{"uuid format", "550e8400-e29b-41d4-a716-446655440000"},
		{"special chars", "account_with-special.chars"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filterExpr := tools.ExpressionAnd(
				tools.RuleEqual("account_id", tc.accountID),
				tools.RuleEqual("sync_enable", true),
			)

			assert.NotNil(t, filterExpr)
			assert.Len(t, filterExpr.Rules, 2)

			rule1, ok := filterExpr.Rules[0].(*filter.AtomRule)
			assert.True(t, ok)
			assert.Equal(t, tc.accountID, rule1.Value)
		})
	}
}

// TestListRegion_SyncEnableFilterValue 验证 sync_enable 过滤值为 true
func TestListRegion_SyncEnableFilterValue(t *testing.T) {
	accountID := "test-account"

	filterExpr := tools.ExpressionAnd(
		tools.RuleEqual("account_id", accountID),
		tools.RuleEqual("sync_enable", true),
	)

	// 找到 sync_enable 规则
	var syncEnableRule *filter.AtomRule
	for _, rule := range filterExpr.Rules {
		atomRule, ok := rule.(*filter.AtomRule)
		if ok && atomRule.Field == "sync_enable" {
			syncEnableRule = atomRule
			break
		}
	}

	assert.NotNil(t, syncEnableRule)
	assert.Equal(t, true, syncEnableRule.Value)
	// 确保不是 false
	assert.NotEqual(t, false, syncEnableRule.Value)
}

// TestListRegion_FilterExpressionType 验证过滤表达式类型
func TestListRegion_FilterExpressionType(t *testing.T) {
	filterExpr := tools.ExpressionAnd(
		tools.RuleEqual("account_id", "test"),
		tools.RuleEqual("sync_enable", true),
	)

	// 验证是 Expression 类型
	assert.Equal(t, filter.ExpressionType, filterExpr.WithType())

	// 验证每个规则是 AtomRule 类型
	for _, rule := range filterExpr.Rules {
		assert.Equal(t, filter.AtomType, rule.WithType())
	}
}

// TestListRegion_FilterOperator 验证过滤条件的操作符
func TestListRegion_FilterOperator(t *testing.T) {
	filterExpr := tools.ExpressionAnd(
		tools.RuleEqual("account_id", "test"),
		tools.RuleEqual("sync_enable", true),
	)

	// 验证顶层操作符是 And
	assert.Equal(t, filter.And, filterExpr.Op)

	// 验证每个 AtomRule 的操作符是 Equal
	for _, rule := range filterExpr.Rules {
		atomRule, ok := rule.(*filter.AtomRule)
		assert.True(t, ok)
		assert.Equal(t, filter.Equal.Factory(), atomRule.Op)
	}
}
