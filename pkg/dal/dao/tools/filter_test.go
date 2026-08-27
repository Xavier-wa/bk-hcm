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

package tools

import (
	"fmt"
	"testing"

	"hcm/pkg/criteria/enumor"
	"hcm/pkg/runtime/filter"

	"github.com/stretchr/testify/assert"
)

func TestCombineOrRules(t *testing.T) {
	opt := filter.NewExprOption(filter.RuleFields(map[string]enumor.ColumnType{
		"id": enumor.String,
	}))

	testCases := []struct {
		name     string
		count    int
		wantType filter.RuleType
	}{
		{name: "empty rules returns or expression", count: 0, wantType: filter.ExpressionType},
		{name: "single rule returns itself", count: 1, wantType: filter.AtomType},
		{name: "within rule limit returns or expression", count: 2, wantType: filter.ExpressionType},
		{name: "exact rule limit returns or expression", count: int(filter.DefaultMaxRuleLimit),
			wantType: filter.ExpressionType},
		{name: "over rule limit still passes validate", count: int(filter.DefaultMaxRuleLimit) + 1,
			wantType: filter.ExpressionType},
		{name: "two level nested or expression", count: int(filter.DefaultMaxRuleLimit)*2 + 1,
			wantType: filter.ExpressionType},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rule := CombineOrRules(genEqualRules(tc.count))
			assert.Equal(t, tc.wantType, rule.WithType())
			assertOrRuleWithinLimit(t, rule)

			wrapped := wrapRule(rule)
			assert.NoError(t, wrapped.Validate(opt))
		})
	}
}

func genEqualRules(n int) []filter.RuleFactory {
	rules := make([]filter.RuleFactory, n)
	for i := 0; i < n; i++ {
		rules[i] = RuleEqual("id", fmt.Sprintf("v%d", i))
	}
	return rules
}

func wrapRule(rule filter.RuleFactory) *filter.Expression {
	expr, ok := rule.(*filter.Expression)
	if ok {
		return expr
	}
	return &filter.Expression{Op: filter.And, Rules: []filter.RuleFactory{rule}}
}

func assertOrRuleWithinLimit(t *testing.T, rule filter.RuleFactory) {
	t.Helper()
	if rule.WithType() != filter.ExpressionType {
		return
	}

	expr, ok := rule.(*filter.Expression)
	assert.True(t, ok)
	assert.Equal(t, filter.Or, expr.Op)
	assert.LessOrEqual(t, len(expr.Rules), int(filter.DefaultMaxRuleLimit))
	for _, child := range expr.Rules {
		assertOrRuleWithinLimit(t, child)
	}
}
