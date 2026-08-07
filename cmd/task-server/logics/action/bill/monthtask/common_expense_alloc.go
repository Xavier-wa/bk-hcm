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

import "github.com/shopspring/decimal"

// allocateCommonExpense allocates batchSum across accounts by CurrentMonthCost ratio.
// The summaryTotal parameter must be the sum of costs.
// When summaryTotal is zero and batchSum is non-zero, it splits equally and puts the
// remainder on the last account so the sum equals batchSum exactly.
// When both are zero, it returns a zero slice of the same length.
func allocateCommonExpense(batchSum, summaryTotal decimal.Decimal, costs []decimal.Decimal) []decimal.Decimal {
	n := len(costs)
	if n == 0 {
		return nil
	}

	result := make([]decimal.Decimal, n)
	// When summaryTotal is zero, it means all participation costs are zero.
	// In this case, we should equal split the batchSum.
	if summaryTotal.IsZero() {
		if batchSum.IsZero() {
			return result
		}
		// Equal split when all participation costs are zero.
		share := batchSum.Div(decimal.NewFromInt(int64(n)))
		allocated := decimal.Zero
		for i := 0; i < n-1; i++ {
			result[i] = share
			allocated = allocated.Add(share)
		}
		result[n-1] = batchSum.Sub(allocated)
		return result
	}

	// Otherwise, we should split the batchSum by the ratio of the participation costs.
	for i, cost := range costs {
		result[i] = batchSum.Mul(cost).Div(summaryTotal)
	}
	return result
}
