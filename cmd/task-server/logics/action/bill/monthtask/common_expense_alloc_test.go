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

package monthtask

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocateCommonExpenseEmpty(t *testing.T) {
	assert.Nil(t, allocateCommonExpense(decimal.NewFromInt(10), decimal.NewFromInt(10), nil))
	assert.Nil(t, allocateCommonExpense(decimal.NewFromInt(10), decimal.NewFromInt(10), []decimal.Decimal{}))
}

func TestAllocateCommonExpenseRatio(t *testing.T) {
	batchSum := decimal.NewFromInt(100)
	costs := []decimal.Decimal{
		decimal.NewFromInt(30),
		decimal.NewFromInt(70),
		decimal.Zero,
	}
	summaryTotal := decimal.NewFromInt(100)
	got := allocateCommonExpense(batchSum, summaryTotal, costs)
	require.Len(t, got, 3)
	assert.True(t, got[0].Equal(decimal.NewFromInt(30)))
	assert.True(t, got[1].Equal(decimal.NewFromInt(70)))
	assert.True(t, got[2].IsZero())
	assert.True(t, got[0].Add(got[1]).Add(got[2]).Equal(batchSum))
}

func TestAllocateCommonExpenseEqualSplitWhenZeroTotal(t *testing.T) {
	batchSum := decimal.NewFromInt(10)
	costs := []decimal.Decimal{decimal.Zero, decimal.Zero, decimal.Zero}
	got := allocateCommonExpense(batchSum, decimal.Zero, costs)
	require.Len(t, got, 3)

	// 10/3 cannot be exact; remainder must land on the last item.
	sum := got[0].Add(got[1]).Add(got[2])
	assert.True(t, sum.Equal(batchSum), "sum=%s", sum)
	assert.True(t, got[0].Equal(got[1]))
	assert.False(t, got[0].IsZero())
}

func TestAllocateCommonExpenseBothZero(t *testing.T) {
	got := allocateCommonExpense(decimal.Zero, decimal.Zero, []decimal.Decimal{decimal.Zero, decimal.Zero})
	require.Len(t, got, 2)
	assert.True(t, got[0].IsZero())
	assert.True(t, got[1].IsZero())
}
