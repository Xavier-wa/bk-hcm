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

package dispatcher

import (
	"strconv"
	"testing"
	"time"

	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/enumor"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"

	"github.com/stretchr/testify/assert"
)

// currentTestYear 返回当前年份，用于构造测试数据的 expect_time
func currentTestYear() string {
	return strconv.Itoa(time.Now().Year()) + "-01-01"
}

// makeAutoApproveDemand builds a rpt.ResPlanDemand for auto approve test use.
func makeAutoApproveDemand(family string, cpuCore int64, cbsSize int64) rpt.ResPlanDemand {
	return rpt.ResPlanDemand{
		Updated: &rpt.UpdatedRPDemandItem{
			ObsProject: enumor.ObsProjectNormal,
			ExpectTime: currentTestYear(),
			RegionID:   "ap-shanghai",
			RegionName: "上海",
			AreaName:   "华东",
			Cvm: rpt.Cvm{
				DeviceFamily: family,
				CpuCore:      cpuCore,
			},
			Cbs: rpt.Cbs{
				DiskSize: cbsSize,
			},
		},
	}
}

// makeAutoApproveDemandWithTime builds a demand with specified expect_time.
func makeAutoApproveDemandWithTime(family string, cpuCore int64, cbsSize int64, expectTime string) rpt.ResPlanDemand {
	return rpt.ResPlanDemand{
		Updated: &rpt.UpdatedRPDemandItem{
			ObsProject: enumor.ObsProjectNormal,
			ExpectTime: expectTime,
			RegionID:   "ap-shanghai",
			RegionName: "上海",
			AreaName:   "华东",
			Cvm: rpt.Cvm{
				DeviceFamily: family,
				CpuCore:      cpuCore,
			},
			Cbs: rpt.Cbs{
				DiskSize: cbsSize,
			},
		},
	}
}

// makeCbsOnlyDemand builds a CBS-only (pure disk) demand with empty Cvm.
// Such demands are produced by CBS split logic, e.g. when a ticket is split
// into a CVM part and a pure-disk part.
func makeCbsOnlyDemand(cbsSize int64) rpt.ResPlanDemand {
	return rpt.ResPlanDemand{
		Updated: &rpt.UpdatedRPDemandItem{
			ObsProject: enumor.ObsProjectNormal,
			ExpectTime: "2025-01-01",
			RegionID:   "ap-shanghai",
			RegionName: "上海",
			AreaName:   "华东",
			Cvm:        rpt.Cvm{}, // empty CVM => pure disk demand
			Cbs: rpt.Cbs{
				DiskSize: cbsSize,
			},
		},
	}
}

// makeDeleteDemand builds a delete type demand (Original != nil, Updated == nil).
func makeDeleteDemand() rpt.ResPlanDemand {
	return rpt.ResPlanDemand{
		Original: &rpt.OriginalRPDemandItem{
			DemandID:   "demand-001",
			ObsProject: enumor.ObsProjectNormal,
			ExpectTime: currentTestYear(),
			RegionID:   "ap-shanghai",
			RegionName: "上海",
			AreaName:   "华东",
			Cvm: rpt.Cvm{
				DeviceFamily: string(enumor.DeviceFamilyStandard),
				CpuCore:      100,
			},
		},
		Updated: nil,
	}
}

// makeChangeDemand builds a change type demand (Original != nil, Updated != nil).
func makeChangeDemand() rpt.ResPlanDemand {
	return rpt.ResPlanDemand{
		Original: &rpt.OriginalRPDemandItem{
			DemandID:   "demand-001",
			ObsProject: enumor.ObsProjectNormal,
			ExpectTime: currentTestYear(),
			RegionID:   "ap-shanghai",
			RegionName: "上海",
			AreaName:   "华东",
			Cvm: rpt.Cvm{
				DeviceFamily: string(enumor.DeviceFamilyStandard),
				CpuCore:      100,
			},
		},
		Updated: &rpt.UpdatedRPDemandItem{
			ObsProject: enumor.ObsProjectNormal,
			ExpectTime: currentTestYear(),
			RegionID:   "ap-shanghai",
			RegionName: "上海",
			AreaName:   "华东",
			Cvm: rpt.Cvm{
				DeviceFamily: string(enumor.DeviceFamilyStandard),
				CpuCore:      200,
			},
		},
	}
}

// ---- TestCheckPredictionAutoApprove ----

type autoApproveTestCase struct {
	name              string
	demands           rpt.ResPlanDemands
	wantCanApprove    bool
	wantReasonContain string
	wantCPUCores      int64
	wantCBSSizeGB     int64
}

// buildBasicConditionCases returns test cases for basic auto-approve conditions.
func buildBasicConditionCases() []autoApproveTestCase {
	standardFamily := string(enumor.DeviceFamilyStandard)

	return []autoApproveTestCase{
		{
			name: "all conditions met: standard family, CPU under threshold, CBS under threshold",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand(standardFamily, 500, 10000),
				makeAutoApproveDemand(standardFamily, 500, 10000),
			},
			wantCanApprove:    true,
			wantReasonContain: "满足自动过单条件",
			wantCPUCores:      1000,
			wantCBSSizeGB:     20000,
		},
		{
			name:              "empty demands",
			demands:           rpt.ResPlanDemands{},
			wantCanApprove:    true,
			wantReasonContain: "满足自动过单条件",
			wantCPUCores:      0,
			wantCBSSizeGB:     0,
		},
		{
			name: "append type with nil Updated is skipped",
			demands: rpt.ResPlanDemands{
				{Original: nil, Updated: nil},
				makeAutoApproveDemand(standardFamily, 500, 10000),
			},
			wantCanApprove:    true,
			wantReasonContain: "满足自动过单条件",
			wantCPUCores:      500,
			wantCBSSizeGB:     10000,
		},
		{
			// 纯磁盘单（CVM 为空）：应当跳过机型校验，允许自动过单
			name: "CBS-only demand (pure disk) should be auto-approved",
			demands: rpt.ResPlanDemands{
				makeCbsOnlyDemand(10000),
			},
			wantCanApprove:    true,
			wantReasonContain: "满足自动过单条件",
			wantCPUCores:      0,
			wantCBSSizeGB:     10000,
		},
		{
			// 多个纯磁盘单：累加 CBS 容量
			name: "multiple CBS-only demands should be auto-approved",
			demands: rpt.ResPlanDemands{
				makeCbsOnlyDemand(10000),
				makeCbsOnlyDemand(20000),
				makeCbsOnlyDemand(15000),
			},
			wantCanApprove:    true,
			wantReasonContain: "满足自动过单条件",
			wantCPUCores:      0,
			wantCBSSizeGB:     45000,
		},
		{
			// 纯磁盘单 + 标准型 CVM 单的组合：均应满足自动过单条件
			name: "mix CBS-only and standard CVM demands should be auto-approved",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand(standardFamily, 500, 10000),
				makeCbsOnlyDemand(20000),
			},
			wantCanApprove:    true,
			wantReasonContain: "满足自动过单条件",
			wantCPUCores:      500,
			wantCBSSizeGB:     30000,
		},
		{
			// 纯磁盘单超阈值：仍需校验 CBS 容量
			name: "CBS-only demand exceeding CBS threshold should not be auto-approved",
			demands: rpt.ResPlanDemands{
				makeCbsOnlyDemand(50000),
			},
			wantCanApprove:    false,
			wantReasonContain: "CBS容量超出阈值",
			wantCPUCores:      0,
			wantCBSSizeGB:     50000,
		},
	}
}

// buildBoundaryValueCases returns test cases for threshold boundary values.
func buildBoundaryValueCases() []autoApproveTestCase {
	standardFamily := string(enumor.DeviceFamilyStandard)

	return []autoApproveTestCase{
		{
			name: "boundary: CPU exactly at threshold 1500",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand(standardFamily, 1500, 10000),
			},
			wantCanApprove:    true,
			wantReasonContain: "满足自动过单条件",
			wantCPUCores:      1500,
			wantCBSSizeGB:     10000,
		},
		{
			name: "boundary: CBS exactly at threshold 46080GB",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand(standardFamily, 500, constant.AutoApproveCBSSizeThreshold),
			},
			wantCanApprove:    true,
			wantReasonContain: "满足自动过单条件",
			wantCPUCores:      500,
			wantCBSSizeGB:     constant.AutoApproveCBSSizeThreshold,
		},
	}
}

// buildThresholdExceedCases returns test cases for exceeded thresholds.
func buildThresholdExceedCases() []autoApproveTestCase {
	standardFamily := string(enumor.DeviceFamilyStandard)

	return []autoApproveTestCase{
		{
			name: "CPU exceeds threshold",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand(standardFamily, 1600, 10000),
			},
			wantCanApprove:    false,
			wantReasonContain: "CPU核心数超出阈值",
			wantCPUCores:      1600,
			wantCBSSizeGB:     10000,
		},
		{
			name: "CBS exceeds threshold",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand(standardFamily, 500, 50000),
			},
			wantCanApprove:    false,
			wantReasonContain: "CBS容量超出阈值",
			wantCPUCores:      500,
			wantCBSSizeGB:     50000,
		},
		{
			name: "multiple conditions not met",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand("计算型", 1600, 50000),
			},
			wantCanApprove:    false,
			wantReasonContain: "包含非标准型机型",
			wantCPUCores:      1600,
			wantCBSSizeGB:     50000,
		},
	}
}

// buildDeviceFamilyCases returns test cases for device family validation.
func buildDeviceFamilyCases() []autoApproveTestCase {
	standardFamily := string(enumor.DeviceFamilyStandard)

	return []autoApproveTestCase{
		{
			name: "contains non-standard family",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand(standardFamily, 500, 10000),
				makeAutoApproveDemand("计算型", 500, 10000),
			},
			wantCanApprove:    false,
			wantReasonContain: "包含非标准型机型: 计算型",
			wantCPUCores:      1000,
			wantCBSSizeGB:     20000,
		},
		{
			name: "empty device family",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand("", 500, 10000),
			},
			wantCanApprove:    false,
			wantReasonContain: "包含未指定机型的需求",
			wantCPUCores:      500,
			wantCBSSizeGB:     10000,
		},
		{
			name: "multiple same non-standard families (dedup)",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand("计算型", 100, 1000),
				makeAutoApproveDemand("计算型", 100, 1000),
				makeAutoApproveDemand("计算型", 100, 1000),
			},
			wantCanApprove:    false,
			wantReasonContain: "包含非标准型机型: 计算型",
			wantCPUCores:      300,
			wantCBSSizeGB:     3000,
		},
		{
			name: "multiple different non-standard families",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand("计算型", 100, 1000),
				makeAutoApproveDemand("内存型", 100, 1000),
			},
			wantCanApprove:    false,
			wantReasonContain: "计算型",
			wantCPUCores:      200,
			wantCBSSizeGB:     2000,
		},
	}
}

// buildDemandTypeCases returns test cases for demand type (delete/change) validation.
func buildDemandTypeCases() []autoApproveTestCase {
	standardFamily := string(enumor.DeviceFamilyStandard)

	return []autoApproveTestCase{
		{
			name: "contains delete type demand",
			demands: rpt.ResPlanDemands{
				makeDeleteDemand(),
				makeAutoApproveDemand(standardFamily, 500, 10000),
			},
			wantCanApprove:    false,
			wantReasonContain: "包含删除类型需求",
			wantCPUCores:      0,
			wantCBSSizeGB:     0,
		},
		{
			name: "contains change type demand",
			demands: rpt.ResPlanDemands{
				makeChangeDemand(),
				makeAutoApproveDemand(standardFamily, 500, 10000),
			},
			wantCanApprove:    false,
			wantReasonContain: "包含变更类型需求",
			wantCPUCores:      0,
			wantCBSSizeGB:     0,
		},
		{
			name: "delete type as first demand breaks immediately",
			demands: rpt.ResPlanDemands{
				makeDeleteDemand(),
			},
			wantCanApprove:    false,
			wantReasonContain: "包含删除类型需求",
			wantCPUCores:      0,
			wantCBSSizeGB:     0,
		},
	}
}

// buildNonCurrentYearCases returns test cases for non-current-year demand validation.
func buildNonCurrentYearCases() []autoApproveTestCase {
	standardFamily := string(enumor.DeviceFamilyStandard)
	nextYear := strconv.Itoa(time.Now().Year() + 1)

	return []autoApproveTestCase{
		{
			name: "updated expect_time is next year",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemandWithTime(standardFamily, 500, 10000, nextYear+"-01-01"),
			},
			wantCanApprove:    false,
			wantReasonContain: "包含非今年的预测需求",
			wantCPUCores:      500,
			wantCBSSizeGB:     10000,
		},
		{
			name: "original expect_time is next year in change demand",
			demands: rpt.ResPlanDemands{
				{
					Original: &rpt.OriginalRPDemandItem{
						DemandID:   "demand-001",
						ObsProject: enumor.ObsProjectNormal,
						ExpectTime: nextYear + "-06-01",
						Cvm: rpt.Cvm{
							DeviceFamily: standardFamily,
							CpuCore:      100,
						},
					},
					Updated: &rpt.UpdatedRPDemandItem{
						ObsProject: enumor.ObsProjectNormal,
						ExpectTime: currentTestYear(),
						Cvm: rpt.Cvm{
							DeviceFamily: standardFamily,
							CpuCore:      200,
						},
					},
				},
			},
			wantCanApprove:    false,
			wantReasonContain: "包含非今年的预测需求",
			wantCPUCores:      0,
			wantCBSSizeGB:     0,
		},
		{
			name: "mixed current and next year demands",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand(standardFamily, 500, 10000),
				makeAutoApproveDemandWithTime(standardFamily, 500, 10000, nextYear+"-03-01"),
			},
			wantCanApprove:    false,
			wantReasonContain: "包含非今年的预测需求",
			wantCPUCores:      1000,
			wantCBSSizeGB:     20000,
		},
	}
}

// buildAutoApproveTestCases aggregates all test cases.
func buildAutoApproveTestCases() []autoApproveTestCase {
	var cases []autoApproveTestCase
	cases = append(cases, buildBasicConditionCases()...)
	cases = append(cases, buildBoundaryValueCases()...)
	cases = append(cases, buildThresholdExceedCases()...)
	cases = append(cases, buildDeviceFamilyCases()...)
	cases = append(cases, buildDemandTypeCases()...)
	cases = append(cases, buildNonCurrentYearCases()...)
	return cases
}

func TestCheckPredictionAutoApprove(t *testing.T) {
	kt := testKit()

	for _, tc := range buildAutoApproveTestCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result := checkPredictionAutoApprove(kt, tc.demands)

			assert.Equal(t, tc.wantCanApprove, result.CanAutoApprove,
				"CanAutoApprove mismatch")

			if tc.wantReasonContain != "" {
				assert.Contains(t, result.Reason, tc.wantReasonContain,
					"Reason should contain expected text")
			}

			assert.Equal(t, tc.wantCPUCores, result.TotalCPUCores,
				"TotalCPUCores mismatch")

			assert.Equal(t, tc.wantCBSSizeGB, result.TotalCBSSizeGB,
				"TotalCBSSizeGB mismatch")
		})
	}
}

// TestCheckPredictionAutoApprove_CPUBoundary tests CPU threshold boundary conditions.
func TestCheckPredictionAutoApprove_CPUBoundary(t *testing.T) {
	kt := testKit()
	standardFamily := string(enumor.DeviceFamilyStandard)

	testCases := []struct {
		name           string
		cpuCores       int64
		wantCanApprove bool
	}{
		{"CPU exactly at threshold (1500)", constant.AutoApproveCPUCoreThreshold, true},
		{"CPU one below threshold (1499)", constant.AutoApproveCPUCoreThreshold - 1, true},
		{"CPU one above threshold (1501)", constant.AutoApproveCPUCoreThreshold + 1, false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			demands := rpt.ResPlanDemands{
				makeAutoApproveDemand(standardFamily, tc.cpuCores, 10000),
			}
			result := checkPredictionAutoApprove(kt, demands)
			assert.Equal(t, tc.wantCanApprove, result.CanAutoApprove)
		})
	}
}

// TestCheckPredictionAutoApprove_CBSBoundary tests CBS threshold boundary conditions.
func TestCheckPredictionAutoApprove_CBSBoundary(t *testing.T) {
	kt := testKit()
	standardFamily := string(enumor.DeviceFamilyStandard)

	testCases := []struct {
		name           string
		cbsSizeGB      int64
		wantCanApprove bool
	}{
		{"CBS exactly at threshold (46080)", constant.AutoApproveCBSSizeThreshold, true},
		{"CBS one below threshold (46079)", constant.AutoApproveCBSSizeThreshold - 1, true},
		{"CBS one above threshold (46081)", constant.AutoApproveCBSSizeThreshold + 1, false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			demands := rpt.ResPlanDemands{
				makeAutoApproveDemand(standardFamily, 500, tc.cbsSizeGB),
			}
			result := checkPredictionAutoApprove(kt, demands)
			assert.Equal(t, tc.wantCanApprove, result.CanAutoApprove)
		})
	}
}

// TestCheckPredictionAutoApprove_DemandTypeBreak verifies that delete/change type
// demands cause immediate break and stop processing subsequent demands.
func TestCheckPredictionAutoApprove_DemandTypeBreak(t *testing.T) {
	kt := testKit()
	standardFamily := string(enumor.DeviceFamilyStandard)

	// Delete type in the middle - should not count demands after it
	demands := rpt.ResPlanDemands{
		makeAutoApproveDemand(standardFamily, 100, 1000), // counted
		makeDeleteDemand(), // triggers break
		makeAutoApproveDemand(standardFamily, 200, 2000), // NOT counted
	}

	result := checkPredictionAutoApprove(kt, demands)

	assert.False(t, result.CanAutoApprove)
	assert.Contains(t, result.Reason, "包含删除类型需求")
	// Only first demand's CPU/CBS should be counted
	assert.Equal(t, int64(100), result.TotalCPUCores)
	assert.Equal(t, int64(1000), result.TotalCBSSizeGB)
}

// TestCheckPredictionAutoApprove_NonStandardFamilyDedup verifies that the same
// non-standard family is only recorded once in the reason.
func TestCheckPredictionAutoApprove_NonStandardFamilyDedup(t *testing.T) {
	kt := testKit()

	demands := rpt.ResPlanDemands{
		makeAutoApproveDemand("计算型", 100, 1000),
		makeAutoApproveDemand("计算型", 100, 1000),
		makeAutoApproveDemand("内存型", 100, 1000),
	}

	result := checkPredictionAutoApprove(kt, demands)

	assert.False(t, result.CanAutoApprove)

	// Count occurrences of "计算型" in reason - should be only once
	reasonBytes := []byte(result.Reason)
	count := 0
	for i := 0; i <= len(reasonBytes)-len("计算型"); i++ {
		if string(reasonBytes[i:i+len("计算型")]) == "计算型" {
			count++
		}
	}
	assert.Equal(t, 1, count, "计算型 should appear only once in reason due to dedup")
}

// ---- Integration Tests for Auto Approve Logic ----

// TestAutoApproveCheckResult_Structure verifies the result structure is correctly populated.
func TestAutoApproveCheckResult_Structure(t *testing.T) {
	kt := testKit()
	standardFamily := string(enumor.DeviceFamilyStandard)

	demands := rpt.ResPlanDemands{
		makeAutoApproveDemand(standardFamily, 300, 5000),
		makeAutoApproveDemand(standardFamily, 200, 3000),
		makeAutoApproveDemand(standardFamily, 100, 2000),
	}

	result := checkPredictionAutoApprove(kt, demands)

	// Verify structure
	assert.NotNil(t, result)
	assert.True(t, result.CanAutoApprove)
	assert.Equal(t, int64(600), result.TotalCPUCores, "Total CPU should be sum of all demands")
	assert.Equal(t, int64(10000), result.TotalCBSSizeGB, "Total CBS should be sum of all demands")
	assert.NotEmpty(t, result.Reason)
}

// TestAutoApprove_CombinedThresholds tests when both CPU and CBS exceed thresholds.
func TestAutoApprove_CombinedThresholds(t *testing.T) {
	kt := testKit()
	standardFamily := string(enumor.DeviceFamilyStandard)

	demands := rpt.ResPlanDemands{
		makeAutoApproveDemand(standardFamily, 1600, 50000), // Both exceed
	}

	result := checkPredictionAutoApprove(kt, demands)

	assert.False(t, result.CanAutoApprove)
	assert.Contains(t, result.Reason, "CPU核心数超出阈值")
	assert.Contains(t, result.Reason, "CBS容量超出阈值")
}

// TestAutoApprove_MixedDemandTypes tests mixing append with delete/change types.
func TestAutoApprove_MixedDemandTypes(t *testing.T) {
	kt := testKit()
	standardFamily := string(enumor.DeviceFamilyStandard)

	testCases := []struct {
		name           string
		demands        rpt.ResPlanDemands
		wantCanApprove bool
		wantReason     string
	}{
		{
			name: "delete comes after append - break stops at delete",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand(standardFamily, 100, 1000),
				makeDeleteDemand(),
			},
			wantCanApprove: false,
			wantReason:     "删除类型",
		},
		{
			name: "change comes after append - break stops at change",
			demands: rpt.ResPlanDemands{
				makeAutoApproveDemand(standardFamily, 100, 1000),
				makeChangeDemand(),
			},
			wantCanApprove: false,
			wantReason:     "变更类型",
		},
		{
			name: "only delete type",
			demands: rpt.ResPlanDemands{
				makeDeleteDemand(),
			},
			wantCanApprove: false,
			wantReason:     "删除类型",
		},
		{
			name: "only change type",
			demands: rpt.ResPlanDemands{
				makeChangeDemand(),
			},
			wantCanApprove: false,
			wantReason:     "变更类型",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := checkPredictionAutoApprove(kt, tc.demands)
			assert.Equal(t, tc.wantCanApprove, result.CanAutoApprove)
			assert.Contains(t, result.Reason, tc.wantReason)
		})
	}
}

// TestAutoApprove_LargeDemandList tests with a larger list of demands.
func TestAutoApprove_LargeDemandList(t *testing.T) {
	kt := testKit()
	standardFamily := string(enumor.DeviceFamilyStandard)

	// Create 50 small demands that together don't exceed thresholds
	demands := make(rpt.ResPlanDemands, 50)
	for i := 0; i < 50; i++ {
		demands[i] = makeAutoApproveDemand(standardFamily, 20, 500) // 20*50=1000 CPU, 500*50=25000 CBS
	}

	result := checkPredictionAutoApprove(kt, demands)

	assert.True(t, result.CanAutoApprove)
	assert.Equal(t, int64(1000), result.TotalCPUCores)
	assert.Equal(t, int64(25000), result.TotalCBSSizeGB)
}

// TestAutoApprove_LargeDemandListExceedsThreshold tests with demands that exceed thresholds.
func TestAutoApprove_LargeDemandListExceedsThreshold(t *testing.T) {
	kt := testKit()
	standardFamily := string(enumor.DeviceFamilyStandard)

	// Create 100 demands that together exceed CPU threshold
	demands := make(rpt.ResPlanDemands, 100)
	for i := 0; i < 100; i++ {
		demands[i] = makeAutoApproveDemand(standardFamily, 20, 500) // 20*100=2000 CPU > 1500
	}

	result := checkPredictionAutoApprove(kt, demands)

	assert.False(t, result.CanAutoApprove)
	assert.Equal(t, int64(2000), result.TotalCPUCores)
	assert.Contains(t, result.Reason, "CPU核心数超出阈值")
}

// TestAutoApprove_ZeroValues tests demands with zero CPU and CBS values.
func TestAutoApprove_ZeroValues(t *testing.T) {
	kt := testKit()
	standardFamily := string(enumor.DeviceFamilyStandard)

	demands := rpt.ResPlanDemands{
		makeAutoApproveDemand(standardFamily, 0, 0),
		makeAutoApproveDemand(standardFamily, 0, 0),
	}

	result := checkPredictionAutoApprove(kt, demands)

	assert.True(t, result.CanAutoApprove)
	assert.Equal(t, int64(0), result.TotalCPUCores)
	assert.Equal(t, int64(0), result.TotalCBSSizeGB)
}

// TestAutoApprove_MultipleNonStandardFamilies tests multiple different non-standard families.
func TestAutoApprove_MultipleNonStandardFamilies(t *testing.T) {
	kt := testKit()

	demands := rpt.ResPlanDemands{
		makeAutoApproveDemand("计算型", 100, 1000),
		makeAutoApproveDemand("内存型", 100, 1000),
		makeAutoApproveDemand("GPU型", 100, 1000),
		makeAutoApproveDemand("", 100, 1000), // empty family
	}

	result := checkPredictionAutoApprove(kt, demands)

	assert.False(t, result.CanAutoApprove)
	assert.Contains(t, result.Reason, "计算型")
	assert.Contains(t, result.Reason, "内存型")
	assert.Contains(t, result.Reason, "GPU型")
	assert.Contains(t, result.Reason, "未指定机型")
}
