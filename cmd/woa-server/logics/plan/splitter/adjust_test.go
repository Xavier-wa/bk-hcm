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

package splitter

import (
	"testing"
	"time"

	demandtime "hcm/cmd/woa-server/logics/plan/demand-time"
	"hcm/pkg/criteria/enumor"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	"hcm/pkg/kit"
	"hcm/pkg/tools/times"

	"github.com/stretchr/testify/assert"
)

type stubDemandTime struct{}

func (stubDemandTime) GetDemandYearMonthWeek(*kit.Kit, time.Time) (demandtime.DemandYearMonthWeek, error) {
	return demandtime.DemandYearMonthWeek{}, nil
}

func (stubDemandTime) GetDemandYearMonth(_ *kit.Kit, t time.Time) (int, time.Month, error) {
	return t.Year(), t.Month(), nil
}

func (stubDemandTime) GetDemandDateRangeInMonth(*kit.Kit, time.Time) (times.DateRange, error) {
	return times.DateRange{}, nil
}

func (stubDemandTime) GetDemandDateRangeByYearMonth(*kit.Kit, int, time.Month) (times.DateRange, error) {
	return times.DateRange{}, nil
}

func (stubDemandTime) GetDemandDateRangeInWeek(*kit.Kit, time.Time) times.DateRange {
	return times.DateRange{}
}

func (stubDemandTime) IsDayCrossMonth(*kit.Kit, time.Time) (bool, error) {
	return false, nil
}

func (stubDemandTime) GetDemandStatusByExpectTime(*kit.Kit, string) (enumor.DemandStatus, times.DateRange, error) {
	return "", times.DateRange{}, nil
}

func makeAdjustDemand(originalTime, updatedTime string, cpuCore int64, deviceType string) rpt.ResPlanDemand {
	return rpt.ResPlanDemand{
		Original: &rpt.OriginalRPDemandItem{
			DemandID:   "demand-1",
			ExpectTime: originalTime,
			ObsProject: enumor.ObsProjectNormal,
			RegionID:   "ap-shanghai",
			ZoneID:     "ap-shanghai-2",
			Cvm: rpt.Cvm{
				CpuCore:        cpuCore,
				DeviceType:     deviceType,
				TechnicalClass: "标准型",
			},
		},
		Updated: &rpt.UpdatedRPDemandItem{
			ExpectTime: updatedTime,
			ObsProject: enumor.ObsProjectNormal,
			RegionID:   "ap-shanghai",
			ZoneID:     "ap-shanghai-2",
			Cvm: rpt.Cvm{
				CpuCore:        cpuCore,
				DeviceType:     deviceType,
				TechnicalClass: "标准型",
			},
		},
	}
}

func newAdjustTestSplitter() *SubTicketSplitter {
	s := newTestSplitter()
	s.demandTime = stubDemandTime{}
	return s
}

func TestGetDemandsWithoutTransferOnlyDateChange(t *testing.T) {
	s := newAdjustTestSplitter()
	kt := splitterTestKit()

	demand := makeAdjustDemand("2026-07-07", "2026-08-07", 8, "SA2.LARGE8")
	remain := s.getDemandsWithoutTransfer(kt, rpt.ResPlanDemands{demand})

	assert.Empty(t, remain)
	assert.Len(t, s.adjSplitGroupDemands[enumor.RPTicketTypeDelay], 1)
}

func TestGetDemandsWithoutTransferDateAndQuantityChange(t *testing.T) {
	s := newAdjustTestSplitter()
	kt := splitterTestKit()

	demand := makeAdjustDemand("2026-07-07", "2026-08-07", 8, "SA2.LARGE8")
	demand.Updated.Cvm.CpuCore = 4

	remain := s.getDemandsWithoutTransfer(kt, rpt.ResPlanDemands{demand})

	assert.Len(t, remain, 1)
	assert.Empty(t, s.adjSplitGroupDemands[enumor.RPTicketTypeDelay])
}

func TestGetDemandsWithoutTransferDateAndRegionChange(t *testing.T) {
	s := newAdjustTestSplitter()
	kt := splitterTestKit()

	demand := makeAdjustDemand("2026-07-07", "2026-08-07", 8, "SA2.LARGE8")
	demand.Updated.RegionID = "ap-guangzhou"
	demand.Updated.ZoneID = "ap-guangzhou-3"

	remain := s.getDemandsWithoutTransfer(kt, rpt.ResPlanDemands{demand})

	assert.Empty(t, remain)
	assert.Len(t, s.adjSplitGroupDemands[enumor.RPTicketTypeDelay], 1)
}

func TestGetDemandsWithoutTransferDateAndDeviceTypeChange(t *testing.T) {
	s := newAdjustTestSplitter()
	kt := splitterTestKit()

	demand := makeAdjustDemand("2026-07-07", "2026-08-07", 8, "SA2.LARGE8")
	demand.Updated.Cvm.DeviceType = "SA5.LARGE8"

	remain := s.getDemandsWithoutTransfer(kt, rpt.ResPlanDemands{demand})

	assert.Empty(t, remain)
	assert.Len(t, s.adjSplitGroupDemands[enumor.RPTicketTypeDelay], 1)
}

func TestGetDemandsWithoutTransferPureAdd(t *testing.T) {
	s := newAdjustTestSplitter()
	kt := splitterTestKit()

	demand := rpt.ResPlanDemand{
		Updated: &rpt.UpdatedRPDemandItem{
			ExpectTime: "2026-07-07",
			ObsProject: enumor.ObsProjectNormal,
		},
	}

	remain := s.getDemandsWithoutTransfer(kt, rpt.ResPlanDemands{demand})

	assert.Len(t, remain, 1)
	assert.Empty(t, s.adjSplitGroupDemands[enumor.RPTicketTypeDelay])
}
