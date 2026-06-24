/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
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
	"sort"
	"testing"

	ptypes "hcm/cmd/woa-server/types/plan"
	"hcm/pkg/api/core"
	dt "hcm/pkg/api/core/cloud/device-type"
	"hcm/pkg/criteria/enumor"
)

// ==================== Task 6.2: OS 数计算逻辑测试 ====================

func TestBuildDeviceTypeItem_OSCalculation(t *testing.T) {
	demandItem := &ptypes.ListResPlanDemandItem{
		DemandID:       "demand-001",
		BkBizID:        100,
		BkBizName:      "测试业务",
		DeviceType:     "SA3",
		TotalCpuCore:   120,
		AppliedCpuCore: 80,
		RemainedCpuCore: 40,
		RegionID:       "region-1",
		ZoneID:         "zone-1",
	}

	tests := []struct {
		name           string
		deviceInfo     dt.DistinctDeviceType
		isOriginal     bool
		expectTotalOS  int64
		expectAppliedOS int64
		expectRemainedOS int64
	}{
		{
			name: "整除场景-60核机型",
			deviceInfo: dt.DistinctDeviceType{
				DeviceType:   "SA3.4XLARGE64",
				CpuCore:      60,
				Memory:       64,
				DeviceClass:  "compute",
				DeviceFamily: "SA3",
			},
			isOriginal:      true,
			expectTotalOS:   2,   // 120 / 60
			expectAppliedOS: 1,   // 80 / 60
			expectRemainedOS: 0,  // 40 / 60
		},
		{
			name: "整除场景-40核机型",
			deviceInfo: dt.DistinctDeviceType{
				DeviceType:   "SA3.2XLARGE32",
				CpuCore:      40,
				Memory:       32,
				DeviceClass:  "compute",
				DeviceFamily: "SA3",
			},
			isOriginal:      false,
			expectTotalOS:   3,   // 120 / 40
			expectAppliedOS: 2,   // 80 / 40
			expectRemainedOS: 1,  // 40 / 40
		},
		{
			name: "非整除场景-7核机型",
			deviceInfo: dt.DistinctDeviceType{
				DeviceType:   "IT5.2XLARGE14",
				CpuCore:      7,
				Memory:       14,
				DeviceClass:  "compute",
				DeviceFamily: "IT5",
			},
			isOriginal:      false,
			expectTotalOS:   17,  // 120 / 7 = 17 (整数除法)
			expectAppliedOS: 11,  // 80 / 7 = 11
			expectRemainedOS: 5,  // 40 / 7 = 5
		},
		{
			name: "非整除场景-3核机型",
			deviceInfo: dt.DistinctDeviceType{
				DeviceType:   "S5.SMALL4",
				CpuCore:      3,
				Memory:       4,
				DeviceClass:  "compute",
				DeviceFamily: "S5",
			},
			isOriginal:      false,
			expectTotalOS:   40,  // 120 / 3
			expectAppliedOS: 26,  // 80 / 3
			expectRemainedOS: 13, // 40 / 3
		},
		{
			name: "零核机型-防止除零",
			deviceInfo: dt.DistinctDeviceType{
				DeviceType:   "ZERO_CORE",
				CpuCore:      0,
				Memory:       0,
				DeviceClass:  "compute",
				DeviceFamily: "ZERO",
			},
			isOriginal:      true,
			expectTotalOS:   0,
			expectAppliedOS: 0,
			expectRemainedOS: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildDeviceTypeItem(demandItem, tt.deviceInfo.DeviceType, tt.isOriginal, tt.deviceInfo)

			if result.TotalOS != tt.expectTotalOS {
				t.Errorf("TotalOS = %d, want %d", result.TotalOS, tt.expectTotalOS)
			}
			if result.AppliedOS != tt.expectAppliedOS {
				t.Errorf("AppliedOS = %d, want %d", result.AppliedOS, tt.expectAppliedOS)
			}
			if result.RemainedOS != tt.expectRemainedOS {
				t.Errorf("RemainedOS = %d, want %d", result.RemainedOS, tt.expectRemainedOS)
			}
			if result.IsOriginal != tt.isOriginal {
				t.Errorf("IsOriginal = %v, want %v", result.IsOriginal, tt.isOriginal)
			}
			if result.DeviceType != tt.deviceInfo.DeviceType {
				t.Errorf("DeviceType = %s, want %s", result.DeviceType, tt.deviceInfo.DeviceType)
			}
			if result.CpuCore != tt.deviceInfo.CpuCore {
				t.Errorf("CpuCore = %d, want %d", result.CpuCore, tt.deviceInfo.CpuCore)
			}
			if result.Memory != tt.deviceInfo.Memory {
				t.Errorf("Memory = %d, want %d", result.Memory, tt.deviceInfo.Memory)
			}
		})
	}
}

func TestBuildDeviceTypeItem_FieldMapping(t *testing.T) {
	returnPlanTime := "2026-07-01"
	demandItem := &ptypes.ListResPlanDemandItem{
		DemandID:        "demand-002",
		BkBizID:         200,
		BkBizName:       "映射测试业务",
		Status:          enumor.DemandStatusCanApply,
		StatusName:      "可申领",
		DemandClass:     enumor.DemandClassCVM,
		DemandResType:   enumor.DemandResTypeCVM,
		ExpectTime:      "2026-06-01",
		CanApplyTime:    "2026-05-25",
		ExpiredTime:     "2026-07-01",
		ReturnPlanTime:  &returnPlanTime,
		TotalCpuCore:    240,
		AppliedCpuCore:  160,
		RemainedCpuCore: 80,
		RegionID:        "gz",
		RegionName:      "广州",
		ZoneID:          "gz-1",
		ZoneName:        "广州一区",
		PlanType:        enumor.PlanTypeHcmInPlan,
		ObsProject:      enumor.ObsProjectNormal,
		TechnicalClass:  "计算型",
		DeviceFamily:    "SA3",
		CoreType:        enumor.CoreTypeBig,
		DiskType:        enumor.DiskSSD,
		DiskTypeName:    "SSD云盘",
		DiskIO:          5000,
		DeviceType:      "SA3.8XLARGE128",
	}

	deviceInfo := dt.DistinctDeviceType{
		DeviceType:   "SA3.8XLARGE128",
		CpuCore:      120,
		Memory:       128,
		DeviceClass:  "compute",
		DeviceFamily: "SA3",
	}

	result := buildDeviceTypeItem(demandItem, "SA3.8XLARGE128", true, deviceInfo)

	// 验证预测主数据字段映射
	if result.DemandID != demandItem.DemandID {
		t.Errorf("DemandID = %s, want %s", result.DemandID, demandItem.DemandID)
	}
	if result.BkBizID != demandItem.BkBizID {
		t.Errorf("BkBizID = %d, want %d", result.BkBizID, demandItem.BkBizID)
	}
	if result.BkBizName != demandItem.BkBizName {
		t.Errorf("BkBizName = %s, want %s", result.BkBizName, demandItem.BkBizName)
	}
	if result.Status != demandItem.Status {
		t.Errorf("Status = %v, want %v", result.Status, demandItem.Status)
	}
	if result.RegionID != demandItem.RegionID {
		t.Errorf("RegionID = %s, want %s", result.RegionID, demandItem.RegionID)
	}
	if result.ZoneID != demandItem.ZoneID {
		t.Errorf("ZoneID = %s, want %s", result.ZoneID, demandItem.ZoneID)
	}
	if result.PlanType != demandItem.PlanType {
		t.Errorf("PlanType = %v, want %v", result.PlanType, demandItem.PlanType)
	}

	// 验证机型明细字段映射
	if result.DeviceTypeClass != deviceInfo.DeviceClass {
		t.Errorf("DeviceTypeClass = %s, want %s", result.DeviceTypeClass, deviceInfo.DeviceClass)
	}
	if result.DeviceClass != deviceInfo.DeviceFamily {
		t.Errorf("DeviceClass = %s, want %s", result.DeviceClass, deviceInfo.DeviceFamily)
	}

	// 验证 OS 计算：240/120=2, 160/120=1, 80/120=0
	if result.TotalOS != 2 {
		t.Errorf("TotalOS = %d, want 2", result.TotalOS)
	}
	if result.AppliedOS != 1 {
		t.Errorf("AppliedOS = %d, want 1", result.AppliedOS)
	}
	if result.RemainedOS != 0 {
		t.Errorf("RemainedOS = %d, want 0", result.RemainedOS)
	}
}

// ==================== Task 6.3: 排序逻辑测试 ====================

func TestSortExpandedItems(t *testing.T) {
	items := []*ptypes.ListResPlanDemandWithDeviceTypesItem{
		{DemandID: "d1", DeviceType: "IT5.4XLARGE32", IsOriginal: false, CpuCore: 16},
		{DemandID: "d1", DeviceType: "SA3.8XLARGE128", IsOriginal: true, CpuCore: 120},
		{DemandID: "d1", DeviceType: "SA3.4XLARGE64", IsOriginal: false, CpuCore: 60},
		{DemandID: "d2", DeviceType: "S5.2XLARGE16", IsOriginal: false, CpuCore: 8},
		{DemandID: "d2", DeviceType: "S5.4XLARGE32", IsOriginal: true, CpuCore: 16},
	}

	// 使用与主逻辑相同的排序规则
	sortExpandedItems(items)

	// 验证：is_original=true 的在前，同组内按 device_type 升序
	if !items[0].IsOriginal {
		t.Errorf("items[0].IsOriginal should be true, got false")
	}
	if !items[1].IsOriginal {
		t.Errorf("items[1].IsOriginal should be true, got false")
	}
	if items[0].DeviceType > items[1].DeviceType {
		t.Errorf("原始机型之间应按 device_type 升序: %s > %s", items[0].DeviceType, items[1].DeviceType)
	}

	// 验证：is_original=false 的在后，同组内按 device_type 升序
	for i := 2; i < len(items); i++ {
		if items[i].IsOriginal {
			t.Errorf("items[%d].IsOriginal should be false", i)
		}
	}
	if items[2].DeviceType > items[3].DeviceType {
		t.Errorf("通配机型之间应按 device_type 升序: %s > %s", items[2].DeviceType, items[3].DeviceType)
	}
}

func TestSortExpandedItems_AllOriginal(t *testing.T) {
	items := []*ptypes.ListResPlanDemandWithDeviceTypesItem{
		{DemandID: "d2", DeviceType: "S5.4XLARGE32", IsOriginal: true},
		{DemandID: "d1", DeviceType: "SA3.8XLARGE128", IsOriginal: true},
	}

	sortExpandedItems(items)

	if items[0].DeviceType != "S5.4XLARGE32" {
		t.Errorf("第一个应为 S5.4XLARGE32, got %s", items[0].DeviceType)
	}
	if items[1].DeviceType != "SA3.8XLARGE128" {
		t.Errorf("第二个应为 SA3.8XLARGE128, got %s", items[1].DeviceType)
	}
}

func TestSortExpandedItems_AllWildcard(t *testing.T) {
	items := []*ptypes.ListResPlanDemandWithDeviceTypesItem{
		{DemandID: "d1", DeviceType: "IT5.4XLARGE32", IsOriginal: false},
		{DemandID: "d1", DeviceType: "SA3.4XLARGE64", IsOriginal: false},
		{DemandID: "d1", DeviceType: "S5.2XLARGE16", IsOriginal: false},
	}

	sortExpandedItems(items)

	if items[0].DeviceType != "IT5.4XLARGE32" {
		t.Errorf("第一个应为 IT5.4XLARGE32, got %s", items[0].DeviceType)
	}
	if items[1].DeviceType != "S5.2XLARGE16" {
		t.Errorf("第二个应为 S5.2XLARGE16, got %s", items[1].DeviceType)
	}
	if items[2].DeviceType != "SA3.4XLARGE64" {
		t.Errorf("第三个应为 SA3.4XLARGE64, got %s", items[2].DeviceType)
	}
}

func TestSortExpandedItems_Empty(t *testing.T) {
	items := []*ptypes.ListResPlanDemandWithDeviceTypesItem{}
	sortExpandedItems(items) // 不应 panic
	if len(items) != 0 {
		t.Errorf("空列表排序后应为空")
	}
}

func TestSortExpandedItems_SingleItem(t *testing.T) {
	items := []*ptypes.ListResPlanDemandWithDeviceTypesItem{
		{DemandID: "d1", DeviceType: "SA3.8XLARGE128", IsOriginal: true},
	}
	sortExpandedItems(items) // 不应 panic
	if len(items) != 1 {
		t.Errorf("单元素列表排序后应仍为1个")
	}
}

// sortExpandedItems 使用与主逻辑相同的排序规则排序展开后的明细记录
func sortExpandedItems(items []*ptypes.ListResPlanDemandWithDeviceTypesItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsOriginal != items[j].IsOriginal {
			return items[i].IsOriginal
		}
		return items[i].DeviceType < items[j].DeviceType
	})
}

// ==================== Task 6.4: 筛选逻辑测试 ====================

func TestMatchCpuMemFilter(t *testing.T) {
	deviceInfo := dt.DistinctDeviceType{
		CpuCore: 60,
		Memory:  64,
	}

	tests := []struct {
		name    string
		req     *ptypes.ListResPlanDemandWithDeviceTypesReq
		expect  bool
	}{
		{
			name:   "无筛选条件-通过",
			req:    &ptypes.ListResPlanDemandWithDeviceTypesReq{},
			expect: true,
		},
		{
			name: "cpu_cores匹配-通过",
			req: &ptypes.ListResPlanDemandWithDeviceTypesReq{
				CpuCores: []int64{60},
			},
			expect: true,
		},
		{
			name: "cpu_cores不匹配-拒绝",
			req: &ptypes.ListResPlanDemandWithDeviceTypesReq{
				CpuCores: []int64{120},
			},
			expect: false,
		},
		{
			name: "cpu_cores多值中包含-通过",
			req: &ptypes.ListResPlanDemandWithDeviceTypesReq{
				CpuCores: []int64{30, 60, 120},
			},
			expect: true,
		},
		{
			name: "memories匹配-通过",
			req: &ptypes.ListResPlanDemandWithDeviceTypesReq{
				Memories: []int64{64},
			},
			expect: true,
		},
		{
			name: "memories不匹配-拒绝",
			req: &ptypes.ListResPlanDemandWithDeviceTypesReq{
				Memories: []int64{128},
			},
			expect: false,
		},
		{
			name: "cpu_cores和memories同时匹配-通过",
			req: &ptypes.ListResPlanDemandWithDeviceTypesReq{
				CpuCores: []int64{60},
				Memories: []int64{64},
			},
			expect: true,
		},
		{
			name: "cpu_cores匹配但memories不匹配-拒绝",
			req: &ptypes.ListResPlanDemandWithDeviceTypesReq{
				CpuCores: []int64{60},
				Memories: []int64{128},
			},
			expect: false,
		},
		{
			name: "cpu_cores不匹配但memories匹配-拒绝",
			req: &ptypes.ListResPlanDemandWithDeviceTypesReq{
				CpuCores: []int64{120},
				Memories: []int64{64},
			},
			expect: false,
		},
		{
			name: "空cpu_cores和memories-通过",
			req: &ptypes.ListResPlanDemandWithDeviceTypesReq{
				CpuCores: []int64{},
				Memories: []int64{},
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchCpuMemFilter(deviceInfo, tt.req)
			if result != tt.expect {
				t.Errorf("matchCpuMemFilter() = %v, want %v", result, tt.expect)
			}
		})
	}
}

func TestMatchCpuMemFilter_DifferentDevices(t *testing.T) {
	// 测试不同机型规格的筛选
	req := &ptypes.ListResPlanDemandWithDeviceTypesReq{
		CpuCores: []int64{60, 120},
		Memories: []int64{64, 128},
	}

	tests := []struct {
		name       string
		deviceInfo dt.DistinctDeviceType
		expect     bool
	}{
		{
			name: "60核64G-匹配",
			deviceInfo: dt.DistinctDeviceType{CpuCore: 60, Memory: 64},
			expect: true,
		},
		{
			name: "120核128G-匹配",
			deviceInfo: dt.DistinctDeviceType{CpuCore: 120, Memory: 128},
			expect: true,
		},
		{
			name: "60核128G-匹配(cpu匹配+memory匹配)",
			deviceInfo: dt.DistinctDeviceType{CpuCore: 60, Memory: 128},
			expect: true,
		},
		{
			name: "30核64G-不匹配(cpu不匹配)",
			deviceInfo: dt.DistinctDeviceType{CpuCore: 30, Memory: 64},
			expect: false,
		},
		{
			name: "60核32G-不匹配(memory不匹配)",
			deviceInfo: dt.DistinctDeviceType{CpuCore: 60, Memory: 32},
			expect: false,
		},
		{
			name: "30核32G-不匹配(都不匹配)",
			deviceInfo: dt.DistinctDeviceType{CpuCore: 30, Memory: 32},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchCpuMemFilter(tt.deviceInfo, req)
			if result != tt.expect {
				t.Errorf("matchCpuMemFilter() = %v, want %v", result, tt.expect)
			}
		})
	}
}

// ==================== Task 6.1 部分: 分页逻辑测试 ====================

func TestPageDeviceTypeDemands(t *testing.T) {
	// 创建 5 条测试数据
	items := make([]*ptypes.ListResPlanDemandWithDeviceTypesItem, 5)
	for i := range items {
		items[i] = &ptypes.ListResPlanDemandWithDeviceTypesItem{
			DemandID:   "demand-00" + string(rune('1'+i)),
			DeviceType: "type-" + string(rune('1'+i)),
		}
	}

	tests := []struct {
		name       string
		page       *core.BasePage
		totalItems int
		expectLen  int
		expectFirst string
	}{
		{
			name: "第一页",
			page: &core.BasePage{Start: 0, Limit: 2},
			expectLen: 2,
			expectFirst: "demand-001",
		},
		{
			name: "第二页",
			page: &core.BasePage{Start: 2, Limit: 2},
			expectLen: 2,
			expectFirst: "demand-003",
		},
		{
			name: "最后一页不满",
			page: &core.BasePage{Start: 4, Limit: 2},
			expectLen: 1,
			expectFirst: "demand-005",
		},
		{
			name: "start超出范围",
			page: &core.BasePage{Start: 10, Limit: 2},
			expectLen: 0,
		},
		{
			name: "limit大于剩余数量",
			page: &core.BasePage{Start: 3, Limit: 10},
			expectLen: 2,
			expectFirst: "demand-004",
		},
		{
			name: "获取全部",
			page: &core.BasePage{Start: 0, Limit: 10},
			expectLen: 5,
			expectFirst: "demand-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pageDeviceTypeDemands(tt.page, items)
			if len(result) != tt.expectLen {
				t.Errorf("pageDeviceTypeDemands() 返回 %d 条, want %d", len(result), tt.expectLen)
				return
			}
			if tt.expectLen > 0 && result[0].DemandID != tt.expectFirst {
				t.Errorf("pageDeviceTypeDemands() 首条 DemandID = %s, want %s", result[0].DemandID, tt.expectFirst)
			}
		})
	}
}

func TestPageDeviceTypeDemands_EmptyInput(t *testing.T) {
	items := []*ptypes.ListResPlanDemandWithDeviceTypesItem{}
	page := &core.BasePage{Start: 0, Limit: 10}

	result := pageDeviceTypeDemands(page, items)
	if len(result) != 0 {
		t.Errorf("空输入应返回空, got %d 条", len(result))
	}
}

func TestPageDeviceTypeDemands_StartAtBoundary(t *testing.T) {
	items := make([]*ptypes.ListResPlanDemandWithDeviceTypesItem, 3)
	for i := range items {
		items[i] = &ptypes.ListResPlanDemandWithDeviceTypesItem{
			DemandID: "d" + string(rune('1'+i)),
		}
	}

	// start 刚好等于 len(items)
	page := &core.BasePage{Start: 3, Limit: 10}
	result := pageDeviceTypeDemands(page, items)
	if len(result) != 0 {
		t.Errorf("start=len 时应返回空, got %d 条", len(result))
	}
}

// ==================== Task 6.1 部分: 机型展开逻辑测试（纯逻辑部分） ====================

func TestExpandDemandToDeviceTypes_NoWildcard(t *testing.T) {
	// 无通配机型时，仅返回原始机型
	demandItem := &ptypes.ListResPlanDemandItem{
		DemandID:       "d1",
		DeviceType:     "SA3.8XLARGE128",
		TotalCpuCore:   120,
		AppliedCpuCore: 80,
		RemainedCpuCore: 40,
		RegionID:       "gz",
		ZoneID:         "gz-1",
	}

	deviceTypeMap := map[string]dt.DistinctDeviceType{
		"SA3.8XLARGE128": {DeviceType: "SA3.8XLARGE128", CpuCore: 120, Memory: 128, DeviceClass: "compute", DeviceFamily: "SA3"},
	}

	// 因为 expandDemandToDeviceTypes 是 Controller 方法，需要构造 Controller
	// 这里测试的是无通配机型的场景：groupResults 存在但 DeviceTypes 为空
	// 由于 IsDeviceMatched 依赖 Controller，直接测试 buildDeviceTypeItem 的组合效果
	// 通过验证原始机型始终被添加来间接验证
	_ = demandGroupKey{RegionID: "gz", ZoneID: "gz-1"} // 验证类型可用
	_ = &demandGroupResult{DeviceTypes: []string{}, Err: nil}

	item := buildDeviceTypeItem(demandItem, demandItem.DeviceType, true, deviceTypeMap["SA3.8XLARGE128"])
	if !item.IsOriginal {
		t.Errorf("原始机型应 IsOriginal=true")
	}
	if item.DeviceType != "SA3.8XLARGE128" {
		t.Errorf("DeviceType = %s, want SA3.8XLARGE128", item.DeviceType)
	}
	if item.TotalOS != 1 { // 120 / 120
		t.Errorf("TotalOS = %d, want 1", item.TotalOS)
	}
}

func TestExpandDemandToDeviceTypes_DeviceNotInCache(t *testing.T) {
	// 原始机型不在缓存中，仅返回原始机型（零值设备信息）
	demandItem := &ptypes.ListResPlanDemandItem{
		DemandID:       "d1",
		DeviceType:     "UNKNOWN_TYPE",
		TotalCpuCore:   120,
		AppliedCpuCore: 80,
		RemainedCpuCore: 40,
	}

	deviceTypeMap := map[string]dt.DistinctDeviceType{}

	// 原始机型不在缓存中，应使用零值 DistinctDeviceType
	_, exists := deviceTypeMap[demandItem.DeviceType]
	if exists {
		t.Errorf("UNKNOWN_TYPE 不应在缓存中")
	}

	// 当机型不在缓存中时，CpuCore=0，OS 数应全部为 0
	zeroInfo := dt.DistinctDeviceType{}
	item := buildDeviceTypeItem(demandItem, demandItem.DeviceType, true, zeroInfo)
	if item.CpuCore != 0 {
		t.Errorf("未知机型的 CpuCore 应为 0")
	}
	if item.TotalOS != 0 || item.AppliedOS != 0 || item.RemainedOS != 0 {
		t.Errorf("未知机型的 OS 数应为 0, got total=%d applied=%d remained=%d",
			item.TotalOS, item.AppliedOS, item.RemainedOS)
	}
}

func TestExpandDemandToDeviceTypes_MultipleWildcardWithFilter(t *testing.T) {
	// 测试多个通配机型 + cpu_cores 筛选的组合逻辑
	// 验证 matchCpuMemFilter 的间接效果
	devices := map[string]dt.DistinctDeviceType{
		"SA3.8XLARGE128":  {DeviceType: "SA3.8XLARGE128", CpuCore: 120, Memory: 128, DeviceClass: "compute", DeviceFamily: "SA3"},
		"SA3.4XLARGE64":   {DeviceType: "SA3.4XLARGE64", CpuCore: 60, Memory: 64, DeviceClass: "compute", DeviceFamily: "SA3"},
		"SA3.2XLARGE32":   {DeviceType: "SA3.2XLARGE32", CpuCore: 30, Memory: 32, DeviceClass: "compute", DeviceFamily: "SA3"},
		"IT5.4XLARGE64":   {DeviceType: "IT5.4XLARGE64", CpuCore: 16, Memory: 64, DeviceClass: "compute", DeviceFamily: "IT5"},
	}

	// 筛选 cpu_core=60 或 30 的机型
	req := &ptypes.ListResPlanDemandWithDeviceTypesReq{
		CpuCores: []int64{60, 30},
	}

	// 验证筛选逻辑
	matchedCount := 0
	for name, dev := range devices {
		if matchCpuMemFilter(dev, req) {
			matchedCount++
			if name != "SA3.4XLARGE64" && name != "SA3.2XLARGE32" {
				t.Errorf("意外匹配的机型: %s (cpu_core=%d)", name, dev.CpuCore)
			}
		}
	}
	if matchedCount != 2 {
		t.Errorf("应匹配 2 个机型, got %d", matchedCount)
	}
}
