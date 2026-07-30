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

package plan

import (
	"testing"

	gpusuborder "hcm/pkg/dal/table/resource-plan/res-plan-demand-gpu-suborder"

	"github.com/stretchr/testify/assert"
)

func TestAggregateGpuSubOrderSummary(t *testing.T) {
	subOrders := []gpusuborder.ResPlanDemandGpuSubOrderTable{
		{DemandType: "训练-文生文", DemandYear: 2026, DemandMonth: 3, GPUNum: 64, QpmMax: 0},
		{DemandType: "训练-文生文", DemandYear: 2026, DemandMonth: 4, GPUNum: 64, QpmMax: 0},
		{DemandType: "推理-API", DemandYear: 2026, DemandMonth: 3, GPUNum: 0, QpmMax: 1000},
	}

	details := aggregateGpuSubOrderSummary(subOrders)
	assert.Len(t, details, 2)

	// sorted by demand_type
	assert.Equal(t, "推理-API", details[0].DemandType)
	assert.Equal(t, int64(0), details[0].GPUNum)
	assert.Equal(t, int64(1000), details[0].QpmMax)
	assert.Equal(t, int64(1000), details[0].Months["2026-03"])

	assert.Equal(t, "训练-文生文", details[1].DemandType)
	assert.Equal(t, int64(128), details[1].GPUNum)
	assert.Equal(t, int64(0), details[1].QpmMax)
	assert.Equal(t, int64(64), details[1].Months["2026-03"])
	assert.Equal(t, int64(64), details[1].Months["2026-04"])
}

func TestAggregateGpuSubOrderSummaryEmpty(t *testing.T) {
	details := aggregateGpuSubOrderSummary(nil)
	assert.Empty(t, details)
}
