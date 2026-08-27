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
	"fmt"
	"testing"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/runtime/filter"

	"github.com/stretchr/testify/assert"
)

func TestMergeAuthFilter(t *testing.T) {
	authRule := tools.RuleIn("main_account_id", []string{"m1", "m2"})

	testCases := []struct {
		name      string
		reqFilter *filter.Expression
		authRule  filter.RuleFactory
		// expectRuleNum 合并结果的顶层规则数，0 表示期望直接透传调用方表达式
		expectRuleNum int
	}{
		{
			name:          "full permission passes request filter through",
			reqFilter:     tools.ExpressionAnd(tools.RuleEqual("vendor", "aws")),
			authRule:      nil,
			expectRuleNum: 1,
		},
		{
			name:          "empty request filter keeps only auth rule",
			reqFilter:     tools.AllExpression(),
			authRule:      authRule,
			expectRuleNum: 1,
		},
		{
			name:          "nil request filter keeps only auth rule",
			reqFilter:     nil,
			authRule:      authRule,
			expectRuleNum: 1,
		},
		{
			name:          "request filter is nested as one rule under auth rule",
			reqFilter:     tools.ExpressionAnd(tools.RuleEqual("vendor", "aws")),
			authRule:      authRule,
			expectRuleNum: 2,
		},
		{
			name:          "over in limit auth rule is nested as one rule",
			reqFilter:     tools.ExpressionAnd(tools.RuleEqual("vendor", "aws")),
			authRule:      buildMainAccountInRule(genMainAccountIDs(int(filter.DefaultMaxInLimit) + 1)),
			expectRuleNum: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			merged := mergeAuthFilter(tc.reqFilter, tc.authRule)

			assert.Equal(t, filter.And, merged.Op)
			assert.Len(t, merged.Rules, tc.expectRuleNum)
		})
	}
}

// TestMergeAuthFilterNotFlattenOrExpression 校验调用方顶层 op 为 or 时鉴权条件不会被并进 or 而失效。
func TestMergeAuthFilterNotFlattenOrExpression(t *testing.T) {
	reqFilter := tools.ExpressionOr(
		tools.RuleEqual("main_account_id", "m3"),
		tools.RuleEqual("main_account_id", "m4"),
	)

	merged := mergeAuthFilter(reqFilter, tools.RuleIn("main_account_id", []string{"m1", "m2"}))

	assert.Equal(t, filter.And, merged.Op)
	assert.Len(t, merged.Rules, 2)

	nested, ok := merged.Rules[1].(*filter.Expression)
	assert.True(t, ok)
	assert.Equal(t, filter.Or, nested.Op)
}

func TestBuildMainAccountInRule(t *testing.T) {
	opt := filter.NewExprOption(filter.RuleFields(map[string]enumor.ColumnType{
		"main_account_id": enumor.String,
	}))

	testCases := []struct {
		name     string
		count    int
		wantType filter.RuleType
	}{
		{name: "within in limit returns atom rule", count: 2, wantType: filter.AtomType},
		{name: "exact in limit returns atom rule", count: int(filter.DefaultMaxInLimit), wantType: filter.AtomType},
		{name: "over in limit returns or expression", count: int(filter.DefaultMaxInLimit) + 1,
			wantType: filter.ExpressionType},
		{name: "over rule limit still passes validate",
			count:    int(filter.DefaultMaxInLimit)*int(filter.DefaultMaxRuleLimit) + 1,
			wantType: filter.ExpressionType},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rule := buildMainAccountInRule(genMainAccountIDs(tc.count))
			assert.Equal(t, tc.wantType, rule.WithType())
			assertInRuleWithinLimit(t, rule)

			wrapped := &filter.Expression{Op: filter.And, Rules: []filter.RuleFactory{rule}}
			assert.NoError(t, wrapped.Validate(opt))
		})
	}
}

func TestBuildMainAccountInRuleUnsplitExceedsInLimit(t *testing.T) {
	opt := filter.NewExprOption(filter.RuleFields(map[string]enumor.ColumnType{
		"main_account_id": enumor.String,
	}))
	unsplit := &filter.Expression{Op: filter.And, Rules: []filter.RuleFactory{
		tools.RuleIn("main_account_id", genMainAccountIDs(int(filter.DefaultMaxInLimit)+1)),
	}}
	assert.Error(t, unsplit.Validate(opt))
}

func genMainAccountIDs(n int) []string {
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = fmt.Sprintf("m%d", i)
	}
	return ids
}

func assertInRuleWithinLimit(t *testing.T, rule filter.RuleFactory) {
	t.Helper()
	switch rule.WithType() {
	case filter.AtomType:
		atom, ok := rule.(*filter.AtomRule)
		assert.True(t, ok)
		vals, ok := atom.Value.([]string)
		assert.True(t, ok)
		assert.LessOrEqual(t, len(vals), int(filter.DefaultMaxInLimit))
	case filter.ExpressionType:
		expr, ok := rule.(*filter.Expression)
		assert.True(t, ok)
		assert.Equal(t, filter.Or, expr.Op)
		assert.LessOrEqual(t, len(expr.Rules), int(filter.DefaultMaxRuleLimit))
		for _, child := range expr.Rules {
			assertInRuleWithinLimit(t, child)
		}
	default:
		t.Fatalf("unexpected rule type %s", rule.WithType())
	}
}
