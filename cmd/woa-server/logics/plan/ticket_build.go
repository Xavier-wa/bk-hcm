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
	"fmt"
	"slices"

	ptypes "hcm/cmd/woa-server/types/plan"
	dt "hcm/pkg/api/core/cloud/device-type"
	"hcm/pkg/criteria/enumor"
	dmtypes "hcm/pkg/dal/dao/types/meta"
	rpt "hcm/pkg/dal/table/resource-plan/res-plan-ticket"
	tabletypes "hcm/pkg/dal/table/types"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	cvt "hcm/pkg/tools/converter"
	"hcm/pkg/tools/times"

	"github.com/shopspring/decimal"
)

// ResPlanTicketResourceSummary is aggregated resource summary of res plan ticket demands.
type ResPlanTicketResourceSummary struct {
	OriginalOS       float64
	OriginalCPUCore  int64
	OriginalMemory   int64
	OriginalDiskSize int64
	UpdatedOS        float64
	UpdatedCPUCore   int64
	UpdatedMemory    int64
	UpdatedDiskSize  int64
}

// BuildResPlanDemandsFromCreateReq converts API create demand requests to table demands with only Updated set.
func BuildResPlanDemandsFromCreateReq(demandClass enumor.DemandClass, reqDemands []ptypes.CreateResPlanDemandReq,
	zoneMap map[string]string, regionAreaMap map[string]dmtypes.RegionArea,
	deviceTypeMap map[string]dt.DistinctDeviceType) rpt.ResPlanDemands {

	demands := make(rpt.ResPlanDemands, len(reqDemands))
	for idx, demand := range reqDemands {
		demands[idx] = rpt.ResPlanDemand{
			DemandClass: demandClass,
			Updated: &rpt.UpdatedRPDemandItem{
				ObsProject:     demand.ObsProject,
				ExpectTime:     demand.ExpectTime,
				ReturnPlanTime: demand.ReturnPlanTime,
				ZoneID:         demand.ZoneID,
				ZoneName:       zoneMap[demand.ZoneID],
				RegionID:       demand.RegionID,
				RegionName:     regionAreaMap[demand.RegionID].RegionName,
				AreaName:       regionAreaMap[demand.RegionID].AreaName,
				DemandSource:   demand.DemandSource,
				Remark:         demand.Remark,
			},
		}

		if slices.Contains(demand.DemandResTypes, enumor.DemandResTypeCVM) {
			deviceType := demand.Cvm.DeviceType
			demands[idx].Updated.Cvm = rpt.Cvm{
				ResMode:        demand.Cvm.ResMode,
				DeviceType:     deviceType,
				DeviceClass:    deviceTypeMap[deviceType].DeviceClass,
				DeviceFamily:   deviceTypeMap[deviceType].DeviceFamily,
				TechnicalClass: deviceTypeMap[deviceType].TechnicalClass,
				CoreType:       string(deviceTypeMap[deviceType].CoreType),
				Os:             tabletypes.Decimal{Decimal: cvt.PtrToVal(demand.Cvm.Os)},
				CpuCore:        cvt.PtrToVal(demand.Cvm.CpuCore),
				Memory:         cvt.PtrToVal(demand.Cvm.Memory),
			}
		}

		if slices.Contains(demand.DemandResTypes, enumor.DemandResTypeCBS) {
			demands[idx].Updated.Cbs = rpt.Cbs{
				DiskType:     demand.Cbs.DiskType,
				DiskTypeName: demand.Cbs.DiskType.Name(),
				DiskIo:       cvt.PtrToVal(demand.Cbs.DiskIo),
				DiskSize:     cvt.PtrToVal(demand.Cbs.DiskSize),
			}
		}
	}

	return demands
}

// buildAndValidateDemandsFromCreateReq builds table demands from API create requests and validates them.
// NOTE: this function is only used for overwrite ticket.
func (c *Controller) buildAndValidateDemandsFromCreateReq(kt *kit.Kit, demandClass enumor.DemandClass,
	reqDemands []ptypes.CreateResPlanDemandReq) (rpt.ResPlanDemands, *ResPlanTicketResourceSummary, error) {

	demands, err := c.buildDemandsFromCreateReq(kt, demandClass, reqDemands)
	if err != nil {
		return nil, nil, err
	}

	summary, err := c.validateAndSummarizeDemands(kt, demands, false)
	if err != nil {
		return nil, nil, err
	}

	return demands, summary, nil
}

// buildDemandsFromCreateReq builds table demands from API create requests.
func (c *Controller) buildDemandsFromCreateReq(kt *kit.Kit, demandClass enumor.DemandClass,
	reqDemands []ptypes.CreateResPlanDemandReq) (rpt.ResPlanDemands, error) {

	zoneMap, regionAreaMap, deviceTypeMap, err := c.resFetcher.GetMetaMaps(kt)
	if err != nil {
		logs.Errorf("get meta maps failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	return BuildResPlanDemandsFromCreateReq(demandClass, reqDemands, zoneMap, regionAreaMap, deviceTypeMap), nil
}

// validateAndSummarizeDemands validates demands expect_time and returns aggregated resource summary.
// NOTE: this function is used for all ticket types.
func (c *Controller) validateAndSummarizeDemands(kt *kit.Kit, demands rpt.ResPlanDemands,
	checkReportDeadline bool) (*ResPlanTicketResourceSummary, error) {

	if err := c.validateDemandsExpectTimeCrossMonth(kt, demands); err != nil {
		return nil, err
	}
	// 非本年度预测提报截止时间仅在新提单时校验；overwrite 场景主单已提交，不再拦截
	if checkReportDeadline {
		if err := c.validateNonCurrentYearReportDeadline(kt, demands); err != nil {
			logs.Errorf("failed to validate non current year report deadline, err: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
	}

	return CalcResPlanTicketResourceSummary(demands), nil
}

// validateDemandsExpectTimeCrossMonth validates Updated expect times are not cross month.
func (c *Controller) validateDemandsExpectTimeCrossMonth(kt *kit.Kit, demands rpt.ResPlanDemands) error {
	for _, demand := range demands {
		if demand.Updated == nil {
			continue
		}
		// 期望交付时间的预测需求月和其自然月必须一致，否则需要选择该周的其他时间
		et, err := times.ParseDay(demand.Updated.ExpectTime)
		if err != nil {
			logs.Errorf("failed to parse expect time, err: %v, expect_time: %s, rid: %s", err,
				demand.Updated.ExpectTime, kt.Rid)
			return err
		}
		isCross, err := c.demandTime.IsDayCrossMonth(kt, et)
		if err != nil {
			logs.Errorf("failed to check if expect time is cross month, err: %v, expect_time: %s, rid: %s",
				err, et.String(), kt.Rid)
			return err
		}
		if isCross {
			return fmt.Errorf("expect_time should not be cross month, expect_time: %s", demand.Updated.ExpectTime)
		}
	}
	return nil
}

// CalcResPlanTicketResourceSummary aggregates resource fields from demands.
func CalcResPlanTicketResourceSummary(demands rpt.ResPlanDemands) *ResPlanTicketResourceSummary {
	var originalOs, updatedOs decimal.Decimal
	var originalCpuCore, originalMemory, originalDiskSize int64
	var updatedCpuCore, updatedMemory, updatedDiskSize int64
	for _, demand := range demands {
		if demand.Original != nil {
			originalOs = originalOs.Add(demand.Original.Cvm.Os.Decimal)
			originalCpuCore += demand.Original.Cvm.CpuCore
			originalMemory += demand.Original.Cvm.Memory
			originalDiskSize += demand.Original.Cbs.DiskSize
		}
		if demand.Updated != nil {
			updatedOs = updatedOs.Add(demand.Updated.Cvm.Os.Decimal)
			updatedCpuCore += demand.Updated.Cvm.CpuCore
			updatedMemory += demand.Updated.Cvm.Memory
			updatedDiskSize += demand.Updated.Cbs.DiskSize
		}
	}

	return &ResPlanTicketResourceSummary{
		OriginalOS:       originalOs.InexactFloat64(),
		OriginalCPUCore:  originalCpuCore,
		OriginalMemory:   originalMemory,
		OriginalDiskSize: originalDiskSize,
		UpdatedOS:        updatedOs.InexactFloat64(),
		UpdatedCPUCore:   updatedCpuCore,
		UpdatedMemory:    updatedMemory,
		UpdatedDiskSize:  updatedDiskSize,
	}
}
